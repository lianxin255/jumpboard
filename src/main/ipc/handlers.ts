import { ipcMain } from 'electron'
import { join } from 'path'
import { removeSync, pathExistsSync, readFileSync } from 'fs-extra'
import { IPC } from '@shared/types'
import type { MappingInput } from '@shared/types'
import { listHosts, createHost, updateHost, deleteHost, getDefaultHost, type CreateHostInput } from '../db/hostRepo'
import { listMappings, createMapping, updateMapping, deleteMapping, listEnabledMappings, getMapping } from '../db/mappingRepo'
import { closeDb } from '../db/connection'
import { setActiveEngine, getActiveEngine, refreshActiveStatus } from '../nginx/engineRegistry'
import { listAllTemplates, listBuiltinTemplates, saveCustomTemplate, deleteCustomTemplate } from '../templates/templateRepo'
import { applyMappings } from '../nginx/applyFlow'
import { resolvePaths } from '../paths'
import { listLogs } from '../db/logRepo'
import { listSnapshots, getSnapshot } from '../db/snapshotRepo'
import { isPortAvailable } from '../services/portCheck'
import { checkMapping } from '../services/healthCheck'
import { exportData, importData } from '../services/dataTransfer'
import { SshClient } from '../ssh/client'
import { encryptPassword, decryptPassword } from '../crypto'

export { setActiveEngine }

/** 拦截 remoteConfig.passwordEncrypted 的 PLAIN: 前缀，存储前加密 */
function encryptPlainPassword(input: any): any {
  if (input?.remoteConfig?.passwordEncrypted?.startsWith('PLAIN:')) {
    const plain = input.remoteConfig.passwordEncrypted.slice(6)
    return {
      ...input,
      remoteConfig: {
        ...input.remoteConfig,
        passwordEncrypted: plain ? encryptPassword(plain) : null
      }
    }
  }
  return input
}

export function registerIpcHandlers(): void {
  // Host
  ipcMain.handle(IPC.HOST_LIST, () => listHosts())
  ipcMain.handle(IPC.HOST_CREATE, (_e, input: CreateHostInput) => createHost(encryptPlainPassword(input)))
  ipcMain.handle(IPC.HOST_UPDATE, (_e, id: string, patch: Partial<CreateHostInput>) => updateHost(id, encryptPlainPassword(patch)))
  ipcMain.handle(IPC.HOST_DELETE, (_e, id: string) => { deleteHost(id); return true })

  // Mapping
  ipcMain.handle(IPC.MAPPING_LIST, (_e, hostId: string) => listMappings(hostId))
  ipcMain.handle(IPC.MAPPING_CREATE, (_e, input: MappingInput) => createMapping(input))
  ipcMain.handle(IPC.MAPPING_UPDATE, (_e, id: string, patch: Partial<MappingInput>) => updateMapping(id, patch))
  ipcMain.handle(IPC.MAPPING_DELETE, (_e, id: string) => { deleteMapping(id); return true })

  // Engine
  ipcMain.handle(IPC.ENGINE_START, async () => {
    const eng = getActiveEngine()
    if (!eng) return { status: 'unknown', error: '没有活动的 nginx 引擎（未选中主机或主机非本地类型）' }
    try {
      await eng.start()
      return { status: eng.status(), error: null }
    } catch (e: any) {
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })
  ipcMain.handle(IPC.ENGINE_STOP, async () => {
    const eng = getActiveEngine()
    if (!eng) return { status: 'unknown', error: null }
    try {
      await eng.stop()
      return { status: eng.status(), error: null }
    } catch (e: any) {
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })
  ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const host = getDefaultHost()
    if (!host) return { ok: false, output: 'no default host' }
    const paths = resolvePaths()
    const mappings = listEnabledMappings(host.id)
    const result = await applyMappings(eng, mappings, {
      nginxLogDir: paths.nginxLogDir,
      nginxTempDir: paths.tempDir,
      hostId: host.id
    })
    return { ok: result.ok, output: result.output }
  })
  ipcMain.handle(IPC.ENGINE_TEST, async () => { const eng = getActiveEngine(); return eng ? eng.test() : { ok: false, output: 'no engine' } })
  ipcMain.handle(IPC.ENGINE_STATUS, () => getActiveEngine()?.status() ?? 'unknown')

  // Template
  ipcMain.handle(IPC.TEMPLATE_LIST, () => listAllTemplates())
  ipcMain.handle(IPC.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates())
  ipcMain.handle(IPC.TEMPLATE_SAVE_CUSTOM, (_e, tpl: any) => { saveCustomTemplate(tpl); return true })
  ipcMain.handle(IPC.TEMPLATE_DELETE_CUSTOM, (_e, id: string) => { deleteCustomTemplate(id); return true })

  // Log
  ipcMain.handle(IPC.LOG_LIST, (_e, hostId: string | null, limit: number) => listLogs(hostId, limit))

  // 读 nginx 运行日志（access.log + error.log 的最后 N 行）
  ipcMain.handle(IPC.NGINX_LOG_TAIL, (_e, lines: number) => {
    const paths = resolvePaths()
    const result: { access: string[]; error: string[] } = { access: [], error: [] }
    try {
      const accessLog = join(paths.nginxLogDir, 'access.log')
      if (pathExistsSync(accessLog)) {
        const content = readFileSync(accessLog, 'utf-8')
        result.access = content.split('\n').filter(Boolean).slice(-lines)
      }
    } catch { /* 忽略 */ }
    try {
      const errorLog = join(paths.nginxLogDir, 'error.log')
      if (pathExistsSync(errorLog)) {
        const content = readFileSync(errorLog, 'utf-8')
        result.error = content.split('\n').filter(Boolean).slice(-lines)
      }
    } catch { /* 忽略 */ }
    return result
  })

  // Snapshot
  ipcMain.handle(IPC.SNAPSHOT_LIST, (_e, hostId: string) => listSnapshots(hostId, 50))
  ipcMain.handle(IPC.SNAPSHOT_GET, (_e, id: string) => getSnapshot(id))
  ipcMain.handle(IPC.SNAPSHOT_ROLLBACK, async (_e, snapshotId: string) => {
    // 回滚：用快照 content 覆盖 conf → nginx -t → reload
    const snap = getSnapshot(snapshotId)
    if (!snap) return { ok: false, output: 'snapshot not found' }
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const oldConf = await eng.readConf()
    await eng.writeConf(snap.content)
    const testResult = await eng.test()
    if (!testResult.ok) {
      await eng.writeConf(oldConf)
      return { ok: false, output: testResult.output }
    }
    try {
      await eng.reload()
      return { ok: true, output: testResult.output }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // Health check
  ipcMain.handle(IPC.HEALTH_CHECK_NOW, async (_e, mappingId: string) => {
    const m = getMapping(mappingId)
    if (!m) return 'unknown'
    return await checkMapping(m)
  })

  // Port check
  ipcMain.handle(IPC.PORT_CHECK, (_e, port: number) => isPortAvailable(port))

  // Host: 测试 SSH 连接
  ipcMain.handle(IPC.HOST_TEST_CONNECTION, async (_e, remoteConfig: any) => {
    try {
      let password: string | undefined
      if (remoteConfig.passwordEncrypted?.startsWith('PLAIN:')) {
        password = remoteConfig.passwordEncrypted.slice(6)
      } else if (remoteConfig.passwordEncrypted) {
        password = decryptPassword(remoteConfig.passwordEncrypted)
      }
      const client = new SshClient({
        host: remoteConfig.host,
        port: remoteConfig.port,
        username: remoteConfig.username,
        password
      })
      const r = await client.exec('echo ok')
      client.close()
      return { ok: r.code === 0, output: r.stdout + r.stderr }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // Engine: 刷新 active engine 状态
  ipcMain.handle(IPC.ENGINE_REFRESH_STATUS, async () => {
    await refreshActiveStatus()
    return getActiveEngine()?.status() ?? 'unknown'
  })

  // 一键清除所有痕迹
  ipcMain.handle(IPC.DATA_WIPE, async () => {
    try {
      const eng = getActiveEngine()
      if (eng && eng.status() === 'running') {
        await eng.stop()
      }
      const paths = resolvePaths()
      closeDb()
      for (const sub of ['data.db', 'data.db-shm', 'data.db-wal', 'nginx-conf', 'logs', 'snapshots', 'temp', 'templates']) {
        const p = join(paths.dataDir, sub)
        if (pathExistsSync(p)) {
          removeSync(p)
        }
      }
      return { ok: true }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // 导出配置
  ipcMain.handle(IPC.DATA_EXPORT, () => {
    return exportData()
  })

  // 导入配置
  ipcMain.handle(IPC.DATA_IMPORT, (_e, payload: any) => {
    try {
      const result = importData(payload)
      return { ok: true, ...result }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // 获取本机所有网卡 IP（排除回环）
  ipcMain.handle(IPC.GET_LOCAL_IPS, () => {
    const os = require('os')
    const interfaces = os.networkInterfaces() as Record<string, Array<{ address: string; family: string; internal: boolean }>>
    const ips: { interface: string; address: string; family: string }[] = []
    for (const name of Object.keys(interfaces)) {
      const addrs = interfaces[name]
      if (!addrs) continue
      for (const addr of addrs) {
        if (addr.family === 'IPv4' && !addr.internal) {
          ips.push({ interface: name, address: addr.address, family: addr.family })
        }
      }
    }
    return ips
  })
}
