# 安全审计修复（第十一批）

> 起因：全项目安全扫描（agent 深扫 + 交叉验证），发现此前未修的真实隐患。
> 原则：只修真实可利用的；已修项（路径穿越/白名单 fail-closed/IPC 部分校验/wipe token）跳过。

## 隐患清单（本次发现）

| # | 严重度 | 隐患 | 证据 | 修复方案 |
|---|---|---|---|---|
| H1 | 高 | DATA_IMPORT 零字段校验：恶意备份 JSON 可注入 reloadCmd（远程 RCE）、localConfig.nginxPath（本地任意执行）、confPath（远程任意文件覆盖）、非法 mapping 字段直通 conf | dataTransfer.ts:38-93 原样 INSERT，无 validation | importData 逐条校验：port/protocol/proxyPassUrl/proxyMode/name 消毒；remoteConfig 的 reloadCmd/testCmd 导入时重置为默认；nginxPath/confPath 要求绝对路径或重置 |
| H2 | 高 | mapping.name 未消毒直插 conf 注释行：`\n` 换行可逃逸注释注入完整 server 块 | renderers.ts:93/107/135/152 `# mapping: ${m.name}` 无 sanitize | renderers 所有 name 插值处过 sanitize() |
| M1 | 中 | 导出文件含 SSH 密码密文（Windows DPAPI 同用户任意进程可解密），无任何提示 | dataTransfer.ts:17 SELECT * 带 passwordEncrypted | 导出剔除 passwordEncrypted（导入后要求重填密码）+ UI 提示 |
| M2 | 中 | HOST_TEST_CONNECTION 任意 host:port 探测：渲染层可遍历内网做端口/服务指纹 oracle，错误信息原样返回 | handlers.ts:309-329 零限制 | 输出脱敏：成功返回 ok:true，失败只返回"连接失败"不透出具体错误细节 |
| M3 | 中 | Electron 窗口加固缺失：无 setWindowOpenHandler/will-navigate 拦截/无 CSP | index.ts:12-32 无 webContents 处理；index.html 无 CSP | setWindowOpenHandler→shell.openExternal+deny；will-navigate 拒绝非 file://；index.html 加 CSP meta |
| L1 | 低 | 0.0.0.0/0 是合法白名单条目 = allow 0.0.0.0/0; deny all 实际全放行，白名单静默失效 | validation.ts 允许 /0 掩码 | WHITELIST_ADD 拒绝 0.0.0.0/0（handler + 前端提示） |
| L2 | 低 | IP 前导零（01.2.3.04）通过校验（nginx 按十进制解析不产生绕过，仅规范化分叉） | validation.ts:15 /^\d+$/ | 每段改 /^(0|[1-9]\d{0,2})$/ 拒绝前导零 |

## 查证为安全/已修（跳过）
- SNAPSHOT_ROLLBACK SQL 注入：参数化 ✓
- renderProxyPass 逃逸：handler 校验 + sanitize 双保险 ✓
- preload 透传：全 invoke 包装 ✓
- 日志打印密码：无 ✓
- YAML 炸弹：yaml@2.9 默认 maxAliasCount=100 ✓
- v-html：仅静态图标常量 ✓
- extraDirectives：设计后门（自述），保持

## 实施步骤
1. H2 name 消毒（renderers.ts 一处 helper + 4 处插值）
2. H1 importData 校验（复用 @shared/validation + 字段白名单 + 重置危险字段）
3. M1 导出剔除凭据 + 前端提示
4. M2 测试连接输出脱敏
5. M3 Electron 加固（setWindowOpenHandler/will-navigate/CSP）
6. L1+L2 validation 收紧
7. 测试：每个修复对应新测试 + 全量回归

## 进度
- [x] H2 mapping.name 消毒（renderers.ts 加 safeName helper，4 处插值替换）
- [x] H1 importData 逐条校验（sanitizeImportedHost/Mapping；reloadCmd/testCmd/nginxPath/confPath 重置默认；passwordEncrypted 拒收；非法跳过+计数；悬空 hostId 过滤）
- [x] M1 导出剔除 passwordEncrypted（exportData 层）+ 导入/导出 toast 提示
- [x] M2 HOST_TEST_CONNECTION 入参收敛 + 输出脱敏（不透出 ECONNREFUSED/timeout 细节）
- [x] M3 Electron 加固（setWindowOpenHandler→shell.openExternal+deny / will-navigate 拦截 / index.html CSP meta）
- [x] L1 0.0.0.0/0 拒绝（isValidIpOrCidr 拒 /0 掩码）
- [x] L2 前导零拒绝（每段 /^(0|[1-9]\d{0,2})$/）
- [x] 测试：新增 15 条安全测试（securityAudit.test.ts 8 + renderers H2 2 + validation L1/L2 3，及既有套件适配），全量 176 通过 + typecheck + build

## 修复明细
| 隐患 | 文件 | 修复 |
|---|---|---|
| H2 name 注入 | renderers.ts | safeName()（=sanitize）应用于 4 处 `# mapping:` 注释插值 |
| H1 导入 RCE | dataTransfer.ts | sanitizeImportedHost：reloadCmd/testCmd→默认值，nginxPath/confPath→默认，passwordEncrypted→null，标量收敛；sanitizeImportedMapping：复用 isValidProtocol/Port/ProxyMode/ProxyPassUrl，name/listenPath/serverNames/tls 剥注入字符，长度上限 |
| M1 凭据泄露 | dataTransfer.ts + SettingsPanel.vue | exportData 的 remoteConfig.passwordEncrypted 置 null；toast 明示「不含密码，导入后需重填」 |
| M2 探测 oracle | handlers.ts | host/port/username 类型+范围收敛；错误输出统一为两类话术，不透协议细节 |
| M3 窗口加固 | index.ts + index.html | setWindowOpenHandler deny+openExternal；will-navigate 白名单（file://+dev server）；CSP default-src 'self' |
| L1/L2 | validation.ts | 拒 /0 掩码；拒前导零段 |

## 验证
- typecheck ✓ / **176 测试全过**（23 文件）✓ / build ✓
- H1 测试覆盖：恶意 reloadCmd 重置 / 密码拒收 / 非法 protocol+port 跳过计数 / name 换行消毒 / 非法 proxyPassUrl 拒为 null / 悬空 hostId 过滤 / 正常数据回归
- H2 测试覆盖：注释行无换行、注入指令不成为独立行
- L1/L2 测试覆盖：/0 与前导零各 3 断言
