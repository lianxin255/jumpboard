# mygo v0.6.4 → v0.7.4 升级差异与 JumpBoard 升级方案

> 调研日期：2026-11-03  
> 当前依赖：`github.com/egoist/mygo v0.6.4`（`go.mod:6`），同时通过 `tool github.com/egoist/mygo/cmd/mygo`（`go.mod:29`）使用同版本 CLI。  
> 目标版本：v0.7.4。  
> 信息来源：实际读取了 GitHub REST API 返回的五个 release body、各 tag 的 `go.mod`，并对照 JumpBoard 当前工作区源码。未仅凭 release 标题推断影响。

## 1. 结论摘要

1. **可以直接升级到 v0.7.4，未发现需要修改 JumpBoard Go 代码的 API breaking change。** 在 `/tmp` 的当前源码副本中仅把 mygo 改为 v0.7.4 后，`go test ./...`、race 测试、`go vet ./...`、Windows 交叉编译和 macOS universal 打包均通过。
2. **上游列出的 5 条 Breaking changes 都没有直接击中本仓库的现有 API 调用。** JumpBoard 没有显式使用 child window、CJK 应用名或 audio API，也没有使用 Web `Page` 的双指缩放/滑动导航；但 DMG/Windows 安装包改名会影响外部固定下载链接，发布时仍需核对产物。
3. **本次升级对 JumpBoard 有实际价值。** macOS 原生 dialog、Cmd+Q/编辑快捷键、Windows 首帧/resize、终端滚动/滚动条、updater 系统代理/PAC 支持、macOS 空闲功耗等都有修复或改善。
4. **最大的剩余风险不是编译，而是平台行为和发布链路。** 本机未执行 Windows 原生测试/打包，未做真实签名更新 vA→vB，也未进行人工桌面 UI 冒烟；终端的 `libghostty-vt`、更新代理场景和 nginx 子进程仍需随真实应用验证。
5. **建议从干净基线用独立 worktree/分支单独升级，不与当前未提交的 AI/UI 改动混在一个变更里。** 没有必须停留在 v0.6.4 的代码理由；若近期要发布，应先完成 Windows、终端和真实更新回归，再合入。

## 2. 验证范围与证据

以下命令均在当前源码的临时副本执行，仓库的 `go.mod` / `go.sum` 未改变：

| 验证 | 结果 |
|---|---|
| `go mod edit -require=github.com/egoist/mygo@v0.7.4 && go mod tidy` | 通过 |
| `go test ./...` | 全部通过，包括 `e2e`、`internal/native`、`internal/ui`、`internal/appt` |
| `go test ./... -race -count=1` | 全部通过 |
| `go vet ./...` | 通过 |
| `GOOS=windows GOARCH=amd64 go build ./...` | 通过，仅证明交叉编译，不代表 Windows 运行验证 |
| `go tool mygo build -platform darwin/universal` | v0.7.4 CLI 打包通过，生成 `JumpBoard.app`、`jumpboard-1.2.0.dmg` 和 `install.sh` |

限制：测试覆盖的是**当前工作区**（包含已有未提交改动）的临时副本；没有人工打开真实 App，没有 Windows 实机结果，没有配置签名私钥，因此没有生成或验证签名更新包。部分 terminal 测试在 `libghostty-vt` 不可用时允许 skip（`e2e/terminal_test.go:21-25`），所以“测试全过”不等于真实终端动态库已验收。

上游 v0.6.4、v0.7.0～v0.7.4 的 `go.mod` 均声明 `go 1.27.3`，直接及间接依赖版本一致；本仓库同样是 Go 1.27.3，不需要额外 toolchain 或依赖迁移。

## 3. 按版本分组的变更清单

以下保留功能性与构建/发布条目；每条均带 PR 号。GitHub Showcase 与贡献者名单不影响 SDK，单独列在对应版本末尾。

### v0.7.0（2026-10-27）

Release：https://github.com/egoist/mygo/releases/tag/v0.7.0  
Compare：https://github.com/egoist/mygo/compare/v0.6.4...v0.7.0

#### Breaking changes

- **#764**：macOS child window 不再随父窗口移动，保持原位置，与 Windows/Linux 一致。
- **#768**：CJK 应用名会生成新的默认 ASCII `slug`；已发布 Linux 应用应显式固定旧 `slug`，避免旧 executable/Debian 包残留。
- **#769**：DMG 和 Windows installer 改用 `slug` 命名，例如 `my-app-1.2.0-arm64.dmg`、`my-app-setup-1.2.0-amd64.exe`；固定下载链接和 CI glob 需同步。
- **#746**：`audio.Player.Position` 改为实际听到的位置，通常比旧读数晚约 40ms；`LatencyPlayback` 下约晚 250ms。

#### 新增能力与 Highlights

- **#739**：`Context.SetZoom` 提供清晰 native UI 缩放，文本、图标、SVG、图像及输入/菜单/拖拽/WebView 同步。
- **#787**：`FlyoutOptions.FitContent`、`WindowOptions.FitContent` 根据 Web 内容尺寸调整窗口/Flyout。
- **#776**：`NewMenuIcon`、`MenuItem.Icon` 支持 app/context/tray menu 图标。
- **#748**：更新构建新增 macOS/Linux `install.sh`、Windows `install.ps1`；安装前校验签名或 checksum，Windows 升级前关闭运行中的 App（含 tray App），新增 `publisher`。
- **#753、#754、#757、#759、#760**：局部动画和 GPU 增量重绘；新增 `Painter.AnimationFrameRate`、`Element.Cache`。
- **#778**：`Page.ThemeColor`、`OnThemeColorChanged` 读取并监听 `<meta name="theme-color">`。
- **#781**：`Page.IsAudible`、`OnAudibleChanged`、`SetAudioMuted` 支持页面声音状态与静音。
- **#777**：`Page.ProcessID` 返回页面进程 ID（macOS/Windows）。
- **#785**：`WindowOpenRequest.UserInitiated` 区分用户打开与脚本弹窗。
- **#763**：Windows `Window.SetWindowButtonsTheme` 独立设置隐藏标题栏的 caption buttons 主题。
- **#775**：`App.IsActive` 查询 App 是否在前台。
- **#772**：`Element.Hidden` 以 `display:none` 方式隐藏元素并保留内部状态。
- **#774**：path stroke 新增 cap、join、dash。
- **#736**：`Glass.Clarity` 调整大面积玻璃材质清晰度。
- **#737**：Markdown 新增 `Options.InlineMath`，streaming 中未完成 `$…` 也可按 math 渲染。
- **#800**：Linux native UI 支持触摸点击、长按右键、拖动滚动和 pinch zoom。
- **#791**：code editor 新增 Dart、GraphQL、R、Scala 高亮。
- **#590**：terminal 支持 Shift-click 扩展/缩小跨行选区。
- **#743**：`audio.SetLatency(audio.LatencyPlayback)` 改善 macOS 外观切换时播放缓冲。
- **#761**：`Updater.SetPrereleases` 支持 beta 用户接收 prerelease；`mygo build` 提示预发布版本应发布为 prerelease。
- **#768**：新增 `slug` 配置，用于 Linux executable/package、更新归档和安装脚本等 ASCII 文件名。
- **#782**：`windows.displayName` 分离 Start menu/shortcut/Settings/installer 显示名与 executable 名。

#### 修复

- **#773**：修复非 ASCII 应用名下载更新时的 `400 Bad Request`。
- **#806**：macOS 更新后保持单一 Dock tile。
- **#775**：后台检查发现更新时，更新窗口正确置前或等待 App 回到前台。
- **#747**：macOS 更新重启后，App 启动的 child process 不再因某个已启动程序退出而被终止。
- **#805**：修复 Cmd+Q 和其他退出路径不能始终结束 App。
- **#793**：`SearchField`、`Combobox`、`TokenField` 的 `AutoFocus`/`Focus` 正确聚焦输入区，立即可输入和 Submit。
- **#795**：scroll view/clipped box 移动动画更平滑，例如 Windows 侧栏折叠。
- **#792**：terminal Mirror 在新 session 开始时清理旧图像。
- **#764**：同时修复 macOS child window 在多显示器上的显示、点击、置前、层级和 fullscreen 行为。
- **#765**：修复 macOS drag region 快速按下并拖动时窗口不一定移动。
- **#766**：macOS tray 动态数字标题保持固定宽度。
- **#803**：修复 accessibility client 给 text field 设置数字值导致的崩溃。
- **#784**：Windows terminal 每个滚轮 notch 滚动三行，与 Windows Terminal 一致。
- **#804**：Windows 隐藏标题栏窗口不再显示系统 accent strip。
- **#767**：Windows hidden/off-screen window 不再每帧卡顿约一秒。
- **#758**：Windows `MYGO_GPU=0` 时动画按显示器刷新率运行，不再占满一个 CPU core。
- **#744**：网站新增 Showcase（非 SDK 变化）。

### v0.7.1（2026-10-27）

Release：https://github.com/egoist/mygo/releases/tag/v0.7.1  
Compare：https://github.com/egoist/mygo/compare/v0.7.0...v0.7.1

#### Breaking changes

- 无。

#### 新增能力

- **#808**：`Font.LineHeight` 为 `ShapeText`、`ShapeRichText` 设置字号倍数行高（需求 #807）。

#### 修复

- **#811**：macOS 通过 `DragWindow()` 或 `--app-region: drag` 移动窗口时不再抖动。

### v0.7.2（2026-10-28）

Release：https://github.com/egoist/mygo/releases/tag/v0.7.2  
Compare：https://github.com/egoist/mygo/compare/v0.7.1...v0.7.2

#### Breaking changes

- **#846**：Web `Page` 的 pinch zoom 和 swipe navigation 默认关闭：Linux/Windows 不再默认双指缩放，Windows 不再默认触摸滑动返回；需要时设置 `PageOptions.PinchZoom`、`PageOptions.SwipeNavigation`，两者也支持 macOS。

#### 新增能力与 Highlights

- **#848**：`PageOptions.Session`、`NewSession`、`NewEphemeralSession`、`Session.ClearData` 提供独立 cookies/storage/cache/service workers。
- **#849**：`Page.OnContextMenu` 可查看、编辑、替换或阻止 WebView context menu。
- **#852**：`Element.PopoverMenu` 按 App theme 绘制菜单并支持键盘/屏幕阅读器；`MenuItem.Radio` 支持单选项。
- **#851**：`plugins/automation` 可通过当前用户 socket 驱动窗口、accessibility tree、输入、菜单、截图和页面脚本。
- **#839**：`ui.Video`、`ui.VideoPlayer`、`ui.NewGIFDecoder` 支持 native UI 视频/动画 GIF。
- **#853、#850**：`TextLayout.Lines`、段落 justify/indent、soft hyphen、`Span.BaselineShift`；rich text 新增首行缩进、段距、可见段落与稳定 paragraph ID。
- **#847**：`PageOptions.BackgroundThrottling: ThrottlingOff` 允许 hidden/minimized/covered page 全速运行。
- **#844**：`Page.SessionState`、`PageOptions.SessionState` 保存/恢复历史、滚动位置和表单；Windows 仅保留 URL。
- **#842**：`PageOptions.PictureInPicture`、`Page.TogglePictureInPicture`（macOS/Windows）。
- **#843**：`Page.CaptureState`、`OnCaptureStateChanged`、`SetCaptureState` 管理摄像头、麦克风和屏幕捕获（macOS/Linux）。
- **#845**：`Page.SetAuthHandler`、`SetCertificateErrorHandler` 处理 HTTP 认证和证书错误。
- **#856**：`PageOptions.Autoplay: AutoplayAllow` 允许无需点击播放声音。
- **#829**：`PageOptions.Untrusted` 隔离不可信页面；`CallerPage` 标识 custom scheme 请求来源。
- **#871**：`PageOptions.PreloadAllFrames` 让 preload script 进入 iframe。
- **#868**：`DecodeImage` 支持 bitmap/SVG，`ImageSize` 返回自然尺寸（需求 #867）。
- **#799**：新增 `netproxy`，支持系统代理/PAC；updater、fetch、websocket plugin 使用它。
- **#820**：`Updater.SetSource` 支持自有更新服务，仍验证构建签名 key。
- **#819**：`windows.installerInclude` 支持 NSIS `customInit`、`customInstall`、`customUnInstall` hooks。
- **#854**：`windows.installDirName` 单独指定安装目录名。
- **#833**：`macos.icon` 接受 `.icns`。

#### 修复

- **#827**：editor annotations、hunk actions、hover cards 上方的滚轮事件可继续传递给周围滚动容器。
- **#826**：屏幕阅读器按行内内容 label 阅读该行。
- **#858**：修复无 GPU 绘制路径在上传后释放 image pixels 时崩溃。
- **#859**：更新窗口显示完整错误详情。
- **#880**：diff 删除条跨行均匀显示。
- **#874**：calendar 月份箭头居中。
- **#818**：macOS Control-click tray icon 打开右键菜单。
- **#660**：Windows 透明 native UI 窗口正确显示后方内容，不再为黑色。
- **#855**：Windows 快速 resize 时新边缘显示背景色而非黑色。
- **#831**：Windows hidden window 不再留下 embedded WebView 拦截点击。
- **#854**：Windows installer 升级保留原安装目录。
- **#841**：Linux WebView 上方的 native UI 窗口仅在自身内容变化时重绘。
- **#877**：Linux 窗口由按钮隐藏后再次显示时正常绘制。
- **#860、#861、#862、#863、#869**：Showcase 更新（非 SDK 变化）。

### v0.7.3（2026-10-29）

Release：https://github.com/egoist/mygo/releases/tag/v0.7.3  
Compare：https://github.com/egoist/mygo/compare/v0.7.2...v0.7.3

#### Breaking changes

- 无。

#### 新增能力与 Highlights

- **#886**：`Element.PopoverMenuButton`、`ui.PopoverMenuButton` 按 theme 绘制菜单，支持键盘/屏幕阅读器及空间不足时向上展开。
- **#891**：terminal 新增 `Options.ScrollMultiplier`、`SetScrollMultiplier`。

#### 修复

- **#893**：Windows native UI resize 时内容不再被放大并在右侧/底部裁切（报告 #892）。
- **#884**：恢复 `SearchField`、`TokenField`、`Combobox` 的 `Placeholder` 显示（报告 #881）。
- **#886**：修复从 toolbar overflow menu 选择 menu button 后无法打开菜单。
- **#885**：修复 macOS native UI 点击后打开的 dialogs/sheets 不显示、不等待回答（报告 #883）。
- **#889**：macOS 无文本输入焦点时，window-wide Cmd+V/C/X/A/Z/Shift+Z 也会执行。

### v0.7.4（2026-11-01）

Release：https://github.com/egoist/mygo/releases/tag/v0.7.4  
Compare：https://github.com/egoist/mygo/compare/v0.7.3...v0.7.4

#### Breaking changes

- 无。

#### 新增能力与 Highlights

- **#913、#915**：Windows native UI 首帧准备好前保持不可见；`Hidden: true` 窗口可先隐藏绘制首帧，再用 `w.OnReadyToShow(w.Show)` 显示（macOS/Windows/Linux，报告 #905）。
- **#943**：`WebViewOptions.ScrollToContainer` 把 WebView 滚轮/触摸板滚动交给 native scroll container。
- **#932**：`ui.Themed(c, theme)` 为子树设置独立 theme（需求 #931）。
- **#971**：Edit menu 的 Undo/Redo/Cut/Copy/Paste/Delete/Select All 根据焦点能力启用；密码框禁止复制，只读框禁止粘贴。
- **#934**：`mygo build` 生成 Arch Linux `PKGBUILD`（需求 #933）。
- **#928**：降低 macOS idle native UI 功耗，唤醒约从 135 次/秒降至 13 次/秒，CPU 约从 0.8% 降至 0.3%。
- **#937**：`ui.DecimalInput` 以字符串编辑精确十进制数（需求 #936）。
- **#910**：`Painter.FillRadialGradient`、`Element.RadialGradient`（需求 #907）。
- **#927**：`Element.IgnoreInput` 忽略指针/键盘但保留辅助技术触发（需求 #926）。
- **#903**：`Calendar(c, &date).Browse()` 只浏览月份、不选择日期（需求 #898）。
- **#972**：`Element.Scrollbars(false)` 隐藏滚动条但保留滚动能力。
- **#969、#972**：输入新增 `PlaceholderColor`、`CaretWidth`。
- **#969**：新增 `DottedUnderline`、`Context.Instant`。
- **#949**：新增 `ui.Scrollbar`、`Painter.Scrollbar`。
- **#967**：自定义 `HandleInput` 元素支持屏幕阅读器/语音控制文本纠正。
- **#969**：Markdown 新增 `Options.Anchor`、`Theme.LinkWeight`。
- **#938**：`MessageOptions.DestructiveButtons` 突出危险按钮。
- **#952**：`Notification.ShowContext` 在 context 结束时停止等待通知权限。
- **#960**：`updater.AvailableVersion()` 返回最近一次检查发现的版本。
- **#657**：Windows 支持 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`、`WEBVIEW2_USER_DATA_FOLDER`。
- **#900**：`linux.command` 接受名称列表，支持短命令 alias。
- **#902**：`mygo build -skip-installer` 跳过 Windows installer/NSIS 下载（需求 #897）。

#### 修复

- **#909**：Grid 子元素 `FillWidth()`/`FillHeight()` 等百分比相对自身 cell，避免覆盖相邻 cell（报告 #906）。
- **#896**：部分滚出视野的元素，其 `PopoverMenu` 在指针位置打开（报告 #894）。
- **#944、#949**：code editor、rich text、terminal 滚动条遵循 `Theme.ScrollbarWidth` 和 High Contrast，并在 hover 时显示。
- **#941**：大型 App 的 delta updates 可完成构建（报告 #939）。
- **#735**：editor 选区拖过左右边缘时横向滚动。
- **#935**：指针位于 diff hunk header/expand bar 上时仍可滚动。
- **#947**：内容高度 diff 和 split diff 旧侧可为长行显示水平滚动条。
- **#911**：split diff 长度差异大的 hunk 间 filler 正确显示网纹，不再冻结滚动（报告 #904）。
- **#920**：terminal 绘制 block sextants，可显示 Expo 等工具输出的 QR code。
- **#950**：macOS 非前台窗口中的 page 第一次点击即可到达按钮/链接（报告 #946）。
- **#942**：macOS 26 多显示器下，tray flyout 在被点击的 icon 副本下方打开。
- **#938**：macOS Escape/Cmd+. 可选择 cancel button；单按钮 alert、`Dialog.Error`、页面 `alert()` 可用 Escape 关闭。
- **#925**：`mygo build` 可在 macOS 27 制作 disk images。
- **#975**：Windows page 从首帧起使用 App 强制的 light/dark appearance，不再闪现系统外观。
- **#895**：Windows native UI 窗口缩小时边缘显示背景色而非黑色。
- **#918**：Windows 10 frameless window 内容延伸到边缘，不再出现白线（报告 #916）。
- **#923**：Windows 11 高缩放下菜单不再截断最长 label（报告 #922）。
- **#912**：Windows 从 Notepad 复制的文本可通过 `clipboard.ReadData` 粘贴。
- **#953**：病毒扫描短暂占用新文件时，`mygo build` 不再失败。
- **#662**：installer 的“Create a desktop shortcut”跟随语言，可用 `mygo.desktopShortcut` 翻译。
- **#917、#919**：Showcase 更新（非 SDK 变化）。

## 4. 对 JumpBoard 的具体影响分析

### 4.1 本仓库实际使用面

JumpBoard 是 native UI App，不是以 Web `Page` 为主体的壳：

- App/window/updater/clipboard/dialog：`main.go:37-198` 的 `main`，特别是 `main.go:86-106` 更新器、`main.go:125-144` `NewWindow`。
- Tray/Flyout/global shortcut/关闭到托盘：`internal/native/native.go:65-215` 的 `Setup`、`setHotkey`；`internal/native/flyout.go:29-45` 的 `toggleFlyout`。
- Native menu：`internal/native/menu.go:34-50`；角色菜单行为由 `internal/native/menu_test.go` 固定。
- Theme：`internal/ui/theme.go:185-193` 的 `ApplyAppearance`，以及 `internal/ui/theme.go:215-244` 对 `ui.Theme` 字段的直接定制。
- 大量 native widgets、List/Table、 drag/drop、Popover/Dialog：`internal/ui/view.go`、`dialogs.go`、`pages.go`、`chat.go`、`ai_settings.go`。
- Terminal：`internal/ui/terminal.go:267-317` 等处使用 `plugins/terminal`，并依赖 `libghostty-vt` 的打包/加载。
- Markdown streaming：`internal/ui/chat.go:306-313`、`internal/ui/interpret.go:111-124`。
- Editor/diff：`internal/ui/dialogs.go:508-526`、`651-683`。
- Tester：`e2e/ui_test.go:16-20` 及整个 e2e suite；截图工具 `cmd/shots/main.go:98-163`。
- CLI/发布：`Makefile:43-49`、`.github/workflows/ci.yml:33-42`、`.github/workflows/release.yml:62-106`。

### 4.2 会直接落到本 App 的变化

| 变化 | JumpBoard 位置 | 影响与动作 |
|---|---|---|
| macOS dialog/sheet 修复（#885）、message Escape cancel（#938） | `main.go:111-112` 绑定文件 dialog，`main.go:226-230` 启动错误 message；`internal/ui/files.go`、`interpret.go` | **直接受益。** 必须手测导入/导出和 message 取消，因为自动化 mock 无法覆盖真实系统 dialog。 |
| Cmd+Q/退出修复（#805）、编辑快捷键与 Edit menu（#889、#971） | `main.go:174-193`；`internal/native/native.go:152-167`；`internal/native/menu.go` | **直接影响。** 验证 Cmd+Q、tray 退出、关闭到托盘，以及输入框和非输入焦点下的 Cut/Copy/Paste/Undo。 |
| 首帧、resize、透明/背景及隐藏窗口修复（#913、#915、#660、#855、#893、#895、#918、#767、#804） | `main.go:125-144` 主窗口；`internal/native/flyout.go`；关闭到托盘路径 | **直接影响，主要改善 Windows。** 检查白/黑闪、边缘、resize、隐藏后恢复和隐藏标题栏 accent strip；macOS 也可观察首帧。 |
| macOS 窗口拖动修复（#765、#811） | `internal/ui/view.go:240-242`、`page_header.go:90-103` 的 `DragWindow()` | **直接受益。** 手测品牌区、标题栏 spacer 快速拖动。 |
| macOS tray flyout 多显示器修复（#942）、child window 行为（#764） | `internal/native/flyout.go:34-45` | **Flyout 属相关回归面。** 本仓库未显式创建 parent/child window，因此不能把 #764 当作确定的破坏性影响；多显示器位置仍需手测。 |
| Tray 右键与高缩放菜单修复（#818、#923） | `internal/native/native.go:79-121`；`internal/native/menu.go` | **直接影响。** 验证 macOS Control-click tray menu，以及 Windows 11 高缩放下的 app/tray 菜单长标签。 |
| Terminal 修复/能力（#784、#590、#891、#920、#944、#949） | `internal/ui/terminal.go`；`e2e/terminal*_test.go` | **直接影响。** Windows 滚轮行为改变；滚动条视觉、选区和特殊字符需回归。`ScrollMultiplier` 是可选项，本次不必接入。 |
| Unified diff 显示/滚动修复（#880、#935、#947） | `internal/ui/dialogs.go:508-526` snapshot diff，固定 `editor.DiffUnified` | **直接影响。** 验证长行、unified diff 和 hunk header 上滚动；#911 仅修复 split diff filler，不属于当前路径。 |
| Native UI 重绘与 idle 优化（#753、#754、#757、#759、#760、#928） | 整个 `internal/ui`，尤其动画/Spinner/列表 | **直接受益。** race/e2e 已通过，但仍观察动画、列表滚动、CPU 和残影。 |
| 构建产物改名与安装脚本（#769、#748） | `mygo.json`；`release.yml:80-106`；`README.md:47-52` | **实际发生。** 临时构建已生成 `jumpboard-1.2.0.dmg` 和新的 `install.sh`。workflow 使用目录 wildcard，无需改 glob；README 仍写旧 `JumpBoard 1.2.0.dmg`，升级时必须同步。 |
| Updater 改用 `netproxy`（#799）、delta 构建修复（#941） | `main.go:86-106` updater；签名发布构建 | **直接行为变化。** 更新检查、完整包/delta 下载会遵循系统代理、PAC/WPAD、环境变量和 bypass 规则；需覆盖代理成功/失败及 delta 产物。 |
| 更新后 child process 修复（#747）、Dock tile（#806） | `main.go:86-108` updater；nginx 子进程 | **直接相关。** 真实更新后确认 nginx/转发状态和 Dock 图标；当前未做签名更新验证。#859 修复的是未使用的 `plugins/updater` 更新窗口，不计入本 App 收益。 |
| Windows installer 关闭 tray App（#748）、保留安装目录（#854） | tray/close-to-tray；Windows release | **直接影响安装/升级。** 必须在 Windows 验证运行中升级能关闭并重启 JumpBoard。 |

### 4.3 Breaking changes 分类

| 上游 Breaking change | 分类 | 本仓库依据 |
|---|---|---|
| macOS child window 不随父窗口移动（#764） | **未显式使用；Flyout 需回归** | 没有 child-window API；只有 `NewFlyout`（`internal/native/flyout.go:34`）。 |
| CJK 名称默认 `slug` 改变（#768） | **不受影响** | `mygo.json:2` 名称为 ASCII `JumpBoard`。 |
| DMG/Windows installer 用 `slug` 改名（#769） | **实际受影响，但当前 workflow 兼容** | 名称会变；`release.yml:85-106` 使用 wildcard 上传，无旧文件名硬编码。外部文档/脚本若固定旧链接需另查并更新。 |
| `audio.Player.Position` 语义（#746） | **未使用** | 未 import/use `audio`。 |
| Web `Page` 手势默认关闭（#846） | **未使用** | 未使用 `Page`、`PageOptions` 或嵌入式 Web `WebView`；主体是 `mygo/ui` native controls。 |

### 4.4 新增但本次不建议顺手接入的能力

以下只是“可用”，不是升级所必需；为避免扩大变更范围，本次保持现有行为：

- `Context.SetZoom`（#739）、`FitContent`（#787）、menu icons（#776）、`Element.Cache`/`AnimationFrameRate`（#753 等）。
- `PopoverMenu`/`PopoverMenuButton`（#852、#886）、`ui.Themed`（#932）、`DecimalInput`（#937）、`IgnoreInput`（#927）。
- `Updater.SetPrereleases`（#761）、`SetSource`（#820）、`AvailableVersion`（#960）。
- `plugins/automation`（#851）、Page Session/权限/PIP（#848、#842 等）、WebView2 调试环境变量（#657）。
- Terminal `ScrollMultiplier`（#891）、Markdown `InlineMath`/Anchor（#737、#969）。
- `slug`（#768）：当前默认产物名兼容 wildcard 流程，不必为了升级新增配置；但 `README.md:51` 的旧 DMG 名称必须更新。若希望永久固定发布文件名，可另开决策。

### 4.5 明确未使用、因此无需迁移的功能

- 所有 Web `Page`/Session/页面权限/页面音频 API（#777～#785、#842～#856、#871、#943、#950）；#975 也仅修复 Web `Page` 首帧外观，不影响当前 native UI theme。
- `plugins/updater` 的更新窗口修复（#859）：本仓库使用 `mygo.Updater` 加自有设置页，未 import 该插件。
- `SearchField`/`Combobox`/`TokenField` 的 AutoFocus/Focus 修复（#793）与 Placeholder 修复（#884）：本仓库只用其中 `SearchField`（`internal/ui/pages.go:95`），且未设置这些选项；不能算作已证实的直接收益。
- `audio`（#743、#746）。
- native video/GIF（#839）。
- Grid 布局修复 #909：当前代码未使用 `ui.Grid`。
- split diff filler 修复 #911：当前 snapshot 固定使用 `editor.DiffUnified`。
- Markdown `InlineMath`（#737）：当前 `chat.go`、`interpret.go` 均使用空 `markdown.Options{}`，未启用该 renderer 或相应扩展；一般 `$…` 冒烟可做，但不算本次直接行为变化。
- Linux installer/Arch/PKGBUILD、`linux.command`（#900、#934）：当前 CI/release 仅 macOS、Windows。
- Windows installer 自定义 NSIS hooks（#819）、`windows.displayName`（#782）、`installDirName`（#854）：当前 `mygo.json` 未配置。

## 5. 升级步骤与验证方式

### 5.1 推荐执行顺序

先提交或暂存当前 AI/UI 工作，再从干净基线建立独立 worktree；不要直接在当前脏工作区 `git switch -c`，那会把所有未提交改动带进升级分支：

```sh
git worktree add ../jumpboard-mygo-0.7.4 -b chore/mygo-0.7.4 main
cd ../jumpboard-mygo-0.7.4
go get github.com/egoist/mygo@v0.7.4
go mod tidy
go list -m github.com/egoist/mygo
```

`tool github.com/egoist/mygo/cmd/mygo` 与 library 共用 module 版本，不需要另设 CLI 版本。预期只有 `go.mod` / `go.sum` 因升级改变；若 `go mod tidy` 带来无关依赖大改，应先停下核对。

本次不必修改 Go 调用代码，也不必新增任何上游可选能力。升级时应同步两处已有文档：`README.md:51` 的 DMG 名称改为实际产物 `jumpboard-1.2.0.dmg`，并核对 Windows installer/安装脚本说明；`docs/release-and-updates.md:53` 的 mygo 文档链接从 v0.6.4 更新为 v0.7.4。不要借机改写无关文档。

### 5.2 自动验证

```sh
go test ./... -count=1
go test ./... -race -count=1
go vet ./...
go build ./...
GOOS=windows GOARCH=amd64 go build ./...
go tool mygo build -platform darwin/universal
```

在 Windows runner/实机继续：

```sh
go test ./... -race -count=1
go vet ./...
go tool mygo build -platform windows/amd64
```

发布前检查：

- macOS `build/darwin-universal` 和 Windows `build/windows-amd64` 的实际文件名；确认 draft release 不多传旧产物，并确认新增 `install.sh` / `install.ps1` 是否符合发布预期。
- 配置签名 key 的构建能生成更新 archive、`update-<platform>.json`、delta 包和签名（含 #941 的大型 App delta 场景）；不能只用本报告中的 unsigned build 代替。
- Updater 经 `netproxy` 的直连、系统代理/PAC、`HTTP(S)_PROXY`/`NO_PROXY` 和代理失败路径均有预期结果，manifest、完整包和 delta 下载都要覆盖。
- `go tool mygo` 打包后 terminal 仍包含并加载匹配的 `libghostty-vt`。

### 5.3 必须人工回归的 UI 点

1. **窗口与生命周期**：首次启动首帧；Cmd+Q、tray 退出、关闭到托盘、第二实例唤醒、全局快捷键；确认退出后 nginx 不残留。
2. **Tray/Flyout**：左右键菜单、打开/关闭面板、多显示器位置、主窗口隐藏/恢复。
3. **Theme**：light/dark/system 首帧与运行时切换、sidebar Vibrancy/Mica、Windows resize/缩小时边缘颜色。
4. **输入与菜单**：日志 SearchField、AI 设置、表单和 chat Submit；输入及非输入焦点下 Copy/Paste/Undo/Select All，确认 #889/#971 没有破坏既有交互。
5. **Dialogs**：备份导入/导出、nginx/access/app log 导出、文件解读选择器、取消/Escape；这些真实系统 dialog 不被 mock e2e 完整覆盖。
6. **映射列表与弹出层**：多选、拖排、右键菜单、文件拖入、workspace chips popup、dialog 焦点与滚动。
7. **Terminal**：shell/probe/serial 三类连接，滚动倍率、滚动条、Shift-click 选区、窗口 resize、退出/重开和打包后的动态库加载。
8. **AI/Markdown 与 diff**：流式回答中的代码块和 `$…`、完成后的正文、snapshot diff 长行和 hunk 区域滚动。
9. **真实更新**：从已安装的 vA 升到 vB，验证签名、manifest、完整包/delta 下载、安装/重启、Dock tile、nginx 子进程与转发状态；覆盖直连、系统代理/PAC、`HTTP(S)_PROXY`/`NO_PROXY` 和代理失败；Windows 额外验证 tray App 在 installer 升级前被关闭。

## 6. 风险与是否建议立刻升级

### 主要风险

- **平台覆盖不完整**：本报告只有 macOS 环境测试/打包和 Windows 交叉编译证据；#893、#913、#895 等 Windows native UI 修复也改变了渲染路径，必须跑 Windows CI 和人工冒烟。
- **渲染行为可能变化而编译不变**：项目直接定制 `ui.Theme`，并对像素、坐标、Vibrancy 有断言。虽然当前 e2e/race 全过，真实窗口的材质、首帧、菜单和 dialog 仍需目测。
- **Terminal 是 Go API 之外的二进制耦合**：`internal/ui/terminal.go:272-275` 明确依赖 mygo 打包的 `libghostty-vt`；普通 unit test 通过不能替代真实终端冒烟。
- **发布产物集合发生变化**：#748 新增安装脚本，#769 改 installer/DMG 名称。现有 wildcard 能上传它们，但外部固定链接、下载说明或更新预期要核对。
- **真实 updater 尚未验证**：本地打包未使用私钥，不能证明签名 archive、manifest、delta 和 vA→vB 更新链路正常；#799 引入的系统代理/PAC 行为也需覆盖直连与代理成功/失败。
- **当前工作区有较多未提交改动**：本报告验证包含这些改动。若直接在同一工作区升级，后续失败会难以区分是 AI/UI 改动还是 mygo 升级导致。

### 建议

**建议升级，但以独立 PR/分支落地，并在下一次发布前完成上述平台回归。** 当前没有需要预先重写 JumpBoard 代码的阻塞项，也没有必须留在 v0.6.4 的上游兼容理由。

不建议“立刻合入并马上发布”的唯一实际理由是验证边界：Windows 实机、真实签名更新、真实终端和系统 dialog 尚未验收。若当前正处于 AI 功能集中开发期，先提交或隔离现有改动，再升级，能显著降低定位成本。

## 7. 回滚

若升级独立且尚未叠加其他改动，回滚只需恢复 `go.mod` / `go.sum` 到 v0.6.4 并重新执行：

```sh
go mod tidy
go test ./... -count=1
```

不要在没有核对工作区的情况下用 `git reset --hard` 或整仓恢复；本次报告未修改依赖文件，无需回滚仓库。
