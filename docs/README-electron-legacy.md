# NginxJump

> 内网跳板机 nginx 管理软件 · Electron 桌面壳 + IPC 架构

[![Test](https://img.shields.io/badge/tests-176%20passed-brightgreen)](#测试)
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
apps/desktop (Electron 33 壳)
  ├─ main 进程：窗口 + 生命周期 + IPC handler 注册 + nginx 进程管理
  ├─ preload：contextBridge 暴露类型安全的 invoke/on
  └─ renderer：Vue 3 前端（从 apps/web 合并）
packages/server (业务逻辑层)
  ├─ ipc.ts：registerIpcHandlers(ipcMain) — 45 个 IPC handler
  ├─ paths.ts：Electron app API + PORTABLE_EXECUTABLE_DIR 路径解析
  ├─ crypto.ts：safeStorage (OS keychain) + AES-256-GCM fallback
  ├─ nginx 进程管理 + 配置生成 + 快照
  └─ db / ssh / services / templates / utils
packages/shared (类型 + 纯函数)
  └─ 类型定义 + 校验 + API 路径常量
```

**工作原理**：Electron main 进程启动时初始化数据目录、打开 SQLite 数据库、注册 IPC handler、切换 nginx 引擎到默认 host、创建窗口。前端通过 `window.electronAPI.invoke(API.XXX, ...args)` 调用 main 进程的 handler，handler 内部调用 `@ngj/server` 的业务逻辑。退出时 main 进程先停止 nginx 引擎再关闭数据库连接。

## 快速开始

### 前置要求

- Node.js >= 22
- pnpm >= 9

### 开发

```bash
pnpm install

# 本地开发：electron-vite dev（HMR + 热重载）
pnpm dev

# 测试
pnpm test

# 代码检查
pnpm lint
pnpm typecheck
```

### 打包

```bash
# Windows 绿色版（macOS 交叉编译，零 Wine）
pnpm build:win:dir
# 产出 release/win-unpacked/，zip 后直接给用户

# Windows portable exe（需 Docker Wine）
pnpm build:win

# macOS dmg
pnpm build:mac
```

**跨平台原生模块**：使用 `prebuild-install` 从 GitHub Releases 下载目标平台（win32-x64 / darwin-arm64）的 Electron ABI 预编译二进制，无需本地编译，不会污染 pnpm store。

## 项目结构

```
nginx-jump-gui/
├── packages/
│   ├── shared/          # @ngj/shared — 类型 + 校验 + API 路径常量
│   └── server/          # @ngj/server — IPC handler + nginx 管理 + DB
├── apps/
│   └── desktop/         # @ngj/desktop — Electron 应用壳
│       ├── src/
│       │   ├── main/       # main 进程
│       │   ├── preload/    # contextBridge + IPC 类型
│       │   └── renderer/  # Vue 3 前端（Pinia + 组件 + IPC API 客户端）
│       ├── electron.vite.config.ts
│       └── electron-builder.config.cjs
├── resources/
│   └── nginx/           # 内置 nginx 二进制 + mime.types
└── scripts/
    └── build-desktop.mjs  # 打包编排脚本
```

## 测试

- **176 个测试全部通过**
- 测试框架：vitest（antfu 同目录测试约定）
- Lint：`@antfu/eslint-config`
- 类型检查：tsc + vue-tsc（main / preload / renderer 三处）

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面壳 | Electron 33 |
| 构建工具 | electron-vite（三进程统一 Vite 配置） |
| IPC | @electron-toolkit/typed-ipc + contextBridge |
| 后端 | Node.js + better-sqlite3 + ssh2 |
| 前端 | Vue 3 + Pinia |
| 原生模块 | prebuild-install（跨平台预编译二进制下载） |
| 打包 | electron-builder（portable + dmg + --dir） |
| 测试 | vitest |
| 规范 | @antfu/eslint-config |

## License

MIT
