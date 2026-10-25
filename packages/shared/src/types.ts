// ============ Host ============
export type HostType = 'local' | 'remote'

export interface Host {
  id: string
  name: string
  type: HostType
  isDefault: number // 0 | 1
  localConfig: LocalHostConfig | null
  remoteConfig: RemoteHostConfig | null
  status: EngineStatus
  createdAt: number
}

export interface LocalHostConfig {
  nginxPath: string // 可空，空则用 bundled
  confDir: string // 可空，空则用 data/nginx-conf
  logDir: string // 可空
}

export interface RemoteHostConfig {
  host: string
  port: number // SSH 端口，默认 22
  username: string
  authType: 'password' | 'key'
  passwordEncrypted: string | null // 加密后 base64
  keyPath: string | null
  nginxPath: string
  confPath: string // 远程 conf 主文件路径
  reloadCmd: string // 默认 nginx -s reload
  testCmd: string // 默认 nginx -t
}

// ============ Mapping ============
export type Protocol = 'tcp' | 'udp' | 'http' | 'https'
export type HealthStatus = 'healthy' | 'unhealthy' | 'unknown' | 'checking'

/**
 * HTTP/HTTPS 代理模式：
 * - standard：标准反代（自动生成 Host/X-Real-IP/WebSocket/大body等头）
 * - minimal：精简代理（仅 proxy_pass，不加额外头，适合 GitLab 等对 Host 敏感的服务）
 * - custom：完全自定义（用户写的 proxy_headers 替代自动生成的）
 */
export type ProxyMode = 'standard' | 'minimal' | 'custom'

export interface Mapping {
  id: string
  hostId: string
  name: string
  enabled: number // 0 | 1
  protocol: Protocol
  listenPort: number | null
  serverNames: string | null
  listenPath: string | null
  upstreamHost: string
  upstreamPort: number
  tlsCertPath: string | null
  tlsKeyPath: string | null
  redirectHttp: number // 0 | 1
  extraDirectives: string | null
  proxyMode: ProxyMode
  proxyHeaders: string | null // custom 模式下的自定义代理头指令
  proxyPassUrl: string | null // 自定义 proxy_pass 值（如 http://1.2.3.4/），null 走默认 host:port 拼
  healthCheckEnabled: number
  healthCheckIntervalSec: number
  healthCheckTargetPath: string | null
  healthCheckExpectedStatus: number | null
  healthStatus: HealthStatus
  lastCheckedAt: number | null
  sortOrder: number
  createdAt: number
  updatedAt: number
}

// 输入用（无 id/时间戳/healthStatus，由后端填）
export type MappingInput = Omit<Mapping, 'id' | 'createdAt' | 'updatedAt' | 'healthStatus' | 'lastCheckedAt' | 'sortOrder'>

// ============ Nginx Engine ============
export type EngineStatus = 'running' | 'stopped' | 'unknown'

export interface EngineTestResult {
  ok: boolean
  output: string
}

// ============ HTTP API 路径 ============
// 原 IPC 通道名（'host:list' 等）改为 HTTP 路径常量。
// 路由表 routes.ts 1:1 映射，前端 api/http.ts 用同名方法调用。
export const API = {
  // Host
  HOST_LIST: '/api/host/list',
  HOST_CREATE: '/api/host/create',
  HOST_UPDATE: '/api/host/update',
  HOST_DELETE: '/api/host/delete',
  HOST_SET_DEFAULT: '/api/host/set-default',
  HOST_SWITCH: '/api/host/switch',
  HOST_TEST_CONNECTION: '/api/host/test-connection',
  // Mapping
  MAPPING_LIST: '/api/mapping/list',
  MAPPING_CREATE: '/api/mapping/create',
  MAPPING_UPDATE: '/api/mapping/update',
  MAPPING_DELETE: '/api/mapping/delete',
  // Engine
  ENGINE_START: '/api/engine/start',
  ENGINE_STOP: '/api/engine/stop',
  ENGINE_RELOAD: '/api/engine/reload',
  ENGINE_TEST: '/api/engine/test',
  ENGINE_STATUS: '/api/engine/status',
  ENGINE_DIRTY_GET: '/api/engine/dirty-get',
  ENGINE_PID_GET: '/api/engine/pid-get',
  ENGINE_KIND_GET: '/api/engine/kind-get',
  ENGINE_REFRESH_STATUS: '/api/engine/refresh-status',
  // Template
  TEMPLATE_LIST: '/api/template/list',
  TEMPLATE_LIST_BUILTIN: '/api/template/list-builtin',
  TEMPLATE_SAVE_CUSTOM: '/api/template/save-custom',
  TEMPLATE_DELETE_CUSTOM: '/api/template/delete-custom',
  // Log
  LOG_LIST: '/api/log/list',
  LOG_CLEAR: '/api/log/clear',
  NGINX_LOG_TAIL: '/api/nginx-log/tail',
  NGINX_LOG_CLEAR: '/api/nginx-log/clear',
  NGINX_LOG_READ_ENABLED_GET: '/api/nginx-log/read-enabled-get',
  NGINX_LOG_READ_ENABLED_SET: '/api/nginx-log/read-enabled-set',
  // Snapshot
  SNAPSHOT_LIST: '/api/snapshot/list',
  SNAPSHOT_GET: '/api/snapshot/get',
  SNAPSHOT_ROLLBACK: '/api/snapshot/rollback',
  // Health check
  HEALTH_CHECK_NOW: '/api/health/check-now',
  // Port check
  PORT_CHECK: '/api/port/check',
  // Network
  GET_LOCAL_IPS: '/api/network/local-ips',
  // 数据管理
  DATA_WIPE: '/api/data/wipe',
  DATA_EXPORT: '/api/data/export',
  DATA_IMPORT: '/api/data/import',
  // IP 白名单（全局访问控制）
  WHITELIST_LIST: '/api/whitelist/list',
  WHITELIST_ADD: '/api/whitelist/add',
  WHITELIST_REMOVE: '/api/whitelist/remove',
  WHITELIST_TOGGLE_IP: '/api/whitelist/toggle-ip',
  WHITELIST_GET_ENABLED: '/api/whitelist/get-enabled',
  WHITELIST_SET_ENABLED: '/api/whitelist/set-enabled',
} as const

// ============ Log ============
export type LogLevel = 'info' | 'warn' | 'error'
export type LogSource = 'app' | 'nginx-access' | 'nginx-error'

export interface LogEntry {
  id: string
  hostId: string | null
  level: LogLevel
  source: LogSource
  message: string
  timestamp: number
}

// ============ ConfigSnapshot ============
export interface ConfigSnapshot {
  id: string
  hostId: string
  content: string
  reason: string
  createdAt: number
}

export type ApiPath = typeof API[keyof typeof API]
