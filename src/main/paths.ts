import { join, dirname } from 'path'
import { mkdirSync } from 'fs'

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
  } catch {
    return null
  }
}

/**
 * data 目录根：用户数据跟 app 走（绿色版核心要求）。
 *
 * 用 Electron 成熟的 app.getPath('exe') + app.isPackaged API 定位，
 * 不再手写 PORTABLE_EXECUTABLE_DIR 等环境变量探测。
 *
 * - Windows portable：exe 旁目录
 * - macOS .app：.app 所在目录（dirname 三次从 MacOS/Foo 回到 Foo.app 父目录）
 * - 开发：项目根目录（process.cwd()）
 * - 测试：appRootForTest 覆盖
 */
function getDataRoot(): string {
  if (appRootForTest) return appRootForTest
  const app = getElectronApp()
  if (app?.isPackaged) {
    const exePath = app.getPath('exe')
    if (process.platform === 'darwin') {
      // Foo.app/Contents/MacOS/Foo → Foo.app 所在目录
      return dirname(dirname(dirname(exePath)))
    }
    return dirname(exePath)
  }
  // 开发环境
  return process.cwd()
}

/**
 * resources 目录：内置 nginx + 模板所在。
 * 打包后在 process.resourcesPath（Electron 标准 API）。
 * 开发时在项目根的 resources/ 下。
 */
function getResourcesDir(): string {
  const app = getElectronApp()
  if (app?.isPackaged && process.resourcesPath) {
    return process.resourcesPath
  }
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
    bundledNginxDir: join(resourcesDir, 'nginx'),
    ensureDataDirs: () => {
      for (const d of [
        dataDir,
        join(dataDir, 'nginx-conf'),
        join(dataDir, 'nginx-conf', 'http'),
        join(dataDir, 'nginx-conf', 'stream'),
        join(dataDir, 'logs'),
        join(dataDir, 'snapshots'),
        join(dataDir, 'temp')
      ]) {
        mkdirSync(d, { recursive: true })
      }
    }
  }
}
