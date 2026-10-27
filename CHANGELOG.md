# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — 终端 v2

- **多标签**：终端页可同时开多个会话标签（本地终端 / 端口探测 / 串口），切换与关闭互不影响；「全部关闭」与 ⌘Q 一并挂断
- **串口**：列出本机串行设备，选定波特率打开，原始 8-N-1 读写；打不开或没设备时给出可读提示（不涉及 SSH/凭据/远程主机）

## [1.2.0] — 2026-10-26

（1.0.0 之后的全部内容：mygo 原生化重构 + P1/P2 批次，随 1.2.0 号码入库）

### Added — P1/P2 批次

- **流量计数**：映射表每条规则显示累计 ↓/↑ 字节数（TCP/UDP/HTTP 三路，按规则归集）
- **吞吐曲线**：规则详情弹层显示近 60 秒的吞吐折线与当前速率
- **分组**：映射可归入一个短标签；表格 chip、分组过滤、表单输入与建议
- **快照 diff**：快照弹层改为与当前配置的统一 diff 视图（+N/−N 摘要），弹层内回滚需确认
- **AI 解读**：导入的 nginx 配置可让模型用中文解读，流式渲染 Markdown
- **配置查看**：设置页可只读查看应用实际生成的 nginx.conf；导出的配置被外部修改后表格上方出现提示条
- **粘贴导入**：剪贴板里的备份 JSON 或 nginx 配置可直接粘贴导入，先预览再决定
- **排序**：映射右键菜单可上移/下移，调整规则在配置中的顺序
- **deeplink**：`jumpboard://` 链接可唤起窗口，`jumpboard://import?data=…` 直接进入导入预览
- **应用内更新**：设置页「更新」卡片，手动检查、查看发布说明、安装并重启（mygo 签名更新器；开发构建显示不支持）
- **终端**：应用内跑本地 shell，或对转发端口做 TCP/HTTP 探测会话（mygo 终端插件，随包分发 libghostty-vt，macOS 最低系统升至 13.0；SSH/串口/多标签不在本版）
- **发布流水线**：`v*` tag 自动构建 macOS/Windows 产物并挂到 draft release，配好 secret 后同时产出签名更新包（见 `docs/release-and-updates.md`）

### Fixed — P1/P2 批次

- 回滚不再清空全部快照历史；回滚前先把当前配置存为一条 `rollback` 快照，二次回滚可撤销第一次

### Removed — Electron / Node 壳

主产品是 Go + mygo。Electron 桌面壳、`packages/server`、`packages/shared`、根 `package.json`、pnpm 工作区以及对应的 CI（`unit-test.yml`、`autofix.yml`）已从仓库删除，构建只走 `go tool mygo` / `go run .`。旧版说明留在 `docs/README-electron-legacy.md`，索引见 `legacy/README.md`。

### Changed — 从 Tauri 迁移回 Electron + IPC 架构（历史记录，壳已删除）

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
