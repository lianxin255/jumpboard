import type { EngineStatus, EngineTestResult } from '@shared/types'

export type { EngineStatus, EngineTestResult }

export interface NginxEngine {
  start(): Promise<void>
  stop(): Promise<void>
  reload(): Promise<void>
  test(): Promise<EngineTestResult>
  status(): EngineStatus
  writeConf(content: string): Promise<void>
  readConf(): Promise<string>
}

export interface LocalEngineConfig {
  nginxPath: string      // nginx.exe 路径（bundled 或用户指定）
  confPath: string       // 主 conf 路径
  logDir: string
  tempDir: string
}
