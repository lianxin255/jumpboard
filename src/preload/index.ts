import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '@shared/types'

const api = {
  // Host
  listHosts: () => ipcRenderer.invoke(IPC.HOST_LIST),
  createHost: (input: any) => ipcRenderer.invoke(IPC.HOST_CREATE, input),
  updateHost: (id: string, patch: any) => ipcRenderer.invoke(IPC.HOST_UPDATE, id, patch),
  deleteHost: (id: string) => ipcRenderer.invoke(IPC.HOST_DELETE, id),
  setDefaultHost: (id: string) => ipcRenderer.invoke(IPC.HOST_SET_DEFAULT, id),
  switchHost: (id: string) => ipcRenderer.invoke(IPC.HOST_SWITCH, id),
  // Mapping
  listMappings: (hostId: string) => ipcRenderer.invoke(IPC.MAPPING_LIST, hostId),
  createMapping: (input: any) => ipcRenderer.invoke(IPC.MAPPING_CREATE, input),
  updateMapping: (id: string, patch: any) => ipcRenderer.invoke(IPC.MAPPING_UPDATE, id, patch),
  deleteMapping: (id: string) => ipcRenderer.invoke(IPC.MAPPING_DELETE, id),
  // Engine
  startEngine: () => ipcRenderer.invoke(IPC.ENGINE_START),
  stopEngine: () => ipcRenderer.invoke(IPC.ENGINE_STOP),
  reloadEngine: () => ipcRenderer.invoke(IPC.ENGINE_RELOAD),
  testEngine: () => ipcRenderer.invoke(IPC.ENGINE_TEST),
  engineStatus: () => ipcRenderer.invoke(IPC.ENGINE_STATUS),
  isEngineDirty: () => ipcRenderer.invoke(IPC.ENGINE_DIRTY_GET),
  getEnginePid: () => ipcRenderer.invoke(IPC.ENGINE_PID_GET),
  getEngineKind: () => ipcRenderer.invoke(IPC.ENGINE_KIND_GET),
  // Template
  listTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST),
  listBuiltinTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: (tpl: any) => ipcRenderer.invoke(IPC.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: (id: string) => ipcRenderer.invoke(IPC.TEMPLATE_DELETE_CUSTOM, id),
  // Log
  listLogs: (hostId: string | null, limit: number) => ipcRenderer.invoke(IPC.LOG_LIST, hostId, limit),
  clearLogs: () => ipcRenderer.invoke(IPC.LOG_CLEAR),
  tailNginxLogs: (lines: number) => ipcRenderer.invoke(IPC.NGINX_LOG_TAIL, lines),
  clearNginxLogs: () => ipcRenderer.invoke(IPC.NGINX_LOG_CLEAR),
  isNginxLogReadEnabled: () => ipcRenderer.invoke(IPC.NGINX_LOG_READ_ENABLED_GET),
  setNginxLogReadEnabled: (enabled: boolean) => ipcRenderer.invoke(IPC.NGINX_LOG_READ_ENABLED_SET, enabled),
  // Snapshot
  listSnapshots: (hostId: string) => ipcRenderer.invoke(IPC.SNAPSHOT_LIST, hostId),
  getSnapshot: (id: string) => ipcRenderer.invoke(IPC.SNAPSHOT_GET, id),
  rollbackSnapshot: (snapshotId: string) => ipcRenderer.invoke(IPC.SNAPSHOT_ROLLBACK, snapshotId),
  // Health check
  checkMappingNow: (mappingId: string) => ipcRenderer.invoke(IPC.HEALTH_CHECK_NOW, mappingId),
  // Port check
  checkPort: (port: number) => ipcRenderer.invoke(IPC.PORT_CHECK, port),
  // Host 管理
  testHostConnection: (remoteConfig: any) => ipcRenderer.invoke(IPC.HOST_TEST_CONNECTION, remoteConfig),
  refreshEngineStatus: () => ipcRenderer.invoke(IPC.ENGINE_REFRESH_STATUS),
  // 数据管理
  wipeAllData: (confirmToken: string) => ipcRenderer.invoke(IPC.DATA_WIPE, confirmToken),
  exportData: () => ipcRenderer.invoke(IPC.DATA_EXPORT),
  importData: (payload: any) => ipcRenderer.invoke(IPC.DATA_IMPORT, payload),
  // Network
  getLocalIPs: () => ipcRenderer.invoke(IPC.GET_LOCAL_IPS),
  // IP 白名单
  listWhitelist: () => ipcRenderer.invoke(IPC.WHITELIST_LIST),
  addWhitelist: (ip: string, note?: string) => ipcRenderer.invoke(IPC.WHITELIST_ADD, ip, note),
  removeWhitelist: (id: string) => ipcRenderer.invoke(IPC.WHITELIST_REMOVE, id),
  toggleWhitelistIp: (id: string, enabled: number) => ipcRenderer.invoke(IPC.WHITELIST_TOGGLE_IP, id, enabled),
  isWhitelistEnabled: () => ipcRenderer.invoke(IPC.WHITELIST_GET_ENABLED),
  setWhitelistEnabled: (enabled: boolean) => ipcRenderer.invoke(IPC.WHITELIST_SET_ENABLED, enabled),
}

contextBridge.exposeInMainWorld('electronAPI', api)

export type ElectronAPI = typeof api
