import { ipcMain } from 'electron'
import { join } from 'path'
import { removeSync, pathExistsSync, unlinkSync } from 'fs-extra'
import { IPC } from '@shared/types'
import type { MappingInput } from '@shared/types'
import { isValidIpOrCidr, isValidPort, isValidProtocol, isValidProxyMode, isValidProxyPassUrl, clampInt } from '@shared/validation'
import { tailFile } from '../utils/tailFile'
import { listHosts, createHost, updateHost, deleteHost, getDefaultHost, setDefaultHost, type CreateHostInput } from '../db/hostRepo'
import { listMappings, createMapping, updateMapping, deleteMapping, listEnabledMappings, getMapping } from '../db/mappingRepo'
import { closeDb, getDb } from '../db/connection'
import { setActiveEngine, getActiveEngine, getActiveHostId, refreshActiveStatus, isEngineDirty, setEngineDirty } from '../nginx/engineRegistry'
import { switchEngineToHost } from '../nginx/hostEngineSwitch'
import { listAllTemplates, listBuiltinTemplates, saveCustomTemplate, deleteCustomTemplate } from '../templates/templateRepo'
import { applyMappings } from '../nginx/applyFlow'
import { safeApplyConf } from '../nginx/safeApply'
import { resolvePaths } from '../paths'
import { listLogs, clearLogs } from '../db/logRepo'
import { listSnapshots, getSnapshot } from '../db/snapshotRepo'
import { listWhitelist, addWhitelist, removeWhitelist, toggleWhitelist, isWhitelistEnabled, setWhitelistEnabled, getEnabledWhitelistIps } from '../db/whitelistRepo'
import { isPortAvailable } from '../services/portCheck'
import { checkMapping } from '../services/healthCheck'
import { logger } from '../services/logger'
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
  ipcMain.handle(IPC.HOST_CREATE, (_e, input: CreateHostInput) => {
    const r = createHost(encryptPlainPassword(input))
    logger.info(null, 'app', `新增主机「${input.name}」· ${input.type === 'local' ? '本地' : '远程SSH'}`)
    return r
  })
  ipcMain.handle(IPC.HOST_UPDATE, (_e, id: string, patch: Partial<CreateHostInput>) => updateHost(id, encryptPlainPassword(patch)))
  ipcMain.handle(IPC.HOST_DELETE, (_e, id: string) => {
    const h = listHosts().find(x => x.id === id)
    deleteHost(id)
    logger.info(null, 'app', `删除主机「${h?.name ?? id}」`)
    return true
  })
  // 设为默认主机
  ipcMain.handle(IPC.HOST_SET_DEFAULT, (_e, id: string) => { setDefaultHost(id); return true })
  // 切换活动主机：真正重建/切换 nginx 引擎（local 重建子进程，remote 重连 SSH）
  ipcMain.handle(IPC.HOST_SWITCH, async (_e, id: string) => {
    try {
      const r = await switchEngineToHost(id)
      if (r.ok) {
        const h = listHosts().find(x => x.id === id)
        logger.info(id, 'app', `切换到主机「${h?.name ?? id}」`)
      }
      return r
    } catch (e: any) {
      return { ok: false, error: e?.message ?? String(e) }
    }
  })

  // Mapping —— 增删改后标记「待应用」，提示用户改动不会立即生效
  ipcMain.handle(IPC.MAPPING_LIST, (_e, hostId: string) => listMappings(hostId))
  ipcMain.handle(IPC.MAPPING_CREATE, (_e, input: MappingInput) => {
    // main 侧校验 protocol/port（渲染进程校验可绕过）
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
  })
  ipcMain.handle(IPC.MAPPING_UPDATE, (_e, id: string, patch: Partial<MappingInput>) => {
    // patch 只校验提供的字段
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
  })
  ipcMain.handle(IPC.MAPPING_DELETE, (_e, id: string) => {
    const m = getMapping(id)
    deleteMapping(id); setEngineDirty(true)
    logger.info(m?.hostId ?? null, 'app', `删除映射「${m?.name ?? id}」`)
    return true
  })

  // Engine
  ipcMain.handle(IPC.ENGINE_START, async () => {
    const eng = getActiveEngine()
    if (!eng) return { status: 'unknown', error: '没有活动的 nginx 引擎（未选中主机或主机非本地类型）' }
    // remote 引擎不支持进程级启停（远程 nginx 由 systemd 管理），明示拒绝而非调假动作。
    if (!eng.supportsProcessControl) {
      return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
    }
    const host = getDefaultHost()
    try {
      await eng.start()
      logger.info(host?.id ?? null, 'app', '引擎启动 · nginx 已开始运行')
      return { status: eng.status(), error: null }
    } catch (e: any) {
      logger.error(host?.id ?? null, 'app', `引擎启动失败 · ${e?.message ?? String(e)}`)
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })
  ipcMain.handle(IPC.ENGINE_STOP, async () => {
    const eng = getActiveEngine()
    if (!eng) return { status: 'unknown', error: null }
    if (!eng.supportsProcessControl) {
      return { status: eng.status(), error: '远程主机不支持在此应用内启停 nginx，请在服务器上用 systemctl 管理' }
    }
    const host = getDefaultHost()
    try {
      await eng.stop()
      logger.info(host?.id ?? null, 'app', '引擎停止 · 所有转发已失效')
      return { status: eng.status(), error: null }
    } catch (e: any) {
      logger.error(host?.id ?? null, 'app', `引擎停止失败 · ${e?.message ?? String(e)}`)
      return { status: eng.status(), error: e?.message ?? String(e) }
    }
  })
  ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    // 用活动 engine 对应的 hostId（而非 getDefaultHost），避免切到非默认主机后
    // 把默认主机的 mappings 写到当前主机（#1 数据错配 bug）。
    const hostId = getActiveHostId()
    if (!hostId) return { ok: false, output: 'no active host' }
    const mappings = listEnabledMappings(hostId)
    // 白名单开启时传入 IP 列表，conf 生成时注入 allow/deny all
    const wlEnabled = isWhitelistEnabled()
    const wlIps = wlEnabled ? getEnabledWhitelistIps() : undefined
    // 安全守卫：白名单开关 ON 但没有任何有效 IP —— fail-open（静默全开）不可接受
    if (wlEnabled && (!wlIps || wlIps.length === 0)) {
      return { ok: false, output: '白名单已开启但未配置任何 IP。请先添加 IP 或关闭白名单后再应用。' }
    }
    try {
      const result = await applyMappings(eng, mappings, {
        hostId,
        whitelistIps: wlIps,
      })
      // 应用成功 = 实时 conf 与 DB 一致，清掉「待应用」标记
      if (result.ok) setEngineDirty(false)
      return { ok: result.ok, output: result.output }
    } catch (e: any) {
      // applyMappings 内部已有 safeApplyConf 兜底，这里防 generateFullConf 等逃逸的异常
      return { ok: false, output: e?.message ?? String(e) }
    }
  })
  ipcMain.handle(IPC.ENGINE_TEST, async () => { const eng = getActiveEngine(); return eng ? eng.test() : { ok: false, output: 'no engine' } })
  ipcMain.handle(IPC.ENGINE_STATUS, () => getActiveEngine()?.status() ?? 'unknown')
  // 是否有「待应用」的改动（增删改 mapping 后置脏，reload 成功后清脏）
  ipcMain.handle(IPC.ENGINE_DIRTY_GET, () => isEngineDirty())
  // 获取 nginx 主进程 PID（local 同步 child.pid，remote 异步读 pidfile/pgrep）
  ipcMain.handle(IPC.ENGINE_PID_GET, async () => {
    const eng = getActiveEngine()
    if (!eng || !eng.getPid) return null
    // Promise.resolve 包装同时兼容同步(local)和异步(remote)返回
    return await Promise.resolve(eng.getPid())
  })
  // 引擎能力：local/remote + 是否支持进程级启停。UI 据此决定是否显示启停按钮。
  ipcMain.handle(IPC.ENGINE_KIND_GET, () => {
    const eng = getActiveEngine()
    if (!eng) return null
    return { engineKind: eng.engineKind, supportsProcessControl: eng.supportsProcessControl }
  })

  // Template
  ipcMain.handle(IPC.TEMPLATE_LIST, () => listAllTemplates())
  ipcMain.handle(IPC.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates())
  ipcMain.handle(IPC.TEMPLATE_SAVE_CUSTOM, (_e, tpl: any) => { saveCustomTemplate(tpl); return true })
  ipcMain.handle(IPC.TEMPLATE_DELETE_CUSTOM, (_e, id: string) => { deleteCustomTemplate(id); return true })

  // Log
  ipcMain.handle(IPC.LOG_LIST, (_e, hostId: string | null, limit: number) => listLogs(hostId, clampInt(limit, 0, 5000)))
  // 清空操作日志
  ipcMain.handle(IPC.LOG_CLEAR, () => { clearLogs(); logger.info(null, 'app', '操作日志已清空'); return true })

  // 读 nginx 运行日志（access.log + error.log + stream_access.log 的最后 N 行）
  // nginx 用相对路径 logs/ 写日志，实际写到 <prefix>/logs/（即 data/nginx-home/logs/）
  ipcMain.handle(IPC.NGINX_LOG_TAIL, (_e, lines: number) => {
    const safeLines = clampInt(lines, 1, 10000)
    // 读取开关：关闭时直接返回空，避免大日志文件 OOM
    const readRow = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('nginx_log_read_enabled') as { value: string } | undefined
    if (readRow?.value === '0') return { access: [], error: [] }

    const paths = resolvePaths()
    const nginxLogsDir = join(paths.dataDir, 'nginx-home', 'logs')
    const result: { access: string[]; error: string[] } = { access: [], error: [] }
    // 尾部倒读（tailFile），避免大日志全量 readFileSync 打爆主进程内存
    try {
      // http access + stream access 各自独立取尾部 N 行，合并后不再截断。
      // 之前合并后 slice(-safeLines) 会把 access.log 内容挤掉（stream_access 占满尾部）。
      const accLines: string[] = []
      for (const f of ['access.log', 'stream_access.log']) {
        const p = join(nginxLogsDir, f)
        if (pathExistsSync(p)) {
          accLines.push(...tailFile(p, safeLines))
        }
      }
      result.access = accLines
    } catch { /* 忽略 */ }
    try {
      const errorLog = join(nginxLogsDir, 'error.log')
      if (pathExistsSync(errorLog)) {
        result.error = tailFile(errorLog, safeLines)
      }
    } catch { /* 忽略 */ }
    return result
  })

  // 清空 nginx 日志文件（用 unlink 删除，nginx 下次写时自动重建，比截断更可靠）
  ipcMain.handle(IPC.NGINX_LOG_CLEAR, () => {
    try {
      const paths = resolvePaths()
      const nginxLogsDir = join(paths.dataDir, 'nginx-home', 'logs')
      for (const f of ['access.log', 'error.log', 'stream_access.log']) {
        try { unlinkSync(join(nginxLogsDir, f)) } catch { /* 文件不存在则忽略 */ }
      }
      logger.info(null, 'app', 'Nginx 日志已清空')
      return true
    } catch { return false }
  })

  // nginx 日志读取开关（关闭后不读 nginx 日志文件，避免大日志 OOM）
  ipcMain.handle(IPC.NGINX_LOG_READ_ENABLED_GET, () => {
    const row = getDb().prepare('SELECT value FROM app_settings WHERE key = ?').get('nginx_log_read_enabled') as { value: string } | undefined
    return row?.value !== '0'  // 默认开启（未设置时返回 true）
  })
  ipcMain.handle(IPC.NGINX_LOG_READ_ENABLED_SET, (_e, enabled: boolean) => {
    getDb().prepare('INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value')
      .run('nginx_log_read_enabled', enabled ? '1' : '0')
    return true
  })

  // Snapshot
  ipcMain.handle(IPC.SNAPSHOT_LIST, (_e, hostId: string) => listSnapshots(hostId, 50))
  ipcMain.handle(IPC.SNAPSHOT_GET, (_e, id: string) => getSnapshot(id))
  ipcMain.handle(IPC.SNAPSHOT_ROLLBACK, async (_e, snapshotId: string) => {
    // 回滚：用快照 content 走 safeApplyConf（备份→写入→校验→reload，失败回滚）
    const snap = getSnapshot(snapshotId)
    if (!snap) return { ok: false, output: 'snapshot not found' }
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const result = await safeApplyConf(eng, snap.content)
    if (result.ok) {
      // 回滚后磁盘 conf = 快照内容，但 DB mappings 没变 → 两者偏离。
      // setEngineDirty(true) 让 UI 显示「待应用」，明示用户：点应用会用 DB 重新生成 conf 覆盖回滚。
      // （之前错误地 setEngineDirty(false)，让用户误以为 DB 与 conf 一致，下次应用会静默覆盖回滚）
      setEngineDirty(true)
      logger.info(snap.hostId, 'app', `手动回滚到快照 ${snap.id} · DB 与 conf 已偏离，点应用会覆盖`)
    } else {
      // 回滚是破坏性操作，失败必须记日志（与 applyFlow 一致），运维才能排查。
      // safeApplyConf 失败时已把磁盘 conf 回滚到回滚前的状态，不会留下坏 conf。
      logger.error(snap.hostId, 'app', `回滚到快照 ${snap.id} 失败(${result.failStage ?? 'unknown'}): ${result.output}`)
    }
    return result
  })

  // Health check
  ipcMain.handle(IPC.HEALTH_CHECK_NOW, async (_e, mappingId: string) => {
    const m = getMapping(mappingId)
    if (!m) return 'unknown'
    return await checkMapping(m)
  })

  // Port check
  ipcMain.handle(IPC.PORT_CHECK, (_e, port: number) => {
    // 非法端口直接返回 false，不尝试监听（防 0=随机端口、负数/字符串异常）
    if (!isValidPort(port)) return false
    return isPortAvailable(port)
  })

  // Host: 测试 SSH 连接
  ipcMain.handle(IPC.HOST_TEST_CONNECTION, async (_e, remoteConfig: any) => {
    // 入参校验：host/port/username 收敛（防渲染层把它当任意内网探测接口）
    const host = typeof remoteConfig?.host === 'string' ? remoteConfig.host.slice(0, 255) : ''
    if (!host) return { ok: false, output: '缺少主机地址' }
    const port = isValidPort(remoteConfig?.port) ? Number(remoteConfig.port) : 22
    const username = typeof remoteConfig?.username === 'string' ? remoteConfig.username.slice(0, 64) : 'root'
    try {
      let password: string | undefined
      if (typeof remoteConfig.passwordEncrypted === 'string' && remoteConfig.passwordEncrypted.startsWith('PLAIN:')) {
        password = remoteConfig.passwordEncrypted.slice(6)
      } else if (typeof remoteConfig.passwordEncrypted === 'string' && remoteConfig.passwordEncrypted) {
        password = decryptPassword(remoteConfig.passwordEncrypted)
      }
      const client = new SshClient({ host, port, username, password })
      const r = await client.exec('echo ok')
      client.close()
      // 输出脱敏：不透出 banner/协议细节，只区分「连接并认证成功/失败」
      return { ok: r.code === 0, output: r.code === 0 ? 'ok' : '认证失败或命令执行失败' }
    } catch {
      // 错误细节（ECONNREFUSED/timeout/SSH 版本）是内网端口/服务探测 oracle，统一收敛
      return { ok: false, output: '连接失败（地址不可达或超时）' }
    }
  })

  // Engine: 刷新 active engine 状态
  ipcMain.handle(IPC.ENGINE_REFRESH_STATUS, async () => {
    await refreshActiveStatus()
    return getActiveEngine()?.status() ?? 'unknown'
  })

  // 一键清除所有痕迹
  // 一键清除所有痕迹。要求传 confirmToken='CONFIRM_WIPE'，防 DevTools 直接调 wipeAllData 绕过 UI 确认。
  ipcMain.handle(IPC.DATA_WIPE, async (_e, confirmToken?: string) => {
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
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // 导出配置
  ipcMain.handle(IPC.DATA_EXPORT, () => {
    return exportData()
  })

  // 导入配置 —— 导入后实时 conf 与 DB 偏离，标记「待应用」
  ipcMain.handle(IPC.DATA_IMPORT, (_e, payload: any) => {
    try {
      const result = importData(payload)
      setEngineDirty(true)
      logger.info(null, 'app', `数据导入完成 · ${result.hostsImported} 主机 / ${result.mappingsImported} 映射`)
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

  // IP 白名单（全局访问控制）
  ipcMain.handle(IPC.WHITELIST_LIST, () => listWhitelist())
  ipcMain.handle(IPC.WHITELIST_ADD, (_e, ip: string, note?: string) => {
    // main 侧校验：渲染进程的 validIpOrCidr 可被绕过，这里兜底。
    // 非法 ip 抛错（与 DB 约束失败同路径），前端 catch 能感知。
    if (!isValidIpOrCidr(ip)) {
      throw new Error('IP 格式不正确，应为 IPv4(1.2.3.4) 或 CIDR(10.0.0.0/24)')
    }
    const r = addWhitelist(ip, note)
    setEngineDirty(true)
    logger.info(null, 'app', `白名单添加 ${ip}`)
    return r
  })
  ipcMain.handle(IPC.WHITELIST_REMOVE, (_e, id: string) => {
    removeWhitelist(id); setEngineDirty(true); return true
  })
  ipcMain.handle(IPC.WHITELIST_TOGGLE_IP, (_e, id: string, enabled: number) => {
    toggleWhitelist(id, enabled); setEngineDirty(true); return true
  })
  ipcMain.handle(IPC.WHITELIST_GET_ENABLED, () => isWhitelistEnabled())
  ipcMain.handle(IPC.WHITELIST_SET_ENABLED, (_e, enabled: boolean) => {
    setWhitelistEnabled(enabled); setEngineDirty(true)
    logger.info(null, 'app', enabled ? '白名单已开启 · 仅允许列表 IP 访问' : '白名单已关闭 · 全部允许')
    return true
  })
}
