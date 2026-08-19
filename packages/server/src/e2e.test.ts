import type { ChildProcess } from 'node:child_process'
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'

/**
 * 端到端集成测试：启动 esbuild 打包的 server.cjs sidecar，通过真实 HTTP 验证完整生命周期。
 *
 * 前提：build/app/server.cjs 已构建（pnpm run -r build 或 node scripts/build-app.mjs --resources）。
 * 本地 dev 环境（macOS）需要 build/app/node_modules 下有 better-sqlite3/ssh2/cpu-features 的 symlink。
 * nginx 用 resources/nginx/nginx（macOS arm64 prebuilt）。
 *
 * 覆盖验收清单：host seed → mapping CRUD → engine test/start/reload/status/pid →
 * snapshot → data export → 静态文件托管 → SIGTERM 优雅退出无残留。
 */
const SERVER_CJS = join(process.cwd(), 'build/app/server.cjs')
const RESOURCES_DIR = join(process.cwd(), 'resources')
const NGINX_BIN = join(RESOURCES_DIR, 'nginx', process.platform === 'win32' ? 'nginx.exe' : 'nginx')

// eslint 告警：sidecar 进程依赖 build 产物，CI 跳过（无 build/app/server.cjs 时整体 skip）
const HAS_BUILD = existsSync(SERVER_CJS) && existsSync(NGINX_BIN)
const _describe = HAS_BUILD ? describe : describe.skip

let sidecar: ChildProcess | null = null
let dataRoot: string
const PORT = 48731 // 避开 dev 默认 47831，防止本地冲突
const BASE = `http://127.0.0.1:${PORT}`

async function waitForPort(port: number, timeoutMs = 15000): Promise<void> {
  const net = await import('node:net')
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    await new Promise(resolve => setTimeout(resolve, 200))
    const ok = await new Promise<boolean>((resolve) => {
      const sock = new net.Socket()
      sock.setTimeout(1000)
      sock.once('connect', () => { sock.destroy(); resolve(true) })
      sock.once('error', () => resolve(false))
      sock.once('timeout', () => { sock.destroy(); resolve(false) })
      sock.connect(port, '127.0.0.1')
    })
    if (ok)
      return
  }
  throw new Error(`sidecar 端口 ${port} 在 ${timeoutMs}ms 内未就绪`)
}

async function api(path: string, opts: RequestInit = {}): Promise<any> {
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers: { 'Content-Type': 'application/json', ...opts.headers },
  })
  return res.json()
}

async function apiPost(path: string, body?: unknown): Promise<any> {
  return api(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
}

_describe('sidecar e2e（真实 HTTP + nginx）', () => {
  let hostId: string

  beforeAll(async () => {
    dataRoot = join(tmpdir(), `ngj-e2e-${Date.now()}`)
    mkdirSync(dataRoot, { recursive: true })

    sidecar = spawn('node', [SERVER_CJS], {
      env: {
        ...process.env,
        NGJ_DATA_ROOT: dataRoot,
        NGJ_RESOURCES_DIR: RESOURCES_DIR,
        NGJ_PORT: String(PORT),
        NGJ_WEB_DIST: join(process.cwd(), 'apps/web/dist'),
        NGJ_SECRET: 'vitest-e2e-secret',
      },
      stdio: ['pipe', 'pipe', 'pipe'],
    })

    sidecar.stdout?.on('data', (d: Buffer) => {
      const s = d.toString().trim()
      if (s)
        console.log('[sidecar stdout]', s)
    })
    sidecar.stderr?.on('data', (d: Buffer) => {
      const s = d.toString().trim()
      if (s)
        console.error('[sidecar stderr]', s)
    })

    await waitForPort(PORT)
  }, 20000)

  afterAll(async () => {
    if (sidecar && sidecar.exitCode === null) {
      sidecar.kill('SIGTERM')
      await new Promise<void>((resolve) => {
        const t = setTimeout(() => { sidecar?.kill('SIGKILL'); resolve() }, 10000)
        sidecar?.once('exit', () => { clearTimeout(t); resolve() })
      })
    }
    // 清理临时 data 目录
    if (dataRoot && existsSync(dataRoot))
      rmSync(dataRoot, { recursive: true, force: true })
  }, 15000)

  it('host 自动 seed：首次启动创建「本机网关」', async () => {
    const hosts = await api('/api/host/list')
    expect(Array.isArray(hosts)).toBe(true)
    expect(hosts.length).toBe(1)
    expect(hosts[0].name).toBe('本机网关')
    expect(hosts[0].type).toBe('local')
    expect(hosts[0].isDefault).toBe(1)
    hostId = hosts[0].id
  })

  it('模板列表返回内置 13 个模板', async () => {
    const templates = await api('/api/template/list-builtin')
    expect(templates.length).toBe(13)
    const ids = templates.map((t: any) => t.id)
    expect(ids).toContain('ssh')
    expect(ids).toContain('mysql')
    expect(ids).toContain('mongodb')
  })

  it('mapping CRUD：创建 → 查询 → 更新 → 删除', async () => {
    // 创建
    const created = await apiPost('/api/mapping/create', {
      name: 'test-ssh',
      protocol: 'tcp',
      listenPort: 18022,
      upstreamHost: '127.0.0.1',
      upstreamPort: 22,
      enabled: 1,
      hostId,
      serverNames: null,
      listenPath: null,
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
    })
    expect(created.id).toBeTruthy()
    expect(created.name).toBe('test-ssh')
    expect(created.listenPort).toBe(18022)
    const mappingId = created.id

    // 查询
    const list = await api(`/api/mapping/list?hostId=${hostId}`)
    expect(list.length).toBe(1)
    expect(list[0].id).toBe(mappingId)

    // 更新（updateMapping 返回 void，需重新查询确认）
    await apiPost('/api/mapping/update', {
      id: mappingId,
      patch: { name: 'test-ssh-renamed', listenPort: 19022 },
    })
    const listAfterUpdate = await api(`/api/mapping/list?hostId=${hostId}`)
    const updated = listAfterUpdate.find((m: any) => m.id === mappingId)
    expect(updated.name).toBe('test-ssh-renamed')
    expect(updated.listenPort).toBe(19022)

    // dirty 应为 true（mapping 变更后未 reload）
    const dirty = await api('/api/engine/dirty-get')
    expect(dirty).toBe(true)

    // 删除
    const delResult = await apiPost('/api/mapping/delete', { id: mappingId })
    expect(delResult).toBe(true)
    const list2 = await api(`/api/mapping/list?hostId=${hostId}`)
    expect(list2.length).toBe(0)
  })

  it('engine 完整生命周期：test → start → status → reload → stop', async () => {
    // 先创建一个 mapping，让 conf 有内容
    await apiPost('/api/mapping/create', {
      name: 'lifecycle-test',
      protocol: 'tcp',
      listenPort: 28022,
      upstreamHost: '127.0.0.1',
      upstreamPort: 22,
      enabled: 1,
      hostId,
      serverNames: null,
      listenPath: null,
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
    })

    // nginx -t（配置校验）
    const testResult = await apiPost('/api/engine/test')
    expect(testResult.ok).toBe(true)

    // 启动 nginx
    const startResult = await apiPost('/api/engine/start')
    expect(startResult.status).toBe('running')
    await new Promise(r => setTimeout(r, 500))

    // 状态查询
    const status = await api('/api/engine/status')
    expect(status).toBe('running')

    // pid 查询（route 直接返回 number，非 { pid: number }）
    const pidResult = await api('/api/engine/pid-get')
    expect(typeof pidResult).toBe('number')
    expect(pidResult).toBeGreaterThan(0)

    // reload（重新生成 conf + nginx -s reload）
    const reloadResult = await apiPost('/api/engine/reload')
    expect(reloadResult.ok).toBe(true)
    await new Promise(r => setTimeout(r, 500))

    // reload 后仍 running
    const status2 = await api('/api/engine/status')
    expect(status2).toBe('running')

    // reload 后 dirty 应回到 false
    const dirty = await api('/api/engine/dirty-get')
    expect(dirty).toBe(false)

    // 停止
    const stopResult = await apiPost('/api/engine/stop')
    expect(stopResult.status).toBe('stopped')
    await new Promise(r => setTimeout(r, 500))

    const status3 = await api('/api/engine/status')
    expect(status3).toBe('stopped')
  })

  it('snapshot：reload 生成快照 → 列表 → 回滚', async () => {
    // 上一轮 reload 应已生成快照
    const snapshots = await api(`/api/snapshot/list?hostId=${hostId}`)
    expect(snapshots.length).toBeGreaterThan(0)
    expect(snapshots[0].reason).toBe('reload')

    // 回滚前需要 nginx 在运行（reload 依赖活动引擎的子进程）
    await apiPost('/api/engine/start')
    await new Promise(r => setTimeout(r, 500))

    // 回滚到第一个快照（route 参数是 snapshotId，不是 id）
    const rollbackResult = await apiPost('/api/snapshot/rollback', { snapshotId: snapshots[0].id })
    expect(rollbackResult.ok).toBe(true)
  })

  it('whitelist：add → list → toggle → remove', async () => {
    await apiPost('/api/whitelist/add', { ip: '192.168.1.100', note: 'test' })
    const list = await api('/api/whitelist/list')
    expect(list.length).toBe(1)
    expect(list[0].ip).toBe('192.168.1.100')

    // toggle enabled（route 需要 { id, enabled }）
    await apiPost('/api/whitelist/toggle-ip', { id: list[0].id, enabled: 0 })
    const list2 = await api('/api/whitelist/list')
    expect(list2[0].enabled).toBe(0)

    // remove
    await apiPost('/api/whitelist/remove', { id: list[0].id })
    const list3 = await api('/api/whitelist/list')
    expect(list3.length).toBe(0)
  })

  it('data export：导出包含 hosts/mappings/snapshots/logs', async () => {
    const exported = await api('/api/data/export')
    expect(exported).toHaveProperty('hosts')
    expect(exported).toHaveProperty('mappings')
    expect(exported).toHaveProperty('snapshots')
    expect(exported).toHaveProperty('logs')
    expect(exported.hosts.length).toBe(1)
  })

  it('静态文件托管：/ 返回 index.html', async () => {
    const res = await fetch(`${BASE}/`)
    expect(res.status).toBe(200)
    const html = await res.text()
    expect(html).toContain('<!DOCTYPE html>')
  })

  it('local IPs：返回本机网卡列表', async () => {
    const ips = await api('/api/network/local-ips')
    expect(Array.isArray(ips)).toBe(true)
    expect(ips.length).toBeGreaterThan(0)
  })

  it('日志列表：reload/rollback 操作应产生日志', async () => {
    const logs = await api('/api/log/list')
    expect(Array.isArray(logs)).toBe(true)
    expect(logs.length).toBeGreaterThan(0)
  })

  it('优雅退出：sidecar 进程收到 SIGTERM 后退出 + 无残留 nginx', async () => {
    // 先启动 nginx，验证 SIGTERM 后被清理
    await apiPost('/api/engine/start')
    await new Promise(r => setTimeout(r, 500))
    const pidBefore = await api('/api/engine/pid-get')
    expect(typeof pidBefore).toBe('number')
    expect(pidBefore).toBeGreaterThan(0)
    const nginxPid = pidBefore

    // 发 SIGTERM
    sidecar!.kill('SIGTERM')
    await new Promise<void>((resolve) => {
      const t = setTimeout(() => { sidecar?.kill('SIGKILL'); resolve() }, 10000)
      sidecar?.once('exit', () => { clearTimeout(t); resolve() })
    })
    expect(sidecar!.exitCode).toBe(0)

    // 验证 nginx 子进程已退出（pid 不存在）
    await new Promise(r => setTimeout(r, 500))
    let nginxStillRunning = false
    try {
      process.kill(nginxPid, 0)
      nginxStillRunning = true
    }
    catch {
      // pid 不存在 = 已退出
    }
    expect(nginxStillRunning).toBe(false)
  })
})
