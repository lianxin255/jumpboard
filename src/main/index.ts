import { app, BrowserWindow, dialog } from 'electron'
import { join } from 'path'
import { copyFileSync, existsSync } from 'fs'
import { resolvePaths } from './paths'
import { openDb, closeDb } from './db/connection'
import { registerIpcHandlers } from './ipc/handlers'
import { setActiveEngine, getActiveEngine } from './nginx/engineRegistry'
import { LocalNginxEngine } from './nginx/localEngine'
import { RemoteNginxEngine } from './nginx/remoteEngine'
import { getDefaultHost, listHosts, createHost, setDefaultHost } from './db/hostRepo'
import { listEnabledMappings } from './db/mappingRepo'
import { generateFullConf } from './nginx/confGenerator'
import { startHealthCheckLoop, stopHealthCheckLoop } from './services/healthCheck'

function createWindow() {
  const win = new BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 900,
    minHeight: 600,
    autoHideMenuBar: true,
    icon: join(__dirname, '../../build/icon.png'),
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  if (process.env.ELECTRON_RENDERER_URL) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

async function initEngineForDefaultHost() {
  const host = getDefaultHost()
  if (!host) return
  const paths = resolvePaths()

  let engine: LocalNginxEngine | RemoteNginxEngine
  if (host.type === 'local') {
    const nginxPath = host.localConfig?.nginxPath
      || join(paths.bundledNginxDir, process.platform === 'win32' ? 'nginx.exe' : 'nginx')
    const confPath = join(paths.nginxConfDir, 'nginx.conf')
    engine = new LocalNginxEngine({
      nginxPath, confPath, logDir: paths.nginxLogDir, tempDir: paths.tempDir
    })
    // 把内置 mime.types 复制到 nginx 工作目录，让 `include mime.types;` 能解析。
    // Windows nginx 对 include 的相对路径是相对 `-p` prefix（即 tempDir）解析的，
    // 而不是相对 conf 文件目录。tempDir 和 nginxConfDir 都覆盖（belt-and-suspenders），
    // 确保无论 nginx 以哪种解析方式都能找到。
    // 注意：此处依赖 whenReady 中已调用过 paths.ensureDataDirs()，tempDir/nginxConfDir 已存在。
    const bundledMime = join(paths.bundledNginxDir, 'mime.types')
    if (existsSync(bundledMime)) {
      for (const targetDir of [paths.tempDir, paths.nginxConfDir]) {
        const dest = join(targetDir, 'mime.types')
        try { copyFileSync(bundledMime, dest) } catch { /* 忽略，可能已存在或目标只读 */ }
      }
    }
    // local：生成初始 conf（即使没 mapping，也要有最小 conf）
    const mappings = listEnabledMappings(host.id)
    const conf = generateFullConf(mappings, {
      nginxLogDir: paths.nginxLogDir,
      nginxTempDir: paths.tempDir
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
    // 异步刷新状态（不 await，不阻塞启动）
    if (engine instanceof RemoteNginxEngine) {
      engine.refreshStatus().catch(() => {})
    }
  } else {
    return
  }
  setActiveEngine(engine)
}

app.whenReady().then(async () => {
  try {
    const paths = resolvePaths()
    paths.ensureDataDirs()
    openDb()

    // 首次启动：如果没有 host，创建默认 local host
    if (listHosts().length === 0) {
      const h = createHost({
        name: '本机网关',
        type: 'local',
        isDefault: 1,
        localConfig: { nginxPath: '', confDir: '', logDir: '' },
        remoteConfig: null
      })
      setDefaultHost(h.id)
    }

    registerIpcHandlers()
    await initEngineForDefaultHost()
    createWindow()
    startHealthCheckLoop()

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) createWindow()
    })
  } catch (err) {
    console.error('[startup] fatal:', err)
    dialog.showErrorBox(
      'Nginx 跳板机启动失败',
      `启动过程中发生错误：\n\n${err instanceof Error ? err.message : String(err)}\n\n程序将退出。`
    )
    app.exit(1)
  }
})

// 退出无痕：所有窗口关闭时退出 app（nginx 子进程在 before-quit 里杀）
app.on('window-all-closed', () => {
  app.quit()
})

// 关键：退出时确保 nginx 子进程被杀掉 —— "退出无痕"的核心
// 退出安全分析：
//   1. first quit → before-quit 触发 → eng.status() === 'running' → preventDefault → stop() → app.quit() 再次触发 before-quit
//   2. 第二次 before-quit → stop 已完成，eng.status() === 'stopped' → 跳过 preventDefault → closeDb() → 继续退出
//   注意：第二次 closeDb 会再次执行（若 dbInstance 已 null 则 no-op），幂等。
//   I1 兜底：用 .catch().finally() 确保 app.quit() 总会触发，即使 stop() 抛错；
//            用 return 跳过 closeDb()，仅在第二次 pass 时执行 closeDb()。
app.on('before-quit', (e) => {
  const eng = getActiveEngine()
  if (eng && eng.status() === 'running') {
    e.preventDefault()
    eng.stop()
      .catch((err) => console.error('[exit] engine.stop failed:', err))
      .finally(() => app.quit())
    return
  }
  stopHealthCheckLoop()
  closeDb()
})
