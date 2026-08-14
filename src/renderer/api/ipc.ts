export interface ElectronAPI {
  listHosts(): Promise<any[]>
  createHost(input: any): Promise<any>
  updateHost(id: string, patch: any): Promise<any>
  deleteHost(id: string): Promise<boolean>
  setDefaultHost(id: string): Promise<boolean>
  switchHost(id: string): Promise<{ ok: boolean; error?: string | null }>
  listMappings(hostId: string): Promise<any[]>
  createMapping(input: any): Promise<any>
  updateMapping(id: string, patch: any): Promise<any>
  deleteMapping(id: string): Promise<boolean>
  startEngine(): Promise<{ status: string; error: string | null }>
  stopEngine(): Promise<{ status: string; error: string | null }>
  reloadEngine(): Promise<{ ok: boolean; output: string }>
  testEngine(): Promise<{ ok: boolean; output: string }>
  engineStatus(): Promise<string>
  isEngineDirty(): Promise<boolean>
  getEnginePid(): Promise<number | null>
  getEngineKind(): Promise<{ engineKind: 'local' | 'remote'; supportsProcessControl: boolean } | null>
  // Template
  listTemplates(): Promise<any[]>
  listBuiltinTemplates(): Promise<any[]>
  saveCustomTemplate(tpl: any): Promise<boolean>
  deleteCustomTemplate(id: string): Promise<boolean>
  // Log
  listLogs(hostId: string | null, limit: number): Promise<any[]>
  clearLogs(): Promise<boolean>
  tailNginxLogs(lines: number): Promise<{ access: string[]; error: string[] }>
  clearNginxLogs(): Promise<boolean>
  isNginxLogReadEnabled(): Promise<boolean>
  setNginxLogReadEnabled(enabled: boolean): Promise<boolean>
  // Snapshot
  listSnapshots(hostId: string): Promise<any[]>
  getSnapshot(id: string): Promise<any>
  rollbackSnapshot(snapshotId: string): Promise<{ ok: boolean; output: string }>
  // Health check
  checkMappingNow(mappingId: string): Promise<string>
  // Port check
  checkPort(port: number): Promise<boolean>
  // Host 管理
  testHostConnection(remoteConfig: any): Promise<{ ok: boolean; output: string }>
  refreshEngineStatus(): Promise<string>
  // 数据管理
  wipeAllData(confirmToken: string): Promise<{ ok: boolean; output?: string }>
  exportData(): Promise<any>
  importData(payload: any): Promise<{ ok: boolean; output?: string; hostsImported?: number; mappingsImported?: number; snapshotsImported?: number; logsImported?: number; hostsSkipped?: number; mappingsSkipped?: number }>
  // Network
  getLocalIPs(): Promise<{ interface: string; address: string; family: string }[]>
  // IP 白名单
  listWhitelist(): Promise<any[]>
  addWhitelist(ip: string, note?: string): Promise<any>
  removeWhitelist(id: string): Promise<boolean>
  toggleWhitelistIp(id: string, enabled: number): Promise<boolean>
  isWhitelistEnabled(): Promise<boolean>
  setWhitelistEnabled(enabled: boolean): Promise<boolean>
}

declare global {
  interface Window { electronAPI: ElectronAPI }
}

export const api: ElectronAPI = window.electronAPI
