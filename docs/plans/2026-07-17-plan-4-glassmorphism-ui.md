# Plan 4: 毛玻璃 iOS 风格 UI 设计系统

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前功能完备但样式朴素的界面，重构为设计文档 §8 描述的 glassmorphism + iOS 风格。建立统一设计系统（CSS 变量 + 全局工具类），所有组件切换到使用变量，补一个功能遗漏（MappingForm 暴露健康检查开关）。

**Architecture:** 新增 `src/renderer/styles/` 目录：`tokens.css`（CSS 自定义属性 = 设计 token）、`glass.css`（毛玻璃工具类）、`base.css`（reset + 全局基础样式 + 滚动条）。在 `main.ts` 全局引入。每个组件删掉硬编码颜色/间距，改用 `var(--xxx)` 和工具类 `class="glass-card"` 等。**不改任何业务逻辑**（IPC/store/handler 一律不动），只改 template 的 class 和 style 块。

**Tech Stack:** 纯 CSS（CSS Custom Properties + backdrop-filter）。无新 JS 依赖。深浅色双主题用 `prefers-color-scheme` + `:root[data-theme]` 切换。

**Spec 引用:** `docs/specs/2026-07-17-nginx-jump-gui-design.md` §8（UI 设计）。毛玻璃 = 半透明 + backdrop-filter blur + 细边框 + 柔和阴影；iOS = 圆角 12-16px、SF 字体栈、系统蓝 `#0A84FF`、分组列表、大量留白。

**前置条件:** Plan 1/2/3 完成（commit `6cc73f8` 之后的某 commit）。当前 6 组件 + App.vue 共 576 行，各自 scoped style，颜色硬编码。无全局样式。

---

## 文件结构

```
nginx-jump-gui/src/renderer/
├── styles/                  # 新：设计系统
│   ├── tokens.css           # CSS 变量（颜色/间距/圆角/阴影/字体）
│   ├── glass.css            # 毛玻璃工具类（.glass-card/.glass-panel/.glass-input）
│   └── base.css             # reset + body + 滚动条 + 字体
├── main.ts                  # 改：import 三个样式文件
└── components/
    ├── App.vue              # 改：背景渐变光斑 + topbar 用变量
    ├── MappingList.vue      # 改：卡片用 .glass-card
    ├── MappingForm.vue      # 改：表单卡片 + 控件用 .glass-input；补健康检查开关
    ├── TemplatePicker.vue   # 改：模板卡片 + 分组用变量
    ├── EngineControl.vue    # 改：胶囊容器 + 状态点
    ├── LogViewer.vue        # 改：日志行配色用变量
    └── HistoryPanel.vue     # 改：分栏容器用变量
```

无新增测试（纯视觉重构，逻辑不变，现有 82 tests 自动覆盖）。无新增依赖。

---

## Task 1: 设计 token + 全局样式 + 引入

**Files:**
- Create: `src/renderer/styles/tokens.css`, `src/renderer/styles/glass.css`, `src/renderer/styles/base.css`
- Modify: `src/renderer/main.ts`（import 三个 css）

- [ ] **Step 1: 创建 `src/renderer/styles/tokens.css`**

```css
/* 设计 token：颜色/间距/圆角/阴影/字体。深浅色双主题。 */

:root {
  /* 品牌色（iOS 系统蓝） */
  --color-accent: #0A84FF;
  --color-accent-hover: #0071E3;
  --color-accent-soft: rgba(10, 132, 255, 0.12);

  /* 语义色 */
  --color-success: #34c759;
  --color-warning: #ff9500;
  --color-danger: #ff3b30;

  /* 中性色（浅色主题） */
  --bg-base: #f5f5f7;
  --bg-elevated: rgba(255, 255, 255, 0.72);
  --bg-glass: rgba(255, 255, 255, 0.6);
  --bg-glass-strong: rgba(255, 255, 255, 0.8);
  --bg-input: rgba(255, 255, 255, 0.8);
  --bg-hover: rgba(0, 0, 0, 0.04);

  --text-primary: #1d1d1f;
  --text-secondary: #6e6e73;
  --text-tertiary: #aeaeb2;

  --border-subtle: rgba(0, 0, 0, 0.08);
  --border-strong: rgba(0, 0, 0, 0.12);

  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.06);
  --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.08);
  --shadow-lg: 0 12px 40px rgba(0, 0, 0, 0.12);

  /* 毛玻璃模糊 */
  --blur-sm: 8px;
  --blur-md: 20px;
  --blur-lg: 40px;

  /* 间距（4 倍数体系） */
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 20px;
  --space-6: 24px;
  --space-8: 32px;

  /* 圆角（iOS 风格） */
  --radius-sm: 8px;
  --radius-md: 12px;
  --radius-lg: 16px;
  --radius-xl: 20px;
  --radius-pill: 999px;

  /* 字体 */
  --font-sans: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  --font-mono: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  --font-size-xs: 11px;
  --font-size-sm: 12px;
  --font-size-base: 14px;
  --font-size-md: 15px;
  --font-size-lg: 18px;

  /* 动效 */
  --transition-fast: 0.15s ease;
  --transition-base: 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 深色主题 */
@media (prefers-color-scheme: dark) {
  :root:not([data-theme='light']) {
    --color-accent: #0A84FF;
    --color-accent-soft: rgba(10, 132, 255, 0.2);

    --bg-base: #1c1c1e;
    --bg-elevated: rgba(40, 40, 42, 0.72);
    --bg-glass: rgba(40, 40, 42, 0.6);
    --bg-glass-strong: rgba(50, 50, 52, 0.8);
    --bg-input: rgba(50, 50, 52, 0.8);
    --bg-hover: rgba(255, 255, 255, 0.06);

    --text-primary: #f5f5f7;
    --text-secondary: #aeaeb2;
    --text-tertiary: #636366;

    --border-subtle: rgba(255, 255, 255, 0.1);
    --border-strong: rgba(255, 255, 255, 0.16);

    --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.3);
    --shadow-md: 0 4px 16px rgba(0, 0, 0, 0.4);
    --shadow-lg: 0 12px 40px rgba(0, 0, 0, 0.5);
  }
}

/* 手动主题覆盖（预留，Settings 页可设 data-theme="dark"/"light"） */
:root[data-theme='dark'] {
  --bg-base: #1c1c1e;
  --bg-elevated: rgba(40, 40, 42, 0.72);
  --bg-glass: rgba(40, 40, 42, 0.6);
  --bg-glass-strong: rgba(50, 50, 52, 0.8);
  --bg-input: rgba(50, 50, 52, 0.8);
  --bg-hover: rgba(255, 255, 255, 0.06);
  --text-primary: #f5f5f7;
  --text-secondary: #aeaeb2;
  --text-tertiary: #636366;
  --border-subtle: rgba(255, 255, 255, 0.1);
  --border-strong: rgba(255, 255, 255, 0.16);
}
```

- [ ] **Step 2: 创建 `src/renderer/styles/glass.css`**

```css
/* 毛玻璃工具类。组合 backdrop-filter + 半透明 + 细边框 + 阴影。 */

.glass-card {
  background: var(--bg-glass);
  backdrop-filter: blur(var(--blur-md)) saturate(1.5);
  -webkit-backdrop-filter: blur(var(--blur-md)) saturate(1.5);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
}

.glass-panel {
  background: var(--bg-elevated);
  backdrop-filter: blur(var(--blur-lg)) saturate(1.8);
  -webkit-backdrop-filter: blur(var(--blur-lg)) saturate(1.8);
  border: 1px solid var(--border-subtle);
}

.glass-input {
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-family: var(--font-sans);
  font-size: var(--font-size-base);
  padding: var(--space-2) var(--space-3);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  width: 100%;
  box-sizing: border-box;
}

.glass-input:focus {
  outline: none;
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px var(--color-accent-soft);
}

.glass-input::placeholder {
  color: var(--text-tertiary);
}

/* 胶囊按钮 */
.btn {
  padding: var(--space-2) var(--space-4);
  border: none;
  border-radius: var(--radius-sm);
  font-family: var(--font-sans);
  font-size: var(--font-size-base);
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.btn-primary {
  background: var(--color-accent);
  color: #fff;
}
.btn-primary:hover:not(:disabled) { background: var(--color-accent-hover); }

.btn-ghost {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.btn-ghost:hover:not(:disabled) { background: var(--border-subtle); }

.btn:disabled { opacity: 0.4; cursor: not-allowed; }
```

- [ ] **Step 3: 创建 `src/renderer/styles/base.css`**

```css
/* reset + 全局基础 */

* { margin: 0; padding: 0; box-sizing: border-box; }

html, body, #app {
  height: 100%;
  font-family: var(--font-sans);
  font-size: var(--font-size-base);
  color: var(--text-primary);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

body {
  /* 毛玻璃背景：渐变 + 柔和光斑营造氛围 */
  background: var(--bg-base);
  background-image:
    radial-gradient(circle at 15% 20%, rgba(10, 132, 255, 0.08) 0%, transparent 40%),
    radial-gradient(circle at 85% 80%, rgba(175, 82, 222, 0.06) 0%, transparent 40%);
  background-attachment: fixed;
  overflow: hidden;
}

/* iOS 风滚动条 */
::-webkit-scrollbar { width: 8px; height: 8px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb {
  background: var(--border-strong);
  border-radius: var(--radius-pill);
}
::-webkit-scrollbar-thumb:hover { background: var(--text-tertiary); }

/* select 下拉也用主题色 */
select {
  font-family: var(--font-sans);
  color: var(--text-primary);
}

/* 标题层级 */
h1, h2, h3 { font-weight: 600; letter-spacing: -0.01em; }

/* 链接 */
a { color: var(--color-accent); text-decoration: none; }
```

- [ ] **Step 4: `src/renderer/main.ts` 引入样式**

读 `src/renderer/main.ts`，在 `import { createApp }` 之后、`createApp(App)` 之前加：

```typescript
import './styles/tokens.css'
import './styles/glass.css'
import './styles/base.css'
```

- [ ] **Step 5: 验证**

Run: `npm run typecheck` — exit 0（css 不参与 ts 类型检查，但确认无语法错误干扰）
Run: `npm run build` — 成功（确认 css 被 vite 处理）
Run: `npx vitest run` — 82 tests 全过（无回归）

- [ ] **Step 6: 提交**

```bash
git add src/renderer/styles/ src/renderer/main.ts
git commit -m "feat(ui): 设计系统 - tokens/glass/base 全局样式 + 深浅色主题"
```

---

## Task 2: App.vue 毛玻璃外壳 + 顶栏

**Files:**
- Modify: `src/renderer/App.vue`

- [ ] **Step 1: 改 App.vue 的 `<style scoped>` 和 template class**

读当前 App.vue。把 `.topbar` 改成用 `.glass-panel` 工具类（毛玻璃顶栏），nav-tabs 用变量，整体用 tokens。

新的 `<style scoped>` 替换：

```css
.layout { display: flex; flex-direction: column; height: 100vh; }

.topbar {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-6);
  border-bottom: 1px solid var(--border-subtle);
}

h1 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  letter-spacing: -0.02em;
}

.topbar select {
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--bg-input);
  color: var(--text-primary);
  font-family: var(--font-sans);
}

main { flex: 1; overflow-y: auto; }

.nav-tabs {
  display: flex;
  gap: var(--space-1);
  margin-left: var(--space-2);
  padding: var(--space-1);
  background: var(--bg-hover);
  border-radius: var(--radius-md);
}

.nav-tabs button {
  padding: var(--space-2) var(--space-4);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: var(--font-size-base);
  font-family: var(--font-sans);
  font-weight: 500;
  transition: all var(--transition-fast);
}

.nav-tabs button:hover { color: var(--text-primary); }

.nav-tabs button.active {
  background: var(--bg-glass-strong);
  color: var(--text-primary);
  box-shadow: var(--shadow-sm);
}
```

template 里 `.topbar` 的 div 加上 glass-panel 类（或在 scoped 里用 background var）。**推荐**：把 `<header class="topbar">` 改成 `<header class="topbar glass-panel">`，然后 scoped 的 `.topbar` 不再写 background/border（由 glass-panel 提供），只写布局。但要保留 topbar 特有的布局属性。最终 template：

```html
<header class="topbar glass-panel">
```

scoped `.topbar` 删掉 `background` 和 `border-bottom` 行（glass-panel 已提供），保留 display/gap/padding。

- [ ] **Step 2: 验证**

Run: `npm run typecheck`
Run: `npm run build`
Run: `npx vitest run`（82 green）

- [ ] **Step 3: 提交**

```bash
git add src/renderer/App.vue
git commit -m "feat(ui): App 顶栏毛玻璃 + nav-tabs iOS 分段控件风格"
```

---

## Task 3: MappingList 卡片毛玻璃化

**Files:**
- Modify: `src/renderer/components/MappingList.vue`

- [ ] **Step 1: 改 template class + style**

template 里 `.card` div 改成 `class="card glass-card"`。`.new-btn` 改成 `class="new-btn btn btn-primary"`（去掉 new-btn 自己的 background/color/border，由 btn-primary 提供）。

新的 `<style scoped>`：

```css
.mapping-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-4);
}

.empty {
  color: var(--text-tertiary);
  padding: var(--space-8);
  text-align: center;
  font-size: var(--font-size-base);
}

.card {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}

.card:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-lg);
  border-color: var(--color-accent);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
  background: var(--text-tertiary);
  flex-shrink: 0;
}
.dot.healthy { background: var(--color-success); }
.dot.unhealthy { background: var(--color-danger); }

.info { flex: 1; min-width: 0; }
.name { font-weight: 600; font-size: var(--font-size-md); }
.meta {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-top: 2px;
}

.new-btn { margin-top: var(--space-2); }
```

（new-btn 只保留 margin，视觉由 btn btn-primary 提供。）

- [ ] **Step 2: 验证 + 提交**

Run: typecheck + build + 82 tests
```bash
git add src/renderer/components/MappingList.vue
git commit -m "feat(ui): MappingList 卡片毛玻璃 + hover 微交互"
```

---

## Task 4: MappingForm 毛玻璃 + 健康检查开关（功能补全）

**Files:**
- Modify: `src/renderer/components/MappingForm.vue`

**这个 task 包含一个功能补全**：Plan 3 留的遗漏——表单没暴露 `healthCheckEnabled` / `healthCheckIntervalSec`。现在加上，让用户能开启健康检查。

- [ ] **Step 1: FormState 加健康检查字段**

读当前 MappingForm.vue。在 FormState interface 加：

```typescript
  healthCheckEnabled: boolean
  healthCheckIntervalSec: number
```

- [ ] **Step 2: templateToForm 填充**

```typescript
    healthCheckEnabled: m.healthCheckEnabled === 1,
    healthCheckIntervalSec: m.healthCheckIntervalSec ?? 30,
```

loadExisting 的赋值也加这两行。

- [ ] **Step 3: template 加健康检查折叠区**

在"附加指令" textarea 之后、error div 之前加：

```html
      <label class="checkbox"><input type="checkbox" v-model="form.healthCheckEnabled" /> 启用健康检查</label>
      <label v-if="form.healthCheckEnabled">检查间隔（秒）<input v-model.number="form.healthCheckIntervalSec" type="number" min="5" /></label>
```

- [ ] **Step 4: save 构建 input 时填健康检查字段**

找到 save() 里构建 input 的对象，把固定的 `healthCheckEnabled: 0, healthCheckIntervalSec: 30` 改成：

```typescript
      healthCheckEnabled: f.healthCheckEnabled ? 1 : 0,
      healthCheckIntervalSec: f.healthCheckIntervalSec,
```

- [ ] **Step 5: style 毛玻璃化**

`.form-card` 改成 `class="form-card glass-card"`，scoped 删 background/border/border-radius/box-shadow（glass-card 提供）。

input/select/textarea 用全局 `.glass-input` 类：把 template 里每个 `<input>` `<select>` `<textarea>` 加 `class="glass-input"`（或保留原 class 同时加）。scoped 里的 input 样式删掉（由 glass-input 全局提供）。

label.checkbox 的 input 不加 glass-input（checkbox 不需要）。

label 样式改用变量：`color: var(--text-secondary); font-size: var(--font-size-sm)`。

error: `color: var(--color-danger)`。

actions 的 button 改成 `class="btn btn-ghost"`（取消）和 `class="btn btn-primary"`（保存）。

- [ ] **Step 6: 验证 + 提交**

Run: typecheck + build + 82 tests（健康检查字段写入 input，不影响现有测试——mappingRepo 接受这两个字段）
```bash
git add src/renderer/components/MappingForm.vue
git commit -m "feat(ui): MappingForm 毛玻璃 + 补健康检查开关（Plan 3 遗漏）"
```

---

## Task 5: TemplatePicker 毛玻璃

**Files:**
- Modify: `src/renderer/components/TemplatePicker.vue`

- [ ] **Step 1: 改 template + style**

`.picker-card` → `class="picker-card glass-card"`。`.tpl-card` → 加 `glass-card`（或保留 class 但 style 用变量）。

实际上 `.tpl-card` 是网格里的小卡，给它 glass-card 太重，改成半透明 + 变量：

```css
.tpl-card {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-glass);
  cursor: pointer;
  text-align: left;
  align-items: flex-start;
  transition: all var(--transition-fast);
}
.tpl-card:hover {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
  transform: translateY(-1px);
}
```

group-title: `color: var(--text-tertiary); font-size: var(--font-size-xs); text-transform: uppercase; letter-spacing: 0.05em;`

name: `color: var(--text-primary)`。desc: `color: var(--text-secondary)`。

custom-tag: `background: var(--color-accent); color: #fff`。

`.actions button` 改成 `class="btn btn-ghost"`。

picker-overlay: `background: rgba(0,0,0,0.25); backdrop-filter: blur(var(--blur-sm))`。

- [ ] **Step 2: 验证 + 提交**

```bash
git add src/renderer/components/TemplatePicker.vue
git commit -m "feat(ui): TemplatePicker 毛玻璃卡片 + hover 微交互"
```

---

## Task 6: EngineControl 胶囊化

**Files:**
- Modify: `src/renderer/components/EngineControl.vue`

- [ ] **Step 1: 改 template + style**

`.engine-ctrl` 容器改用 glass 风格胶囊：

```css
.engine-ctrl {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--bg-glass);
  backdrop-filter: blur(var(--blur-sm));
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-pill);
  margin-left: auto;
}
```

（margin-left: auto 把它推到顶栏最右）

status-dot: `var(--radius-pill)`，颜色用变量（running→success, stopped→danger）。

status-text: `color: var(--text-secondary); font-size: var(--font-size-sm)`。

button 改成 `class="btn btn-primary"`（启动/应用/停止三个），但尺寸小一点。可以加 `.engine-ctrl .btn { padding: 4px 12px; font-size: var(--font-size-sm) }` 覆盖。

error-text: `color: var(--color-danger)`。

- [ ] **Step 2: 验证 + 提交**

```bash
git add src/renderer/components/EngineControl.vue
git commit -m "feat(ui): EngineControl 胶囊容器 + 顶栏右对齐"
```

---

## Task 7: LogViewer + HistoryPanel 配色变量化

**Files:**
- Modify: `src/renderer/components/LogViewer.vue`, `src/renderer/components/HistoryPanel.vue`

- [ ] **Step 1: LogViewer style 用变量**

toolbar: `border-bottom: 1px solid var(--border-subtle); padding: var(--space-3) var(--space-4)`。
h3: `font-size: var(--font-size-lg)`。
button 改 `class="btn btn-ghost"`。
log-list: `font-family: var(--font-mono); font-size: var(--font-size-sm); padding: var(--space-2) var(--space-4)`。
time/source: `color: var(--text-tertiary)`。
level 颜色函数改用变量（在 script 的 levelColor 里返回变量值字符串，但 CSS 变量不能直接在 inline style 用——改回硬色或用 class）：
  - 简化：levelColor 返回 success/warning/danger 对应的 css 变量字符串 `'var(--color-success)'` 等，inline style `:style="{ color: levelColor(level) }"` 能解析 var()。验证：浏览器 inline style 支持 var()。

empty: `color: var(--text-tertiary)`。

- [ ] **Step 2: HistoryPanel style 用变量**

toolbar 同 LogViewer。button 用 btn btn-ghost / btn-primary。
snap-list: `border-right: 1px solid var(--border-subtle); width: 240px`。
snap-item: `padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle)`。
snap-item:hover: `background: var(--bg-hover)`。
snap-item.active: `background: var(--color-accent-soft)`。
snap-time: `color: var(--text-secondary); font-size: var(--font-size-sm)`。
snap-reason: `color: var(--text-tertiary); font-size: var(--font-size-xs)`。
snap-content pre: `font-family: var(--font-mono); font-size: var(--font-size-sm)`。
empty: `color: var(--text-tertiary)`。

- [ ] **Step 3: 验证 + 提交**

Run: typecheck + build + 82 tests
```bash
git add src/renderer/components/LogViewer.vue src/renderer/components/HistoryPanel.vue
git commit -m "feat(ui): LogViewer + HistoryPanel 配色变量化 + iOS 分组列表"
```

---

## Task 8: 端到端验证

- [ ] **Step 1: 全量回归**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 82 tests 全过
Run: `npm run build` — 成功

- [ ] **Step 2: dev 模式视觉验证**

Run: `npm run rebuild:electron && npm run dev`（后台 25s 后 kill）
Expected: 窗口打开，无 fatal。视觉确认：
- 顶栏毛玻璃 + 背景渐变光斑
- 映射卡片半透明 + 模糊 + hover 上浮
- 模板选择器卡片网格
- 日志/历史 tab 切换
- 健康检查开关在表单里出现

- [ ] **Step 3: Playwright 截图验证（dev server 5173）**

dev server 起着时，用 Playwright 访问 http://localhost:5173 截图，肉眼确认毛玻璃效果（虽然 electronAPI 在浏览器里 undefined 导致 IPC 失败，但视觉层 CSS 生效）。

- [ ] **Step 4: 恢复 node ABI**

Run: `npm run rebuild:node`，确认 82 tests 仍绿。

- [ ] **Step 5: 提交 plan 文档**

```bash
git add docs/plans/2026-07-17-plan-4-glassmorphism-ui.md
git commit -m "docs: Plan 4 实施计划（毛玻璃 iOS 风格 UI）"
```

---

## Plan 4 完成标准

- [ ] 8 个 task 完成
- [ ] `npm run typecheck` 无错误
- [ ] `npx vitest run` 全绿（82，无回归——纯 CSS + 1 个表单字段补全）
- [ ] `npm run build` 成功
- [ ] dev 模式视觉确认：毛玻璃顶栏/卡片、深浅色主题、健康检查开关

## 后续 Plan

- **Plan 5**: 远程 nginx SSH 管理
- **Plan 6**: electron-builder portable 打包 + 一键清除 + 导入导出
