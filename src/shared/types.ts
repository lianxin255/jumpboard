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
  nginxPath: string       // 可空，空则用 bundled
  confDir: string         // 可空，空则用 data/nginx-conf
  logDir: string          // 可空
}

export interface RemoteHostConfig {
  host: string
  port: number            // SSH 端口，默认 22
  username: string
  authType: 'password' | 'key'
  passwordEncrypted: string | null  // safeStorage 加密后 base64
  keyPath: string | null
  nginxPath: string
  confPath: string        // 远程 conf 主文件路径
  reloadCmd: string       // 默认 nginx -s reload
  testCmd: string         // 默认 nginx -t
}

// ============ Mapping ============
export type Protocol = 'tcp' | 'udp' | 'http' | 'https'
export type HealthStatus = 'healthy' | 'unhealthy' | 'unknown' | 'checking'

export interface Mapping {
  id: string
  hostId: string
  name: string
  enabled: number         // 0 | 1
  protocol: Protocol
  listenPort: number | null
  serverNames: string | null
  listenPath: string | null
  upstreamHost: string
  upstreamPort: number
  tlsCertPath: string | null
  tlsKeyPath: string | null
  redirectHttp: number    // 0 | 1
  extraDirectives: string | null
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

// ============ IPC 通道 ============
export const IPC = {
  // Host
  HOST_LIST: 'host:list',
  HOST_CREATE: 'host:create',
  HOST_UPDATE: 'host:update',
  HOST_DELETE: 'host:delete',
  HOST_SET_DEFAULT: 'host:set-default',
  HOST_SWITCH: 'host:switch',
  // Mapping
  MAPPING_LIST: 'mapping:list',
  MAPPING_CREATE: 'mapping:create',
  MAPPING_UPDATE: 'mapping:update',
  MAPPING_DELETE: 'mapping:delete',
  // Engine
  ENGINE_START: 'engine:start',
  ENGINE_STOP: 'engine:stop',
  ENGINE_RELOAD: 'engine:reload',
  ENGINE_TEST: 'engine:test',
  ENGINE_STATUS: 'engine:status',
  ENGINE_DIRTY_GET: 'engine:dirty-get',
  ENGINE_PID_GET: 'engine:pid-get',
  // Template
  TEMPLATE_LIST: 'template:list',
  TEMPLATE_LIST_BUILTIN: 'template:list-builtin',
  TEMPLATE_SAVE_CUSTOM: 'template:save-custom',
  TEMPLATE_DELETE_CUSTOM: 'template:delete-custom',
  // Log
  LOG_LIST: 'log:list',
  LOG_ADD: 'log:add',
  NGINX_LOG_TAIL: 'nginx:log-tail',
  // Snapshot
  SNAPSHOT_LIST: 'snapshot:list',
  SNAPSHOT_GET: 'snapshot:get',
  SNAPSHOT_ROLLBACK: 'snapshot:rollback',
  // Health check
  HEALTH_CHECK_NOW: 'health:check-now',
  // Port check
  PORT_CHECK: 'port:check',
  // Network
  GET_LOCAL_IPS: 'network:get-local-ips',
  // Host 管理
  HOST_TEST_CONNECTION: 'host:test-connection',
  ENGINE_REFRESH_STATUS: 'engine:refresh-status',
  // 数据管理
  DATA_WIPE: 'data:wipe',
  DATA_EXPORT: 'data:export',
  DATA_IMPORT: 'data:import',
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

export type IpcChannel = typeof IPC[keyof typeof IPC]
