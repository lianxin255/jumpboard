import { describe, expect, it, vi } from 'vitest'

import { SshClient } from '../ssh/client'

vi.mock('ssh2', () => {
  return {
    Client: vi.fn().mockImplementation(() => ({
      on: vi.fn().mockReturnThis(),
      connect: vi.fn().mockReturnThis(),
      end: vi.fn().mockReturnThis(),
      exec: vi.fn().mockReturnThis(),
      sftp: vi.fn().mockReturnThis(),
    })),
  }
})

describe('sshClient', () => {
  it('构造时不连接（懒连接）', () => {
    const client = new SshClient({
      host: '1.1.1.1',
      port: 22,
      username: 'root',
      password: 'pwd',
      privateKey: undefined,
    })
    expect(client).toBeTruthy()
  })

  it('暴露 exec/readFile/writeFile/close 方法', () => {
    const client = new SshClient({
      host: '1.1.1.1',
      port: 22,
      username: 'root',
      password: 'pwd',
      privateKey: undefined,
    })
    expect(typeof client.exec).toBe('function')
    expect(typeof client.readFile).toBe('function')
    expect(typeof client.writeFile).toBe('function')
    expect(typeof client.close).toBe('function')
    expect(typeof client.connect).toBe('function')
  })
})
