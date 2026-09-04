import type { ChildProcess } from 'node:child_process'
import { EventEmitter } from 'node:events'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LocalNginxEngine } from './localEngine'

/**
 * localEngine win32 分支的跨平台覆盖。
 *
 * 背景：win32 专有逻辑（stop 用 taskkill /T /F 杀进程树、reload 用 nginx -s reload）
 * 无法在 macOS/Linux CI 上被真实进程用例触达，真 Windows 上又因 CVE-2024-27980
 * （spawn .bat 需 shell:true）跑不了真实进程用例——这两个分支此前在所有平台都零覆盖。
 *
 * 做法：mock child_process + 把 process.platform 置为 win32，
 * 在任何平台上断言 win32 分支调用正确的进程控制命令。
 */

const { spawnMock, execFileMock } = vi.hoisted(() => ({
  spawnMock: vi.fn(),
  execFileMock: vi.fn(),
}))

vi.mock('node:child_process', () => ({
  spawn: spawnMock,
  execFile: execFileMock,
}))

const platformDesc = Object.getOwnPropertyDescriptor(process, 'platform')!

function setPlatform(value: string): void {
  Object.defineProperty(process, 'platform', { value })
}

afterEach(() => {
  Object.defineProperty(process, 'platform', platformDesc)
  vi.clearAllMocks()
})

function makeEngine(): LocalNginxEngine {
  // prefixDir 必须真实存在（test()/start() 会 ensureDirSync），其余路径仅作为参数断言用
  const prefix = join(tmpdir(), `ngj-win32-mock-${Date.now()}`)
  return new LocalNginxEngine({
    nginxPath: '/fake/nginx',
    confPath: join(prefix, 'conf', 'nginx.conf'),
    logDir: join(prefix, 'logs'),
    tempDir: join(prefix, 'temp'),
    prefixDir: prefix,
  })
}

function makeFakeChild(pid = 4242): ChildProcess {
  const c = new EventEmitter() as any
  c.pid = pid
  c.killed = false
  c.kill = vi.fn(() => {
    c.killed = true
    return true
  })
  c.stderr = new EventEmitter()
  return c as ChildProcess
}

/** execFile mock：同步成功回调（taskkill / nginx -s 都是发完命令即返回） */
function execFileSucceeds(): void {
  execFileMock.mockImplementation((_cmd: any, _args: any, cb: any) => cb(null, '', ''))
}

const flush = () => new Promise(r => setTimeout(r, 0))

describe('localNginxEngine win32 分支（mock child_process，跨平台）', () => {
  it('win32 stop：调用 taskkill /PID <pid> /T /F 杀整个进程树，随后进程退出置 stopped', async () => {
    setPlatform('win32')
    execFileSucceeds()
    const eng = makeEngine()
    const child = makeFakeChild()
    ;(eng as any).child = child

    const stopping = eng.stop()
    await flush()
    expect(execFileMock).toHaveBeenCalledWith('taskkill', ['/PID', '4242', '/T', '/F'], expect.any(Function))

    child.emit('exit', 0)
    await stopping
    expect(eng.status()).toBe('stopped')
    expect(child.kill).not.toHaveBeenCalled()
  })

  it('win32 stop：taskkill 后进程 3s 未退出 → 再次 taskkill 兜底', async () => {
    vi.useFakeTimers()
    try {
      setPlatform('win32')
      execFileSucceeds()
      const eng = makeEngine()
      const child = makeFakeChild()
      ;(eng as any).child = child

      const stopping = eng.stop()
      const tick = vi.advanceTimersByTimeAsync(3100)
      await tick
      // 兜底走 killTree → 仍是 taskkill（杀整棵树），不是 SIGKILL
      const taskkillCalls = execFileMock.mock.calls.filter((c: any[]) => c[0] === 'taskkill')
      expect(taskkillCalls.length).toBeGreaterThanOrEqual(2)

      child.emit('exit', null)
      await stopping
      expect(eng.status()).toBe('stopped')
    }
    finally {
      vi.useRealTimers()
    }
  })

  it('win32 reload：走 nginx -s reload（Windows 不响应 SIGHUP），且不影响 running 状态', async () => {
    setPlatform('win32')
    execFileSucceeds()
    const eng = makeEngine()
    ;(eng as any).child = makeFakeChild()

    await eng.reload()
    expect(execFileMock).toHaveBeenCalledWith(
      '/fake/nginx',
      ['-p', expect.stringMatching(/ngj-win32-mock-/), '-c', 'conf/nginx.conf', '-s', 'reload'],
      expect.any(Function),
    )
    expect(eng.status()).toBe('running')
  })

  it('非 win32 stop：SIGTERM 优雅停止，不调用 taskkill', async () => {
    setPlatform('darwin')
    const eng = makeEngine()
    const child = makeFakeChild()
    ;(eng as any).child = child
    execFileSucceeds()

    const stopping = eng.stop()
    await flush()
    expect(child.kill).toHaveBeenCalledWith('SIGTERM')
    expect(execFileMock).not.toHaveBeenCalled()

    child.emit('exit', 0)
    await stopping
    expect(eng.status()).toBe('stopped')
  })

  it('win32 test：nginx -t 带 -p 正斜杠 prefix + 相对 conf', async () => {
    setPlatform('win32')
    execFileSucceeds()
    const eng = makeEngine()

    const r = await eng.test()
    expect(r.ok).toBe(true)
    expect(execFileMock).toHaveBeenCalledWith(
      '/fake/nginx',
      ['-t', '-p', expect.stringMatching(/ngj-win32-mock-/), '-c', 'conf/nginx.conf'],
      expect.any(Function),
    )
  })

  it('win32 test：nginx -t 失败 → ok:false 带输出', async () => {
    setPlatform('win32')
    execFileMock.mockImplementation((_cmd: any, _args: any, cb: any) => cb(new Error('emerg: bad directive'), '', 'emerg: bad directive'))
    const eng = makeEngine()

    const r = await eng.test()
    expect(r.ok).toBe(false)
    expect(r.output).toContain('bad directive')
  })
})
