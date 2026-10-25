import { contextBridge, ipcRenderer } from 'electron'

/**
 * Preload 脚本：通过 contextBridge 暴露安全的 IPC 调用接口。
 * renderer 只能通过 window.electronAPI.invoke(channel, ...args) 调用后端，
 * 无法直接访问 ipcRenderer 或 Node.js API（contextIsolation）。
 */
const api = {
  invoke: (channel: string, ...args: any[]) => ipcRenderer.invoke(channel, ...args),
  on: (channel: string, listener: (...args: any[]) => void) => {
    ipcRenderer.on(channel, (_event, ...args) => listener(...args))
  },
}

if (process.contextIsolated) {
  try {
    contextBridge.exposeInMainWorld('electronAPI', api)
  }
  catch (error) {
    console.error(error)
  }
}
else {
  // @ts-expect-error window.electronAPI
  window.electronAPI = api
}

export type ElectronAPI = typeof api
