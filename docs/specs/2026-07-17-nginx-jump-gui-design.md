# Nginx 跳板机管理软件 - 设计文档

- **状态**: Draft，待用户 review
- **日期**: 2026-07-17
- **目标平台**: Windows（绿色版，单文件夹可携）
- **技术栈**: Electron (Node.js) + Vue 3 + Vite + SQLite

---

## 1. 背景与目标

### 1.1 场景

研发网内的 Windows 机器作为反向代理网关，把"当前网络能访问的内网服务端口"通过 nginx 代理出来，供用户的外网机访问。本质是一台被人工维护的反向代理跳板机。

### 1.2 核心诉求

1. **手动维护**端口映射规则（TCP/UDP 端口转发 + HTTP/HTTPS 反向代理）
2. **内置 nginx**，双击即用；也要能**远程管理**其他 nginx 实例
3. **绿色版**：单文件夹、免安装、不写注册表/AppData
4. **退出无痕**：关掉软件 nginx 立即停止，不在系统留服务/常驻进程，降低被内网扫描发现的概率
5. **毛玻璃 iOS 风格** GUI

### 1.3 非目标（明确不做）

- 不做多用户/权限/审计（单人工具）
- 不做 nginx 注册为 Windows 服务（与"退出无痕"冲突）
- 不做 nginx 独立进程模式（同上）
- 不做运行时流量加密/隐身（运行期间监听的端口内网可见，不在范围内）

---

## 2. 运行形态与"无痕"定义

### 2.1 无痕的边界

- ✅ **能做到**：软件关闭后 nginx 进程立即退出、监听端口立即关闭、不注册任何服务、不写注册表、不污染 Program Files。第二天网管扫不到任何残留服务。
- ⚠️ **做不到**：软件运行期间 nginx 监听的端口，内网扫描器在那段时间仍可见。这是反向代理的固有特性，不在本项目范围。

### 2.2 nginx 运行方式

**仅子进程托管**。软件启动时按需拉起 nginx 子进程，软件退出时确保子进程退出。

- 主进程通过 `child_process.spawn` 启动 `nginx.exe`，持有 child handle
- 退出钩子（`before-quit` / `will-quit` / 异常 / SIGINT）统一杀掉 nginx 进程
- 进程意外退出时通过 `disown` 检测 + 重启策略处理（可选）

砍掉"独立进程"和"Windows 服务"两种模式：均与"退出无痕"诉求冲突。

---

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    Electron Renderer (Vue 3)             │
│   映射列表 / 模板选择器 / 新增编辑表单 / 主机管理 /        │
│   配置历史 / 日志查看 / 状态探活展示                       │
│   毛玻璃 iOS 风格 UI                                     │
└───────────────────────┬─────────────────────────────────┘
                  IPC (contextBridge, invoke/handle)
┌───────────────────────┴─────────────────────────────────┐
│                  Electron Main (Node.js)                 │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │
│  │ 映射仓库     │→│ 配置生成器   │→│ nginx 引擎       │  │
│  │ (SQLite)     │ │ (拼conf文件) │ │ (本地+远程统一)  │  │
│  └──────────────┘ └──────────────┘ └──────────────────┘  │
│  ┌────────────────────────────────────────────────────┐  │
│  │  本地引擎                │   远程引擎               │  │
│  │  - child_process.spawn  │   - ssh2 连接池          │  │
│  │  - nginx -t / -s reload │   - SFTP 读写 conf       │  │
│  │  - 日志 tail            │   - exec reload/test     │  │
│  │  - 进程生命周期托管      │   - 多主机管理            │  │
│  └────────────────────────────────────────────────────┘  │
│  ┌────────────┐ ┌──────────────┐ ┌──────────────────┐   │
│  │ 端口占用检测│ │ 健康检查轮询  │ │ 配置历史+回滚     │   │
│  └────────────┘ └──────────────┘ └──────────────────┘   │
└─────────────────────────────────────────────────────────┘
                         │
        ┌────────────────┴────────────────┐
        ▼                                  ▼
  本地 nginx.exe (bundled)        远程 nginx (Linux/Win)
```

### 3.1 三层职责

- **Renderer（Vue）**：纯 UI，不碰文件系统/进程/网络。所有数据通过 IPC 拿。
- **Main（Node）业务核心**：映射仓库、配置生成器、nginx 引擎、辅助服务（端口检测/健康检查/历史）。所有副作用都在这层。
- **nginx 引擎抽象**：对外暴露统一接口，`local` 和 `remote` 两个实现。

### 3.2 nginx 引擎统一接口

```typescript
interface NginxEngine {
  start: () => Promise<void>
  stop: () => Promise<void>
  reload: () => Promise<void> // 热重载
  test: () => Promise<{ ok: boolean, output: string }> // nginx -t
  status: () => Promise<EngineStatus> // running/stopped/unknown
  tailLogs: (lines: number) => Promise<LogEntry[]>
  writeConf: (content: string) => Promise<void>
  readConf: () => Promise<string>
}
```

- `LocalEngine`：child_process 管理 nginx.exe，本地文件读写
- `RemoteEngine`：ssh2 连接池 + SFTP 读写 conf + exec `nginx -t`/`nginx -s reload`

---

## 4. 数据模型

存储：**SQLite (better-sqlite3)**，数据库文件在 `data/data.db`，随程序目录走。

### 4.1 Host（nginx 实例）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | uuid |
| name | TEXT | 显示名，如"研发网网关" |
| type | TEXT | `local` \| `remote` |
| isDefault | INT | 0/1，默认选中的实例 |
| localConfig | TEXT(JSON) | local 时填：nginxPath, confDir, logDir |
| remoteConfig | TEXT(JSON) | remote 时填：host, port, username, authType(password/key), passwordEncrypted/keyPath, nginxPath, confPath, reloadCmd, testCmd |
| status | TEXT | 缓存的状态：running/stopped/unknown（运行时实时刷新） |
| createdAt | INT | unix ms |

### 4.2 Mapping（一条映射规则）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | uuid |
| hostId | TEXT FK | 所属 nginx 实例 |
| name | TEXT | 规则名 |
| enabled | INT | 0/1 |
| protocol | TEXT | `tcp` \| `udp` \| `http` \| `https` |
| listenPort | INT NULL | tcp/udp 监听端口 |
| serverNames | TEXT NULL | http/https 域名（逗号分隔） |
| listenPath | TEXT NULL | http/https 路径前缀 |
| upstreamHost | TEXT | 目标内网主机 |
| upstreamPort | INT | 目标端口 |
| tlsCertPath | TEXT NULL | https 证书 |
| tlsKeyPath | TEXT NULL | https 私钥 |
| redirectHttp | INT | https 时 HTTP→HTTPS 跳转 |
| extraDirectives | TEXT | 高级逃生口，原生 nginx 指令 |
| healthCheckEnabled | INT | 0/1 |
| healthCheckIntervalSec | INT | 默认 30 |
| healthCheckTargetPath | TEXT NULL | http/https 探活的路径 |
| healthCheckExpectedStatus | INT NULL | http/https 期望状态码，默认 200 |
| healthStatus | TEXT | healthy/unhealthy/unknown/检查中 |
| lastCheckedAt | INT NULL | |
| sortOrder | INT | 列表排序 |
| createdAt | INT | |
| updatedAt | INT | |

### 4.3 ConfigSnapshot（配置历史）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | |
| hostId | TEXT FK | |
| content | TEXT | 生成的完整 conf 原文 |
| reason | TEXT | reload/manual-edit/rollback |
| createdAt | INT | |

保留策略：每 host 最近 50 条，超出按时间裁剪。

### 4.4 AppLog（操作 + nginx 日志镜像）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | |
| hostId | TEXT NULL | |
| level | TEXT | info/warn/error |
| source | TEXT | `app` \| `nginx-access` \| `nginx-error` |
| message | TEXT | |
| timestamp | INT | |

保留策略：最近 10000 条，循环裁剪。

---

## 5. 配置生成器

### 5.1 生成分片结构

不在一个 `nginx.conf` 里堆所有规则，而是按职责分文件，主 conf 用 `include` 串起来。好处：单条规则的增删只改一个分片，减少 diff，便于人肉核对。

```
data/nginx-conf/
├── nginx.conf              ← 主文件：events/http/stream + include
├── http/
│   ├── 00-upstreams.conf   ← 所有 upstream 块
│   ├── 10-servers.conf     ← 所有 http/https server 块
│   └── 99-custom.conf      ← 用户全局自定义指令
├── stream/
│   ├── 10-tcp.conf         ← 所有 stream server (tcp)
│   └── 20-udp.conf         ← 所有 stream server (udp)
└── snippets/               ← 单条映射的片段（可选，用于精确 diff）
```

### 5.2 生成流程

1. 从 SQLite 读出该 host 的所有 `enabled=1` 的 mapping
2. 按 protocol 分桶：tcp/udp 进 stream，http/https 进 http
3. 用模板引擎渲染（Handlebars 或简单字符串拼接）
4. **生成完整 conf 后必跑 `nginx -t` 校验**，失败则不 reload、原配置不动、错误回显给用户
5. 校验通过 → 写文件 → `nginx -s reload` → 成功后存一条 ConfigSnapshot

### 5.3 模板示例（TCP）

```nginx
# generated from mapping: <name>
server {
    listen <listenPort>;
    proxy_pass <upstreamHost>:<upstreamPort>;
    proxy_timeout 1h;
    proxy_connect_timeout 60s;
    <extraDirectives>
}
```

### 5.4 模板示例（HTTPS 网站）

```nginx
# HTTP -> HTTPS 跳转
server {
    listen 80;
    server_name <serverNames>;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name <serverNames>;
    ssl_certificate <tlsCertPath>;
    ssl_certificate_key <tlsKeyPath>;

    location <listenPath> {
        proxy_pass http://<upstreamHost>:<upstreamPort>;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        <extraDirectives>
    }
}
```

---

## 6. 模板系统

### 6.1 内置模板清单（MVP 13 个）

| 类别 | 模板 | protocol | 预填要点 | 附带指令 |
|------|------|----------|---------|---------|
| 网络层 | TCP 转发 | tcp | listenPort 待填 | `proxy_timeout 1h` |
| | UDP 转发 | udp | listenPort 待填 | — |
| | SSH | tcp | upstreamPort=22 | `proxy_timeout 1h` |
| | RDP | tcp | upstreamPort=3389 | `proxy_timeout 1h` |
| Web | 网站反代 | http | serverNames 待填, listenPath=/ | 转发头全套 |
| | HTTPS 网站 | https | + 证书路径 | + HTTP→HTTPS 跳转 |
| | API 接口 | http | listenPort 待填, listenPath=/api | 超时调短、`proxy_buffering off` |
| | WebSocket | http | listenPath 待填 | `Upgrade`/`Connection` 头 |
| 数据库 | MySQL | tcp | upstreamPort=3306 | `proxy_timeout 1h` |
| | PostgreSQL | tcp | upstreamPort=5432 | `proxy_timeout 1h` |
| | Redis | tcp | upstreamPort=6379 | `proxy_timeout 1h` |
| | MongoDB | tcp | upstreamPort=27017 | `proxy_timeout 1h` |
| 消息 | MQTT | tcp | upstreamPort=1883 | `proxy_timeout 1h` |

### 6.2 模板文件格式（YAML）

存放位置：
- 内置：`resources/templates/builtin/*.yml`（随包发布，只读）
- 自定义：`data/templates/custom/*.yml`（用户"存为模板"生成，可删）

```yaml
# builtin/mysql.yml
name: MySQL 数据库
icon: database
category: 数据库
description: 把内网 MySQL 端口代理出来，默认 3306
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 3306
  healthCheck:
    enabled: true
    intervalSec: 30
extraDirectives: |
  proxy_timeout 1h;
  proxy_connect_timeout 60s;
```

### 6.3 用户流程

点"新建映射" → 模板选择器（卡片网格，iOS 大图标 + 类别分组） → 选模板 → 进预填表单 → 改关键字段 → 保存。

### 6.4 自定义模板

任意 Mapping 可"存为模板"，写入 `data/templates/custom/<name>.yml`，UI 中和内置模板同展但带"自定义"标记，可删除。

---

## 7. 辅助服务

### 7.1 端口占用检测

- 新建/编辑映射保存前：检查 `listenPort` 在本机是否被占
- 实现：尝试 `net.createServer().listen(port)` 失败即被占，或 Windows `netstat -ano` 解析
- 被占时表单标红提示，但不阻止保存（用户可能就是想覆盖）

### 7.2 健康检查

- **TCP/UDP**：尝试建立 TCP/UDP socket 到 `upstreamHost:upstreamPort`，连通即 healthy
- **HTTP/HTTPS**：`GET healthCheckTargetPath`，状态码 = `expectedStatus` 即 healthy（默认 200）
- 调度：后端定时器，按 `intervalSec` 轮询所有 `healthCheckEnabled=1` 的映射，结果写回 `healthStatus`
- UI：列表上每个映射显示绿/红/灰点

### 7.3 配置历史 + 回滚

- 每次 reload 成功后存一条 ConfigSnapshot
- 历史页列表展示，点"回滚"→ 用该快照覆盖当前 conf → `nginx -t` → 通过则 reload
- 每 host 保留最近 50 条

### 7.4 日志

- **应用日志**：所有关键操作（start/stop/reload/CRUD）写 AppLog
- **nginx 日志**：tail `data/logs/access.log` 和 `error.log`，镜像最新 N 条到 AppLog，UI 实时滚动展示
- 远程 nginx：通过 SSH tail 远程日志文件

---

## 8. UI 设计（毛玻璃 iOS 风格）

### 8.1 设计语言

- **Glassmorphism**：半透明卡片 + backdrop-filter blur + 细边框 + 柔和阴影
- **iOS 风格**：圆角（12-16px）、SF 字体栈、系统蓝 `#0A84FF`、分组列表（grouped table view）、底部 tab 或侧栏导航、大量留白
- **配色**：浅色优先，深色适配。背景用渐变 + 模糊光斑营造毛玻璃氛围
- **微交互**：hover/press 状态过渡，列表项滑动操作（iOS 风 swipe action）

### 8.2 主界面布局

```
┌────────────────────────────────────────────────────────┐
│  [☀︎] Nginx 跳板机          [研发网网关 ▾]  [⟳] [⚡清除] │  ← 顶栏：实例切换、刷新、一键清除
├──────────┬─────────────────────────────────────────────┤
│          │                                             │
│  映射    │   映射列表（分组卡片）                        │
│  模板    │   ┌─────────────────────────────────────┐    │
│  主机    │   │ ● MySQL 内库      TCP  9000→3306   │    │
│  历史    │   │   192.168.1.10    healthy          │    │
│  日志    │   └─────────────────────────────────────┘    │
│  设置    │   ┌─────────────────────────────────────┐    │
│          │   │ ● 内网 Gitlab     HTTPS gw/gitlab  │    │
│          │   │   192.168.1.20    healthy          │    │
│          │   └─────────────────────────────────────┘    │
│          │                                             │
│          │              [ + 新建映射 ]                  │
└──────────┴─────────────────────────────────────────────┘
```

### 8.3 主要页面

1. **映射列表**：卡片列表，每张卡片显示 名称/协议/监听→目标/健康点/启用开关。点卡片进编辑。
2. **新建映射**：先选模板（卡片网格）→ 预填表单。表单按 protocol 动态字段。
3. **模板**：内置 + 自定义模板管理，可新建/删除自定义模板。
4. **主机**：nginx 实例管理（local/remote），remote 含 SSH 连接测试按钮。
5. **历史**：ConfigSnapshot 列表，diff 视图 + 回滚。
6. **日志**：应用日志 + nginx 日志 tab 切换，实时滚动，按 level 着色。
7. **设置**：数据目录、清除痕迹、导出/导入配置、关于。

### 8.4 "一键清除所有痕迹"按钮

顶栏的 `⚡清除`：
- 弹确认框（二次确认，iOS 风ActionSheet）
- 停 nginx → 删除 `data/` 下所有内容（DB + conf + logs + snapshots）→ 重启到空状态
- 用于"跑路"场景

---

## 9. 绿色版与目录结构

### 9.1 打包目标

`electron-builder` 的 `portable` target，输出单个自解压 exe，或解压即用的文件夹版。**不写注册表、不进 Program Files、不创建快捷方式**（除非用户手动）。

### 9.2 程序目录

```
nginx-jump-gui/                ← 整个文件夹拷走即用，删掉彻底没了
├── nginx-gui.exe              ← Electron 主程序
├── resources/
│   ├── app.asar               ← 应用代码
│   ├── nginx/                 ← 内置 nginx (Windows 版)
│   │   ├── nginx.exe
│   │   └── conf/mime.types 等默认文件
│   └── templates/builtin/     ← 13 个内置模板 yml
└── data/                      ← 运行时生成，可整体删
    ├── data.db                ← SQLite
    ├── nginx-conf/            ← 生成的 conf 分片
    ├── snapshots/             ← 配置历史
    ├── logs/                  ← nginx access/error 日志
    └── templates/custom/      ← 用户自建模板
```

### 9.3 关键约束

- `app.getPath('userData')` 强制重定向到 exe 旁的 `data/`，**绝不写 `%APPDATA%`**
- nginx 用 `-p <data目录> -c <生成conf>` 启动，temp/logs 全限在 `data/` 内
- 退出时 `SIGTERM` nginx 子进程并清理 temp 文件
- 内置 nginx 用官方 Windows 稳定版，单独存放在 `resources/nginx/`

---

## 10. 远程 nginx 管理

### 10.1 能力

对 `type=remote` 的 Host：
- SSH2 连接（密码或私钥认证），连接池复用
- SFTP 读写远程 conf 文件
- exec 执行 `nginx -t` / `nginx -s reload`（命令可配，适配不同发行版路径）
- tail 远程 access/error 日志

### 10.2 与本地的差异

- 远程实例**不托管进程生命周期**（start/stop 由用户在远程自己管），只做 conf 生成 + reload + 日志 + 健康检查
- 远程实例的配置历史同样存本地 SQLite，按 hostId 区分

---

## 11. 错误处理

| 场景 | 处理 |
|------|------|
| nginx 启动失败 | 捕获 stderr，UI 红色 toast，写入 AppLog |
| `nginx -t` 校验失败 | 不 reload，原配置不动，错误详情回显 |
| reload 失败 | nginx 通常保留旧配置继续跑；记录错误，UI 提示 |
| 远程 SSH 连不上 | 连接池标记 unhealthy，UI 标灰，自动重连指数退避 |
| 健康检查超时 | 标 unhealthy，不影响 nginx 本身 |
| 端口被占 | 表单标红警告，不阻止保存（用户可能知悉） |
| SQLite 写失败 | 事务回滚，UI 提示重试 |
| 退出时 nginx 未正常退出 | 强制 `taskkill /F /PID`，记录 |

---

## 12. 测试策略

- **配置生成器**：单元测试覆盖 13 种模板 × 各种字段组合，断言生成的 conf 文本。这是最核心、最易回归的部分。
- **nginx 引擎**：本地用真实 nginx.exe 做集成测试（CI 起 nginx，测 start/reload/test/stop）。
- **数据层**：better-sqlite3 仓库的 CRUD 单元测试，用临时 db 文件。
- **健康检查 / 端口检测**：用本地 echo server / 占用端口做集成测试。
- **UI**：组件级测试（Vitest + @vue/test-utils），关键交互流（新建映射→选模板→保存→出现在列表）做 e2e（Playwright 对 Electron）。
- **打包验证**：CI 产出 portable 后，脚本验证"单文件夹运行 + 退出后无残留进程"。

---

## 13. 开放问题（实现阶段再定）

- 前端组件库选型：纯手写 vs Element Plus vs Naive UI（毛玻璃风格可能需要自定义主题，倾向 Naive UI 的暗色/主题能力 + 大量自定义 CSS）
- 远程 SSH 私钥密码加密存储方案（ Electron safeStorage API）
- 多语言（先做中文，预留 i18n）
- 自动更新：绿色版做增量更新较复杂，MVP 不做，后续可加

---

## 14. 范围确认

本设计覆盖一个完整的 MVP，包含：
- 内置 nginx 子进程托管 + 远程 nginx 管理
- TCP/UDP/HTTP/HTTPS 四类映射的完整 CRUD
- 13 个内置模板 + 自定义模板
- 健康检查、端口检测、配置历史回滚、日志
- 毛玻璃 iOS 风格 UI
- 绿色版打包、退出无痕、一键清除

**不包含**：多用户权限、nginx 服务化、运行时隐身、自动更新。
