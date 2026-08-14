# 第四批：协议图标抽取 + 时间格式化统一

> 状态：规划中
> 起因：体检报告 + 精确统计发现协议图标 SVG 重复 3 处、时间格式化 3 套语义不一致。
> 范围：纯前端去重，风险低。

## 目标
1. 抽协议图标 SVG 到 `src/renderer/utils/icons.ts`（消除 TemplatePicker/MappingList/MappingForm 的重复）
2. 抽时间格式化到 `src/renderer/utils/time.ts`（统一 3 套不一致的语义）

## 非目标
- antd 死代码清理（改动面大，单独一批）
- 批量操作（需新 UI + store 改造，单独一批）
- 主机图标重复（App.vue + HostManager，量小，顺手做）

## 现状

### 协议图标
- `TemplatePicker.vue:31` 有完整的 `iconPaths` Record（10+ 协议 → SVG path）
- `MappingList.vue:200` 和 `MappingForm.vue` 各处内联了协议 SVG path（tcp/http 等重复）
- App.vue navItems 用 `v-html` 内嵌 SVG 字符串（:40-46）

### 时间格式化（3 套）
- `MappingList.vue` relTime：相对时间（"3 分钟前"）
- `HistoryPanel.vue:61` formatTime：dayjs 今天/昨天/日期
- `LogViewer.vue:120` relTime：dayjs + relativeTime 插件 + zh-cn locale

三套语义不一致，且 LogViewer 单独 import 了 relativeTime 插件。

## 实施步骤

### Step 1：抽协议图标
- 新建 `src/renderer/utils/icons.ts`，导出 `protocolIconPaths: Record<string, string>` 和 `getProtocolIcon(protocol): string`
- TemplatePicker 改为 import（删本地 iconPaths）
- MappingList/MappingForm 内联的协议 SVG 改用 getProtocolIcon
- **验证**：typecheck + 视觉一致

### Step 2：抽时间格式化
- 新建 `src/renderer/utils/time.ts`
- 统一两个函数：`relTime(ts)`（相对时间）+ `formatTime(ts)`（今天/昨天/日期）
- dayjs + relativeTime + zh-cn 在此文件统一初始化
- 3 个组件改为 import，删本地实现
- **验证**：typecheck + 三个组件的时间显示一致

### Step 3：验证 + 文档

## 风险
- 极低：纯函数抽取 + import 替换，不改逻辑
- 时间格式化统一后语义可能微变（如 MappingList 原本不用 dayjs，改后用 dayjs 的相对时间）—— 需确认输出格式用户可接受

## 进度
- [x] Step 1 协议图标抽取（TemplatePicker，实际只有 1 处用 iconPaths）
- [x] Step 2 时间格式化统一（time.ts + MappingList/HistoryPanel/LogViewer 三组件替换）
- [x] Step 3 验证：typecheck + 130 测试 + build

## 决策记录
- **Step 1 范围比预期小**：MappingList:200 的 SVG 是空状态图标（非协议），MappingForm 把 protocol 作 icon 值传但不自渲染。只有 TemplatePicker 有 iconPaths。
- **Step 2 LogViewer 保留 dayjs import**：LogViewer 的 absTime（HH:mm:ss）和 nginx error log 时间解析仍需 dayjs，只删了 relativeTime/locale 初始化（移到 time.ts）。其他两组件完全删 dayjs。
- **语义微变**：MappingList relTime 从手写"X 秒前"变成 dayjs fromNow 的"几秒前"（<44s 时）。用户无感，可接受。

## 验证
- typecheck ✓ / 130 测试全过 ✓ / build ✓
- grep 确认无残留本地 iconPaths/formatTime/relTime 定义 ✓
- grep 确认 dayjs 只在 utils/time.ts 一处 import ✓

## review 订正（已完成）
- **dayjs 插件初始化时机竞态**：time.ts 在模块顶层 extend 插件，靠组件 import 副作用触发，存在"先调 fromNow 后 extend"的脆弱依赖。
  - 修复：main.ts 显式 `import './utils/time'` 尽早初始化，不靠组件副作用。
- **LogViewer 保留 dayjs import 多余**：absTime 只用 dayjs.format，完全可以在 time.ts 导出 absTime，让 LogViewer 彻底去掉 dayjs import。
  - 修复：time.ts 加 absTime 导出，LogViewer 删 dayjs import + 本地 absTime。dayjs 现在只在 time.ts 一处 import。

## 教训
- "纯函数抽取无风险"的判断不严谨——dayjs 插件初始化是**有状态的副作用**，靠 import 顺序是脆弱设计。应在应用入口显式保证初始化时机。
- review 第一轮"无订正"的结论要警惕，第四批改 dayjs 时我没仔细想插件初始化时机，第二轮才抓到。
