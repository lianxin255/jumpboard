import { spawn, execFile, type ChildProcess } from 'child_process'
import { existsSync, mkdirSync, writeFileSync, readFileSync } from 'fs'
import type { NginxEngine, EngineStatus, EngineTestResult, LocalEngineConfig } from './types'

export class LocalNginxEngine implements NginxEngine {
  private child: ChildProcess | null = null
  private readonly cfg: LocalEngineConfig

  constructor(cfg: LocalEngineConfig) {
    this.cfg = cfg
  }

  status(): EngineStatus {
    return this.child && !this.child.killed ? 'running' : 'stopped'
  }

  async start(): Promise<void> {
    if (this.child && !this.child.killed) {
      throw new Error('nginx already running')
    }
    if (!existsSync(this.cfg.nginxPath)) {
      throw new Error(`nginx binary not found: ${this.cfg.nginxPath}`)
    }
    for (const d of [this.cfg.logDir, this.cfg.tempDir]) {
      if (!existsSync(d)) mkdirSync(d, { recursive: true })
    }
    // nginx 用 -g 'daemon off;' 前台运行，便于进程生命周期管理
    this.child = spawn(this.cfg.nginxPath, [
      '-c', this.cfg.confPath,
      '-p', this.cfg.tempDir,
      '-g', 'daemon off;'
    ], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe']
    })

    this.child.on('exit', () => { this.child = null })

    // 等待一小段时间确认进程还在（没立即崩）
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        if (this.child && !this.child.killed) resolve()
        else reject(new Error('nginx exited immediately after start'))
      }, 500)
      this.child!.once('exit', (code) => {
        if (code !== 0 && code !== null) {
          clearTimeout(timer)
          reject(new Error(`nginx exited with code ${code}`))
        }
      })
    })
  }

  async stop(): Promise<void> {
    if (!this.child) return
    this.child.kill('SIGTERM')
    await new Promise<void>((resolve) => {
      const t = setTimeout(() => {
        if (this.child) this.child!.kill('SIGKILL')
        resolve()
      }, 3000)
      const onExit = () => { clearTimeout(t); this.child = null; resolve() }
      this.child!.once('exit', onExit)
    })
  }

  async reload(): Promise<void> {
    if (!this.child) throw new Error('nginx not running, cannot reload')
    // 注意：Windows 对 SIGHUP 支持很弱，真正的 Windows reload 需要 `nginx -s reload` + pid 文件。
    // 此处 SIGHUP 适用于 macOS/Linux（开发环境）。Windows reload 在 Task 12 / Plan 6 打包时另行处理。
    this.child.kill('SIGHUP')
  }

  async test(): Promise<EngineTestResult> {
    return new Promise((resolve) => {
      execFile(this.cfg.nginxPath, ['-t', '-c', this.cfg.confPath, '-p', this.cfg.tempDir],
        (err: any, stdout: string, stderr: string) => {
          const output = (stdout + stderr).trim()
          if (err) resolve({ ok: false, output })
          else resolve({ ok: true, output })
        })
    })
  }

  async writeConf(content: string): Promise<void> {
    writeFileSync(this.cfg.confPath, content, 'utf-8')
  }

  async readConf(): Promise<string> {
    if (!existsSync(this.cfg.confPath)) return ''
    return readFileSync(this.cfg.confPath, 'utf-8')
  }
}
