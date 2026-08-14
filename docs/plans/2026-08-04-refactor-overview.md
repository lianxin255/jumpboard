# nginx-jump-gui 重构总览

> 本文档是项目重构工作的索引。每批改动有独立计划文档，完成后更新此处状态。
> 工作方式：**先规划文档落地 → 按步骤执行 → 每步更新文档 → 每批结束 code review（用挑剔眼光找问题）**

## 体检报告来源
基于两个探索 agent（前端交互层 + 主进程架构/安全面）+ 人工复核，覆盖 50 个源文件（约 6000 行）。
原始发现分 7 类：安全面、真 Bug、架构重构、前端交互、IPC 调用、重复代码、性能。

## 重构批次

### 第一批：安全漏洞 + Bug + 死代码 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch1-security-bugfix.md`
- 内容：A1 模板路径穿越 / A4 白名单 fail-closed 统一 / B1 测试连接按钮 / B3 死配置清理
- review 订正 3 处（A4 remote 绕过、返回类型重载、B1 disabled）
- 验证：111 测试全过

### 第二批：engine 接口 + conf 流水线 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch2-engine-conf-pipeline.md`
- 内容：engine 接口显式化（engineKind/supportsProcessControl）+ remote start/stop 不再假动作 + safeApplyConf 收敛三条流水线 + confChain 互斥锁 + 前端按 supportsProcessControl 显隐启停按钮
- review 订正 1 处（failStage 区分日志级别，避免 reload 失败在 error 过滤下消失）
- 验证：119 测试全过（新增 safeApply 7 条）

### 第三批：前端去重 + saveAsTemplate 字段保真 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch3-frontend-dedup.md`
- 内容：useAsyncAction composable（基建）+ ToggleSwitch 组件（消除 4 份重复）+ ModalShell 组件（消除 3 份重复，CSS -5kB）+ saveAsTemplate 字段保真（修 upstreamHost 硬编码 '' bug + 补 8 个字段）+ templateMapping 工具（强类型约束）
- review 无订正（改动经挑剔复核，ToggleSwitch 事件模式/ModalShell 结构/saveAsTemplate name 覆盖均正确）
- 验证：130 测试全过（新增 useAsyncAction 6 + templateMapping 5）

### 第四批：协议图标 + 时间格式化抽取 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch4-icons-time.md`
- 内容：协议图标抽到 icons.ts（TemplatePicker）+ 时间格式化抽到 time.ts（统一 3 套不一致语义：MappingList/HistoryPanel/LogViewer）
- review 无订正（纯函数抽取，grep 确认无残留）
- 验证：130 测试全过

### 第五批：antd 死代码清理 + LogViewer 轮询 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch5-antd-logviewer.md`
- 内容：清理 antd 全量引入（main.ts use(Antd) + Root.vue 175 行主题配置 + components.d.ts）+ LogViewer 加 10s 轮询（visibilitychange 暂停/恢复）
- **JS bundle 4555kB → 368kB（-4MB）**，antdv-next 全量打包是主要体积来源
- review 订正：删 components.d.ts 缓存 + 合并 LogViewer 两个 onMounted
- 验证：130 测试全过

### 第六批：前端交互增强 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch6-interactions.md`
- 内容：Esc 关弹窗（FeedbackHost + ModalShell）+ Ctrl+S 保存（MappingForm + HostManager）+ 克隆映射（复用 templateData 机制）+ 批量操作（多选 + 批量启停/删除）
- review 订正：Esc 冲突（ModalShell 加 .fb-overlay 守卫）+ Set mutate 不响应（clear() 改替换）
- 验证：130 测试全过

### 第十批：全面 review 修复 ✅ 已完成
- 计划：`docs/plans/2026-08-04-batch10-comprehensive-review-fixes.md`
- 内容：九批重构后做全面回顾 review，两个 agent 挖出 15 个前几轮漏掉的问题。修复 14 个（#15 privateKeyPath 记录为功能缺失不修）
- 高危修复：ENGINE_RELOAD 用活动 host（修数据错配）+ ROLLBACK dirty 语义 + SSH 断开重连 + Esc 时序竞态 + selectedIds 残留
- 中危修复：LogViewer hostId watch + reload code:null + refreshStatus 精确匹配 + MAPPING_UPDATE hostId + NGINX_LOG_TAIL 双源 + listLogs tiebreaker + logger 防击穿 + batchToggle 准确计数
- 验证：158 测试全过

---

## 全部十批完成 ✅

| 批次 | 内容 | 测试 |
|---|---|---|
| 一 | 安全漏洞 + Bug 修复 | 111 |
| 二 | engine 接口 + safeApplyConf + 并发锁 | 119 |
| 三 | Toggle/Modal 去重 + saveAsTemplate 修 bug | 130 |
| 四 | 图标 + 时间格式化抽取 | 130 |
| 五 | antd 清理 + LogViewer 轮询 | 130 |
| 六 | Esc/Ctrl+S/克隆/批量操作 | 130 |
| 七 | IPC 校验 + validation.ts | 149 |
| 八 | tailFile 防 OOM + wipe token + 清理 | 156 |
| 九 | engine 注入可测性 + 日志裁剪优化 | 157 |
| 十 | 全面 review 修复（15 问题） | 158 |

从 109 测试到 158（+49），CSS -5kB，JS -4181kB，修 6 个真 bug（含 3 个数据正确性），补齐安全面 + 性能 + 可测性。
> 每批开始前先写独立计划文档，从下方候选池选。

**架构 / 安全**
- ~~SSH exec/sftp 超时控制~~ ❌ 用户不做
- ~~IPC 输入校验~~ ✅ 第七批完成
- ~~preload API 面收窄~~ ✅ 第八批完成
- ~~IPC 统一错误包装 wrapIpc~~ ❌ 决策不做
- ~~engine 接口可测性~~ ✅ 第九批完成（RemoteNginxEngine 注入 SshClient）

**前端组件抽取**
- ~~公共 Toggle 组件（4 份重复）+ Modal 骨架（4 份重复）~~ ✅ 第三批完成
- ~~协议图标 SVG 抽 icons.ts + 时间格式化统一（3 套语义不一致）~~ ✅ 第四批完成
- ~~useIpcTask 组合式~~ → useAsyncAction 已建（第三批），但现有带业务分支的函数未替换（降级为基建）

**前端交互增强**
- ~~saveAsTemplate 字段保真~~ ✅ 第三批完成
- ~~antd 死代码清理~~ ✅ 第五批完成（bundle -4MB）
- ~~LogViewer 轮询~~ ✅ 第五批完成
- ~~快捷键（Esc 关弹窗 / Ctrl+S 保存）~~ ✅ 第六批完成
- ~~批量操作（映射启停/删除）~~ ✅ 第六批完成
- ~~克隆映射~~ ✅ 第六批完成
- 排序（sortOrder 字段存在但 UI 无入口）—— 中收益

**性能**
- ~~NGINX_LOG_TAIL 改尾部倒读~~ ✅ 第八批完成
- ~~日志裁剪优化~~ ✅ 第九批完成（trimLogs 改 rowid）

**清理**
- ~~版本号统一~~ ✅ 第八批完成
- ~~toFwdSlash 合并~~ ✅ 第八批完成

## review 原则（每批都要执行）
1. **用挑剔眼光**：上一轮自我 review 偏肯定，第二轮总能抓到问题
2. **typecheck + test 在所有改动闭环后重跑**：不能只看单点改动
3. **"统一/全覆盖"类断言要验证**：初版常只改一个分支就宣称统一
4. **行为变化要标注**：日志级别、返回类型、错误语义的变化都会级联，文档记录
