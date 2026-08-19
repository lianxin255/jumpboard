import { API } from '@ngj/shared/types'

/**
 * HTTP API 客户端：替代原 Electron contextBridge IPC。
 * 所有方法签名与原 ipc.ts 保持一致（返回 Promise<T>）。
 * PORT 从 import.meta.env.VITE_SERVER_PORT 读，默认 47831。
 */
const PORT = Number(import.meta.env.VITE_SERVER_PORT ?? 47831)
const BASE = `http://127.0.0.1:${PORT}`

async function getJson<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

async function postJson<T>(path: string, body?: any): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

export interface NgjApi {
  // Host
  listHosts: () => Promise<any[]>
  createHost: (input: any) => Promise<any>
  updateHost: (id: string, patch: any) => Promise<any>
  deleteHost: (id: string) => Promise<boolean>
  setDefaultHost: (id: string) => Promise<boolean>
  switchHost: (id: string) => Promise<{ ok: boolean, error?: string | null }>
  testHostConnection: (remoteConfig: any) => Promise<{ ok: boolean, output: string }>
  // Mapping
  listMappings: (hostId: string) => Promise<any[]>
  createMapping: (input: any) => Promise<any>
  updateMapping: (id: string, patch: any) => Promise<any>
  deleteMapping: (id: string) => Promise<boolean>
  // Engine
  startEngine: () => Promise<{ status: string, error: string | null }>
  stopEngine: () => Promise<{ status: string, error: string | null }>
  reloadEngine: () => Promise<{ ok: boolean, output: string }>
  testEngine: () => Promise<{ ok: boolean, output: string }>
  engineStatus: () => Promise<string>
  isEngineDirty: () => Promise<boolean>
  getEnginePid: () => Promise<number | null>
  getEngineKind: () => Promise<{ engineKind: 'local' | 'remote', supportsProcessControl: boolean } | null>
  refreshEngineStatus: () => Promise<string>
  // Template
  listTemplates: () => Promise<any[]>
  listBuiltinTemplates: () => Promise<any[]>
  saveCustomTemplate: (tpl: any) => Promise<boolean>
  deleteCustomTemplate: (id: string) => Promise<boolean>
  // Log
  listLogs: (hostId: string | null, limit: number) => Promise<any[]>
  clearLogs: () => Promise<boolean>
  tailNginxLogs: (lines: number) => Promise<{ access: string[], error: string[] }>
  clearNginxLogs: () => Promise<boolean>
  isNginxLogReadEnabled: () => Promise<boolean>
  setNginxLogReadEnabled: (enabled: boolean) => Promise<boolean>
  // Snapshot
  listSnapshots: (hostId: string) => Promise<any[]>
  getSnapshot: (id: string) => Promise<any>
  rollbackSnapshot: (snapshotId: string) => Promise<{ ok: boolean, output: string }>
  // Health check
  checkMappingNow: (mappingId: string) => Promise<string>
  // Port check
  checkPort: (port: number) => Promise<boolean>
  // 数据管理
  wipeAllData: (confirmToken: string) => Promise<{ ok: boolean, output?: string }>
  exportData: () => Promise<any>
  importData: (payload: any) => Promise<{ ok: boolean, output?: string, hostsImported?: number, mappingsImported?: number, snapshotsImported?: number, logsImported?: number, hostsSkipped?: number, mappingsSkipped?: number }>
  // Network
  getLocalIPs: () => Promise<{ interface: string, address: string, family: string }[]>
  // IP 白名单
  listWhitelist: () => Promise<any[]>
  addWhitelist: (ip: string, note?: string) => Promise<any>
  removeWhitelist: (id: string) => Promise<boolean>
  toggleWhitelistIp: (id: string, enabled: number) => Promise<boolean>
  isWhitelistEnabled: () => Promise<boolean>
  setWhitelistEnabled: (enabled: boolean) => Promise<boolean>
}

export const api: NgjApi = {
  // Host
  listHosts: () => getJson(API.HOST_LIST),
  createHost: input => postJson(API.HOST_CREATE, input),
  updateHost: (id, patch) => postJson(API.HOST_UPDATE, { id, patch }),
  deleteHost: id => postJson(API.HOST_DELETE, { id }),
  setDefaultHost: id => postJson(API.HOST_SET_DEFAULT, { id }),
  switchHost: id => postJson(API.HOST_SWITCH, { id }),
  testHostConnection: remoteConfig => postJson(API.HOST_TEST_CONNECTION, remoteConfig),
  // Mapping
  listMappings: hostId => getJson(`${API.MAPPING_LIST}?hostId=${encodeURIComponent(hostId)}`),
  createMapping: input => postJson(API.MAPPING_CREATE, input),
  updateMapping: (id, patch) => postJson(API.MAPPING_UPDATE, { id, patch }),
  deleteMapping: id => postJson(API.MAPPING_DELETE, { id }),
  // Engine
  startEngine: () => postJson(API.ENGINE_START),
  stopEngine: () => postJson(API.ENGINE_STOP),
  reloadEngine: () => postJson(API.ENGINE_RELOAD),
  testEngine: () => postJson(API.ENGINE_TEST),
  engineStatus: () => getJson(API.ENGINE_STATUS),
  isEngineDirty: () => getJson(API.ENGINE_DIRTY_GET),
  getEnginePid: () => getJson(API.ENGINE_PID_GET),
  getEngineKind: () => getJson(API.ENGINE_KIND_GET),
  refreshEngineStatus: () => postJson(API.ENGINE_REFRESH_STATUS),
  // Template
  listTemplates: () => getJson(API.TEMPLATE_LIST),
  listBuiltinTemplates: () => getJson(API.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: tpl => postJson(API.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: id => postJson(API.TEMPLATE_DELETE_CUSTOM, { id }),
  // Log
  listLogs: (hostId, limit) => getJson(`${API.LOG_LIST}?hostId=${hostId ?? 'null'}&limit=${limit}`),
  clearLogs: () => postJson(API.LOG_CLEAR),
  tailNginxLogs: lines => getJson(`${API.NGINX_LOG_TAIL}?lines=${lines}`),
  clearNginxLogs: () => postJson(API.NGINX_LOG_CLEAR),
  isNginxLogReadEnabled: () => getJson(API.NGINX_LOG_READ_ENABLED_GET),
  setNginxLogReadEnabled: enabled => postJson(API.NGINX_LOG_READ_ENABLED_SET, { enabled }),
  // Snapshot
  listSnapshots: hostId => getJson(`${API.SNAPSHOT_LIST}?hostId=${encodeURIComponent(hostId)}`),
  getSnapshot: id => getJson(`${API.SNAPSHOT_GET}?id=${encodeURIComponent(id)}`),
  rollbackSnapshot: snapshotId => postJson(API.SNAPSHOT_ROLLBACK, { snapshotId }),
  // Health check
  checkMappingNow: mappingId => postJson(API.HEALTH_CHECK_NOW, { mappingId }),
  // Port check
  checkPort: port => getJson(`${API.PORT_CHECK}?port=${port}`),
  // 数据管理
  wipeAllData: confirmToken => postJson(API.DATA_WIPE, { confirmToken }),
  exportData: () => getJson(API.DATA_EXPORT),
  importData: payload => postJson(API.DATA_IMPORT, payload),
  // Network
  getLocalIPs: () => getJson(API.GET_LOCAL_IPS),
  // IP 白名单
  listWhitelist: () => getJson(API.WHITELIST_LIST),
  addWhitelist: (ip, note) => postJson(API.WHITELIST_ADD, { ip, note }),
  removeWhitelist: id => postJson(API.WHITELIST_REMOVE, { id }),
  toggleWhitelistIp: (id, enabled) => postJson(API.WHITELIST_TOGGLE_IP, { id, enabled }),
  isWhitelistEnabled: () => getJson(API.WHITELIST_GET_ENABLED),
  setWhitelistEnabled: enabled => postJson(API.WHITELIST_SET_ENABLED, { enabled }),
}
