import { dirname, join } from 'node:path'
import { closeDb, openDb } from '@ngj/server/db/connection'
import { createHost, getDefaultHost, listHosts, setDefaultHost } from '@ngj/server/db/hostRepo'
import { registerIpcHandlers } from '@ngj/server/ipc'
import { getActiveEngine } from '@ngj/server/nginx/engineRegistry'
import { switchEngineToHost } from '@ngj/server/nginx/hostEngineSwitch'
import { resolvePaths } from '@ngj/server/paths'
import { startHealthCheckLoop, stopHealthCheckLoop } from '@ngj/server/services/healthCheck'
import { logger } from '@ngj/server/services/logger'
import { app, BrowserWindow, dialog, ipcMain, shell } from 'electron'

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
      nodeIntegration: false,
      sandbox: false,
    },
  })

  if (process.env.ELECTRON_RENDERER_URL) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  }
  else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }

  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//.test(url))
      shell.openExternal(url)
    return { action: 'deny' }
  })

  win.webContents.on('will-navigate', (e, url) => {
    const devServer = process.env.ELECTRON_RENDERER_URL
    const allowed = url.startsWith('file://') || (devServer ? url.startsWith(devServer) : false)
    if (!allowed) {
      e.preventDefault()
      if (/^https?:\/\//.test(url))
        shell.openExternal(url)
    }
  })
}

app.whenReady().then(async () => {
  try {
    if (process.platform === 'win32' && app.isPackaged) {
      const exeDir = dirname(app.getPath('exe'))
      // eslint-disable-next-line no-control-regex -- \x00-\x7F is the ASCII range; nginx.exe uses ANSI code page
      if (/[^\x00-\x7F]/.test(exeDir)) {
        dialog.showErrorBox(
          '路径包含中文，无法启动',
          '本程序内置的 nginx 不支持包含中文的路径。\n\n'
          + `当前路径：\n${exeDir}\n\n`
          + '解决方法：把整个程序文件夹复制或移动到纯英文路径下（例如 D:\\nginx-jump\\），再重新运行。',
        )
        app.exit(1)
        return
      }
    }

    const paths = resolvePaths()
    paths.ensureDataDirs()
    openDb()

    if (listHosts().length === 0) {
      const h = createHost({
        name: '本机网关',
        type: 'local',
        isDefault: 1,
        localConfig: { nginxPath: '', confDir: '', logDir: '' },
        remoteConfig: null,
      })
      setDefaultHost(h.id)
    }

    registerIpcHandlers(ipcMain)
    const def = getDefaultHost()
    if (def) {
      const r = await switchEngineToHost(def.id)
      if (!r.ok)
        logger.warn(def.id, 'app', `启动时切换默认主机失败：${r.error ?? '未知原因'}`)
    }
    createWindow()
    startHealthCheckLoop()

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0)
        createWindow()
    })
  }
  catch (err) {
    console.error('[startup] fatal:', err)
    dialog.showErrorBox(
      'Nginx 跳板机启动失败',
      `启动过程中发生错误：\n\n${err instanceof Error ? err.message : String(err)}\n\n程序将退出。`,
    )
    app.exit(1)
  }
})

app.on('window-all-closed', () => {
  app.quit()
})

app.on('before-quit', (e) => {
  const eng = getActiveEngine()
  if (eng && eng.status() === 'running') {
    e.preventDefault()
    eng.stop()
      .catch((err: any) => console.error('[exit] engine.stop failed:', err))
      .finally(() => app.quit())
    return
  }
  stopHealthCheckLoop()
  closeDb()
})
