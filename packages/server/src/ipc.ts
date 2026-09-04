import type { MappingInput } from '@ngj/shared/types'
import type { IpcMain } from 'electron'
import type { CreateHostInput } from './db/hostRepo'
import { join } from 'node:path'
import { API } from '@ngj/shared/types'
import {
  clampInt,
  isValidIpOrCidr,
  isValidPort,
  isValidProtocol,
  isValidProxyPassUrl,
} from '@ngj/shared/validation'
import { pathExistsSync, removeSync, unlinkSync } from 'fs-extra'
import { decryptPassword, encryptPassword } from './crypto'
import { closeDb, getDb, openDb } from './db/connection'
import {
  createHost,
  deleteHost,
  ensureDefaultHost,
  getDefaultHost,
  listHosts,
  setDefaultHost,
  updateHost,
} from './db/hostRepo'
import { clearLogs, listLogs } from './db/logRepo'
import {
  createMapping,
  deleteMapping,
  getMapping,
  listEnabledMappings,
  listMappings,
  updateMapping,
} from './db/mappingRepo'
import { getSnapshot, listSnapshots } from './db/snapshotRepo'
import {
  addWhitelist,
  getEnabledWhitelistIps,
  isWhitelistEnabled,
  listWhitelist,
  removeWhitelist,
  setWhitelistEnabled,
  toggleWhitelist,
} from './db/whitelistRepo'
import { applyMappings } from './nginx/applyFlow'
import {
  getActiveEngine,
  getActiveHostId,
  isEngineDirty,
  refreshActiveStatus,
  setActiveEngine,
  setEngineDirty,
} from './nginx/engineRegistry'
import { switchEngineToHost } from './nginx/hostEngineSwitch'
import { safeApplyConf } from './nginx/safeApply'
import { resolvePaths } from './paths'
import { exportData, importData } from './services/dataTransfer'
import { checkMapping } from './services/healthCheck'
import { logger } from './services/logger'
import { isPortAvailable } from './services/portCheck'
import { SshClient } from './ssh/client'
import {
  deleteCustomTemplate,
  listAllTemplates,
  listBuiltinTemplates,
  saveCustomTemplate,
} from './templates/templateRepo'
import { tailFile } from './utils/tailFile'

export { setActiveEngine }

/** 拦截 remoteConfig.passwordEncrypted 的 PLAIN: 前缀，存储前加密 */
function encryptPlainPassword(input: any): any {
  if (input?.remoteConfig?.passwordEncrypted?.startsWith('PLAIN:')) {
    const plain = input.remoteConfig.passwordEncrypted.slice(6)
    return {
      ...input,
      remoteConfig: {
        ...input.remoteConfig,
        passwordEncrypted: plain ? encryptPassword(plain) : null,
      },
    }
  }
  return input
}

/**
 * 注册所有 IPC handler。与原 routes.ts 1:1 对应，业务逻辑不变。
 * channel 名复用 @ngj/shared/types 的 API 路径常量。
 */
export function registerIpcHandlers(ipcMain: IpcMain): void {
  // ── Host ──
  ipcMain.handle(API.HOST_LIST, () => listHosts())

  ipcMain.handle(API.HOST_CREATE, (_e, input: CreateHostInput) => {
    const r = createHost(encryptPlainPassword(input))
    logger.info(null, 'app', `新增主机「${input.name}」· ${input.type === 'local' ? '本地' : '远程SSH'}`)
    return r
  })

  ipcMain.handle(API.HOST_UPDATE, (_e, id: string, patch: any) => {
    return updateHost(id, encryptPlainPassword(patch))
  })

  ipcMain.handle(API.HOST_DELETE, (_e, id: string) => {
    const h = listHosts().find(x => x.id === id)
    deleteHost(id)
    logger.info(null, 'app', `删除主机「${h?.name ?? id}」`)
    return true
  })

  ipcMain.handle(API.HOST_SET_DEFAULT, (_e, id: string) => {
    setDefaultHost(id)
    return true
  })

  ipcMain.handle(API.HOST_SWITCH, async (_e, id: string) => {
    try {
      const r = await switchEngineToHost(id)
      if (r.ok) {
        const h = listHosts().find(x => x.id === id)
        logger.info(id, 'app', `切换到主机「${h?.name ?? id}」`)
      }
      return r
    }
    catch (e: any) {
      return { ok: false, error: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.HOST_TEST_CONNECTION, async (_e, remoteConfig: any) => {
    const host = typeof remoteConfig?.host === 'string' ? remoteConfig.host.slice(0, 255) : ''
    if (!host)
      return { ok: false, output: '缺少主机地址' }
    const port = isValidPort(remoteConfig?.port) ? Number(remoteConfig.port) : 22
    const username = typeof remoteConfig?.username === 'string' ? remoteConfig.username.slice(0, 64) : 'root'
    try {
      let password: string | undefined
      if (typeof remoteConfig.passwordEncrypted === 'string' && remoteConfig.passwordEncrypted.startsWith('PLAIN:')) {
        password = remoteConfig.passwordEncrypted.slice(6)
      }
      else if (typeof remoteConfig.passwordEncrypted === 'string' && remoteConfig.passwordEncrypted) {
        password = decryptPassword(remoteConfig.passwordEncrypted)
      }
      const client = new SshClient({ host, port, username, password })
      const r = await client.exec('echo ok')
      client.close()
      return { ok: r.code === 0, output: r.code === 0 ? 'ok' : '认证失败或命令执行失败' }
    }
    catch {
      return { ok: false, output: '连接失败（地址不可达或超时）' }
    }
  })

  // ── Mapping ──
  ipcMain.handle(API.MAPPING_LIST, (_e, hostId: string) => listMappings(hostId))

  ipcMain.handle(API.MAPPING_CREATE, (_e, input: MappingInput) => {
    if (input.protocol && !isValidProtocol(input.protocol))
      throw new Error(`非法协议: ${input.protocol}`)
    if (input.listenPort != null && !isValidPort(input.listenPort))
      throw new Error(`非法监听端口: ${input.listenPort}`)
    if (input.upstreamPort != null && !isValidPort(input.upstreamPort))
      throw new Error(`非法上游端口: ${input.upstreamPort}`)
    if (input.proxyPassUrl != null && input.proxyPassUrl.trim() && !isValidProxyPassUrl(input.proxyPassUrl))
      throw new Error('proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符')
    const r = createMapping(input); setEngineDirty(true)
    logger.info(input.hostId, 'app', `新建映射「${input.name}」· ${(input.protocol || '').toUpperCase()}`)
    return r
  })

  ipcMain.handle(API.MAPPING_UPDATE, (_e, id: string, patch: any) => {
    if (patch.protocol != null && !isValidProtocol(patch.protocol))
      throw new Error(`非法协议: ${patch.protocol}`)
    if (patch.listenPort != null && !isValidPort(patch.listenPort))
      throw new Error(`非法监听端口: ${patch.listenPort}`)
    if (patch.upstreamPort != null && !isValidPort(patch.upstreamPort))
      throw new Error(`非法上游端口: ${patch.upstreamPort}`)
    if (patch.proxyPassUrl != null && patch.proxyPassUrl.trim() && !isValidProxyPassUrl(patch.proxyPassUrl))
      throw new Error('proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符')
    const m = getMapping(id)
    updateMapping(id, patch); setEngineDirty(true)
    const name = patch.name ? `「${patch.name}」` : ''
    logger.info(m?.hostId ?? null, 'app', `修改映射 ${name}`)
    return true
  })

  ipcMain.handle(API.MAPPING_DELETE, (_e, id: string) => {
    const m = getMapping(id)
    deleteMapping(id); setEngineDirty(true)
    logger.info(m?.hostId ?? null, 'app', `删除映射「${m?.name ?? id}」`)
    return true
  })

  // ── Engine ──
  ipcMain.handle(API.ENGINE_START, async () => {
    const eng = getActiveEngine()
    if (!eng)
      return { status: 'unknown', error: '没有活动的 nginx 引擎（未选中主机或主机非本地类型）' }
    if (!eng.supportsProcessControl)
      return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
    const host = getDefaultHost()
    try {
      await eng.start()
      logger.info(host?.id ?? null, 'app', '引擎启动 · nginx 已开始运行')
      return { status: eng.status(), error: null }
    }
    catch (e: any) {
      logger.error(host?.id ?? null, 'app', `引擎启动失败 · ${e?.message ?? String(e)}`)
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.ENGINE_STOP, async () => {
    const eng = getActiveEngine()
    if (!eng)
      return { status: 'unknown', error: null }
    if (!eng.supportsProcessControl)
      return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
    const host = getDefaultHost()
    try {
      await eng.stop()
      logger.info(host?.id ?? null, 'app', '引擎停止 · 所有转发已失效')
      return { status: eng.status(), error: null }
    }
    catch (e: any) {
      logger.error(host?.id ?? null, 'app', `引擎停止失败 · ${e?.message ?? String(e)}`)
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.ENGINE_RELOAD, async () => {
    const eng = getActiveEngine()
    if (!eng)
      return { ok: false, output: 'no active engine' }
    const hostId = getActiveHostId()
    if (!hostId)
      return { ok: false, output: 'no active host' }
    const mappings = listEnabledMappings(hostId)
    const wlEnabled = isWhitelistEnabled()
    const wlIps = wlEnabled ? getEnabledWhitelistIps() : undefined
    if (wlEnabled && (!wlIps || wlIps.length === 0))
      return { ok: false, output: '白名单已开启但未配置任何 IP。请先添加 IP 或关闭白名单后再应用。' }
    try {
      // 本地引擎停止态：conf 写入 + nginx -t 校验即可（启动时加载磁盘 conf），
      // 走 reload 必然失败并回滚，用户永远无法在停止态应用配置
      const skipReload = eng.supportsProcessControl && eng.status() !== 'running'
      const result = await applyMappings(eng, mappings, { hostId, whitelistIps: wlIps, skipReload })
      if (result.ok)
        setEngineDirty(false)
      const output = result.ok && skipReload
        ? `${result.output}\n引擎未运行：配置已写入并通过校验，启动 nginx 后生效。`
        : result.output
      return { ok: result.ok, output }
    }
    catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.ENGINE_TEST, async () => {
    const eng = getActiveEngine()
    if (!eng)
      return { ok: false, output: 'no engine' }
    // 远程主机 SSH 失败时 engine.test() 会 reject，兜底成结果对象与其它 handler 一致
    try {
      return await eng.test()
    }
    catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.ENGINE_STATUS, () => getActiveEngine()?.status() ?? 'unknown')
  ipcMain.handle(API.ENGINE_DIRTY_GET, () => isEngineDirty())

  ipcMain.handle(API.ENGINE_PID_GET, async () => {
    const eng = getActiveEngine()
    if (!eng || !eng.getPid)
      return null
    return await Promise.resolve(eng.getPid())
  })

  ipcMain.handle(API.ENGINE_KIND_GET, () => {
    const eng = getActiveEngine()
    if (!eng)
      return null
    return { engineKind: eng.engineKind, supportsProcessControl: eng.supportsProcessControl }
  })

  ipcMain.handle(API.ENGINE_REFRESH_STATUS, async () => {
    await refreshActiveStatus()
    return getActiveEngine()?.status() ?? 'unknown'
  })

  // ── Template ──
  ipcMain.handle(API.TEMPLATE_LIST, () => listAllTemplates())
  ipcMain.handle(API.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates())
  ipcMain.handle(API.TEMPLATE_SAVE_CUSTOM, (_e, tpl: any) => { saveCustomTemplate(tpl); return true })
  ipcMain.handle(API.TEMPLATE_DELETE_CUSTOM, (_e, id: string) => { deleteCustomTemplate(id); return true })

  // ── Log ──
  ipcMain.handle(API.LOG_LIST, (_e, hostId: string | null, limit: number) => {
    return listLogs(hostId, clampInt(limit, 0, 5000))
  })

  ipcMain.handle(API.LOG_CLEAR, () => { clearLogs(); logger.info(null, 'app', '操作日志已清空'); return true })

  // ── Nginx Log ──
  ipcMain.handle(API.NGINX_LOG_TAIL, (_e, lines: number) => {
    const safeLines = clampInt(lines, 1, 10000)
    const readRow = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('nginx_log_read_enabled') as { value: string } | undefined
    if (readRow?.value === '0')
      return { access: [], error: [] }

    const paths = resolvePaths()
    const nginxLogsDir = join(paths.dataDir, 'nginx-home', 'logs')
    const result: { access: string[], error: string[] } = { access: [], error: [] }
    try {
      const accLines: string[] = []
      for (const f of ['access.log', 'stream_access.log']) {
        const p = join(nginxLogsDir, f)
        if (pathExistsSync(p))
          accLines.push(...tailFile(p, safeLines))
      }
      result.access = accLines
    }
    catch { /* 忽略 */ }
    try {
      const errorLog = join(nginxLogsDir, 'error.log')
      if (pathExistsSync(errorLog))
        result.error = tailFile(errorLog, safeLines)
    }
    catch { /* 忽略 */ }
    return result
  })

  ipcMain.handle(API.NGINX_LOG_CLEAR, () => {
    try {
      const paths = resolvePaths()
      const nginxLogsDir = join(paths.dataDir, 'nginx-home', 'logs')
      for (const f of ['access.log', 'error.log', 'stream_access.log']) {
        try { unlinkSync(join(nginxLogsDir, f)) }
        catch { /* 文件不存在则忽略 */ }
      }
      logger.info(null, 'app', 'Nginx 日志已清空')
      return true
    }
    catch { return false }
  })

  ipcMain.handle(API.NGINX_LOG_READ_ENABLED_GET, () => {
    const row = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('nginx_log_read_enabled') as { value: string } | undefined
    return row?.value !== '0'
  })

  ipcMain.handle(API.NGINX_LOG_READ_ENABLED_SET, (_e, enabled: boolean) => {
    getDb().prepare('INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value').run('nginx_log_read_enabled', enabled ? '1' : '0')
    return true
  })

  // ── Snapshot ──
  ipcMain.handle(API.SNAPSHOT_LIST, (_e, hostId: string) => listSnapshots(hostId, 50))
  ipcMain.handle(API.SNAPSHOT_GET, (_e, id: string) => getSnapshot(id))

  ipcMain.handle(API.SNAPSHOT_ROLLBACK, async (_e, snapshotId: string) => {
    const snap = getSnapshot(snapshotId)
    if (!snap)
      return { ok: false, output: 'snapshot not found' }
    const eng = getActiveEngine()
    if (!eng)
      return { ok: false, output: 'no active engine' }
    // 停止态与 ENGINE_RELOAD 一致：只写入+校验，启动时生效
    const skipReload = eng.supportsProcessControl && eng.status() !== 'running'
    const result = await safeApplyConf(eng, snap.content, { skipReload })
    if (result.ok) {
      setEngineDirty(true)
      logger.info(snap.hostId, 'app', `手动回滚到快照 ${snap.id} · DB 与 conf 已偏离，点应用会覆盖`)
    }
    else {
      logger.error(snap.hostId, 'app', `回滚到快照 ${snap.id} 失败(${result.failStage ?? 'unknown'}): ${result.output}`)
    }
    return result
  })

  // ── Health check ──
  ipcMain.handle(API.HEALTH_CHECK_NOW, async (_e, mappingId: string) => {
    const m = getMapping(mappingId)
    if (!m)
      return 'unknown'
    return await checkMapping(m)
  })

  // ── Port check ──
  ipcMain.handle(API.PORT_CHECK, (_e, port: number) => {
    if (!isValidPort(port))
      return false
    return isPortAvailable(port)
  })

  // ── Network ──
  ipcMain.handle(API.GET_LOCAL_IPS, () => {
    const os = require('node:os')
    const interfaces = os.networkInterfaces() as Record<string, Array<{ address: string, family: string, internal: boolean }>>
    const ips: { interface: string, address: string, family: string }[] = []
    for (const name of Object.keys(interfaces)) {
      const addrs = interfaces[name]
      if (!addrs)
        continue
      for (const addr of addrs) {
        if (addr.family === 'IPv4' && !addr.internal)
          ips.push({ interface: name, address: addr.address, family: addr.family })
      }
    }
    return ips
  })

  // ── Data management ──
  ipcMain.handle(API.DATA_WIPE, async (_e, confirmToken: string) => {
    if (confirmToken !== 'CONFIRM_WIPE')
      return { ok: false, output: '缺少确认 token，请通过界面操作' }
    try {
      const eng = getActiveEngine()
      if (eng && eng.status() === 'running')
        await eng.stop()
      const paths = resolvePaths()
      closeDb()
      for (const sub of ['data.db', 'data.db-shm', 'data.db-wal', 'nginx-conf', 'logs', 'snapshots', 'temp', 'templates', 'nginx-home']) {
        const p = join(paths.dataDir, sub)
        if (pathExistsSync(p))
          removeSync(p)
      }
      // 自愈：重开 DB（schema 由 migrations 重建）+ 补默认主机 + 重建引擎，
      // 应用无需重启即可继续使用（healthCheckLoop 也不再对着关闭的 DB 空转）
      openDb()
      const def = ensureDefaultHost() ?? getDefaultHost()
      setActiveEngine(null)
      setEngineDirty(false)
      if (def)
        await switchEngineToHost(def.id)
      logger.warn(null, 'app', '清空所有数据 · 数据库已重置')
      return { ok: true }
    }
    catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  ipcMain.handle(API.DATA_EXPORT, () => exportData())

  ipcMain.handle(API.DATA_IMPORT, (_e, payload: any) => {
    try {
      const result = importData(payload)
      setEngineDirty(true)
      logger.info(null, 'app', `数据导入完成 · ${result.hostsImported} 主机 / ${result.mappingsImported} 映射`)
      return { ok: true, ...result }
    }
    catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // ── IP 白名单 ──
  ipcMain.handle(API.WHITELIST_LIST, () => listWhitelist())

  ipcMain.handle(API.WHITELIST_ADD, (_e, ip: string, note?: string) => {
    if (!isValidIpOrCidr(ip))
      throw new Error('IP 格式不正确，应为 IPv4(1.2.3.4) 或 CIDR(10.0.0.0/24)')
    const r = addWhitelist(ip, note)
    setEngineDirty(true)
    logger.info(null, 'app', `白名单添加 ${ip}`)
    return r
  })

  ipcMain.handle(API.WHITELIST_REMOVE, (_e, id: string) => {
    removeWhitelist(id); setEngineDirty(true); return true
  })

  ipcMain.handle(API.WHITELIST_TOGGLE_IP, (_e, id: string, enabled: number) => {
    toggleWhitelist(id, enabled); setEngineDirty(true); return true
  })

  ipcMain.handle(API.WHITELIST_GET_ENABLED, () => isWhitelistEnabled())

  ipcMain.handle(API.WHITELIST_SET_ENABLED, (_e, enabled: boolean) => {
    setWhitelistEnabled(enabled); setEngineDirty(true)
    logger.info(null, 'app', enabled ? '白名单已开启 · 仅允许列表 IP 访问' : '白名单已关闭 · 全部允许')
    return true
  })
}
