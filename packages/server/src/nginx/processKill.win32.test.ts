import { afterEach, describe, expect, it, vi } from 'vitest'
import { killTree } from './processKill'

/**
 * killTree 的 win32 分支（taskkill /T /F）在 macOS/Linux 无法被真实平台触达，
 * mock child_process + 置平台为 win32，在所有平台上断言调用契约。
 */

const { execFileMock } = vi.hoisted(() => ({ execFileMock: vi.fn() }))

vi.mock('node:child_process', () => ({
  execFile: execFileMock,
}))

const platformDesc = Object.getOwnPropertyDescriptor(process, 'platform')!

afterEach(() => {
  Object.defineProperty(process, 'platform', platformDesc)
  vi.clearAllMocks()
})

describe('killTree win32 分支（mock，跨平台）', () => {
  it('win32：调用 taskkill /PID <pid> /T /F 杀整棵进程树', async () => {
    Object.defineProperty(process, 'platform', { value: 'win32' })
    execFileMock.mockImplementation((_cmd: any, _args: any, cb: any) => cb(null, '', ''))

    await killTree(4321)

    expect(execFileMock).toHaveBeenCalledWith('taskkill', ['/PID', '4321', '/T', '/F'], expect.any(Function))
  })

  it('win32：taskkill 失败也不抛错（强杀尽力而为）', async () => {
    Object.defineProperty(process, 'platform', { value: 'win32' })
    execFileMock.mockImplementation((_cmd: any, _args: any, cb: any) => cb(new Error('access denied')))

    await expect(killTree(4321)).resolves.toBeUndefined()
  })

  it('非 win32：直接 SIGKILL，不走 taskkill', async () => {
    Object.defineProperty(process, 'platform', { value: 'darwin' })
    const realKill = process.kill
    const killSpy = vi.spyOn(process, 'kill').mockImplementation((_pid: any, sig: any) => {
      expect(sig).toBe('SIGKILL')
      return true
    })
    try {
      await killTree(999999)
      expect(execFileMock).not.toHaveBeenCalled()
    }
    finally {
      process.kill = realKill
      killSpy.mockRestore()
    }
  })
})
