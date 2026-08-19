import type { EngineStatus, EngineTestResult } from '@ngj/shared/types'
import type { NginxEngine } from './types'
import { decryptPassword } from '../crypto'
import { SshClient } from '../ssh/client'

export interface RemoteEngineConfig {
  host: string
  port: number
  username: string
  passwordEncrypted?: string | null
  privateKeyPath?: string | null
  nginxPath: string
  confPath: string
  reloadCmd: string
  testCmd: string
}

export class RemoteNginxEngine implements NginxEngine {
  readonly engineKind = 'remote' as const
  readonly supportsProcessControl = false
  private readonly ssh: SshClient
  private readonly cfg: RemoteEngineConfig
  private cachedStatus: EngineStatus = 'unknown'

  /**
   * @param cfg 远程引擎配置
   * @param sshClient 可选注入的 SshClient（测试用）。生产环境不传，内部 new SshClient。
   *                 解耦后测试可直接传 mock client，不必 mock 整个 @main/ssh/client 模块。
   */
  constructor(cfg: RemoteEngineConfig, sshClient?: SshClient) {
    this.cfg = cfg
    if (sshClient) {
      this.ssh = sshClient
    }
    else {
      const password = cfg.passwordEncrypted ? decryptPassword(cfg.passwordEncrypted) : undefined
      this.ssh = new SshClient({
        host: cfg.host,
        port: cfg.port,
        username: cfg.username,
        password,
      })
    }
  }

  status(): EngineStatus {
    return this.cachedStatus
  }

  async refreshStatus(): Promise<EngineStatus> {
    try {
      const r = await this.ssh.exec('systemctl is-active nginx 2>/dev/null || pgrep -x nginx >/dev/null && echo active || echo inactive')
      const out = (r.stdout + r.stderr).trim()
      // 按行精确匹配，避免 'inactive'.includes('active') 误判
      const lines = out.split('\n').map(l => l.trim())
      if (lines.includes('active') || lines.includes('running')) {
        this.cachedStatus = 'running'
      }
      else {
        this.cachedStatus = 'stopped'
      }
    }
    catch {
      this.cachedStatus = 'unknown'
    }
    return this.cachedStatus
  }

  // remote 不支持进程级启停：远程 nginx 由 systemd/管理员管理。
  // 之前这里是假动作（只 refreshStatus），UI 会显示「已启动」误导用户。现在明示不支持。
  // UI 应根据 supportsProcessControl 隐藏启停按钮；若仍被调用则抛错而非静默成功。
  async start(): Promise<void> {
    throw new Error('远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理')
  }

  async stop(): Promise<void> {
    throw new Error('远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理')
  }

  async reload(): Promise<void> {
    const r = await this.ssh.exec(this.cfg.reloadCmd)
    // code !== 0（含 null：SSH 信号/超时断开）都当失败，与 test() 的语义一致
    if (r.code !== 0) {
      throw new Error(`reload 失败 (code ${r.code}): ${r.stderr || r.stdout}`)
    }
  }

  async test(): Promise<EngineTestResult> {
    const r = await this.ssh.exec(this.cfg.testCmd)
    const output = (r.stdout + r.stderr).trim()
    return { ok: r.code === 0, output }
  }

  async writeConf(content: string): Promise<void> {
    await this.ssh.writeFile(this.cfg.confPath, content)
  }

  async readConf(): Promise<string> {
    return await this.ssh.readFile(this.cfg.confPath)
  }

  async getPid(): Promise<number | null> {
    try {
      // cat pidfile 是 nginx 规范做法，pgrep -xo（最老进程=master）兜底
      const r = await this.ssh.exec('cat /run/nginx.pid 2>/dev/null || pgrep -xo nginx')
      const out = r.stdout.trim()
      const n = parseInt(out, 10)
      return Number.isFinite(n) && n > 0 ? n : null
    }
    catch {
      return null
    }
  }

  close(): void {
    this.ssh.close()
  }
}
