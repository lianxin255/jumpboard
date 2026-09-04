import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { LocalNginxEngine } from '../nginx/localEngine'

// 本测试需要一个能跑的 nginx.exe，CI 环境用 mock；本地有 nginx 时跑真实验证。
// 这里用脚本假装 nginx，验证进程生命周期管理逻辑。
describe('localNginxEngine 进程管理', () => {
  let workDir: string
  let fakeNginx: string

  beforeEach(() => {
    workDir = join(tmpdir(), `nginx-engine-${Date.now()}`)
    mkdirSync(workDir, { recursive: true })
    // 假 nginx：保持运行直到收到信号。只在非 win32 被 spawn（win32 用例 skip，见下）
    fakeNginx = join(workDir, 'fake-nginx.sh')
    writeFileSync(fakeNginx, '#!/bin/bash\nwhile true; do sleep 1; done\n')
    require('node:fs').chmodSync(fakeNginx, 0o755)
  })

  afterEach(() => {
    rmSync(workDir, { recursive: true, force: true })
  })

  it('status 初始为 stopped', () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    expect(eng.status()).toBe('stopped')
  })

  // 依赖真实 spawn 的用例在 win32 上跳过：现代 Node（CVE-2024-27980 修复）spawn .bat
  // 不带 shell:true 抛 EINVAL，bash 脚本又无法在 Windows 执行。
  // win32 分支（taskkill / nginx -s reload）的逻辑覆盖见 localEngine.win32.test.ts（mock，跨平台可跑）。
  it('start 后变 running，stop 后变 stopped', { skip: process.platform === 'win32' }, async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    await eng.start()
    expect(eng.status()).toBe('running')
    await eng.stop()
    expect(eng.status()).toBe('stopped')
  })

  it('重复 start 抛错', { skip: process.platform === 'win32' }, async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    await eng.start()
    await expect(eng.start()).rejects.toThrow()
    await eng.stop()
  })

  it('stop 未运行的引擎不报错', async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    await expect(eng.stop()).resolves.toBeUndefined()
  })

  it('nginx 二进制不存在时 start 抛错（不崩溃进程）', async () => {
    const eng = new LocalNginxEngine({
      nginxPath: join(workDir, 'definitely-does-not-exist'),
      confPath: join(workDir, 'nginx.conf'),
      logDir: workDir,
      tempDir: workDir,
      prefixDir: workDir,
    })
    await expect(eng.start()).rejects.toThrow()
    expect(eng.status()).toBe('stopped')
  })

  it('start→stop→start→stop 循环不会让旧进程的 exit 误清新进程引用', { skip: process.platform === 'win32' }, async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    await eng.start()
    expect(eng.status()).toBe('running')
    await eng.stop()
    // 立即重启（不等旧进程完全 reap）
    await eng.start()
    expect(eng.status()).toBe('running')
    // 给旧进程的 exit 事件一个机会触发；若 bug 存在，status 会变 stopped
    await new Promise(r => setTimeout(r, 300))
    expect(eng.status()).toBe('running') // 关键断言：仍是 running
    await eng.stop()
    expect(eng.status()).toBe('stopped')
  })

  it('nginx 启动后立即退出时 start() reject', async () => {
    let exitFast: string
    if (process.platform === 'win32') {
      // Windows：用 node 自身当立即退出的假 nginx（-p 后的路径被当作 JS 表达式求值后快速退出）
      exitFast = process.execPath
    }
    else {
      exitFast = join(workDir, 'exit-fast.sh')
      writeFileSync(exitFast, '#!/bin/bash\nexit 1\n')
      require('node:fs').chmodSync(exitFast, 0o755)
    }
    const eng = new LocalNginxEngine({ nginxPath: exitFast, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir, prefixDir: workDir })
    await expect(eng.start()).rejects.toThrow()
    expect(eng.status()).toBe('stopped')
  })
})
