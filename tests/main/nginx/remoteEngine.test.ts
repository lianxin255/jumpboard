import { describe, it, expect, vi, beforeEach } from 'vitest'

// mock SshClient — 工厂返回实例 + 暴露 mocks 供断言
const sshMocks = {
  exec: vi.fn(),
  readFile: vi.fn(),
  writeFile: vi.fn(),
  close: vi.fn()
}
vi.mock('@main/ssh/client', () => ({
  SshClient: vi.fn().mockImplementation(() => ({ ...sshMocks }))
}))
// mock crypto decryptPassword（RemoteNginxEngine 构造时调）
vi.mock('@main/crypto', () => ({
  decryptPassword: vi.fn((s: string) => s.startsWith('ENC') ? 'decrypted' : s)
}))

import { RemoteNginxEngine } from '@main/nginx/remoteEngine'

describe('RemoteNginxEngine', () => {
  function makeEngine() {
    return new RemoteNginxEngine({
      host: '1.1.1.1', port: 22, username: 'root', passwordEncrypted: 'pwd',
      nginxPath: '/usr/sbin/nginx', confPath: '/etc/nginx/nginx.conf',
      reloadCmd: 'nginx -s reload', testCmd: 'nginx -t'
    })
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('初始 status 为 unknown', () => {
    expect(makeEngine().status()).toBe('unknown')
  })

  it('test 调用 exec 执行 testCmd，code=0 返回 ok:true', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: '', stderr: 'syntax is ok', code: 0 })
    const eng = makeEngine()
    const r = await eng.test()
    expect(r.ok).toBe(true)
    expect(r.output).toContain('syntax is ok')
  })

  it('test code!=0 返回 ok:false', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: '', stderr: 'invalid', code: 1 })
    const r = await eng_test()
    expect(r.ok).toBe(false)
    async function eng_test() { return await makeEngine().test() }
  })

  it('reload 调用 exec 执行 reloadCmd', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: '', stderr: '', code: 0 })
    const eng = makeEngine()
    await eng.reload()
    expect(sshMocks.exec).toHaveBeenCalledWith('nginx -s reload')
  })

  it('reload code!=0 抛错', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: '', stderr: 'fail', code: 1 })
    await expect(makeEngine().reload()).rejects.toThrow()
  })

  it('writeConf 调用 SFTP writeFile 到 confPath', async () => {
    const eng = makeEngine()
    await eng.writeConf('new')
    expect(sshMocks.writeFile).toHaveBeenCalledWith('/etc/nginx/nginx.conf', 'new')
  })

  it('readConf 调用 SFTP readFile', async () => {
    sshMocks.readFile.mockResolvedValue('old conf')
    const r = await makeEngine().readConf()
    expect(r).toBe('old conf')
    expect(sshMocks.readFile).toHaveBeenCalledWith('/etc/nginx/nginx.conf')
  })

  it('refreshStatus 通过 systemctl 更新 cachedStatus 为 running', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: 'active\n', stderr: '', code: 0 })
    const eng = makeEngine()
    await (eng as any).refreshStatus()
    expect(eng.status()).toBe('running')
  })

  it('start/stop 是 no-op（远程不托管进程），不抛错', async () => {
    sshMocks.exec.mockResolvedValue({ stdout: 'inactive', stderr: '', code: 0 })
    const eng = makeEngine()
    await expect(eng.start()).resolves.toBeUndefined()
    await expect(eng.stop()).resolves.toBeUndefined()
  })
})
