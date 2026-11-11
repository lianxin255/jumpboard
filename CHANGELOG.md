# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.1] — 2026-11-11

### Added — 终端 v2

- **多标签**：终端页可同时开多个会话标签（本地终端 / 端口探测 / 串口），切换与关闭互不影响；「全部关闭」与 ⌘Q 一并挂断
- **串口**：列出本机串行设备，选定波特率打开，原始 8-N-1 读写；打不开或没设备时给出可读提示（不涉及 SSH/凭据/远程主机）

### Added — 自启与排序

- **开机自启**：设置页新增「登录时启动」开关（默认关）；已运行时再次启动不再弹第二个实例，而是唤起既有窗口
- **多选拖拽**：映射表支持多选（单击 / ⌘ 点选 / ⇧ 连选 / ⌘A 全选），拖拽任一选中行即整块排序（此前仅单行拖拽）

### Added — 界面质感

- **拟物按钮**：按钮改为 5 层材质——渐变、描边、顶部内高光、底部内暗边、双层投影；按压时渐变反转、高光变内阴影、投影收缩并下移 1px，点击有了实体感
- **动画曲线令牌**：按角色区分入场、点击落位、退场、位移、归位、旋转六种曲线，取代此前全应用只有一处动画的状况
- **线框空状态**：空页面改为 160×120 的微缩界面线框（表头、渐隐数据行、一个强调色主按钮），不再是抽象图标
- **页面到达光带**：切换页面时一道低对比度强调色光带扫过内容区
- **对话气泡分组**：同一发送方的连续消息收紧间距与圆角，读作一个块而非一叠卡片
- **排版**：标题自动平衡换行（`text-wrap: balance`）防孤词，正文改用更优换行
- **可撤销操作不再弹确认**：批量启用/停用改为直接执行 + Toast 撤销（8 秒），确认弹窗只留给不可逆操作

### Security

- **修复 AI 对话泄露规则凭据**：对话曾把完整映射（含 `proxy_set_header` 里的 token、带凭据的上游 URL、TLS 私钥路径）发给 AI 网关，与设置页「映射摘要里永远不含密钥」的承诺矛盾；改为白名单投影，只发草稿必须命名的字段，诊断摘要同步剔除
- **修复健康检查 SSRF**：健康检查路径以 `@` 开头时会把上游主机变成 userinfo，探测打到外部主机；改为按 URL 部件组装
- **修复 AI 草稿跨场景改写**：草稿按 id 查规则不校验场景，模型给出的 id 指向别的场景时可被覆盖；新增按场景限定的查询
- **修复旧 API Key 发到新地址**：改了网关地址但没重输 Key 时，已存的 Key 会被发往新网关；改为只在地址一致时复用
- **AI 服务地址校验**：只接受 http/https 且有主机名的地址，避免 Key 被发往非预期目标
- **AI 回复长度上限**：非流式 JSON 体 8 MiB、流式文本 16 MiB，防止异常网关撑爆内存
- **错误信息不再带完整 URL**：避免地址里的 userinfo 或 query token 进入界面与日志

### Fixed

- **修复终端 goroutine 泄漏（Windows 致命）**：`startShell` 标记「正在启动」后调用 `hangUp` 清旧会话，而 `hangUp` 把该标记一并清掉，于是每帧重新启动一个 shell。macOS 上 shell 启动太快未暴露；Windows 上 ConPTY 需数秒，最终撞到 Go 的 10000 线程上限（`fatal error: thread exhaustion`，dump 中 9720 个 goroutine 全来自 `startShell`），连 shell 回显都被吞掉
- **修复 AI 冲突守卫对旧库永远失败**：`WHERE` 用 `col=?` 比较可空列，旧库的 `NULL` 与 `''` 不相等，对这类规则的 AI 修改永远报冲突；可空列改为 `COALESCE`
- **修复对话禁用/清空配置**：`"proxyMode": null` 曾被当作「已提供」，更新草稿不再恢复原值，静默把 `custom` 降回 `standard` 并丢掉自定义认证头；`"enabled": null` 曾被解成 `false` 停用规则。现在跳过 JSON null
- **修复对话超时截断**：HTTP 客户端固定 30s 覆盖整个流式响应体，长回复在 30s 被砍断，对话自己的 60s 预算形同虚设
- **修复对话 busy 卡死**：落库失败时 `chatBusy` 已置真就返回，之后每条消息都报「上一条回复还在路上」直到重启
- **修复解读流式状态永远为假**：首个片段就把 busy 清掉，渐进渲染从未生效，且第二次解读可与第一次交错
- **修复更新草稿「填入表单」打开新建表单**：保存后多出一条重复映射
- **修复更新草稿漏恢复名称与端口**：模型省略时会把规则改名成占位符、把监听端口写成 0
- **修复草稿 diff 不显示代理模式与附加指令**：模型改动这两项时审核卡看起来没变，导入后认证头/超时静默丢失
- **修复两个动效从未播放**：`AnimateTo` 在每帧新建的元素上创建新通道并直接返回目标值，页面光带与提示点脉冲实际一次都没跑过
- **修复按钮禁用样式是死代码**：包装函数内查 `IsDisabled()`，而调用方在返回后才设置，禁用面从未绘制
- **Windows 测试超时**：`waitFor` 原本 3 秒封顶，ConPTY 起 shell 需数秒，改为按平台放宽

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

[Unreleased]: https://github.com/lianxin255/jumpboard/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/lianxin255/jumpboard/releases/tag/v1.2.0
[1.0.0]: https://github.com/lianxin255/jumpboard/releases/tag/v1.0.0
