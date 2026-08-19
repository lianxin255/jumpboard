# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — 重构为 Tauri monorepo（antfu 规范）

- **Tauri 2 Rust 壳**：替代 Electron，spawn Node sidecar（`server.cjs`）+ 托盘 + 单实例锁
- **pnpm monorepo**：`packages/{shared,server}` + `apps/{web,desktop}`，pnpm workspace catalogs
- **IPC → HTTP**：前端从 `ipcRenderer.invoke` 改为 `fetch` 调 `127.0.0.1:47831`，路由 1:1 映射
- **esbuild 单文件 sidecar**：`server.cjs` 自包含，app 内零 node_modules
- **密码加密迁移**：Electron safeStorage → AES-256-GCM（scryptSync 从 `NGJ_SECRET` 派生密钥）
- **e2e 集成测试**：真实启动 sidecar + nginx，验证完整生命周期（11 个测试）
- **antfu eslint 规范**：`@antfu/eslint-config`，同目录测试约定
- **Windows 绿色版打包**：`fetch-node.sh` 下载 win32 Node、`fetch-better-sqlite3.sh` 下载 prebuilt

### Fixed

- `bundledNginxDir` 路径：`resources/bundled/nginx` → `resources/nginx`（匹配 tauri.conf resources 映射）
- `localEngine.test()`：补 `ensureDirSync` 确保 logs/temp/conf 目录存在（nginx -t 需写日志和 pid 文件）
- `localEngine.reload()`：`child.kill('SIGHUP')` → `process.kill(pid, 'SIGHUP')`（Node child.kill 会把 `child.killed` 置 true 导致 status 误报 stopped）
- `confGenerator` stream 块：补 `log_format stream` 定义（stream 模块无默认 log_format，nginx -t 报错）
- `index.ts send()`：字符串响应也 `JSON.stringify`（engine status 返回原始文本但 content-type 是 application/json）
- `templateRepo.getBuiltinDir()`：`resources/bundled/templates` → `resources/templates/builtin`
- Tauri 2 Rust API 适配：`#[tauri::main]` 宏移除、`single_instance::new` → `::init`、`app.exit()` → `app.handle().exit()`

### Changed

- 项目结构：单仓 Electron → pnpm monorepo + Tauri 壳
- `paths.ts`：`process.resourcesPath`（Electron）→ `NGJ_RESOURCES_DIR` 环境变量（Tauri 注入）
- `package.json`：根包 scripts 统一管理（dev/build/test/lint/typecheck/tauri）
- `tsconfig.json`：antfu 标准 + `@ngj/*` 路径别名
- `vitest.config.ts`：单扁平配置（vitest 1.6.1 projects 模式有 DataCloneError bug）

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

[Unreleased]: https://github.com/lianxin255/nginx-jump-gui/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/lianxin255/nginx-jump-gui/releases/tag/v1.0.0
