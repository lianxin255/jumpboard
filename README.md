# NginxJump

内网跳板机的 nginx 管理工具：在一台机器上跑内置 nginx，用图形界面维护端口转发与反向代理规则，把内网的数据库、Web、SSH 等服务的端口暴露出去。

**Go 单二进制 + 原生界面**：没有 Electron，没有 Chromium，没有 Node，没有前端构建。界面由 [mygo](https://github.com/egoist/mygo) 用 GPU 直接绘制（macOS 用 Metal，Windows 用 Direct3D 11），中英文都支持，亮色/暗色两套主题。

| | 旧版（Electron） | 现在 |
|---|---|---|
| 体积 | 160 MB 目录 | mac 42 MB（含 Intel + Apple silicon）/ Windows 16 MB |
| 运行时 | Node + Chromium | 无 |
| 平台 | mac arm64 / win x64 | mac 通用 / win x64 |
| 界面代码 | Vue 4314 行 | Go 约 2600 行 |
| 后端代码 | TypeScript 3356 行 | Go 约 3600 行 |

旧版说明见 [docs/README-electron-legacy.md](docs/README-electron-legacy.md)。

## 功能

- **TCP / UDP 端口转发**与 **HTTP / HTTPS 反向代理**
- **内置 nginx**：双击即用，无需另外安装；退出时停止，不留残留进程
- **绿色便携**：数据在程序旁的 `data/` 目录，拷走即用
- **配置快照与回滚**：每次成功应用存一份，可回看、可回滚
- **健康检查**：定时探测上游可达性，表格里直接看状态
- **IP 白名单**：allow/deny 注入到每个 server 块；开着但没配置 IP 时拒绝应用，不会把自己锁在外面
- **模板**：内置 13 个（MySQL / Redis / MongoDB / PostgreSQL / MQTT / RDP / SSH / WebSocket / Web…），可自建
- **导入导出**：备份全量配置；导入时逐条消毒，恶意备份无法注入 nginx 指令
- **亮色 / 暗色主题**：默认跟随系统
- **AI 配置**（可选）：用一句话描述要开的映射，模型给出草稿，你确认后才会保存

## 快速开始

需要 Go 1.27 或更新版本。

```sh
make build      # 产出 mac 通用包与 Windows exe
make test       # 单元 + 界面 e2e + 真 nginx 转发测试
make shots      # 无窗口渲染所有页面截图（亮/暗）
```

单平台：

```sh
go tool mygo build -platform darwin/universal   # 或 windows/amd64
```

产物在 `build/<platform>/`：

| 平台 | 文件 |
|---|---|
| macOS | `NginxJump.app`、`NginxJump 1.1.0.dmg` |
| Windows | `NginxJump.exe`、`nginx/`、`icon.png` |

## 数据在哪

数据跟着程序走，方便整个目录拷贝迁移：

| 场景 | 数据目录 |
|---|---|
| Windows 绿色版 | `<exe 目录>/data` |
| macOS | `<NginxJump.app 的父目录>/data`（不写进 .app，升级不丢） |
| 开发 / 测试 | 工作目录下的 `data`，或 `NGJ_DATA_ROOT` |

如果程序目录不可写（例如装在 `/Applications` 且当前用户不是管理员），会自动回落到用户目录，并在设置页显示实际位置。

目录内容：

```
data/
├── data.db              SQLite：映射、快照、日志、白名单、设置
├── nginx-home/          nginx 的 -p 目录
│   ├── conf/nginx.conf  由应用生成，每次应用时重写
│   ├── conf/mime.types
│   ├── logs/            access.log / error.log / stream_access.log
│   └── temp/            pid 与临时文件
└── templates/custom/    自定义模板
```

旧版（Electron）的 `data/data.db` 可以直接被读取，schema 完全一致，无需迁移。

## AI 配置（可选）

在 **设置 → AI 配置** 里填接口地址（任何 OpenAI 兼容的服务，包括本地网关）、模型名和 API Key。

- Key 存在系统凭据库（macOS 钥匙串 / Windows 凭据管理器），**不进数据库、不进导出文件**
- 只在你说「AI 建映射」时，把你输入的那句话发出去；当前映射列表不会被上传
- **AI 只生成草稿**：草稿填进普通表单，保存时走和手填完全一样的校验，再经过 `nginx -t` 与失败回滚
- 没配置时，映射页不会出现 AI 入口

## 开发

```
main.go                     启动、单实例、退出钩子
internal/model/             纯类型与校验（端口、IP/CIDR、proxy_pass、注入字符）
internal/store/             SQLite：schema 与原 Electron 版一致 + 各表读写
internal/nginx/             配置生成（含 golden 测试）、引擎（起停/reload/回滚）、端口探测
internal/services/          健康检查、日志尾部读取、模板、导入导出、操作日志
internal/appt/              应用状态与动作：界面与数据之间的那一层
internal/ui/                mygo 原生界面：4 个页面 + 弹层 + 亮暗主题 + AI 面板
internal/ai/                OpenAI 兼容客户端 + 草稿校验
e2e/                        端到端测试（界面 / 真 nginx / AI / 数据）
cmd/shots/                  无窗口渲染截图
resources/                  图标与各平台的内置 nginx
```

### 测试

四层，都能单独跑：

```sh
make test          # 全部
make test-ui       # 界面 e2e（用 ui.Tester，不需要窗口，锁屏也能跑）
make test-engine   # 真 nginx：起停、转发、回滚、退出清理
make test-race     # 全量竞态检测
make cover         # 覆盖率
```

**界面测试用 `ui.Tester` 而不是真实窗口**：屏幕锁定时真实窗口不出帧（`Update` 和 `CapturePage` 都无效），依赖它的测试会静默失败。

### 内置 nginx

| 平台 | 位置 | 说明 |
|---|---|---|
| macOS | `resources/darwin/nginx/nginx` | 自编通用二进制（arm64 + x86_64），OpenSSL 与 PCRE2 静态链接，`otool -L` 只剩系统库 |
| Windows | `resources/windows-amd64/nginx/nginx.exe` | 官方 1.26.3。官方 Windows 版是 32 位，在 x64 上走 WOW64，只依赖系统 DLL（无 VC 运行库） |

重建 macOS 通用 nginx：

```sh
bash scripts/build-nginx-universal.sh
```

## License

MIT
