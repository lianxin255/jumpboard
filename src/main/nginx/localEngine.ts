import { spawn, execFile, type ChildProcess } from 'child_process'
import { join } from 'path'
import { existsSync, mkdirSync, writeFileSync, readFileSync } from 'fs'
import type { NginxEngine, EngineStatus, EngineTestResult, LocalEngineConfig } from './types'

export class LocalNginxEngine implements NginxEngine {
  private child: ChildProcess | null = null
  private readonly cfg: LocalEngineConfig

  constructor(cfg: LocalEngineConfig) {
    this.cfg = cfg
  }

  status(): EngineStatus {
    // 注意：nginx 自身崩溃时，killed 标志仍为 false，直到 exit 事件触发置空 this.child。
    // 此窗口期（通常 <10ms）status 可能短暂报告 running。GUI 通过错误日志反映真实崩溃。
    return this.child && !this.child.killed ? 'running' : 'stopped'
  }

  async start(): Promise<void> {
    if (this.child && !this.child.killed) {
      throw new Error('nginx already running')
    }
    if (!existsSync(this.cfg.nginxPath)) {
      throw new Error(`nginx binary not found: ${this.cfg.nginxPath}`)
    }
    for (const d of [this.cfg.logDir, this.cfg.tempDir, this.cfg.prefixDir]) {
      if (!existsSync(d)) mkdirSync(d, { recursive: true })
    }
    // nginx -p prefix 指向 prefixDir（data/nginx-home，跟 exe 走、可写、持久）。
    // 不再用 nginx.exe 所在目录（portable 下在临时解压目录，可能只读/被清理）。
    // nginx 启动早期用 <prefix>/logs/error.log 记日志，必须确保 logs 子目录存在。
    const prefixLogsDir = join(this.cfg.prefixDir, 'logs')
    if (!existsSync(prefixLogsDir)) mkdirSync(prefixLogsDir, { recursive: true })
    this.child = spawn(this.cfg.nginxPath, [
      '-c', this.cfg.confPath,
      '-p', this.cfg.prefixDir,
      '-g', 'daemon off;'
    ], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe']
    })

    // 收集 stderr 用于错误诊断（nginx 启动失败的真实原因写在这里）
    let stderrBuf = ''

    // 关键：必须监听 'error' 事件，否则 spawn 失败（EPERM/ENOENT 竞态等）
    // 会触发 unhandled 'error' event 导致 Electron main 进程崩溃
    await new Promise<void>((resolve, reject) => {
      const child = this.child!
      let settled = false

      // 收集 stderr
      if (child.stderr) {
        child.stderr.on('data', (d: Buffer) => { stderrBuf += d.toString() })
      }

      const onExit = (code: number | null) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        child.removeListener('error', onError)
        // 启动窗口期内子进程就退出 = 启动失败。该 child 已死，
        // 必须清空 this.child，否则 status() 会因 killed=false 误报 running。
        if (this.child === child) this.child = null
        const detail = stderrBuf.trim()
        if (code !== 0 && code !== null) {
          reject(new Error(`nginx exited with code ${code}${detail ? '\n' + detail : ''}`))
        } else {
          reject(new Error(`nginx exited immediately after start${detail ? '\n' + detail : ''}`))
        }
      }

      const onError = (err: Error) => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        child.removeListener('exit', onExit)
        this.child = null
        reject(err)
      }

      const timer = setTimeout(() => {
        if (settled) return
        settled = true
        // 启动成功：500ms 后进程仍存活。移除一次性监听器，保留长期的 exit 监听用于状态跟踪
        child.removeListener('exit', onExit)
        child.removeListener('error', onError)
        resolve()
      }, 500)

      child.once('exit', onExit)
      child.once('error', onError)
    })

    // 注册长期 exit 监听，用于 status() 跟踪进程是否退出
    // 关键：捕获具体的子进程实例 c，并用 identity 守卫。
    // 否则旧子进程的 exit 晚到（SIGKILL 是异步的）会误 null 已经指向新子进程的 this.child。
    const c = this.child!
    c.on('exit', () => { if (this.child === c) this.child = null })
  }

  async stop(): Promise<void> {
    if (!this.child) return
    // 捕获本次要停止的子进程实例 c。SIGTERM/SIGKILL 发给 c，
    // null 时必须校验 this.child === c，避免晚到的 exit 误清新子进程引用。
    const c = this.child
    c.kill('SIGTERM')
    await new Promise<void>((resolve) => {
      const t = setTimeout(() => {
        if (this.child === c) this.child!.kill('SIGKILL')
        resolve()
      }, 3000)
      const onExit = () => { clearTimeout(t); if (this.child === c) this.child = null; resolve() }
      c.once('exit', onExit)
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
      execFile(this.cfg.nginxPath, ['-t', '-c', this.cfg.confPath, '-p', this.cfg.prefixDir],
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
