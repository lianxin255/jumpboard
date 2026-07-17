export interface ElectronAPI {
  listHosts(): Promise<any[]>
  createHost(input: any): Promise<any>
  updateHost(id: string, patch: any): Promise<any>
  deleteHost(id: string): Promise<boolean>
  listMappings(hostId: string): Promise<any[]>
  createMapping(input: any): Promise<any>
  updateMapping(id: string, patch: any): Promise<any>
  deleteMapping(id: string): Promise<boolean>
  startEngine(): Promise<string>
  stopEngine(): Promise<string>
  reloadEngine(): Promise<{ ok: boolean; output: string }>
  testEngine(): Promise<{ ok: boolean; output: string }>
  engineStatus(): Promise<string>
}

declare global {
  interface Window { electronAPI: ElectronAPI }
}

export const api: ElectronAPI = window.electronAPI
