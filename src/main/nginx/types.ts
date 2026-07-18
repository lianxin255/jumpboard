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

  /** 可选：清理资源（如 SSH 连接）。LocalEngine 不需要，RemoteEngine 需要。 */
  close?(): void
}

export interface LocalEngineConfig {
  nginxPath: string      // nginx.exe 路径（bundled 或用户指定）
  confPath: string       // 主 conf 路径
  logDir: string         // nginx error/access log 目录
  tempDir: string        // nginx pid + temp 文件目录
  prefixDir: string      // nginx -p prefix（家目录，需可写，含 logs/ + mime.types）
}
