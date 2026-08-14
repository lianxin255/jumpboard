# 第二批：engine 接口显式化 + conf 流水线收敛加锁

> 状态：已完成（含 review failStage 订正）
> 起因：第一批 review 发现 engine 抽象用 `as any` 鸭子类型探测、remote start/stop 是假动作误导 UI、三条 conf 流水线重复且行为不一致、conf 写入无并发锁。

## 目标
1. engine 能力差异用**显式接口字段**表达（`engineKind` / `supportsProcessControl`），消除 `as any` + `typeof === 'function'` 鸭子探测
2. remote 的 start/stop **不再假动作**：不支持就明示，UI 据此隐藏按钮
3. 三条重复的 conf 流水线（applyFlow / SNAPSHOT_ROLLBACK / ENGINE_RELOAD）**收敛成一个 `safeApplyConf` 函数**
4. conf 写入流程**加互斥锁**，防 RELOAD/ROLLBACK/HOST_SWITCH 并发竞争写 conf

## 非目标（本批不动）
- SSH exec 超时控制（单独一批，涉及 ssh/client.ts 全量改造）
- IPC 统一错误包装 wrapIpc（单独一批，涉及全部 35 个 handler）
- preload API 面收窄（安全面单独一批）

---

## 现状快照（改动前的问题点）

| 文件:行 | 问题 |
|---|---|
| `types.ts:5-19` | NginxEngine 接口无 engineKind/supportsProcessControl；refreshStatus 不在接口里；close?/getPid? 可选导致调用方鸭子探测 |
| `engineRegistry.ts:37-39` | `as any` + `typeof anyEng.refreshStatus === 'function'` 探测 refreshStatus |
| `handlers.ts:146-147` | `getActiveEngine() as any` + `typeof eng.getPid !== 'function'` 探测 getPid |
| `hostEngineSwitch.ts:67-69` | `instanceof RemoteNginxEngine` 判断后才调 refreshStatus |
| `remoteEngine.ts:53-59` | start/stop 只 refreshStatus（假动作），UI 显示"已启动"误导 |
| `handlers.ts:91-115` | ENGINE_START/STOP 对 remote 引擎也走真启停逻辑，调用假动作后记"nginx 已开始运行"日志 |
| `applyFlow.ts:34-47` | readConf→writeConf→test→reload 流水线，reload 失败**不恢复旧 conf** |
| `handlers.ts:221-243` | SNAPSHOT_ROLLBACK 重新实现一遍 readConf→writeConf→test→reload，test 失败时恢复 |
| `handlers.ts:117-138` | ENGINE_RELOAD 调 applyFlow，但 applyFlow 和 rollback 行为不一致 |
| 无锁 | RELOAD/ROLLBACK/HOST_SWITCH 并发触发时，read→write 非原子，nginx -t 可能校验半写文件 |

---

## 实施步骤（按依赖顺序，每步独立可验证）

### Step 1：接口显式化 ✅ 已完成
- `types.ts`：NginxEngine 加 `readonly engineKind: 'local'|'remote'`、`readonly supportsProcessControl: boolean`、`refreshStatus(): Promise<EngineStatus>`（从可选变正式）
- `localEngine.ts`：实现 `engineKind='local'`、`supportsProcessControl=true`、`refreshStatus()`（返回当前 status，no-op）
- `remoteEngine.ts`：实现 `engineKind='remote'`、`supportsProcessControl=false`
- **验证**：typecheck 通过（local/remote 都满足新接口）

### Step 2：remote start/stop 语义修正 ✅ 已完成
- `remoteEngine.ts`：start/stop 改为抛 `Error('远程主机不支持在此应用内启停 nginx')`，不再假动作
- **验证**：remote 测试需更新（原有测试若 mock 了 start/stop 的旧行为要改）

### Step 3：消除鸭子探测 ✅ 已完成
- `engineRegistry.ts:37-39`：refreshStatus 现在是正式接口方法，直接 `await eng.refreshStatus()`
- `handlers.ts:146`：getPid 用 `if (!eng.getPid)` 判断（接口仍是可选方法，但不再 as any）
- `hostEngineSwitch.ts:67`：refreshStatus 直接调，去掉 `instanceof RemoteNginxEngine`
- **验证**：grep 确认 src/main/nginx + handlers 无 `as any` 残留 ✅

### Step 4：ENGINE_START/STOP handler 适配 supportsProcessControl ⬜ 待做
- `handlers.ts:91-115`：remote 引擎（`supportsProcessControl === false`）调 start/stop 时返回明确错误，不记"已启动"误导日志
- **行为**：remote 场景 ENGINE_START 返回 `{status: eng.status(), error: '远程主机不支持在此应用内启停...'}`，前端据此提示
- **不改动**：UI 隐藏按钮放到 Step 8（前端配合），本步只让后端语义正确

### Step 5：抽取 safeApplyConf 函数 ⬜ 待做（核心）
- 新建 `src/main/nginx/safeApply.ts`
- 签名：`safeApplyConf(eng, newConf, { onTestFail?: (...) => void }): Promise<{ok, output}>`
- 逻辑（统一 applyFlow + rollback 的最佳行为）：
  1. readConf 备份旧 conf
  2. writeConf(newConf)
  3. test()
  4. test 失败 → writeConf(oldConf) 回滚 + 返回 {ok:false}
  5. test 通过 → reload()
  6. reload 失败 → **也回滚 writeConf(oldConf)**（修复 applyFlow 不回滚的 bug）+ 返回 {ok:false}
  7. 成功 → 返回 {ok:true}
- **关键决策**：reload 失败回滚 conf 是对的——nginx reload 失败时旧 worker 仍在跑旧 conf，但磁盘 conf 已是新（坏的）版本，下次重启会用坏 conf。回滚磁盘 conf 保证重启安全。
- **验证**：新增单测覆盖 6 条路径（test 通过/reload 成功、test 失败回滚、reload 失败回滚、read/write 抛错）

### Step 6：applyFlow / rollback 改用 safeApplyConf + 加锁 ⬜ 待做
- `safeApply.ts` 加模块级互斥锁：`let chain: Promise<any> = Promise.resolve()`，safeApplyConf 用 `.then` 串行化
- `applyFlow.ts:34-47`：read/write/test/reload 段替换为 `await safeApplyConf(eng, conf)`
- `handlers.ts:221-243`（SNAPSHOT_ROLLBACK）：替换为 `await safeApplyConf(eng, snap.content)`
- `hostEngineSwitch.ts` 的 local writeConf（初始化 conf，非 apply 流程）也走同一把锁，避免和 apply 并发
- **验证**：applyFlow 测试全过（行为等价 + reload 失败新增回滚）；confGenerator 测试不受影响

### Step 7：验证 ⬜ 待做
- typecheck
- 全量测试（预期 applyFlow 测试可能要小调，因为 reload 失败行为变了——加了回滚）
- 生成 conf 三模式核对不变

### Step 8：前端配合（可选，本批做后端为主）⬜ 待做
- `App.vue`：engineKind===remote 时隐藏"启动/停止"按钮（或显示但 disabled + tooltip）
- preload 暴露 engineKind（需新增 IPC 或在 status 返回里带）

---

## 风险与回滚
- **最高风险**：Step 5/6 改 conf 流水线，若 safeApplyConf 逻辑写错会影响所有 reload/rollback。
  - 缓解：先写单测（6 条路径全过）再替换调用方；typecheck + 全量测试把关
- **reload 失败回滚**是新行为（Step 5 决策），原 applyFlow 不回滚。这是修 bug 但也是行为变化——若用户依赖"reload 失败但 conf 已更新"的副作用会受影响。评估：这种依赖本身是错的，回滚更安全。
- remote start/stop 抛错（Step 2）会破坏"remote 引擎点启动不报错"的现状——但这正是要修的误导行为。

## 进度
- [x] Step 1-3 接口显式化 + 消除鸭子探测（types/localEngine/remoteEngine/engineRegistry/handlers/hostEngineSwitch）
- [x] Step 4 ENGINE_START/STOP 适配 supportsProcessControl（remote 返回明确错误，不记误导日志）
- [x] Step 5 safeApplyConf 抽取 + 7 条单测（6 功能路径 + 1 并发串行化）
- [x] Step 6 applyFlow/rollback 替换为 safeApplyConf + confChain 互斥锁
- [x] Step 7 验证：typecheck + 119 测试全过
- [x] Step 8 前端配合：新增 ENGINE_KIND_GET IPC，App.vue 顶栏启停按钮按 supportsProcessControl 显隐

## review 订正（已完成）
- **failStage**：applyFlow 改用 safeApplyConf 后，初版统一记 warn。review 发现这会让 reload/io 失败在 LogViewer"只看 error"过滤下消失（原代码 reload 失败记 error）。
  - 修复：safeApplyConf 返回值加 `failStage: 'test'|'reload'|'io'`，applyFlow 据此定级别（test→warn，reload/io→error）。
  - 文件：`src/main/nginx/safeApply.ts`（接口 + 三处 return 加 failStage）、`src/main/nginx/applyFlow.ts`（按 failStage 分支记日志）
- **SNAPSHOT_ROLLBACK 不写日志**：回滚是破坏性操作，但成功/失败都不记日志，失败时运维无从排查。
  - 修复：补 success/fail 日志，与 applyFlow 一致。失败时记 error + failStage。
  - 文件：`src/main/ipc/handlers.ts`

## 待续（下一批候选，不在本批范围）
- SSH exec/sftp 超时控制（`ssh/client.ts` 全量改造）
- IPC 统一错误包装 wrapIpc（35 个 handler）
- preload API 面收窄（破坏性操作二次确认）
- 抽公共组件 Toggle/Modal（消除 4 份重复）
- 交互增强：快捷键、批量操作、克隆映射、排序

## 验证
- typecheck ✓ / 119 测试全过（新增 safeApply 7 条 + remoteEngine 接口断言 1 条）✓ / build ✓
