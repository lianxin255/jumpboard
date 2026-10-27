# Spike：应用内终端（2026-10-26）

## 结论：有条件 go

v1（在面板里跑本地 shell，或把任意 `io.ReadWriteCloser` 接成会话）**可行，且落在 2–3 天预算内**。mygo v0.6.4 自带完整终端插件（`plugins/terminal`），不需要 webview、xterm.js 或自研 VT 网格。

条件只有一个：接受约 1–2 MB 的预编译原生库 `libghostty-vt` 随包分发（macOS 最低系统版本从默认升到 13.0，Windows 要求 10 1809+）。不接受这个条件则结论翻转为 no-go——自研渲染路径要数周。

## 依据

1. **mygo 已有终端，不是半成品。** `github.com/egoist/mygo@v0.6.4/plugins/terminal`（Go 插件，无 npm 包）：`terminal.New` + `terminal.View(c, term).Fill().AutoFocus()` 就是一个面板（`examples/terminal/main.go`，`docs/plugins/terminal.md`）。覆盖 16/256/24-bit 色、宽字符与 emoji、框线自绘、回滚重排、备用屏幕、Kitty/modifyOtherKeys 键盘协议、鼠标上报、OSC 8 链接、选区复制、IME。`ui/editor` 是代码编辑器，不适合当终端；有了插件也不需要走 `ui.Canvas`。

2. **PTY 全平台纯 Go，不碰 cgo。** `plugins/terminal/internal/pty`：darwin/linux 用 `/dev/ptmx` + ioctl（`pty_darwin.go` 自带 macOS 的 `TIOCPTYGRANT` 常量，不依赖 cgo 的 creack/pty）；Windows 用 `syscall.NewLazyDLL("kernel32.dll")` 调 `CreatePseudoConsole` 等 ConPTY API（`pty_windows.go`），失败时返回 "ConPTY needs Windows 10 1809 or later"。不需要 purego 做 ConPTY。

3. **VT 解析与网格渲染也是纯 Go 加载。** 模拟器是 Ghostty 的 libghostty-vt（commit `befcdfd2c3a1`），经 purego 运行时加载（`plugins/terminal/internal/vt/ffi.go`、`open_unix.go`）。`ebitengine/purego v0.11.1` 已在本仓库 go.sum 里，正是这个用途（mygo 的 terminal / sqlite / gamepad 插件共用）。`mygo-plugin.json` 列出 darwin-amd64/arm64、windows-amd64/arm64 的预编译库与 sha256，CLI 按 `go list -deps` 自动发现并打进 app（`cmd/mygo/natives.go`），`go run` 时下载到用户缓存并校验。打包后的 app 不联网。

4. **不破坏现有构建约束。** `docs/distribution.md`：`mygo build` 默认 `CGO_ENABLED=0`，任意主机交叉编译。本仓库 `Makefile` 的 `build-win` 是 `mygo build -platform windows/amd64`，`internal/store/store.go` 选 `modernc.org/sqlite` 的注释写明 "no cgo, cross-compiles everywhere"。终端插件不引入 cgo，这条链路保持不变。

5. **SSH/串口不需要再写 PTY。** `Options.Conn` 接任意读写端，带 `Resize(cols, rows)` 的还会收到尺寸（`docs/plugins/terminal.md` "Without a program"）。`Mirror`/`Snapshot` 甚至支持服务端持有会话、多窗口附着。

6. **可测。** `ui.NewTester` 无窗口驱动终端视图（`docs/plugins/terminal.md` "Testing"），与本仓库 e2e 风格一致。

## 若做：v1 范围与依赖

v1（约 2 天）：新增 `internal/terminal` 薄封装 + 一个面板视图；`terminal.New(terminal.Options{Command, Dir, Env, OnExit})` 跑本地 shell，或 `Conn` 接一个「对转发端口发原始 TCP/HTTP 探测」的小会话；关闭时 `term.Close()`。依赖仅为 import `github.com/egoist/mygo/plugins/terminal`（库由 CLI 自动打包，无 go.mod 变化、无 cgo）。配置改动一处：`mygo.json` 加 `"macos": {"minimumSystemVersion": "13.0"}`（`examples/terminal/mygo.json` 同款）。

v2（另计，不在本预算）：多标签会话、串口（`Conn` 包 `golang.org/x/sys` 的 serial）、经转发端口的 SSH（`Conn` 包 `golang.org/x/crypto/ssh`）。插件侧能力已齐，工作量在会话管理与凭据，不在终端本身。

## 风险

- **体积与平台下限**：每架构 1–2 MB 原生库；macOS < 13 与 Windows < 1809 的用户失去终端（应用其余功能不受影响，可运行时探测降级）。
- **库版本锁定**：库与插件 ABI 绑定 Ghostty commit，升级 mygo 时随插件走，不要自行替换 `MYGO_GHOSTTY_VT`。
- **安全面**：终端即本地任意命令执行。默认拒绝程序读剪贴板（`ClipboardRead` 默认 `ClipboardDeny`），保持默认；不要把远端输入直接送进本地 shell。
- **签名**：macOS 上库随 app 一起签名（CLI 处理），公证流程无需额外步骤，但首次打包应实机验证。

## 若不做：替代方案

- 端口验证不必上终端：对转发端口做 TCP dial / HTTP GET 探测，结果写进现有日志面板即可，工作量以小时计。
- 交互式会话交给系统终端：macOS `open -a Terminal`/`iTerm`，Windows `wt.exe`/`conhost`，用 `exec.Command` 带上 `ssh -p <转发端口> user@127.0.0.1` 启动。零依赖、零体积，代价是离开应用窗口。
- 若只是不想要原生库：自研「PTY + VT 解析 + ui.Canvas 网格」在 no-cgo 下可行但远超 2–3 天（选区、宽字符、IME、鼠标上报每一项都是单独的坑），不建议。
