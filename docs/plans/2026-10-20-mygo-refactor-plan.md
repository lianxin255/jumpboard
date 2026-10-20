# NginxJump 用 mygo 完全重写：Electron + Vue → Go 单二进制 + 原生 UI

> 规划日期：2026-10-20（二稿：改为 mygo 原生 UI，删除远程 SSH）
> 分支：`refactor/mygo-go`
> 目标：macOS + Windows 双端可用，绿色免安装，一套 Go 代码画出全部界面，支持亮/暗双主题。

---

## 一、结论与范围

**完全重构：不留 Vue、不留 Node、不留 bundle 步骤。**
界面用 mygo 的 `ui` 包在 Go 里直接画（Metal / Direct3D 11 渲染），后端逻辑同在一份 Go 代码里，最终产物是单个可执行文件。

| 项 | 原方案 | 新方案 |
|---|---|---|
| 界面 | Vue 3 + Pinia + Vite（4314 行） | Go `ui` 包（约 3000 行，含 5 个页面） |
| 后端 | Node + 45 个 IPC handler（3356 行） | Go 服务层（约 4000 行） |
| 壳 | Electron 38 + Chromium | mygo（系统 WebView 都不用，GPU 自绘） |
| 前端构建 | electron-vite + Vite 8 | 无 |
| 运行时依赖 | Node + Chromium + better-sqlite3 | 无（纯 Go，CGO_ENABLED=0） |
| 产物 | 160 MB 绿色目录 | **mac 通用包 28 MB / Windows 14 MB**（Go 9.5 MB + nginx 4.5 MB，无 Chromium） |
| 内存 | ~250 MB | 约 50~80 MB |
| 远程 SSH | 有 | **删除**（按你的决定：只做本地） |
| 主题 | 单一暗色 | **亮色 + 暗色 + 跟随系统** |
| 目标架构 | win x64 / mac arm64 | **Windows amd64 + macOS universal（arm64 + x86_64）** |

已实测（macOS arm64 本机）：
- `mygo build -platform windows/amd64` 无需 Wine，1.8s 产出 exe；
- `ui.Render` 与真实窗口渲染一致，中文字体、`ui.Table`、`ui.Modal`、`ui.Segmented` 全部正常；
- 关闭 Vite / Node / package.json 后 `go tool mygo build` 仍能产出 `.app` + `.dmg`；
- 带 `modernc.org/sqlite` + `yaml.v3` 的 release 构建：mac 9.2 MB、Windows 9.5 MB；
- 内置 nginx 从 `.app/Contents/Resources/nginx/nginx` 启动、`nginx -t`、reload、停止全部通过，嵌套二进制自动签名。
- **自编 macOS 通用 nginx**（OpenSSL + PCRE2 静态链接）：`lipo` 双架构、`otool -L` 只剩系统库、两个 slice 都跑通、放进 universal `.app` 后 deep-verify 通过。
- **Windows 官方 nginx.exe 依赖审计**：仅 `KERNEL32/USER32/ADVAPI32/WS2_32/CRYPT32`，无 VCRUNTIME。
- **mygo 目标架构实测**：`windows/amd64` ✅、`darwin/amd64` ✅、`darwin/universal` ✅（产 24 MB app）、`windows/386` ❌（mygo 不支持）。

界面预览（同一份 Go 代码渲染，暗/亮两套 token）：

| 映射列表（暗） | 新建映射（暗） |
|---|---|
| ![](assets/2026-10-20-native-mapping-dark.png) | ![](assets/2026-10-20-native-form-dark.png) |

| 设置（亮） | 设置（暗） |
|---|---|
| ![](assets/2026-10-20-native-settings-light.png) | ![](assets/2026-10-20-native-settings-dark.png) |

---

## 一·五、平台与架构矩阵（关键结论）

### 你的机器

| 机器 | 目标三元组 | 可执行 | 说明 |
|---|---|---|---|
| Windows x64 | `windows/amd64` | ✅ 交叉编译实测通过（macOS 上 1.8 s 出 exe） | mygo 官方支持 |
| macOS Apple silicon | `darwin/arm64` | ✅ | 本机验证 |
| macOS Intel | `darwin/amd64` | ✅ 编译通过；`darwin/universal` 已实测出通用包 | 见下方 nginx 说明 |

**mygo 只支持 amd64 / arm64**：`GOOS=windows GOARCH=386` 编译直接失败（`internal/text` 等包按 `windows && (amd64 || arm64)` 限定）。你的 Windows 是 64 位，不受影响；这也意味着**不再需要为 32 位 Windows 做任何兼容**。

### 内置 nginx 能不能跨平台用？—— 分两层看

**1. Go 侧（程序本体）：完全跨平台。** 一份代码，`mygo build -platform darwin/universal,windows/amd64` 同时产出两个平台，无需 Wine、无需目标机器。

**2. nginx 二进制：必须按平台/架构各备一份，但 mygo 的 `resources` 已经把这个机制做好了。**

| 目标 | 目录 | 现状 |
|---|---|---|
| Windows x64 | `resources/windows-amd64/nginx/nginx.exe` | ✅ 已有（官方 1.26.3，**PE32 = 32 位**，在 x64 Windows 上通过 WOW64 运行，完全正常；官方 Windows 版 nginx 从来只有 32 位） |
| macOS arm64 | `resources/darwin/nginx/nginx`（通用包） | ✅ 已验证：单文件同时含 arm64 + x86_64 slice |
| macOS x86_64 | 同上（通用包的另一 slice） | ✅ 已实测：Rosetta 下 `arch -x86_64` 跑通，`stream` 监听正常 |

**关键决策：macOS 用「一个通用 nginx」而不是两份。**
- 若放 `darwin-arm64/` + `darwin-amd64/` 两份，mygo 会用 `lipo` 合成通用二进制；实测可行，但**两侧的 `lib/*.dylib` 必须也成对**，否则合成后 arm64 一侧找不到库（我第一次就踩了这个坑：`Library not loaded: @executable_path/lib/libpcre2-8.0.dylib`）。
- 更省事的做法：**自己静态编译一份通用 nginx**（OpenSSL + PCRE2 静态链进去），只放 `resources/darwin/nginx/nginx`。这样没有任何 `.dylib` 依赖，`otool -L` 只剩 `libz` 和 `libSystem`，也不会有两侧 dylib 不匹配的问题。

**我已经把这份通用 nginx 编出来了**（本次实测）：
```
lipo -info → x86_64 arm64
otool -L   → 仅 /usr/lib/libz.1.dylib, /usr/lib/libSystem.B.dylib
模块       → 21 个 stream 模块，与官方 Windows 构建完全一致（含 ssl_preread、realip）
运行验证   → arm64 与 x86_64 两个 slice 都跑通；http 返回、stream 监听、-t 校验、quit 全部正常
打包验证   → 放进 resources/darwin 后 universal .app 内为通用二进制，deep-verify 通过
```
构建配方（一次性，产物入库）：
```sh
# OpenSSL + PCRE2 各编两套静态库，然后
./configure --with-stream --with-stream_ssl_module --with-stream_ssl_preread_module \
  --with-stream_realip_module --with-http_ssl_module \
  --with-openssl=<openssl-src> --with-openssl-opt="no-shared no-tests" \
  --with-cc-opt="-arch <arch> -I<静态 pcre2>/include" \
  --with-ld-opt="-arch <arch> -L<静态 pcre2>/lib -lpcre2-8"
lipo -create nginx-arm64 nginx-x86_64 -output nginx
```

### Windows 侧要不要也换成 64 位 nginx？

**不需要，但可以顺手做。** 理由：
- `nginx.exe` 是 PE32（32 位），imports 只有 `KERNEL32/USER32/ADVAPI32/WS2_32/CRYPT32` —— **全是系统 DLL，没有 VCRUNTIME**。这正是当年从 Tauri 切回 Electron 时踩的坑（VCRUNTIME 缺失），mygo 方案天然没有这个问题。
- 32 位 nginx 在 x64 Windows 上走 WOW64，功能与性能对本场景（几百条端口转发）无差别。
- 若坚持要 64 位：可以自己用 MinGW/交叉工具链编一份 `windows/amd64` 的 nginx，放 `resources/windows-amd64/nginx/`（与 Go 程序同架构）。作为 B6 的可选项。

## 二、界面设计

### 2.1 信息架构（4 个一级页面）

去掉「主机」页（没有远程了），本地网关配置并入「设置」：

| 页面 | 内容 |
|---|---|
| **映射** | 状态条（引擎/ PID / 应用配置 / 新建）+ 表格（规则·协议 / 转发关系 / 健康检查 / 启用 / 最后修改 / 操作）|
| **日志** | 段控切换「操作日志 / nginx 日志」，列表 + 级别过滤 + 清空 |
| **历史** | 快照列表，查看 / 回滚 |
| **设置** | 外观（主题三选一）/ 访问控制（白名单）/ 数据管理（导入导出、清空）/ 网关路径 |

表单用 `ui.Modal` 弹层（已实测：`ui.Form` 的 macOS 风格左标签对齐在原生 UI 里表现很好）。

### 2.2 主题

**亮 + 暗双主题，默认「跟随系统」**，设置页三选一（跟随系统 / 亮色 / 暗色），存 `app_settings`。

实现方式：把现有 `tokens.css` 的调色板逐条翻成 Go 的 `ui.Color`，两套（暗色照搬现值；亮色是新的，保持同一套语义层级）。

```go
type tokens struct {
    dark                                             bool
    base, topbar, surface, surface2, surface3        ui.Color // 背景层级
    fg, fg2, muted, hair, borderSoft                 ui.Color // 文字 / 边框
    accent, accentText, success, warning, danger     ui.Color
    tcp, udp, http, https                            ui.Color // 协议四色
}
```

每帧开头按偏好选一套 token 并 `c.SetTheme(...)`（mygo 每帧会重置主题，所以这是正确做法）。主题值：

| 语义 | 暗色 | 亮色 |
|---|---|---|
| `base` | `#141413` | `#faf9f5` |
| `topbar` | `#0e0e0d` | `#f2f0ea` |
| `surface` | `#30302e` | `#ffffff` |
| `surface2`（悬浮） | `#3d3d3a` | `#f0eee7` |
| `surface3`（输入底） | `#262624` | `#f6f4ee` |
| `fg` | `#faf9f5` | `#1f1e1b` |
| `fg2` | `#b0aea5` | `#55534d` |
| `muted` | `#87867f` | `#86847c` |
| 边框 | `#3d3d3a` | `#d3cfc3` |
| 强调 | `#c96442` | `#c96442` |
| 成功 / 警告 / 危险 | `#5b9a4a` / `#c9a23a` / `#b53333` | `#4d8a3c` / `#a8852b` / `#a52a2a` |
| TCP / UDP / HTTP / HTTPS | `#7aa2c4` / `#9c8ac4` / `#c4a05a` / `#5b9a6a` | `#4f7fa8` / `#7a68a8` / `#a8823c` / `#3f8154` |

圆角统一 3 px（`th.Radius = 3`），沿用原设计稿的极小圆角风格。

### 2.3 状态与刷新

- 界面是「状态 → 视图」的纯函数，mygo 每帧调用。
- 后端耗时操作（起停 nginx、应用配置、健康检查）在 goroutine 里跑，完成后用 `win.Update(func(){...})` 回主线程更新状态并触发重绘。
- 引擎状态/健康状态轮询改成「事件驱动 + 兜底定时」：nginx 进程 `exit` 事件 → `win.Update`；健康检查循环 30 s → `win.Update`。不再需要前端 10 s 轮询。

---

## 三、代码结构

```
nginx-jump-gui/
├── main.go                     # 单实例、路径、窗口、菜单、退出钩子（约 180 行）
├── mygo.json                   # name / identifier / version
├── go.mod                      # go 1.27, mygo v0.5.5, modernc.org/sqlite, yaml.v3
├── internal/
│   ├── model/                  # 纯类型：Mapping / Host / Snapshot / LogEntry / WhitelistEntry / Template
│   ├── store/                  # sqlite 打开 + schema + 6 个 repo + app_settings
│   ├── nginx/
│   │   ├── renderers.go        # server 块渲染（sanitize / safePort / 三种 proxyMode）
│   │   ├── confgen.go          # 完整 conf 组装（http + stream + 白名单）
│   │   ├── engine.go           # 本地引擎：start/stop/reload/test/status/pid
│   │   ├── apply.go            # safeApply（互斥→备份→写→test→reload→回滚）+ 快照
│   │   └── stale.go            # 崩溃遗留 nginx 清理
│   ├── services/
│   │   ├── health.go           # TCP/HTTP/HTTPS 探活 + 30 s 轮询
│   │   ├── portcheck.go        # 监听端口占用检测
│   │   ├── tailfile.go         # 分块反向读日志尾部
│   │   ├── templates.go        # 13 个内置模板（go:embed）+ 自定义
│   │   ├── transfer.go         # 导入导出 + 消毒
│   │   └── logger.go           # 应用日志（写 DB + 裁剪）
│   ├── paths/                  # 绿色版数据根（exe 旁 data/）、resources、内置 nginx
│   └── ui/
│       ├── theme.go            # tokens 亮/暗 + Theme 映射
│       ├── app.go              # 应用状态、生命周期、后台任务接线
│       ├── page_mappings.go    # 映射页 + 表格
│       ├── page_form.go        # 新建/编辑映射弹层
│       ├── page_logs.go        # 日志页
│       ├── page_history.go     # 历史页
│       ├── page_settings.go    # 设置页
│       └── widgets.go          # 状态点、协议徽章、卡片、空状态
├── resources/
│   ├── icon.png
│   ├── darwin/nginx/{nginx,mime.types}          # 通用二进制（arm64+x86_64），无 dylib
│   └── windows-amd64/nginx/{nginx.exe,mime.types}
├── internal/nginx/testdata/    # 从旧 TS 生成的金标准 .conf
└── docs/plans/
```

**删除**：`apps/`、`packages/`、`scripts/`、`pnpm-workspace.yaml`、`package.json`、`tsconfig`、`eslint`、`vitest`、`e2e/*.e2e.test.ts`、`.github/workflows/autofix.yml`（改为纯 Go CI）。

---

## 四、数据存储

- 驱动：`modernc.org/sqlite`（纯 Go，`CGO_ENABLED=0`，双平台交叉编译已验证）。
- schema **逐字照抄**旧 `migrations.ts`（hosts / mappings / config_snapshots / app_logs / ip_whitelist / app_settings + 3 个增量列）。
- 数据目录：
  - Windows：`<exe 目录>/data/`
  - macOS：`<App>.app 的父目录/data/`（不写进包内，避免破坏签名、升级丢数据）
  - 开发：项目根 `./data/`
- 由 `mygo.App.SetPath(mygo.PathUserData, dataRoot)` 统一接管（同时覆盖 cache / logs）。

---

## 五、配置生成（最高风险项）

策略不变：**先固化金标准，再重写**。

1. 用旧的 TS 实现跑出一批 `.conf` 存到 `internal/nginx/testdata/`：
   - 13 个内置模板 × 4 协议；
   - 3 种 `proxyMode`（standard / minimal / custom）× 白名单开/关；
   - TLS、`redirectHttp`、`proxyPassUrl` 自定义、`extraDirectives`、`serverNames`/`listenPath` 组合；
   - 共约 45~60 份。
2. Go 重写 `renderers.go` / `confgen.go`，逐字节比对。
3. 安全函数原样保留：`sanitize`（剔除 `; { } #` 与换行）、`safeName`、`safePort`。

---

## 六、分批计划

> 每批结束跑：`go build ./... && GOOS=windows GOARCH=amd64 go build ./... && go vet ./... && go test ./...`，再做一次 code review、更新本文进度。

### B0 骨架（0.5 天）
`go.mod`、`mygo.json`、`main.go`（单实例、数据目录探测、`SetLanguages` 中文、窗口 1200×760、菜单、启动错误框）、`resources` 平台目录、`go:embed` 模板。
`resources/darwin/nginx/nginx` 通用二进制**已入库**（本次完成），Windows 复用现有 `resources/nginx/nginx.exe`。
**验收**：`go tool mygo build -platform darwin/universal,windows/amd64` 双产物；universal `.app` 内 nginx 为双架构且 deep-verify 通过；双平台 `go build` + `go vet` 全绿；`ui.NewTester` 能跑一个空视图。

### B1 存储层（1 天）
schema + 6 repo + settings。
**验收**：用现有 `data/data.db` 副本打开，条数/内容一致；迁移幂等；CRUD 单测。

### B2 配置生成（1.5 天）
金标准 + `renderers.go` + `confgen.go`。
**验收**：全部 golden 逐字节一致；注入字符用例（`a.com; }`）与旧实现输出相同。

### B3 nginx 引擎（1.5 天）
`engine.go` + `stale.go`：`-p prefix -c conf/nginx.conf -g 'daemon off;'`、500 ms 存活判定、进程树停止（Unix SIGTERM→SIGKILL / Windows `taskkill /T /F`）、reload（Unix SIGHUP / Windows `-s reload`）、`-t`。
**验收**：mac 真机起停 + 真实流量；Windows 交叉编译 + CI 真机跑同一套集成测试。

### B4 应用流水线与服务（2 天）
`safeApply`（`sync.Mutex` 串行化）、快照、健康检查、端口检测、日志尾部、模板、导入导出消毒、应用日志。
**验收**：三条失败路径回滚后 conf 与之前逐字节一致；恶意导入样本全部被清洗。

### B5 界面（3 天，重头）
`theme.go` → `app.go` → 5 个页面 → 弹层表单 → 亮/暗双主题 → 空态/加载态/Toast/确认框。
**验收**：`ui.Render` 逐页出图（亮 + 暗各一套）人工过一遍；`ui.NewTester` 覆盖关键交互（新建映射、启停、切换主题、应用配置）。

### B6 打包与发布（1 天）
- mac：`darwin/universal` → `.app` + `.dmg`，Intel 与 Apple silicon 同一份安装包。
- Windows：绿色 zip（`NginxJump.exe` + `nginx/` 目录 + `mime.types`），首次运行在 exe 旁建 `data/`。
- 可选：`brew install makensis` 后出 NSIS 安装包（`mygo build` 自动带上）。
- 守卫：数据目录可写性探测（不可写回落 `~/Library/Application Support/NginxJump` 并在设置页显示实际路径）、Windows 中文路径检测、启动失败原生错误框。
**验收**：
- mac 在 arm64 与 Intel（Rosetta 或真机）各双击一次，nginx 都起得来；
- Windows 解压即用，`nginx -t` 通过；
- 退出后 `pgrep nginx` / `tasklist` 无残留；
- `codesign --verify --deep --strict` 通过（macOS）。

### B7 清理与回归（1 天）
删旧目录、CI 改 Go、README/CHANGELOG、跨批全面 review。

**合计约 10.5 个工作日。**

---

## 七、风险

| # | 风险 | 对策 |
|---|---|---|
| R1 | 原生 UI 控件覆盖 | 已逐个确认可用：`Table`（列宽拖拽/双击自适应/表头排序/多选/行内按钮，实测行内 `Switch` 与 `Button` 点击有效）、`List`（`Reorder` 拖拽排序，旧 UI 其实没有此功能）、`Modal`、`AlertDialog`、`Toast`、`Segmented`、`Select`、`Combobox`、`SearchField`、`Switch`、`Progress`、`Spinner`、`Badge`、`MenuButton`、`Popover`、`Form`/`Field`、`Scroll`、`ui/editor`（差分查看快照） |
| R9 | 长弹层超出窗口 | 实测：`ui.Modal` 内嵌 `ui.Scroll` 时，16 个字段的长表单可滚动，底部按钮仍可点击（`ui.NewTester` 验证） |
| R10 | 内置 nginx 与 Go 程序架构不匹配 | mac 用自编**通用二进制**（无 dylib 依赖）；Windows 用官方 32 位 exe（WOW64 运行，只依赖系统 DLL） |
| R2 | 中文字体/排版 | 已实测：走系统文字栈（Core Text / DirectWrite），中文正常 |
| R3 | 配置生成与旧实现不一致 | golden 逐字节比对，旧 TS 保留到 B7 才删 |
| R4 | Windows 上没有 WebView2 的问题 | **不存在**——原生 UI 不启动 WebView，无需 WebView2 运行时 |
| R5 | 旧 SQLite 数据兼容 | schema 逐字照抄 + 真实库回归 |
| R6 | 主题切换时控件颜色不同步 | 每帧 `c.SetTheme`，mygo 本身每帧重置主题，语义正确；关闭系统即时跟随由 `mygo.Theme.OnUpdated` 触发重绘 |
| R7 | 巨大表格性能 | `ui.Table` 只构建可见行，10 万行仍是常数开销 |
| R8 | 现有 `data/` 目录迁移 | 首次启动检测旧数据（`data/data.db`）→ 直接复用，不需迁移 |

---

## 七·五、架构自审（哪些地方最容易出问题）

按「会真的疼」排序，不是罗列：

### A1. 单窗口 + 全局状态 + 后台 goroutine 的竞态

原生 UI 的视图在主线程跑，但 nginx 起停、健康检查、日志读取都在 goroutine 里。**所有跨线程写入必须走 `win.Update(fn)`**，绝不能直接改 `app.rows`。`mygo vet` 能查出把 build 值存进结构体，但**查不出数据竞态**——B4/B5 要配 `-race` 跑集成测试。

### A2. 配置应用与引擎状态的耦合

旧的 `safeApply` 串行化 + 回滚逻辑是关键正确性资产，Go 版必须逐条等价：
- 互斥（`sync.Mutex` 而非 Promise 链）
- 失败三阶段（`io` / `test` / `reload`）分别回滚
- 停止态只写+校验不 reload
一个偷懒的写法（比如用 `sync.Once` 或全局状态标志）会让「连续点两次应用配置」产生半写文件。

### A3. 视图重建的性能陷阱

原生 UI 每帧重建整个视图树。虽然 `ui.List`/`ui.Table` 只构建可见行，但**在 view 里做 I/O（读 DB、读文件）会拖垮帧率**。约定：view 只读内存快照，所有数据刷新走 `win.Update`。这条要写进 code review checklist。

### A4. 主题的每帧重置

`c.SetTheme` 的生效范围是「当前帧」——`context.reset` 每帧会把 `c.theme` 重置成系统主题。所以**必须在 view 最开头调用**，不能只在「切换主题时」调用一次。反过来也意味着：跟随系统模式天然正确，系统主题变化时 mygo 会自动重绘。

### A4·5. 锁屏 / 无活跃显示时不出帧（实测踩到）

本机屏幕处于锁屏 + 屏保状态时，`win.CapturePage()` 与 `win.Update()` 都不会触发新帧——`view` 只跑一次，之后停在第一帧。这解释了为什么我用真实窗口截的弹层图是空的。

**结论**：**视觉回归一律走 `ui.Render(view, w, h, scale)` 或 `ui.Tester.Image()`**（无窗口，不受锁屏影响，且与真实渲染像素一致），真实窗口截图只作为人工抽查手段。这条要写进 B5 的验收方式。

### A5. 中文与控件内建文案

`ui` 自带文案（Search / Clear / 日期月份等）默认英文。要中文必须：
```go
import _ "github.com/egoist/mygo/locales/zh-Hans"
...
mygo.App.SetLanguages("zh-Hans", "en")  // 必须在窗口创建前
mygo.App.SetLocale("zh-Hans")           // 或让用户选
```
**实测踩坑**：只 `SetLocale` 不 `SetLanguages` 不生效（`Search` 仍是英文）；必须先声明语言集。另外 `ui.Render`（无窗口）不加载语言，验证要以真实窗口为准。

### A6. 数据目录与「绿色版」

`SetPath(PathUserData, ...)` 会连带影响窗口位置状态的 `window-state.json`、日志、缓存目录——这是我们想要的（全部收进 `data/`）。但 mac 上如果 app 装在 `/Applications` 且当前用户不是管理员，`/Applications` 不可写。方案：
1. 优先 exe/app 同级的 `data/`；
2. 启动时探测可写性，不可写则回落到 `~/Library/Application Support/NginxJump` 并在设置页显示实际路径（避免静默失败）。

### A7. 现有的旧数据

旧 Electron 版把数据放在 `<exe>/data/`（Win）或 `.app` 同级 `data/`（mac）。新版本路径规则一致，**首次启动应直接认到旧库**。B1 的验收里用真实旧库跑一遍；同时准备一个「迁移提示」——如果检测到旧库 schema 缺列，走增量迁移而不是重建。

---

## 八、进度

- [x] 现状盘点、mygo 能力与构建链验证
- [x] 原生 UI 原型（映射 / 表单 / 日志 / 历史 / 设置，亮暗双主题）渲染通过
- [x] 规划文档（本文）
- [ ] B0 骨架
- [ ] B1 存储层
- [ ] B2 配置生成
- [ ] B3 nginx 引擎
- [ ] B4 应用流水线与服务
- [ ] B5 界面
- [ ] B6 打包与发布
- [ ] B7 清理与回归
