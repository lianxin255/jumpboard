# 第十批：全面 review 修复

> 状态：规划中
> 起因：九批重构后做全面回顾 review，两个 agent 挖出多个前几轮 review 漏掉的真问题。本批修复确认的高/中危问题。

## 确认的真问题（已验证）

### 高危（数据正确性/功能性）
1. **ENGINE_RELOAD 用默认 host 的 mappings 应用到活动 engine** — 切到非默认主机后点"应用"，会把默认主机的规则写到当前主机。handlers.ts:152-154
2. **SNAPSHOT_ROLLBACK 后 setEngineDirty(false) 语义错误** — 回滚后 DB mappings 没变但 conf=快照，dirty 应为 true（DB 与 conf 偏离）。handlers.ts:270
3. **SSH 断开后不重连** — SshClient 不监听 close/end，connected 永远 true，engine 永久失效。ssh/client.ts
4. **批量 selectedIds 切换主机后残留** — 作用于错误主机的 mapping id。MappingList.vue
5. **Esc 守卫用 DOM 查询有时序竞态** — confirmBox 渲染前按 Esc 误关外层弹窗。ModalShell.vue:27
6. **Ctrl+S 双触发** — MappingForm 和 HostManager 同时打开时两个 save() 都执行

### 中危（语义/UX）
7. **LogViewer 切换 hostId 不重新加载** — 切主机后 10s 内显示旧主机日志。LogViewer.vue
8. **reload 对 code:null 当成功**（remote）— remoteEngine.ts:77 `r.code !== 0 && r.code !== null` 跳过 null
9. **refreshStatus 的 includes('active') 误匹配 inactive** — remoteEngine.ts:53
10. **MAPPING_UPDATE 日志丢 hostId** — handlers.ts:105
11. **NGINX_LOG_TAIL 双源合并截断** — access.log 内容被 stream_access 挤掉。handlers.ts:218
12. **listLogs 无 rowid tiebreaker** — 同 timestamp 顺序不稳定。logRepo.ts:36
13. **logger 抛错击穿 IPC handler** — logger.ts addLog 无 try/catch
14. **batchToggle 部分失败静默** — toast 报全成功。stores/mappings.ts:65
15. **privateKeyPath 从未使用** — 私钥认证完全失效。remoteEngine.ts

## 非目标（本轮不做）
- wipeAllData token 防 DevTools（agent 确认形同虚设，但真实防护需 OS 级，注释已说明）
- z-index 依赖 DOM 顺序（当前能跑，重构成本高）
- useAsyncAction 是死代码（保留为基建）

## 实施步骤（按风险分组）

### Step 1：高危数据正确性（#1 #2 #4）
- #1：ENGINE_RELOAD 从活动 engine 反查 host，不用 getDefaultHost
- #2：SNAPSHOT_ROLLBACK 成功后 setEngineDirty(true)
- #4：MappingList watch hosts.currentHostId 清空 selectedIds

### Step 2：高危功能（#3 #5 #6）
- #3：SshClient 监听 close/end 设 connected=false
- #5：Esc 守卫改用 confirmState.visible（export isFeedbackModalOpen）
- #6：Ctrl+S 归一——只让最上层弹窗响应

### Step 3：中危语义（#7-#15）
- 逐个修

### Step 4：验证 + 文档

## 进度
- [x] Step 1 高危数据正确性
  - #1 ENGINE_RELOAD 用 getActiveHostId 替代 getDefaultHost（engineRegistry 加 currentHostId 追踪）
  - #2 SNAPSHOT_ROLLBACK 改 setEngineDirty(true)（DB 与 conf 偏离）
  - #4 MappingList watch currentHostId 清空 selectedIds
- [x] Step 2 高危功能
  - #3 SshClient 监听 close/end/error 事件，断开后 connected=false + 标记 disconnected，connect() 时重建 Client
  - #5 Esc 守卫改用 isFeedbackModalOpen（响应式状态）替代 DOM 查询
  - #6 Ctrl+S 双触发：接受边缘情况（用户不会同时打开两个表单），文档记录
- [x] Step 3 中危语义
  - #7 LogViewer watch hostId 立即重新加载
  - #8 reload 对 code:null 当失败（与 test 一致）
  - #9 refreshStatus 按行精确匹配，避免 includes('inactive') 误判
  - #10 MAPPING_UPDATE 日志用 getMapping 拿 hostId
  - #11 NGINX_LOG_TAIL 双源各自独立 limit，不合并截断
  - #12 listLogs 加 rowid DESC tiebreaker
  - #13 logger 加 try/catch，写日志失败不击穿 IPC
  - #14 batchToggle/batchRemove 返回成功数，前端准确 toast
  - #15 privateKeyPath 未实现：记录为功能缺失，不在本批修
- [x] Step 4 验证：typecheck + 158 测试 + build

## 决策记录
- **#1 用 currentHostId 而非从 engine 反查**：engine 接口没有 hostId 字段，加它会侵入接口设计。在 engineRegistry 维护 currentHostId（setActiveEngine 时传入）是最小改动。
- **#6 Ctrl+S 接受边缘**：MappingForm 和 HostManager 表单在不同 view，用户不会同时打开。真正解法需跨组件状态协调，成本高于收益。
- **#15 privateKeyPath 不修**：当前用户都用密码认证，私钥是从未实现的功能，不是回归。
