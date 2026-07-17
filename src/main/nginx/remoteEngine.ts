import { SshClient } from '../ssh/client'
import { decryptPassword } from '../crypto'
import type { NginxEngine } from './types'
import type { EngineStatus, EngineTestResult } from '@shared/types'

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
  private readonly ssh: SshClient
  private readonly cfg: RemoteEngineConfig
  private cachedStatus: EngineStatus = 'unknown'

  constructor(cfg: RemoteEngineConfig) {
    this.cfg = cfg
    const password = cfg.passwordEncrypted ? decryptPassword(cfg.passwordEncrypted) : undefined
    this.ssh = new SshClient({
      host: cfg.host,
      port: cfg.port,
      username: cfg.username,
      password
    })
  }

  status(): EngineStatus {
    return this.cachedStatus
  }

  async refreshStatus(): Promise<EngineStatus> {
    try {
      const r = await this.ssh.exec('systemctl is-active nginx 2>/dev/null || pgrep -x nginx >/dev/null && echo active || echo inactive')
      const out = (r.stdout + r.stderr).trim()
      if (out.includes('active') || out.includes('running')) {
        this.cachedStatus = 'running'
      } else {
        this.cachedStatus = 'stopped'
      }
    } catch {
      this.cachedStatus = 'unknown'
    }
    return this.cachedStatus
  }

  async start(): Promise<void> {
    await this.refreshStatus()
  }

  async stop(): Promise<void> {
    await this.refreshStatus()
  }

  async reload(): Promise<void> {
    const r = await this.ssh.exec(this.cfg.reloadCmd)
    if (r.code !== 0 && r.code !== null) {
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

  close(): void {
    this.ssh.close()
  }
}
