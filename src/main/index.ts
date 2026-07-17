import { app, BrowserWindow, dialog } from 'electron'
import { join } from 'path'
import { resolvePaths } from './paths'
import { openDb, closeDb } from './db/connection'
import { registerIpcHandlers } from './ipc/handlers'
import { setActiveEngine, getActiveEngine } from './nginx/engineRegistry'
import { LocalNginxEngine } from './nginx/localEngine'
import { getDefaultHost, listHosts, createHost, setDefaultHost } from './db/hostRepo'
import { listEnabledMappings } from './db/mappingRepo'
import { generateFullConf } from './nginx/confGenerator'

function createWindow() {
  const win = new BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 900,
    minHeight: 600,
    autoHideMenuBar: true,
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
  if (!host || host.type !== 'local') return
  const paths = resolvePaths()
  const nginxPath = host.localConfig?.nginxPath
    || join(paths.bundledNginxDir, process.platform === 'win32' ? 'nginx.exe' : 'nginx')
  const confPath = join(paths.nginxConfDir, 'nginx.conf')
  const engine = new LocalNginxEngine({
    nginxPath, confPath, logDir: paths.nginxLogDir, tempDir: paths.tempDir
  })
  // 生成初始 conf（即使没 mapping，也要有最小 conf）
  const mappings = listEnabledMappings(host.id)
  const conf = generateFullConf(mappings, {
    nginxLogDir: paths.nginxLogDir,
    nginxTempDir: paths.tempDir
  })
  await engine.writeConf(conf)
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
  closeDb()
})
