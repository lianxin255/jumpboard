# NginxJump

> 内网跳板机 nginx 管理软件 · Tauri 桌面壳 + Node sidecar 后端

[![Test](https://img.shields.io/badge/tests-187%20passed-brightgreen)](#测试)
[![Lint](https://img.shields.io/badge/lint-0%20errors-brightgreen)](#测试)
[![License](https://img.shields.io/badge/license-MIT-blue)](#license)

NginxJump 是一个面向 Windows 的 **nginx 跳板机管理工具**。在内网一台 Windows 机器上运行内置 nginx，通过 GUI 手动维护端口映射规则（TCP/UDP 端口转发 + HTTP/HTTPS 反向代理），将内网服务端口暴露给当前网络可达的外部机器访问。绿色版：单文件夹、免安装、退出无痕。

## 特性

- **TCP/UDP 端口转发** + **HTTP/HTTPS 反向代理** 一站式管理
- **内置 nginx**：双击即用，无需额外安装
- **绿色便携**：单文件夹分发，不写注册表、不进 AppData
- **退出无痕**：关闭应用立即停止 nginx，不留残留进程
- **远程管理**：SSH 连接管理其他主机上的 nginx 实例
- **配置快照与回滚**：每次应用配置自动存快照，支持一键回滚
- **健康检查**：定时探测映射目标可达性
- **白名单**：IP 访问控制（allow/deny）
- **数据导入导出**：完整备份/迁移配置
- **安全审计**：导入数据消毒，防 reloadCmd 注入 / nginxPath 篡改 / conf 注入

## 架构

```
apps/desktop (Tauri 2 Rust 壳)
  ├─ spawn Node sidecar + 托盘 + 单实例锁
  └─ WebviewWindow 指向 127.0.0.1:PORT
packages/server (Node 后端)
  ├─ HTTP server (node:http) + 静态文件托管
  ├─ nginx 进程管理 + 配置生成 + 快照
  └─ esbuild 打成单文件 server.cjs（零 node_modules）
packages/shared (类型 + 纯函数)
  └─ 类型定义 + 校验 + 路径工具
apps/web (Vue 3 前端)
  └─ Pinia stores + 组件 + HTTP API 客户端
```

**工作原理**：Tauri Rust 壳启动时 spawn 一个 Node 子进程（打包后的 `server.cjs`），通过环境变量注入数据目录、资源目录、端口等。Node sidecar 提供 HTTP API + 静态文件托管，前端通过 `fetch` 调用 `127.0.0.1:PORT` 上的 API。退出时 Rust 壳杀掉 sidecar，sidecar 收到 SIGTERM 后优雅停止 nginx。

## 快速开始

### 前置要求

- Node.js >= 22
- pnpm >= 9
- Rust（可选，仅打包桌面应用需要）

### 开发

```bash
pnpm install

# 本地开发：web + server 分别启动
pnpm dev:web    # Vite dev server (http://localhost:5173)
pnpm dev:server # Node sidecar (http://127.0.0.1:47831)

# 测试
pnpm test           # 全部（单元 + e2e）
pnpm test:server    # 仅后端
pnpm test:web       # 仅前端

# 代码检查
pnpm lint
pnpm typecheck
```

> e2e 测试需要先构建 sidecar bundle：`pnpm app:build:resources`

### 打包（Windows 绿色版）

```bash
pnpm app:build
```

产出在 `release/` 目录，包含：
- `NginxJump.exe` — Tauri 壳
- 内嵌 Node 运行时 + nginx + better-sqlite3 原生模块 + 前端静态文件

### macOS 交叉编译 Windows 绿色版

macOS 上可以通过 `cargo-xwin` + `llvm` 交叉编译 Windows exe，无需 Windows 机器：

```bash
# ① 前置（一次性）
brew install llvm
cargo install cargo-xwin
rustup target add x86_64-pc-windows-msvc

# ② 预下载构建素材（Node + better-sqlite3）
pnpm app:build:resources

# ③ 交叉编译 + 打绿色版文件夹
pnpm app:build:win
# 产出 release/NginxJump/ 文件夹，直接拷到 Windows 双击运行
```

### 内网离线打包

nginx.exe 已内置在 `resources/nginx/`（4.5MB，随仓库走）。Node 运行时和 better-sqlite3 原生模块需要在有网环境预下载一次，之后内网机器可离线打包：

```bash
# ① 有网的机器上预下载 Windows 依赖（不触发 tauri build）
pnpm app:build:resources
# 产出：
#   build/node/node.exe           — Node.js win-x64 运行时（~77MB）
#   build/app/better_sqlite3.node  — better-sqlite3 原生模块（~1.8MB）
#   build/app/server.cjs           — esbuild 打包的后端（~430KB）
#   build/app/templates/builtin/    — 内置模板

# ② 把整个项目目录（含 build/）拷到内网 Windows 机器

# ③ 内网机器上离线打包（跳过所有网络下载）
pnpm app:build:offline
```

> `--offline` 模式下如果缺少预构建文件，会报清晰错误提示，不会尝试联网下载。

## 项目结构

```
nginx-jump-gui/
├── packages/
│   ├── shared/          # @ngj/shared — 类型 + 校验 + 路径
│   └── server/          # @ngj/server — HTTP API + nginx 管理 + DB
├── apps/
│   ├── web/             # @ngj/web — Vue 3 前端
│   └── desktop/         # Tauri 2 Rust 壳
├── resources/
│   └── nginx/           # 内置 nginx 二进制 + mime.types
├── scripts/
│   ├── build-app.mjs    # esbuild + web build + tauri build
│   ├── fetch-node.sh    # 下载 win32 Node 运行时
│   └── fetch-better-sqlite3.sh
└── docs/                # 设计文档 + 开发计划
```

## 测试

- **187 个测试全部通过**（176 单元 + 11 端到端）
- e2e 测试启动真实的 esbuild bundle `server.cjs`，通过 HTTP 验证完整 nginx 生命周期：
  host seed → mapping CRUD → engine test/start/reload/status → snapshot rollback → whitelist → data export → SIGTERM 退出无残留
- 测试框架：vitest（antfu 同目录测试约定）
- Lint：`@antfu/eslint-config`
- 类型检查：tsc + vue-tsc

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面壳 | Tauri 2 (Rust) |
| 后端 | Node.js 22 + node:http + better-sqlite3 + ssh2 |
| 前端 | Vue 3 + Pinia + Vite |
| 构建 | esbuild (server) + Vite (web) + Tauri CLI (desktop) |
| 测试 | vitest |
| 规范 | @antfu/eslint-config |

## License

MIT
