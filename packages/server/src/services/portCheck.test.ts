import { createServer } from 'node:net'
import { describe, expect, it } from 'vitest'
import { isPortAvailable } from '../services/portCheck'

describe('portCheck', () => {
  it('空闲端口返回 true', async () => {
    // 用一个不太可能被占的高端口
    expect(await isPortAvailable(39998)).toBe(true)
  })

  it('被占端口返回 false', async () => {
    const server = createServer()
    await new Promise<void>(resolve => server.listen(39999, resolve))
    expect(await isPortAvailable(39999)).toBe(false)
    await new Promise<void>(resolve => server.close(() => resolve()))
  })

  it('null/undefined 端口返回 true（无需检测）', async () => {
    expect(await isPortAvailable(null)).toBe(true)
    expect(await isPortAvailable(undefined)).toBe(true)
  })
})
