import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '@shared/types'

const api = {
  // Host
  listHosts: () => ipcRenderer.invoke(IPC.HOST_LIST),
  createHost: (input: any) => ipcRenderer.invoke(IPC.HOST_CREATE, input),
  updateHost: (id: string, patch: any) => ipcRenderer.invoke(IPC.HOST_UPDATE, id, patch),
  deleteHost: (id: string) => ipcRenderer.invoke(IPC.HOST_DELETE, id),
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
  // Template
  listTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST),
  listBuiltinTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: (tpl: any) => ipcRenderer.invoke(IPC.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: (id: string) => ipcRenderer.invoke(IPC.TEMPLATE_DELETE_CUSTOM, id),
}

contextBridge.exposeInMainWorld('electronAPI', api)

export type ElectronAPI = typeof api
