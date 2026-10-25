# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed — 从 Tauri 迁移回 Electron + IPC 架构

Tauri 方案在 Windows 上遇到 WebView2 Runtime 缺失和 VCRUNTIME 依赖问题，决定切回 Electron。

- **Electron 33 壳**：替代 Tauri 2 Rust 壳，自带 Chromium + Node 运行时，Windows 开箱即用
- **IPC 架构**：替代 HTTP sidecar，`@electron-toolkit/typed-ipc` + `contextBridge` 实现类型安全的 main↔renderer 通信
- **electron-vite**：三进程统一 Vite 构建（main / preload / renderer），HMR 热重载
- **前端合并**：`apps/web/` 合并到 `apps/desktop/src/renderer/`，单一应用包
- **routes.ts → ipc.ts**：`buildRoutes()` 路由表改为 `registerIpcHandlers(ipcMain)`，45 个 handler
- **http.ts → ipc.ts**：前端 API 客户端从 `fetch(127.0.0.1:PORT)` 改为 `window.electronAPI.invoke(API.XXX)`
- **paths.ts 恢复**：`NGJ_RESOURCES_DIR` 环境变量 → Electron `app.getPath('exe')` + `PORTABLE_EXECUTABLE_DIR` + `process.resourcesPath`
- **crypto.ts 恢复**：AES-256-GCM only → `safeStorage`（OS keychain）优先 + AES fallback
- **db/connection.ts 简化**：移除 esbuild/nativeBinding 显式加载逻辑，plain `new Database(path)`
- **prebuild-install 替代 electron-rebuild**：下载目标平台 Electron ABI 预编译二进制，解决 pnpm store 污染问题
- **pnpm deploy --legacy**：创建扁平化 staging 目录，解决 pnpm 符号链接 + electron-builder asar 不兼容

### Added

- **scripts/build-desktop.mjs**：打包编排脚本（electron-vite build → pnpm deploy → prebuild-install → electron-builder）
- **electron-builder.config.cjs**：portable (win) + dmg (mac) + asarUnpack + extraResources
- **@electron-toolkit/preload**：contextBridge 标准暴露
- **@electron-toolkit/typed-ipc**：双向类型检查 IPC

### Fixed

- **pnpm store 污染**：electron-rebuild 从 cwd 向上搜索 lockfile 定位 projectRoot（`getProjectRootPath`），从 STAGE 运行时仍找到 monorepo 根的 `pnpm-lock.yaml`，扫描根 `node_modules` 里的 pnpm 符号链接（指向 store），重建时覆盖 store 里的 `.node` 文件。改用 `prebuild-install` 直接下载预编译二进制，完全隔离。
- **Windows 交叉编译架构**：electron-builder `--dir` 默认跟随主机架构（arm64），显式传 `--x64` 确保产出 x64
- **cpu-features 编译失败**：Python 3.12+ 移除 distutils，`npmRebuild: false` 跳过（ssh2 有 try-catch fallback）

### Removed

- Tauri 全部文件（Cargo.toml / main.rs / tauri.conf.json / capabilities / .cargo）
- `apps/web/`（合并到 `apps/desktop/src/renderer/`）
- `packages/server/src/index.ts`（HTTP server）+ `e2e.test.ts`（HTTP sidecar 专属）
- 旧脚本：`cross-build-win.sh` / `fetch-node.sh` / `fetch-better-sqlite3.sh` / `build-app.mjs` / `make-greenzip.mjs`

## [1.0.0] — 2026-08-11

### 核心功能（Electron 版本基线）

- TCP/UDP 端口转发 + HTTP/HTTPS 反向代理
- 内置 nginx 进程管理（start/stop/reload/test）
- 配置快照与历史回滚
- 健康检查
- IP 白名单
- 远程 nginx 管理（SSH）
- 数据导入导出
- 毛玻璃 iOS 风格 UI
- Windows 绿色版打包
- 215 个测试

[Unreleased]: https://github.com/lianxin255/jumpboard/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/lianxin255/jumpboard/releases/tag/v1.0.0
