import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { isPidAlive } from './processKill'
import { stopStaleNginx } from './staleNginx'

// 依赖 /bin/sleep 的真实进程用例，win32 跳过（killTree 的 taskkill 分支由 mock 测试覆盖）
const skipOnWin = process.platform === 'win32'

function spawnSleeper(): ReturnType<typeof spawn> {
  return spawn('/bin/sleep', ['30'])
}

async function waitPidDead(pid: number, timeoutMs = 5000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (!isPidAlive(pid))
      return true
    await new Promise(r => setTimeout(r, 30))
  }
  return !isPidAlive(pid)
}

describe('stopStaleNginx（孤儿 nginx 清理）', () => {
  let prefix: string

  beforeEach(() => {
    prefix = join(tmpdir(), `ngj-stale-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`)
    mkdirSync(join(prefix, 'logs'), { recursive: true })
  })

  afterEach(() => {
    rmSync(prefix, { recursive: true, force: true })
  })

  it.skipIf(skipOnWin)('pid 存活 → 杀掉进程并移除 pid 文件', async () => {
    const sleeper = spawnSleeper()
    writeFileSync(join(prefix, 'logs', 'nginx.pid'), String(sleeper.pid))

    await stopStaleNginx(prefix)

    expect(await waitPidDead(sleeper.pid!)).toBe(true)
    expect(existsSync(join(prefix, 'logs', 'nginx.pid'))).toBe(false)
  })

  it.skipIf(skipOnWin)('pid 已死 → 仅清理 pid 文件，不报错', async () => {
    const sleeper = spawnSleeper()
    sleeper.kill('SIGKILL')
    await new Promise<void>(r => sleeper.once('exit', () => r()))
    expect(isPidAlive(sleeper.pid!)).toBe(false)

    writeFileSync(join(prefix, 'logs', 'nginx.pid'), String(sleeper.pid))
    await stopStaleNginx(prefix)
    expect(existsSync(join(prefix, 'logs', 'nginx.pid'))).toBe(false)
  })

  it('pid 文件不存在 → no-op', async () => {
    await expect(stopStaleNginx(prefix)).resolves.toBeUndefined()
  })

  it('pid 文件内容非法 → no-op 不抛错', async () => {
    writeFileSync(join(prefix, 'logs', 'nginx.pid'), 'not-a-pid')
    await expect(stopStaleNginx(prefix)).resolves.toBeUndefined()
    // 非法内容也顺手清掉
    expect(existsSync(join(prefix, 'logs', 'nginx.pid'))).toBe(false)
  })
})
