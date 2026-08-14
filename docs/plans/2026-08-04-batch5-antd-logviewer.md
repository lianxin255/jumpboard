# 第五批：antd 死代码清理 + LogViewer 轮询

> 状态：规划中
> 起因：antdv-next 被全量引入但 `<a-*>` 组件零使用（16 个声明全闲置），Root.vue 175 行主题配置无消费方；LogViewer 标题写"实时刷新"但无任何轮询。
> 性质：消除"名实不符"——antd 声称在用实际没用、LogViewer 声称实时实际没轮询。

## 目标
1. 清理 antd 死代码：main.ts 删 use(Antd)、Root.vue 简化为直接渲染 App（删 175 行主题配置）
2. LogViewer 加轮询：让"实时刷新"名副其实

## 非目标
- 不替换现有自绘组件为 antd 组件（那是另一个方向，本批只清理不引入）
- 不改 antdv-next 依赖本身（卸载依赖涉及 package-lock，风险另算）

## 现状核查（已确认）
- `components.d.ts` 自动声明 16 个 A* 组件（auto-import resolver 生成），模板零使用
- `main.ts:3` `import Antd from 'antdv-next'` + `.use(Antd)` —— 全量注册但无消费者
- `Root.vue` 175 行：ThemeProvider + DarkConfigProvider（createStyles + ConfigProvider + ~120 行 token）—— 无消费方
- LogViewer 标题"实时刷新"（:163），但只有手动"刷新"按钮，无 setInterval

## 实施步骤

### Step 1：清理 antd 死代码
- `main.ts`：删 `import Antd` + `.use(Antd)`
- `Root.vue`：简化为直接 `<App />`，删 ThemeProvider/DarkConfigProvider/createStyles/ConfigProvider/theme 全部
- 删 `components.d.ts`（auto-import resolver 会重新生成，但既然不用 antd 组件，这个文件可删；若 resolver 配置还在 vite，保留空文件也行——检查 vite.config）
- **验证**：typecheck + build + 确认应用能启动（无 ConfigProvider 注入报错）

### Step 2：LogViewer 加轮询
- LogViewer 组件内加 setInterval（复用现有 loadAppLogs），页面可见时轮询、不可见时暂停
- 轮询间隔：操作日志 10s（和 App.vue 引擎状态轮询一致）
- 用 visibilitychange 控制：tab 隐藏时 clearInterval，可见时恢复
- onUnmounted 清理 timer
- **验证**：typecheck + 确认无内存泄漏（timer 清理）

### Step 3：验证 + 文档

## 风险
- **Step 1 中风险**：删 ConfigProvider 后，若有代码依赖 antd 的 message API（全局挂载在 ConfigProvider）会报错。核查：grep message/notification/Modal.confirm 全局调用。
- **Step 2 低风险**：纯加轮询，不改现有 loadAppLogs 逻辑

## 进度
- [x] Step 1 antd 死代码清理（main.ts 删 use(Antd) + Root.vue 从 175 行简化到 8 行 + 删 components.d.ts）
  - **JS bundle 4555kB → 368kB（-4MB）**，antdv-next 全量打包是主要体积来源
- [x] Step 2 LogViewer 加轮询（10s 操作日志刷新 + visibilitychange 暂停/恢复 + onUnmounted 清理）
- [x] Step 3 验证：typecheck + 130 测试 + build（bundle 368kB）

## 决策记录
- **保留 vite.config 的 AntdvNextResolver**：resolver 只在有 `<a-*>` 时才打包，删配置改动面大无收益。
- **删 components.d.ts**：auto-import resolver 的缓存文件，无 `<a-*>` 时不重新生成，留着是误导。typecheck 确认无需存在。
- **轮询合并到一个 onMounted**：review 发现初版两个 onMounted 分散，合并后 visibilitychange 注册和 startPolling 在同处，可读性更好。

## review 订正
- **components.d.ts 残留 16 个 antd 声明**：auto-import resolver 的缓存，删除后不重新生成（无 `<a-*>`），bundle 不受影响。
- **LogViewer 两个 onMounted 分散**：合并为一个，visibilitychange 注册移入初始化。

## 验证
- typecheck ✓ / 130 测试 ✓ / build ✓（JS bundle 368kB，较第四批 4555kB 降 4MB）
