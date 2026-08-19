import type { MappingInput } from '@ngj/shared/types'
import type { CreateHostInput } from './db/hostRepo';
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
import { closeDb, getDb } from './db/connection'
import {
  createHost,

  deleteHost,
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

/** HTTP 请求上下文：method + 路径参数 + 查询参数 + 请求体 */
export interface RouteContext {
  params: Record<string, string>
  query: URLSearchParams
  body: any
}

export type RouteHandler = (ctx: RouteContext) => any | Promise<any>

/**
 * 路由表：HTTP 方法 + 路径 → handler。
 * 与原 IPC handlers 1:1 对应，签名/返回值不变。
 */
export function buildRoutes(): Record<string, RouteHandler> {
  return {
    // ── Host ──
    [`${'GET'} ${API.HOST_LIST}`]: () => listHosts(),
    [`${'POST'} ${API.HOST_CREATE}`]: (ctx) => {
      const input: CreateHostInput = ctx.body
      const r = createHost(encryptPlainPassword(input))
      logger.info(null, 'app', `新增主机「${input.name}」· ${input.type === 'local' ? '本地' : '远程SSH'}`)
      return r
    },
    [`${'POST'} ${API.HOST_UPDATE}`]: (ctx) => {
      const { id, patch } = ctx.body
      return updateHost(id, encryptPlainPassword(patch))
    },
    [`${'POST'} ${API.HOST_DELETE}`]: (ctx) => {
      const { id } = ctx.body
      const h = listHosts().find(x => x.id === id)
      deleteHost(id)
      logger.info(null, 'app', `删除主机「${h?.name ?? id}」`)
      return true
    },
    [`${'POST'} ${API.HOST_SET_DEFAULT}`]: (ctx) => {
      const { id } = ctx.body
      setDefaultHost(id)
      return true
    },
    [`${'POST'} ${API.HOST_SWITCH}`]: async (ctx) => {
      const { id } = ctx.body
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
    },
    [`${'POST'} ${API.HOST_TEST_CONNECTION}`]: async (ctx) => {
      const remoteConfig = ctx.body
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
    },

    // ── Mapping ──
    [`${'GET'} ${API.MAPPING_LIST}`]: ctx => listMappings(ctx.params.hostId || ctx.query.get('hostId')!),
    [`${'POST'} ${API.MAPPING_CREATE}`]: (ctx) => {
      const input: MappingInput = ctx.body
      if (input.protocol && !isValidProtocol(input.protocol)) {
        throw new Error(`非法协议: ${input.protocol}`)
      }
      if (input.listenPort != null && !isValidPort(input.listenPort)) {
        throw new Error(`非法监听端口: ${input.listenPort}`)
      }
      if (input.upstreamPort != null && !isValidPort(input.upstreamPort)) {
        throw new Error(`非法上游端口: ${input.upstreamPort}`)
      }
      if (input.proxyPassUrl != null && input.proxyPassUrl.trim() && !isValidProxyPassUrl(input.proxyPassUrl)) {
        throw new Error('proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符')
      }
      const r = createMapping(input); setEngineDirty(true)
      logger.info(input.hostId, 'app', `新建映射「${input.name}」· ${(input.protocol || '').toUpperCase()}`)
      return r
    },
    [`${'POST'} ${API.MAPPING_UPDATE}`]: (ctx) => {
      const { id, patch } = ctx.body
      if (patch.protocol != null && !isValidProtocol(patch.protocol)) {
        throw new Error(`非法协议: ${patch.protocol}`)
      }
      if (patch.listenPort != null && !isValidPort(patch.listenPort)) {
        throw new Error(`非法监听端口: ${patch.listenPort}`)
      }
      if (patch.upstreamPort != null && !isValidPort(patch.upstreamPort)) {
        throw new Error(`非法上游端口: ${patch.upstreamPort}`)
      }
      if (patch.proxyPassUrl != null && patch.proxyPassUrl.trim() && !isValidProxyPassUrl(patch.proxyPassUrl)) {
        throw new Error('proxy_pass 自定义值非法：必须 http:// 或 https:// 开头，不含特殊字符')
      }
      const m = getMapping(id)
      updateMapping(id, patch); setEngineDirty(true)
      const name = patch.name ? `「${patch.name}」` : ''
      logger.info(m?.hostId ?? null, 'app', `修改映射 ${name}`)
      return true
    },
    [`${'POST'} ${API.MAPPING_DELETE}`]: (ctx) => {
      const { id } = ctx.body
      const m = getMapping(id)
      deleteMapping(id); setEngineDirty(true)
      logger.info(m?.hostId ?? null, 'app', `删除映射「${m?.name ?? id}」`)
      return true
    },

    // ── Engine ──
    [`${'POST'} ${API.ENGINE_START}`]: async () => {
      const eng = getActiveEngine()
      if (!eng)
        return { status: 'unknown', error: '没有活动的 nginx 引擎（未选中主机或主机非本地类型）' }
      if (!eng.supportsProcessControl) {
        return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
      }
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
    },
    [`${'POST'} ${API.ENGINE_STOP}`]: async () => {
      const eng = getActiveEngine()
      if (!eng)
        return { status: 'unknown', error: null }
      if (!eng.supportsProcessControl) {
        return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
      }
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
    },
    [`${'POST'} ${API.ENGINE_RELOAD}`]: async () => {
      const eng = getActiveEngine()
      if (!eng)
        return { ok: false, output: 'no active engine' }
      const hostId = getActiveHostId()
      if (!hostId)
        return { ok: false, output: 'no active host' }
      const mappings = listEnabledMappings(hostId)
      const wlEnabled = isWhitelistEnabled()
      const wlIps = wlEnabled ? getEnabledWhitelistIps() : undefined
      if (wlEnabled && (!wlIps || wlIps.length === 0)) {
        return { ok: false, output: '白名单已开启但未配置任何 IP。请先添加 IP 或关闭白名单后再应用。' }
      }
      try {
        const result = await applyMappings(eng, mappings, { hostId, whitelistIps: wlIps })
        if (result.ok)
          setEngineDirty(false)
        return { ok: result.ok, output: result.output }
      }
      catch (e: any) {
        return { ok: false, output: e?.message ?? String(e) }
      }
    },
    [`${'POST'} ${API.ENGINE_TEST}`]: async () => {
      const eng = getActiveEngine()
      return eng ? eng.test() : { ok: false, output: 'no engine' }
    },
    [`${'GET'} ${API.ENGINE_STATUS}`]: () => getActiveEngine()?.status() ?? 'unknown',
    [`${'GET'} ${API.ENGINE_DIRTY_GET}`]: () => isEngineDirty(),
    [`${'GET'} ${API.ENGINE_PID_GET}`]: async () => {
      const eng = getActiveEngine()
      if (!eng || !eng.getPid)
        return null
      return await Promise.resolve(eng.getPid())
    },
    [`${'GET'} ${API.ENGINE_KIND_GET}`]: () => {
      const eng = getActiveEngine()
      if (!eng)
        return null
      return { engineKind: eng.engineKind, supportsProcessControl: eng.supportsProcessControl }
    },
    [`${'POST'} ${API.ENGINE_REFRESH_STATUS}`]: async () => {
      await refreshActiveStatus()
      return getActiveEngine()?.status() ?? 'unknown'
    },

    // ── Template ──
    [`${'GET'} ${API.TEMPLATE_LIST}`]: () => listAllTemplates(),
    [`${'GET'} ${API.TEMPLATE_LIST_BUILTIN}`]: () => listBuiltinTemplates(),
    [`${'POST'} ${API.TEMPLATE_SAVE_CUSTOM}`]: (ctx) => { saveCustomTemplate(ctx.body); return true },
    [`${'POST'} ${API.TEMPLATE_DELETE_CUSTOM}`]: (ctx) => { const { id } = ctx.body; deleteCustomTemplate(id); return true },

    // ── Log ──
    [`${'GET'} ${API.LOG_LIST}`]: (ctx) => {
      const hostId = ctx.query.get('hostId')
      const limit = Number(ctx.query.get('limit') || 200)
      return listLogs(hostId === 'null' ? null : hostId, clampInt(limit, 0, 5000))
    },
    [`${'POST'} ${API.LOG_CLEAR}`]: () => { clearLogs(); logger.info(null, 'app', '操作日志已清空'); return true },

    // ── Nginx Log ──
    [`${'GET'} ${API.NGINX_LOG_TAIL}`]: (ctx) => {
      const safeLines = clampInt(Number(ctx.query.get('lines') || 200), 1, 10000)
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
          if (pathExistsSync(p)) {
            accLines.push(...tailFile(p, safeLines))
          }
        }
        result.access = accLines
      }
      catch { /* 忽略 */ }
      try {
        const errorLog = join(nginxLogsDir, 'error.log')
        if (pathExistsSync(errorLog)) {
          result.error = tailFile(errorLog, safeLines)
        }
      }
      catch { /* 忽略 */ }
      return result
    },
    [`${'POST'} ${API.NGINX_LOG_CLEAR}`]: () => {
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
    },
    [`${'GET'} ${API.NGINX_LOG_READ_ENABLED_GET}`]: () => {
      const row = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('nginx_log_read_enabled') as { value: string } | undefined
      return row?.value !== '0'
    },
    [`${'POST'} ${API.NGINX_LOG_READ_ENABLED_SET}`]: (ctx) => {
      const { enabled } = ctx.body
      getDb().prepare('INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value').run('nginx_log_read_enabled', enabled ? '1' : '0')
      return true
    },

    // ── Snapshot ──
    [`${'GET'} ${API.SNAPSHOT_LIST}`]: ctx => listSnapshots(ctx.query.get('hostId')!, 50),
    [`${'GET'} ${API.SNAPSHOT_GET}`]: ctx => getSnapshot(ctx.query.get('id')!),
    [`${'POST'} ${API.SNAPSHOT_ROLLBACK}`]: async (ctx) => {
      const { snapshotId } = ctx.body
      const snap = getSnapshot(snapshotId)
      if (!snap)
        return { ok: false, output: 'snapshot not found' }
      const eng = getActiveEngine()
      if (!eng)
        return { ok: false, output: 'no active engine' }
      const result = await safeApplyConf(eng, snap.content)
      if (result.ok) {
        setEngineDirty(true)
        logger.info(snap.hostId, 'app', `手动回滚到快照 ${snap.id} · DB 与 conf 已偏离，点应用会覆盖`)
      }
      else {
        logger.error(snap.hostId, 'app', `回滚到快照 ${snap.id} 失败(${result.failStage ?? 'unknown'}): ${result.output}`)
      }
      return result
    },

    // ── Health check ──
    [`${'POST'} ${API.HEALTH_CHECK_NOW}`]: async (ctx) => {
      const { mappingId } = ctx.body
      const m = getMapping(mappingId)
      if (!m)
        return 'unknown'
      return await checkMapping(m)
    },

    // ── Port check ──
    [`${'GET'} ${API.PORT_CHECK}`]: (ctx) => {
      const port = Number(ctx.query.get('port'))
      if (!isValidPort(port))
        return false
      return isPortAvailable(port)
    },

    // ── Network ──
    [`${'GET'} ${API.GET_LOCAL_IPS}`]: () => {
      const os = require('node:os')
      const interfaces = os.networkInterfaces() as Record<string, Array<{ address: string, family: string, internal: boolean }>>
      const ips: { interface: string, address: string, family: string }[] = []
      for (const name of Object.keys(interfaces)) {
        const addrs = interfaces[name]
        if (!addrs)
          continue
        for (const addr of addrs) {
          if (addr.family === 'IPv4' && !addr.internal) {
            ips.push({ interface: name, address: addr.address, family: addr.family })
          }
        }
      }
      return ips
    },

    // ── Data management ──
    [`${'POST'} ${API.DATA_WIPE}`]: async (ctx) => {
      const { confirmToken } = ctx.body
      if (confirmToken !== 'CONFIRM_WIPE') {
        return { ok: false, output: '缺少确认 token，请通过界面操作' }
      }
      try {
        const eng = getActiveEngine()
        if (eng && eng.status() === 'running') {
          await eng.stop()
        }
        const paths = resolvePaths()
        closeDb()
        for (const sub of ['data.db', 'data.db-shm', 'data.db-wal', 'nginx-conf', 'logs', 'snapshots', 'temp', 'templates', 'nginx-home']) {
          const p = join(paths.dataDir, sub)
          if (pathExistsSync(p)) {
            removeSync(p)
          }
        }
        logger.warn(null, 'app', '清空所有数据 · 数据库已重置')
        return { ok: true }
      }
      catch (e: any) {
        return { ok: false, output: e?.message ?? String(e) }
      }
    },
    [`${'GET'} ${API.DATA_EXPORT}`]: () => exportData(),
    [`${'POST'} ${API.DATA_IMPORT}`]: (ctx) => {
      try {
        const result = importData(ctx.body)
        setEngineDirty(true)
        logger.info(null, 'app', `数据导入完成 · ${result.hostsImported} 主机 / ${result.mappingsImported} 映射`)
        return { ok: true, ...result }
      }
      catch (e: any) {
        return { ok: false, output: e?.message ?? String(e) }
      }
    },

    // ── IP 白名单 ──
    [`${'GET'} ${API.WHITELIST_LIST}`]: () => listWhitelist(),
    [`${'POST'} ${API.WHITELIST_ADD}`]: (ctx) => {
      const { ip, note } = ctx.body
      if (!isValidIpOrCidr(ip)) {
        throw new Error('IP 格式不正确，应为 IPv4(1.2.3.4) 或 CIDR(10.0.0.0/24)')
      }
      const r = addWhitelist(ip, note)
      setEngineDirty(true)
      logger.info(null, 'app', `白名单添加 ${ip}`)
      return r
    },
    [`${'POST'} ${API.WHITELIST_REMOVE}`]: (ctx) => {
      const { id } = ctx.body
      removeWhitelist(id); setEngineDirty(true); return true
    },
    [`${'POST'} ${API.WHITELIST_TOGGLE_IP}`]: (ctx) => {
      const { id, enabled } = ctx.body
      toggleWhitelist(id, enabled); setEngineDirty(true); return true
    },
    [`${'GET'} ${API.WHITELIST_GET_ENABLED}`]: () => isWhitelistEnabled(),
    [`${'POST'} ${API.WHITELIST_SET_ENABLED}`]: (ctx) => {
      const { enabled } = ctx.body
      setWhitelistEnabled(enabled); setEngineDirty(true)
      logger.info(null, 'app', enabled ? '白名单已开启 · 仅允许列表 IP 访问' : '白名单已关闭 · 全部允许')
      return true
    },
  }
}
