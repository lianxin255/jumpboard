import type { ChildProcess } from 'node:child_process';
import type { EngineStatus, EngineTestResult, LocalEngineConfig, NginxEngine } from './types'
import { execFile, spawn } from 'node:child_process'
import { join } from 'node:path'
import { toFwdSlash } from '@ngj/shared/paths'
import { ensureDirSync, outputFileSync, pathExistsSync, readFileSync } from 'fs-extra'

export class LocalNginxEngine implements NginxEngine {
  readonly engineKind = 'local' as const
  readonly supportsProcessControl = true
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

  // local 的 status 实时来自 child，无需异步刷新。refreshStatus 为接口一致性而存在。
  async refreshStatus(): Promise<EngineStatus> {
    return this.status()
  }

  getPid(): number | null {
    return this.child?.pid ?? null
  }

  async start(): Promise<void> {
    if (this.child && !this.child.killed) {
      throw new Error('nginx already running')
    }
    if (!pathExistsSync(this.cfg.nginxPath)) {
      throw new Error(`nginx binary not found: ${this.cfg.nginxPath}`)
    }
    // 所有 nginx 工作目录都在 prefix 下（conf 内容用相对路径 logs/ temp/）
    // 确保 prefix 下的 logs/ temp/ 存在，nginx 才能写日志和 pid
    ensureDirSync(this.cfg.prefixDir)
    ensureDirSync(join(this.cfg.prefixDir, 'logs'))
    ensureDirSync(join(this.cfg.prefixDir, 'temp'))
    ensureDirSync(join(this.cfg.prefixDir, 'conf'))
    // nginx 用 -p prefix + -c conf/nginx.conf 相对路径启动。
    // -c 用相对路径（基于 -p prefix 解析），彻底避开绝对路径的反斜杠转义/中文路径问题。
    // -p 仍需正斜杠（Windows 反斜杠会被 nginx 当转义符），toFwdSlash 抽到 @shared/paths。
    this.child = spawn(this.cfg.nginxPath, [
      '-p',
      toFwdSlash(this.cfg.prefixDir),
      '-c',
      'conf/nginx.conf',
      '-g',
      'daemon off;',
    ], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
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
        if (settled)
          return
        settled = true
        clearTimeout(timer)
        child.removeListener('error', onError)
        // 启动窗口期内子进程就退出 = 启动失败。该 child 已死，
        // 必须清空 this.child，否则 status() 会因 killed=false 误报 running。
        if (this.child === child)
          this.child = null
        const detail = stderrBuf.trim()
        if (code !== 0 && code !== null) {
          reject(new Error(`nginx exited with code ${code}${detail ? `\n${detail}` : ''}`))
        }
        else {
          reject(new Error(`nginx exited immediately after start${detail ? `\n${detail}` : ''}`))
        }
      }

      const onError = (err: Error) => {
        if (settled)
          return
        settled = true
        clearTimeout(timer)
        child.removeListener('exit', onExit)
        this.child = null
        reject(err)
      }

      const timer = setTimeout(() => {
        if (settled)
          return
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
    c.on('exit', () => {
      if (this.child === c)
        this.child = null
    })
  }

  async stop(): Promise<void> {
    if (!this.child)
      return
    const c = this.child
    const pid = c.pid

    if (process.platform === 'win32') {
      // Windows 上 nginx 的进程模型 + Node child_process 有两个已知问题：
      // 1. nginx master 启动的 worker 进程，master 被杀后会变孤儿（nginx trac #2394）
      // 2. Node 的 child.kill() 只杀直接子进程（master），杀不到 worker
      // 唯一可靠方案：taskkill /PID <pid> /T /F —— /T 杀整个进程树（master+worker），/F 强制
      await new Promise<void>((resolve) => {
        execFile('taskkill', ['/PID', String(pid), '/T', '/F'], () => {
          // 不管 taskkill 返回什么，都等待子进程退出
          resolve()
        })
      })
      // taskkill 后给一点时间让进程真正退出
      await this.waitForExit(c, 3000)
    }
    else {
      // Unix: SIGTERM 优雅停止，3 秒后 SIGKILL 兜底
      c.kill('SIGTERM')
      await new Promise<void>((resolve) => {
        const t = setTimeout(() => {
          if (this.child === c)
            this.child!.kill('SIGKILL')
          resolve()
        }, 3000)
        const onExit = () => {
          clearTimeout(t); if (this.child === c)
            this.child = null; resolve()
        }
        c.once('exit', onExit)
      })
    }
  }

  async reload(): Promise<void> {
    if (!this.child)
      throw new Error('nginx not running, cannot reload')
    if (process.platform === 'win32') {
      // Windows: nginx 不响应 SIGHUP。用 nginx -s reload（通过 pid 文件重读配置）
      await this.sendSignal('reload')
    }
    else {
      // Unix: SIGHUP 让 nginx 重读配置。
      // 注意：不用 this.child.kill('SIGHUP')——Node 的 child.kill() 会立即把 child.killed 置 true，
      // 导致 status() 误报 stopped（nginx 实际仍在运行）。直接用 process.kill(pid, sig) 不影响 child 状态。
      const pid = this.child.pid
      if (pid)
        process.kill(pid, 'SIGHUP')
    }
  }

  /**
   * Windows 专用：执行 nginx -s <signal>，通过 pid 文件让 master 响应。
   * signal: 'reload' | 'stop' | 'quit'
   */
  private async sendSignal(signal: 'reload' | 'stop' | 'quit'): Promise<void> {
    return new Promise((resolve) => {
      execFile(this.cfg.nginxPath, ['-p', toFwdSlash(this.cfg.prefixDir), '-c', 'conf/nginx.conf', '-s', signal], () => resolve(), // 不管成功失败都 resolve，reload/stop 的结果靠后续状态判断
      )
    })
  }

  /** 等待子进程退出，超时后强 kill 兜底 */
  private async waitForExit(c: ChildProcess, timeoutMs: number): Promise<void> {
    return new Promise((resolve) => {
      let done = false
      const finish = () => {
        if (!done) {
          done = true; if (this.child === c)
            this.child = null; resolve()
        }
      }
      const t = setTimeout(() => {
        if (this.child === c) {
          try { c.kill('SIGKILL') }
          catch { /* ignore */ }
        }
        finish()
      }, timeoutMs)
      c.once('exit', () => { clearTimeout(t); finish() })
    })
  }

  async test(): Promise<EngineTestResult> {
    // 与 start() 一致：先确保 prefix 下的 logs/ temp/ conf/ 存在
    // nginx -t 会尝试打开 logs/error.log 和 temp/nginx.pid，目录缺失则 test 失败
    ensureDirSync(this.cfg.prefixDir)
    ensureDirSync(join(this.cfg.prefixDir, 'logs'))
    ensureDirSync(join(this.cfg.prefixDir, 'temp'))
    ensureDirSync(join(this.cfg.prefixDir, 'conf'))
    return new Promise((resolve) => {
      // 与 start() 一致：-p 正斜杠 + -c 相对路径
      execFile(this.cfg.nginxPath, ['-t', '-p', toFwdSlash(this.cfg.prefixDir), '-c', 'conf/nginx.conf'], (err: any, stdout: string, stderr: string) => {
        const output = (stdout + stderr).trim()
        if (err)
          resolve({ ok: false, output })
        else resolve({ ok: true, output })
      })
    })
  }

  async writeConf(content: string): Promise<void> {
    outputFileSync(this.cfg.confPath, content)
  }

  async readConf(): Promise<string> {
    if (!pathExistsSync(this.cfg.confPath))
      return ''
    return readFileSync(this.cfg.confPath, 'utf-8')
  }
}
