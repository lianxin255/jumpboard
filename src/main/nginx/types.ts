import type { EngineStatus, EngineTestResult } from '@shared/types'

export type { EngineStatus, EngineTestResult }

export interface NginxEngine {
  /** 引擎类型：local=本机子进程，remote=远程 SSH。调用方据此分支，不用 as any 探测方法。 */
  readonly engineKind: 'local' | 'remote'
  /**
   * 是否支持进程级控制（start/stop 真正启停 nginx 进程）。
   * - local：true，spawn/kill 本机 nginx
   * - remote：false，远程 nginx 由 systemd/管理员管理，我们只能 reload/test，不能启停
   * UI 据此决定是否显示「启动/停止」按钮——remote 场景下点这些按钮是无意义的假动作。
   */
  readonly supportsProcessControl: boolean

  start(): Promise<void>
  stop(): Promise<void>
  reload(): Promise<void>
  test(): Promise<EngineTestResult>
  status(): EngineStatus
  /** 刷新状态：local 是 no-op（status 实时来自 child），remote 走 SSH 探测。 */
  refreshStatus(): Promise<EngineStatus>
  writeConf(content: string): Promise<void>
  readConf(): Promise<string>

  /** 可选：清理资源（如 SSH 连接）。LocalEngine 不需要，RemoteEngine 需要。 */
  close?(): void

  /** 可选：获取 nginx 主进程 PID。本地同步，remote 异步。 */
  getPid?(): number | null | Promise<number | null>
}

export interface LocalEngineConfig {
  nginxPath: string      // nginx.exe 路径（bundled 或用户指定）
  confPath: string       // 主 conf 路径
  logDir: string         // nginx error/access log 目录
  tempDir: string        // nginx pid + temp 文件目录
  prefixDir: string      // nginx -p prefix（家目录，需可写，含 logs/ + mime.types）
}
