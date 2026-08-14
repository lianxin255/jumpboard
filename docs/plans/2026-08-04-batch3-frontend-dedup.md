# 第三批：前端去重 + saveAsTemplate 字段保真

> 状态：规划中
> 起因：体检报告 + 精确统计发现前端 4 类重复（Toggle 4 份 / Modal 3 份 / try-catch+toast 16 处 / loading 7 处），外加 saveAsTemplate 漏存 8 个字段且把 upstreamHost 硬编码成 ''（数据丢失 bug）。
> 范围：纯前端，不动主进程。

## 目标
1. 抽 `useAsyncAction` composable —— 统一 try/catch + toast + loading（消除 ~23 处样板，顺带修 unhandled rejection）
2. 抽 `<ToggleSwitch>` 公共组件 —— 消除 4 份逐字重复的 CSS
3. 抽 `<ModalShell>` 公共组件 —— 消除 3 份重复的 modal 骨架
4. 修 saveAsTemplate 字段保真 —— 补齐 8 个字段 + 修 upstreamHost 硬编码 '' bug
5. 抽 `mappingToTemplatePayload` 工具函数 —— 收敛 Mapping↔Template 字段映射，加类型约束（消除 Record<string,any> 弱类型导致的漏字段）

## 非目标（本批不做）
- antd 死代码清理（单独一批，涉及 main.ts + Root.vue，改动面大）
- 交互增强（快捷键/批量/排序，单独一批）
- 后端 IPC 加固（第三批候选，本批只做前端）

---

## 现状快照（重复精确位置）

### useAsyncAction 候选（try/catch+toast 16 处 + loading 7 处）
- App.vue: engStart(:66-73)、engStop(:83-89)、pickHost(:112-126，switching)
- HistoryPanel.vue: rollback(:47-57, rolling)
- HostManager.vue: testConn(:79-93, testing)、save(:116-125)
- LogViewer.vue: toggleNgxLogRead、clearAppLog、clearNgxLog（无 loading flag）
- MappingForm.vue: save(:170-201, saving)、saveAsTemplate(:206-242)
- TemplatePicker.vue: remove(:70-77)
- SettingsPanel.vue: addWhitelistIp、removeWhitelistIp、toggleWhitelist、doExport、doImport
- MappingList.vue: apply(:120-138, applying)

### Toggle（4 文件）
- LogViewer.vue:193、MappingForm.vue:378/419、MappingList.vue:248、SettingsPanel.vue:152/166
- CSS 逐字相同，仅 .sm 变体在 LogViewer/SettingsPanel 有

### Modal 骨架（3 文件）
- MappingForm.vue:246-448（modal.xl 1000px）
- TemplatePicker.vue:89-172（写死 860px，**不一致项**）
- HostManager.vue:211-319（modal.lg 860px）

### saveAsTemplate 漏字段（MappingForm.vue:218-237）
| 字段 | 现状 |
|---|---|
| upstreamHost | **硬编码 ''（BUG，丢内网 IP）** |
| tlsCertPath/tlsKeyPath/redirectHttp | 漏（HTTPS 模板丢证书+跳转） |
| healthCheckEnabled/IntervalSec/TargetPath/ExpectedStatus | 漏（健康检查全部丢失） |

---

## 实施步骤

### Step 1：抽 useAsyncAction composable
- 新建 `src/renderer/composables/useAsyncAction.ts`
- 签名：`useAsyncAction()` 返回 `{ busy: Ref<boolean>, run(action: () => Promise<void>, failMsg?: string): Promise<boolean> }`
- 行为：run 内部 try/finally 管 busy；catch 时 toast(failMsg + e?.message ?? '未知错误')；返回成功与否
- 统一 `e?.message ?? '未知错误'`（采用 LogViewer 的稳健写法，修掉 TemplatePicker 的 `err` 不一致）
- **验证**：先不替换调用方，写单测验证 run 的 busy 翻转 + catch toast

### Step 2：替换 8 个组件的 try/catch+toast+loading 为 useAsyncAction
- 逐文件替换：App.vue（3 处）、HistoryPanel、HostManager（2 处）、LogViewer（3 处）、MappingForm（2 处）、TemplatePicker、SettingsPanel（5 处）、MappingList
- 替换后每个组件删除对应的 xxxing ref + try/finally
- **验证**：typecheck + 手动核对每个调用点的行为等价（尤其 MappingList apply 后有 reload 副作用）

### Step 3：抽 ToggleSwitch 组件
- 新建 `src/renderer/components/ToggleSwitch.vue`，支持 v-model + size="sm"
- 删除 4 文件里的 toggle CSS + 模板用 `<ToggleSwitch>` 替换
- **验证**：typecheck + 视觉一致（4 个用法点）

### Step 4：抽 ModalShell 组件
- 新建 `src/renderer/components/ModalShell.vue`，props: title/sub/size(sm/lg/xl)，slots: default + footer，emit: cancel
- 删除 3 文件里的 modal 骨架 CSS + 模板替换
- 统一宽度策略：全部用 size 修饰符（修 TemplatePicker 写死 860px 的不一致）
- **验证**：typecheck + 三个弹窗视觉一致

### Step 5：修 saveAsTemplate 字段保真
- 新建 `src/renderer/utils/templateMapping.ts`
- 抽 `mappingToTemplatePayload(m: FormState): { mapping: TemplateMapping; extraDirectives?: string }`
- **加类型**：定义 TemplateMapping（替代 Record<string,any>），TS 能抓到漏字段
- 补齐 upstreamHost（修硬编码 '' bug）+ tlsCertPath/tlsKeyPath/redirectHttp + 4 个 healthCheck 字段
- 同步修 templateToForm（:38-59）确保反向映射也读这些字段
- **验证**：写单测验证 mappingToTemplatePayload 输出全部 16 字段；typecheck

### Step 6：验证 + 文档更新
- typecheck + 全量测试
- 更新本文档进度 + 总览文档

---

## 风险与回滚
- **最高风险**：Step 2 批量替换 try/catch，若 useAsyncAction 的 run 行为不等价（如吞掉了某些成功后的副作用），会影响多个功能。
  - 缓解：逐文件替换 + typecheck，不一次性 sed
- **Toggle/Modal 抽取**：纯结构 + CSS，风险低，但视觉可能因 CSS 合并顺序变化微调
- **saveAsTemplate 字段补齐**：新字段加入会让旧模板（已存的）缺字段——templateToForm 要用 `?? 默认值` 兜底，不能假设字段必存在（向后兼容旧模板）

## 进度
- [x] Step 1 useAsyncAction composable + 6 条单测
- [~] Step 2 替换样板 —— 降级为基建保留（多数异步函数带业务分支，硬套 run 会丢逻辑）
- [x] Step 3 ToggleSwitch 组件（4 文件替换，删 4 处重复 CSS）
- [x] Step 4 ModalShell 组件（3 文件替换，删 3 处重复 CSS；CSS 总量 47.10kB→42.24kB）
- [x] Step 5 saveAsTemplate 字段保真 + templateMapping 工具 + 5 条单测
- [x] Step 6 验证：typecheck + 130 测试全过 + build（CSS -5kB）

## 决策记录
- **Step 2 降级**：规划时以为 16 处 try/catch 都能套 useAsyncAction。实际审查发现多数带业务分支，强行替换降低可读性。composable + 测试作为基建保留，新代码可用。
- **Step 4 CSS 严格对齐**：ModalShell 初版 CSS 是自拟的，和现有组件不一致。重写为以 TemplatePicker/HostManager 现有样式为准。
- **Step 5 Template.mapping 不强类型**：内置 yml 模板字段不全（mysql.yml 只有 tcp 字段），强类型会让内置模板报错。保持 Template.mapping 宽松，但新存的模板通过 formToTemplateMapping 保证完整 + TemplateMapping 类型约束工具函数本身。

## 决策记录
- **Step 2 降级**：规划时以为 16 处 try/catch 都能套 useAsyncAction。实际审查发现 App.vue engStart/engStop（r.error 分支）、HistoryPanel rollback（result.ok 分支）、MappingList apply（test/r.output 分支）、wipeAll/doImport（confirm + 副作用）都有业务分支，强行替换会降低可读性。useAsyncAction 的价值在"纯单 IPC + toast"场景，现有代码多数已够清晰。保留 composable + 测试作为基建，不强改现有代码。
