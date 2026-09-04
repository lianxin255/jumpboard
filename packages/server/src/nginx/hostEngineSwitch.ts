/**
 * 为指定 host 构建 nginx 引擎并设为活动引擎。
 * 独立成文件：避免 index.ts（启动）与 handlers.ts（IPC）互相 import 形成循环依赖。
 * 启动初始化（默认主机）和运行时切换主机（HOST_SWITCH）复用同一逻辑。
 */
import { join } from 'node:path'
import { copySync, ensureDirSync, pathExistsSync } from 'fs-extra'
import { getHost } from '../db/hostRepo'
import { listEnabledMappings } from '../db/mappingRepo'
import { getEnabledWhitelistIps, isWhitelistEnabled } from '../db/whitelistRepo'
import { resolvePaths } from '../paths'
import { generateFullConf } from './confGenerator'
import { getActiveEngine, setActiveEngine } from './engineRegistry'
import { LocalNginxEngine } from './localEngine'
import { RemoteNginxEngine } from './remoteEngine'
import { stopStaleNginx } from './staleNginx'

export async function switchEngineToHost(hostId: string): Promise<{ ok: boolean, error?: string }> {
  const host = getHost(hostId)
  if (!host)
    return { ok: false, error: '主机不存在' }
  const paths = resolvePaths()

  // 白名单开启但无有效 IP：fail-closed，拒绝切换（与 ENGINE_RELOAD 的守卫一致）。
  // 此检查对 local 和 remote 都生效——之前只在 local 分支内检查，remote 被绕过。
  const wlEnabled = isWhitelistEnabled()
  if (wlEnabled && getEnabledWhitelistIps().length === 0) {
    return { ok: false, error: '白名单已开启但未配置任何 IP，请先添加 IP 或关闭白名单。' }
  }
  const wlIps = wlEnabled ? getEnabledWhitelistIps() : undefined

  let engine: LocalNginxEngine | RemoteNginxEngine
  if (host.type === 'local') {
    // 旧引擎是本地且在运行 → 先停，否则旧 nginx 变孤儿占端口，新引擎 start 时 bind 失败
    const prev = getActiveEngine()
    if (prev?.engineKind === 'local' && prev.status() === 'running') {
      try { await prev.stop() }
      catch { /* 停不掉也要继续切换，错误由后续 start 暴露 */ }
    }
    const nginxPath = host.localConfig?.nginxPath
      || join(paths.bundledNginxDir, process.platform === 'win32' ? 'nginx.exe' : 'nginx')
    // nginx -p prefix：用 data/nginx-home（跟 exe 走、可写、持久）。
    const prefixDir = join(paths.dataDir, 'nginx-home')
    // 上次应用崩溃/强杀遗留的孤儿 nginx（不在引擎注册表里）按 pid 文件清理
    await stopStaleNginx(prefixDir)
    ensureDirSync(prefixDir)
    // conf 放在 prefix/conf/nginx.conf，nginx 用 -c conf/nginx.conf 相对路径读取。
    // 相对路径解析最可靠，彻底避开绝对路径的反斜杠转义/中文路径问题。
    const confDir = join(prefixDir, 'conf')
    ensureDirSync(confDir)
    const confPath = join(confDir, 'nginx.conf')
    engine = new LocalNginxEngine({
      nginxPath,
      confPath,
      logDir: paths.nginxLogDir,
      tempDir: paths.tempDir,
      prefixDir,
    })
    // mime.types 复制到 prefix/conf（nginx include mime.types 相对 conf 目录解析）
    const bundledMime = join(paths.bundledNginxDir, 'mime.types')
    if (pathExistsSync(bundledMime)) {
      try { copySync(bundledMime, join(confDir, 'mime.types')) }
      catch { /* 忽略 */ }
    }
    // local：生成初始 conf（即使没 mapping，也要有最小 conf）。
    // 此处不走 safeApplyConf（不 test/reload），且在 setActiveEngine 之前执行，
    // 此时无 active engine，reload/rollback 不可能并发，无需加锁。
    const mappings = listEnabledMappings(host.id)
    const conf = generateFullConf(mappings, {
      whitelistIps: wlIps,
    })
    await engine.writeConf(conf)
  }
  else if (host.type === 'remote' && host.remoteConfig) {
    // remote：不自动写 conf（远程 conf 由远程自己维护），只建连接 + 异步刷新状态
    engine = new RemoteNginxEngine({
      host: host.remoteConfig.host,
      port: host.remoteConfig.port,
      username: host.remoteConfig.username,
      passwordEncrypted: host.remoteConfig.passwordEncrypted,
      privateKeyPath: host.remoteConfig.keyPath,
      nginxPath: host.remoteConfig.nginxPath,
      confPath: host.remoteConfig.confPath,
      reloadCmd: host.remoteConfig.reloadCmd,
      testCmd: host.remoteConfig.testCmd,
    })
    // 异步刷新状态（不 await，不阻塞）。refreshStatus 是接口方法，local/remote 都有。
    engine.refreshStatus().catch(() => {})
  }
  else {
    return { ok: false, error: '主机类型无效或缺少远程配置' }
  }
  setActiveEngine(engine, hostId)
  return { ok: true }
}
