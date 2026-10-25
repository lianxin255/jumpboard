import type { ChildProcess } from 'node:child_process'
import { execFile } from 'node:child_process'

/**
 * 跨平台强杀整个进程树。
 *
 * Windows 上 nginx master 被杀后 worker 会变孤儿（nginx trac #2394），且 Node 的
 * child.kill() 只杀直接子进程——唯一可靠方案是 taskkill /PID <pid> /T /F。
 * Unix 用 SIGKILL（调用方需要优雅停止时应先发 SIGTERM 再兜底）。
 */
export function killTree(pid: number): Promise<void> {
  return new Promise((resolve) => {
    if (process.platform === 'win32') {
      execFile('taskkill', ['/PID', String(pid), '/T', '/F'], () => resolve())
    }
    else {
      try { process.kill(pid, 'SIGKILL') }
      catch { /* 进程已不存在 */ }
      resolve()
    }
  })
}

/** 进程是否存活（kill(pid, 0) 只探测不发信号） */
export function isPidAlive(pid: number): boolean {
  try {
    process.kill(pid, 0)
    return true
  }
  catch {
    return false
  }
}

/** 等待 ChildProcess 退出，超时后 killTree 兜底 */
export function waitForChildExit(c: ChildProcess, timeoutMs: number): Promise<void> {
  return new Promise((resolve) => {
    let done = false
    const finish = () => {
      if (!done) {
        done = true
        resolve()
      }
    }
    const t = setTimeout(() => {
      if (c.pid)
        void killTree(c.pid)
      finish()
    }, timeoutMs)
    c.once('exit', () => { clearTimeout(t); finish() })
  })
}
