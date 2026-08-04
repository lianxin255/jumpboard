/**
 * 为指定 host 构建 nginx 引擎并设为活动引擎。
 * 独立成文件：避免 index.ts（启动）与 handlers.ts（IPC）互相 import 形成循环依赖。
 * 启动初始化（默认主机）和运行时切换主机（HOST_SWITCH）复用同一逻辑。
 */
import { join } from 'path'
import { ensureDirSync, copySync, pathExistsSync } from 'fs-extra'
import { resolvePaths } from '../paths'
import { getHost } from '../db/hostRepo'
import { listEnabledMappings } from '../db/mappingRepo'
import { isWhitelistEnabled, getEnabledWhitelistIps } from '../db/whitelistRepo'
import { generateFullConf } from './confGenerator'
import { setActiveEngine } from './engineRegistry'
import { LocalNginxEngine } from './localEngine'
import { RemoteNginxEngine } from './remoteEngine'

export async function switchEngineToHost(hostId: string): Promise<boolean> {
  const host = getHost(hostId)
  if (!host) return false
  const paths = resolvePaths()

  let engine: LocalNginxEngine | RemoteNginxEngine
  if (host.type === 'local') {
    const nginxPath = host.localConfig?.nginxPath
      || join(paths.bundledNginxDir, process.platform === 'win32' ? 'nginx.exe' : 'nginx')
    // nginx -p prefix：用 data/nginx-home（跟 exe 走、可写、持久）。
    const prefixDir = join(paths.dataDir, 'nginx-home')
    ensureDirSync(prefixDir)
    // conf 放在 prefix/conf/nginx.conf，nginx 用 -c conf/nginx.conf 相对路径读取。
    // 相对路径解析最可靠，彻底避开绝对路径的反斜杠转义/中文路径问题。
    const confDir = join(prefixDir, 'conf')
    ensureDirSync(confDir)
    const confPath = join(confDir, 'nginx.conf')
    engine = new LocalNginxEngine({
      nginxPath, confPath, logDir: paths.nginxLogDir, tempDir: paths.tempDir, prefixDir
    })
    // mime.types 复制到 prefix/conf（nginx include mime.types 相对 conf 目录解析）
    const bundledMime = join(paths.bundledNginxDir, 'mime.types')
    if (pathExistsSync(bundledMime)) {
      try { copySync(bundledMime, join(confDir, 'mime.types')) } catch { /* 忽略 */ }
    }
    // local：生成初始 conf（即使没 mapping，也要有最小 conf）
    const mappings = listEnabledMappings(host.id)
    // 白名单开启且无有效 IP 时不注入（避免 fail-open 或自锁）
    const wlEnabled = isWhitelistEnabled()
    const wlIps = (wlEnabled && getEnabledWhitelistIps().length > 0) ? getEnabledWhitelistIps() : undefined
    const conf = generateFullConf(mappings, {
      nginxLogDir: paths.nginxLogDir,
      nginxTempDir: paths.tempDir,
      whitelistIps: wlIps,
    })
    await engine.writeConf(conf)
  } else if (host.type === 'remote' && host.remoteConfig) {
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
      testCmd: host.remoteConfig.testCmd
    })
    // 异步刷新状态（不 await，不阻塞）
    if (engine instanceof RemoteNginxEngine) {
      engine.refreshStatus().catch(() => {})
    }
  } else {
    return false
  }
  setActiveEngine(engine)
  return true
}
