export interface ElectronAPI {
  invoke: (channel: string, ...args: any[]) => Promise<any>
  on: (channel: string, listener: (...args: any[]) => void) => void
}

declare global {
  interface Window {
    electronAPI: ElectronAPI
  }
}
