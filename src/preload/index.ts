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
  // Template
  listTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST),
  listBuiltinTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: (tpl: any) => ipcRenderer.invoke(IPC.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: (id: string) => ipcRenderer.invoke(IPC.TEMPLATE_DELETE_CUSTOM, id),
  // Log
  listLogs: (hostId: string | null, limit: number) => ipcRenderer.invoke(IPC.LOG_LIST, hostId, limit),
  tailNginxLogs: (lines: number) => ipcRenderer.invoke(IPC.NGINX_LOG_TAIL, lines),
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
  wipeAllData: () => ipcRenderer.invoke(IPC.DATA_WIPE),
  exportData: () => ipcRenderer.invoke(IPC.DATA_EXPORT),
  importData: (payload: any) => ipcRenderer.invoke(IPC.DATA_IMPORT, payload),
  // Network
  getLocalIPs: () => ipcRenderer.invoke(IPC.GET_LOCAL_IPS),
}

contextBridge.exposeInMainWorld('electronAPI', api)

export type ElectronAPI = typeof api
