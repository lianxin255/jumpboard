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
  // Template
  listTemplates(): Promise<any[]>
  listBuiltinTemplates(): Promise<any[]>
  saveCustomTemplate(tpl: any): Promise<boolean>
  deleteCustomTemplate(id: string): Promise<boolean>
  // Log
  listLogs(hostId: string | null, limit: number): Promise<any[]>
  // Snapshot
  listSnapshots(hostId: string): Promise<any[]>
  getSnapshot(id: string): Promise<any>
  rollbackSnapshot(snapshotId: string): Promise<{ ok: boolean; output: string }>
  // Health check
  checkMappingNow(mappingId: string): Promise<string>
  // Port check
  checkPort(port: number): Promise<boolean>
}

declare global {
  interface Window { electronAPI: ElectronAPI }
}

export const api: ElectronAPI = window.electronAPI
