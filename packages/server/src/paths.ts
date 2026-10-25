import { dirname, join } from 'node:path'
import { ensureDirSync } from 'fs-extra'

let appRootForTest: string | null = null

export function __setAppRootForTest(root: string | null): void {
  appRootForTest = root
}

/**
 * 安全获取 Electron app 实例。测试环境（vitest 无 electron）返回 null。
 * 用动态 require 避免顶层 import 在非 electron 环境崩溃。
 */
function getElectronApp(): any | null {
  try {
    const electron = require('electron')
    return electron?.app ?? null
  }
  catch {
    return null
  }
}

/**
 * data 目录根：用户数据跟 app 走（绿色版核心要求）。
 *
 * - Windows portable：PORTABLE_EXECUTABLE_DIR（NSIS 注入，exe 旁目录）
 * - macOS .app：.app 父目录
 * - 开发：process.cwd()
 * - 测试：appRootForTest 覆盖
 */
function getDataRoot(): string {
  if (appRootForTest)
    return appRootForTest
  const app = getElectronApp()
  if (app?.isPackaged) {
    if (process.platform === 'win32' && process.env.PORTABLE_EXECUTABLE_DIR)
      return process.env.PORTABLE_EXECUTABLE_DIR
    const exePath = app.getPath('exe')
    if (process.platform === 'darwin') {
      // exe: Foo.app/Contents/MacOS/Foo → 4 层 dirname 到 Foo.app 的父目录。
      // 数据必须放 .app 旁边而不是包内：写入 .app 内部会破坏代码签名，
      // 且应用升级覆盖 .app 时会丢用户数据（e2e 实测踩中）。
      return dirname(dirname(dirname(dirname(exePath))))
    }
    return dirname(exePath)
  }
  return process.cwd()
}

/**
 * resources 目录：内置 nginx + 模板所在。
 * 打包后在 process.resourcesPath（Electron 标准 API）。
 * 开发时在项目根的 resources/ 下。
 */
function getResourcesDir(): string {
  const app = getElectronApp()
  if (app?.isPackaged && process.resourcesPath)
    return process.resourcesPath
  return join(getDataRoot(), 'resources')
}

export interface AppPaths {
  /** data 目录根（跟 exe/app 走） */
  dataRoot: string
  dataDir: string
  dbPath: string
  nginxConfDir: string
  nginxLogDir: string
  snapshotDir: string
  tempDir: string
  /** resources 目录（内置 nginx/模板） */
  resourcesDir: string
  bundledNginxDir: string
  ensureDataDirs: () => void
}

export function resolvePaths(): AppPaths {
  const dataRoot = getDataRoot()
  const dataDir = join(dataRoot, 'data')
  const resourcesDir = getResourcesDir()

  return {
    dataRoot,
    dataDir,
    dbPath: join(dataDir, 'data.db'),
    nginxConfDir: join(dataDir, 'nginx-conf'),
    nginxLogDir: join(dataDir, 'logs'),
    snapshotDir: join(dataDir, 'snapshots'),
    tempDir: join(dataDir, 'temp'),
    resourcesDir,
    bundledNginxDir: join(resourcesDir, 'bundled', 'nginx'),
    ensureDataDirs: () => {
      for (const d of [
        dataDir,
        join(dataDir, 'nginx-conf'),
        join(dataDir, 'nginx-conf', 'http'),
        join(dataDir, 'nginx-conf', 'stream'),
        join(dataDir, 'logs'),
        join(dataDir, 'snapshots'),
        join(dataDir, 'temp'),
      ]) {
        ensureDirSync(d)
      }
    },
  }
}
