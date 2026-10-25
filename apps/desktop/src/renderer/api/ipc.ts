import { API } from '@ngj/shared/types'

/**
 * IPC API 客户端：通过 Electron contextBridge 暴露的 ipcRenderer.invoke 调用后端。
 * channel 名复用 @ngj/shared/types 的 API 路径常量。
 * 所有方法签名与原 http.ts 保持一致（返回 Promise<T>）。
 */

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

const ipc = (window as any).electronAPI as {
  invoke: (channel: string, ...args: any[]) => Promise<any>
}

export const api: NgjApi = {
  // Host
  listHosts: () => ipc.invoke(API.HOST_LIST),
  createHost: input => ipc.invoke(API.HOST_CREATE, input),
  updateHost: (id, patch) => ipc.invoke(API.HOST_UPDATE, id, patch),
  deleteHost: id => ipc.invoke(API.HOST_DELETE, id),
  setDefaultHost: id => ipc.invoke(API.HOST_SET_DEFAULT, id),
  switchHost: id => ipc.invoke(API.HOST_SWITCH, id),
  testHostConnection: remoteConfig => ipc.invoke(API.HOST_TEST_CONNECTION, remoteConfig),
  // Mapping
  listMappings: hostId => ipc.invoke(API.MAPPING_LIST, hostId),
  createMapping: input => ipc.invoke(API.MAPPING_CREATE, input),
  updateMapping: (id, patch) => ipc.invoke(API.MAPPING_UPDATE, id, patch),
  deleteMapping: id => ipc.invoke(API.MAPPING_DELETE, id),
  // Engine
  startEngine: () => ipc.invoke(API.ENGINE_START),
  stopEngine: () => ipc.invoke(API.ENGINE_STOP),
  reloadEngine: () => ipc.invoke(API.ENGINE_RELOAD),
  testEngine: () => ipc.invoke(API.ENGINE_TEST),
  engineStatus: () => ipc.invoke(API.ENGINE_STATUS),
  isEngineDirty: () => ipc.invoke(API.ENGINE_DIRTY_GET),
  getEnginePid: () => ipc.invoke(API.ENGINE_PID_GET),
  getEngineKind: () => ipc.invoke(API.ENGINE_KIND_GET),
  refreshEngineStatus: () => ipc.invoke(API.ENGINE_REFRESH_STATUS),
  // Template
  listTemplates: () => ipc.invoke(API.TEMPLATE_LIST),
  listBuiltinTemplates: () => ipc.invoke(API.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: tpl => ipc.invoke(API.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: id => ipc.invoke(API.TEMPLATE_DELETE_CUSTOM, id),
  // Log
  listLogs: (hostId, limit) => ipc.invoke(API.LOG_LIST, hostId, limit),
  clearLogs: () => ipc.invoke(API.LOG_CLEAR),
  tailNginxLogs: lines => ipc.invoke(API.NGINX_LOG_TAIL, lines),
  clearNginxLogs: () => ipc.invoke(API.NGINX_LOG_CLEAR),
  isNginxLogReadEnabled: () => ipc.invoke(API.NGINX_LOG_READ_ENABLED_GET),
  setNginxLogReadEnabled: enabled => ipc.invoke(API.NGINX_LOG_READ_ENABLED_SET, enabled),
  // Snapshot
  listSnapshots: hostId => ipc.invoke(API.SNAPSHOT_LIST, hostId),
  getSnapshot: id => ipc.invoke(API.SNAPSHOT_GET, id),
  rollbackSnapshot: snapshotId => ipc.invoke(API.SNAPSHOT_ROLLBACK, snapshotId),
  // Health check
  checkMappingNow: mappingId => ipc.invoke(API.HEALTH_CHECK_NOW, mappingId),
  // Port check
  checkPort: port => ipc.invoke(API.PORT_CHECK, port),
  // 数据管理
  wipeAllData: confirmToken => ipc.invoke(API.DATA_WIPE, confirmToken),
  exportData: () => ipc.invoke(API.DATA_EXPORT),
  importData: payload => ipc.invoke(API.DATA_IMPORT, payload),
  // Network
  getLocalIPs: () => ipc.invoke(API.GET_LOCAL_IPS),
  // IP 白名单
  listWhitelist: () => ipc.invoke(API.WHITELIST_LIST),
  addWhitelist: (ip, note) => ipc.invoke(API.WHITELIST_ADD, ip, note),
  removeWhitelist: id => ipc.invoke(API.WHITELIST_REMOVE, id),
  toggleWhitelistIp: (id, enabled) => ipc.invoke(API.WHITELIST_TOGGLE_IP, id, enabled),
  isWhitelistEnabled: () => ipc.invoke(API.WHITELIST_GET_ENABLED),
  setWhitelistEnabled: enabled => ipc.invoke(API.WHITELIST_SET_ENABLED, enabled),
}
