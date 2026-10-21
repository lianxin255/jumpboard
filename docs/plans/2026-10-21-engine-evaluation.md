# 评估：还需要 nginx 吗？

> 2026-10-21。结论：**不需要**，已换成 Go 自己实现的引擎；nginx 只剩「配置导出格式」这一角色。

## 一、应用实际需要 nginx 做什么

从生成的 conf 和内置模板反推，全部能力只有这些：

| 需求 | nginx 的做法 | 用到的配置 |
|---|---|---|
| TCP 端口转发 | `stream` + `proxy_pass host:port` | `listen`、`proxy_timeout` |
| UDP 端口转发 | `stream` + `listen … udp` | 同上 |
| HTTP 反代 | `location` + `proxy_pass http://…` | `proxy_set_header`、`proxy_http_version` |
| HTTPS 终止 | `listen 443 ssl` + 证书 | `ssl_certificate`、`ssl_certificate_key` |
| 80→443 跳转 | `return 301` | 额外 server 块 |
| server_name 路由 | `server_name` | |
| location 前缀 | `location /api` | |
| IP 白名单 | `allow` / `deny all` | |
| 访问日志 | `access_log` | |
| 关闭缓冲/大上传/超时 | `proxy_buffering off` 等 | http 顶层 |

这十项都是标准库能直接表达的东西。唯一真正只有 nginx 能做的是**用户自己写的裸 nginx 指令**（`extraDirectives` 是给高级用户开的门）。

## 二、逐项验证：Go 能不能等价实现

| 需求 | Go 实现 | 验证 |
|---|---|---|
| TCP 转发 | `net.Listen` + 双向 `io.Copy` + `CloseWrite` 半关闭 | `TestTCPForward` 跑通回显 |
| UDP 转发 | `ListenUDP` + 每客户端地址一个上游 socket + 空闲回收 | `TestUDPForward` 跑通 |
| HTTP 反代 | `httputil.ReverseProxy`，`FlushInterval=-1` | `TestHTTPProxy` |
| WebSocket / Upgrade | 反向代理透传 Upgrade 头（Go 1.12+） | 依赖 `Hijacker` 实现 |
| HTTPS | `crypto/tls` + `GetCertificate(SNI)` + 通配符 | `TestTLSProxy` |
| 80→443 跳转 | 专用 :80 redirect 监听（带白名单），绑定失败降级为警告 | `TestHTTPSMappingEndToEnd`（TLS 终止 + 301 Location；无权限时应用出警告且 TLS 规则照常跑） |
| server_name 路由 | Host 精确/通配匹配，未知 Host 落默认规则 | `TestHTTPHostAndPathRouting` |
| location 前缀 | 最长前缀，按段边界匹配（`/api` 不匹配 `/apix`） | 同上 |
| IP 白名单 | `net.ParseCIDR` + RemoteAddr，未命中 403 | `TestWhitelistBlocks` |
| 访问日志 | 自己写 `access.log` | 实测生成 |
| 事务性应用 | 先开新端口，全成功才关旧的 | `TestApplyIsTransactional`、`TestReapplyKeepsUnchangedRules` |

## 三、换成自研引擎带来的好处

1. **应用变成事务性的**：不再有「写 conf → nginx -t → reload → 失败回滚」这套流程。端口占不到就整个 apply 失败，正在跑的规则一条不动 —— 这是 e2e 直接验证的行为。
2. **不再需要为每个平台准备 nginx 二进制**：包体从 mac 42.5 MB / win 16.3 MB 降到 **34.3 MB / 17.4 MB**（mac 还多带了一份 Intel slice），仓库里也不再放 15 MB 的二进制。
3. **不再有 conf 半写、mime.types、prefix、进程树清理、崩溃遗留 nginx** 这些周边问题，`internal/nginx` 从 1298 行降到约 600 行（只剩生成与解析）。
4. **每条规则的状态可以直接看到**：监听中 / 端口被占 / 未应用，表里一个点就说清楚；累计连接数与活跃连接数也是现成的。
5. **退出即关端口**：进程退出，监听自然消失，不用 `taskkill /T` 或 `-s quit`。

## 四、代价，以及怎么补

| 代价 | 补法 |
|---|---|
| 裸 nginx 指令不再全部生效 | 引擎认识 `proxy_timeout`、`proxy_read_timeout`、`proxy_connect_timeout`、`proxy_http_version`、`proxy_buffering`、`proxy_set_header`；其余在应用时**明确列成警告**，不再静默忽略（`TestUnsupportedDirectivesAreReported`） |
| 老用户可能习惯把 conf 拿到真 nginx 上用 | 每次应用都写一份标准 `nginx.conf` 到 `data/nginx-conf/`，设置页还有「导出 nginx 配置」；快照回滚读的也是这份文件（`nginx.Parse` 能把它解析回映射，`TestParseRoundTrip`） |
| 没有 nginx 那些成熟模块（gzip、limit_req、sub_filter…） | 本应用是端口转发工具，这些不在此列；需要的人可以用「导出 nginx 配置」把它搬到真 nginx 上继续加 |
| UDP 会话管理要自己写 | 已实现：每客户端地址一个上游 socket，按 `proxy_timeout` 或默认 5 分钟回收，空闲扫描 |

## 五、剩余限制（明确写下来，免得踩）

- **UDP 的 `proxy_timeout` 只保证会话回收**，不像 nginx 那样精确到单个报文方向的超时；对本场景（DNS、游戏、IoT）足够。
- **没有 gzip、限流、URL 重写**。需要这些的用户走「导出 nginx 配置」。
- **HTTPS 上游**：`proxy_pass http://…` 默认用明文连上游；要连 HTTPS 上游得在 `自定义 proxy_pass` 里写 `https://…`（引擎会照做，且不校验上游证书，因为内网多为自签）。
- **HTTP/2 到客户端**：引擎用标准库的 HTTP/1.1 服务端；HTTP/2 上游可以（Transport 自动协商），客户端侧不支持。

## 六、结论

- **端口转发与反代这一层：不需要 nginx**，已用 Go 替换，测试覆盖 TCP/UDP/HTTP/HTTPS/白名单/事务性。
- **nginx 配置这件事仍然保留**：作为导出格式、快照载体、以及「想搬到真 nginx」的出口。
- 如果将来要支持 nginx 的高级模块（gzip、限流、URL 重写），有两条路：在引擎里补这些能力，或者干脆给一个「外部 nginx 模式」把 conf 交给系统 nginx。当前不需要。
