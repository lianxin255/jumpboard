# JumpBoard Design System

一套为高频端口转发工作设计的克制、专业、可持续阅读的原生桌面界面语言。服务现有 JumpBoard 用户：内网服务维护、开发调试与网络配置。目标不是展示数据，而是让“当前哪些映射生效、连接到哪里、下一步做什么”始终明确。

## Design rationale

当前版本的主要问题是工具分散、状态重复、卡片堆叠与页面骨架不稳定，而非缺少功能。此版重新分配现有功能，不新增产品能力。

- **一个全局状态**：侧栏底部说明转发服务是否运行；顶部仅放页面身份与当前页面动作，不重复引擎数据。预检与应用在同一位置显示进行中和结果。
- **场景选择完整**：左上角是固定高度的「场景图标 + 当前场景名 + 展开箭头」，不是并排碎片。菜单显示各场景规则数、当前标记；有未应用改动时明确说明保留在原场景。「管理场景」独立在旁边。
- **一层工作面**：映射使用单一表格面板；AI 使用对话与草稿双栏；终端使用会话标签与工作面；历史使用主从式快照查看。
- **行为分区**：页面顶部放页面级操作，工具区放查找/筛选，行内放对象级操作。创建不混入筛选。
- **设置主从导航**：原有 AI、外观、通用、访问控制、备份、场景、快捷键、更新与危险操作全部保留，按原有分组呈现；一次只读一组。
- **信息有层级**：名称和转发关系优先，协议、健康、流量是辅助。空白来自合理的列宽与留白，不添加装饰性统计卡。

## CONTENT FUNDAMENTALS

中文短句，直接说对象、状态、结果。按钮用动词短语：“新建映射”“应用配置”“导入到当前场景”。不使用夸张宣传、emoji 或无意义的技术术语。说明文字回答“这会怎样”而非重复标题。错误同时给出原因与可执行的下一步。

状态使用同一词表：运行中/已停止，健康/异常/未检查，已启用/已停用，未应用/已生效。危险操作使用明确后果，例如“停止后所有转发规则立即失效”，不以模糊的“确定吗”代替。

## VISUAL FOUNDATIONS

- **Color**：暖白与石墨中性色构成 90% 界面。蓝色 `#315fbf` 只用于主操作、当前导航与焦点；暗色改为 `#a7c2f7`，以深底浅色保持对比。成功、警告、危险为语义色，不用语义色装饰。暗色不是整体压灰：正文 `#f2f4ef`、说明 `#d0d7d0`、辅助 `#aab6ad`，最弱辅助文字在主要表面上仍不低于 6.37:1。输入框使用略亮于面板的 `--surface-raised`，关闭开关保留轨道和边界，不能只靠白色滑块辨认。
- **Typography**：系统 Latin + PingFang SC，无外部字体依赖。页面标题 22px，分区 15px，正文 13px，辅助 12px/11px；CJK 阅读行高 1.75。地址、端口、命令与日志使用系统等宽字体及表格数字。
- **Spacing / control sizes**：4/8 网格；页面边距 28px；工作区内部 12–16px；设置段落 24px。尺寸由 `tokens/geometry.css` 统一提供，页面不得自行增加高度档位：文字按钮 32px、图标按钮 32×32、输入框/选择器 36px、紧凑搜索与分段器外框 32px、开关 36×22、筛选芯片 26px、导航 34px、菜单选项 36px、映射行 64px。不同类别不强求等高，但同一工具行的交互控件必须对齐中心线。
- **Surfaces**：一块边界清晰的工作面板胜过层层卡片。面板 10px 圆角、控件 7px、弹窗 12px、标签 5px。无渐变、无装饰纹理、无大面积毛玻璃。普通面板无投影；仅模态层有环境阴影。
- **Borders**：外边界 `--border`，内部细线 `--border-subtle`。不要为每一行/每个字段再套一层卡片。选中使用浅蓝底，不使用整条高饱和蓝色。
- **Iconography**：沿用 `internal/ui/icons/` 的 16 个原始 SVG。16px 常规、14–16px 按钮、18px 空状态；只用图标配文字，含义不明的图标必须有标签或 tooltip。无 emoji 图标。
- **Motion**：120ms hover/focus，180ms 面板与状态过渡。无持续循环装饰；尊重 `prefers-reduced-motion`。工作数据更新不使用弹跳。
- **Responsiveness**：窗口优先桌面，内容区可伸缩。断点有明确职责：
  - **1120px**：AI 页的分界。≥1120 时对话与待审草稿并排两栏；<1120 时草稿栏会窄到读不全转发地址，因此回落为单列——对话占满，草稿随对话滚动。
  - **1180px**：映射表把「本机 → 上游」从并排改为上下两行完整展示，不再截断端口。
  - **960px**：侧栏收窄为纯图标导航；设置导航与内容改为上下分组。
  操作不能被挤出窗口；长地址必须可换行或完整双行显示，不靠缩小字号解决。
- **States**：所有按钮、图标按钮、芯片、分段器、快照行与开关共享状态规则。hover 中性提亮一级，pressed 略深，focus 2px accent + offset；disabled 保留可辨边界且禁止操作。选中态使用浅蓝底，不能与 hover 同色。开关 36×22，开启为品牌蓝，关闭保留轨道；异步操作期间禁用重复提交，并显示进行中状态。

## Component inventory

Source: `internal/ui/theme.go`, `primitives.go`, `buttons.go`, `page_header.go`, `view.go`, `pages.go`, `dialogs.go`, `chat.go`, `terminal.go`.

- Buttons: `Button`, `IconButton`（主/次/静默/危险）
- Forms: `Input`, `Switch`, `Segmented`
- Navigation: `NavItem`
- Feedback: `Badge`, `Status`, `Notice`, `Panel`, `Dialog`
- Data: `DataTable`

`Table`/`List` 在原生实现中继续使用 mygo 自带选择、拖放与滚动状态。HTML 组件仅定义视觉与交互契约，不替代引擎或业务逻辑。

## Implementation handoff

视觉令牌可落到 `internal/ui/theme.go`，组件几何对应 `primitives.go` 与 `buttons.go`，导航对应 `view.go:sidebar`，窗口标题对应 `page_header.go`。本次交付仅为 HTML 高保真方案与组件规范，未修改生产 Go 界面。

## Source & assets

- Codebase: `/Users/lianxin/Desktop/oepnProject/jumpboard/internal/ui/`
- Existing exports: `designs/exports/` and `/tmp/ui-shots/`
- Copied assets: `assets/icons/` (original project SVGs, unchanged)
- Direction references (principles only, no copied visual assets): Linear's hierarchy, TablePlus/Postman's workspace organization, macOS System Settings' master-detail settings.

## Index

- `tokens/`: semantic color, type, geometry and components
- `components/`: source inventory with typed React references
- `guidelines/`: foundation specimens
- `ui_kits/workbench/`: six-page interactive proposal
- `preview.html`: generated design-system review
- `SKILL.md`: reusable agent guidance
