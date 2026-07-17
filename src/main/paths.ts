import { join, dirname } from 'path'
import { mkdirSync } from 'fs'

let appRootForTest: string | null = null

export function __setAppRootForTest(root: string | null): void {
  appRootForTest = root
}

/**
 * data 目录的根：用户数据跟 exe 走（绿色版核心要求）。
 * - portable 模式：PORTABLE_EXECUTABLE_DIR 指向真正 exe 位置（U 盘/文件夹）
 * - 打包非 portable：exe 所在目录
 * - 开发：项目根目录
 */
function getDataRoot(): string {
  if (appRootForTest) return appRootForTest
  // portable 模式：electron-builder 注入此环境变量，指向真正 exe 所在目录
  if (process.env.PORTABLE_EXECUTABLE_DIR) {
    return process.env.PORTABLE_EXECUTABLE_DIR
  }
  // 打包环境：exe 所在目录（dirname(execPath) 对 nsis 安装版是安装目录）
  try {
    const electron = require('electron')
    if (electron?.app?.isPackaged) {
      return dirname(electron.app.getPath('exe'))
    }
  } catch {
    // 非 electron 环境（测试）
  }
  // 开发环境：项目根目录
  return process.cwd()
}

/**
 * resources 目录：内置 nginx.exe + 模板所在。
 * 打包后这些资源在 Electron 的 process.resourcesPath 下（由 extraResources 配置决定）。
 * 开发时在项目根的 resources/ 下。
 *
 * 关键：portable 模式下 process.resourcesPath 指向临时解压目录（resources 在那里），
 * 而 data 目录跟 exe 走（PORTABLE_EXECUTABLE_DIR）。两者不在同一处，必须分开算。
 */
function getResourcesDir(): string {
  // process.resourcesPath 是 Electron 注入的，打包后指向 app/resources
  // extraResources 把 nginx → resources/nginx，templates → resources/templates
  if (process.resourcesPath && process.resourcesPath !== 'None') {
    return process.resourcesPath
  }
  // 开发环境：项目根的 resources/
  return join(getDataRoot(), 'resources')
}

export interface AppPaths {
  /** data 目录根（跟 exe 走） */
  dataRoot: string
  dataDir: string
  dbPath: string
  nginxConfDir: string
  nginxLogDir: string
  snapshotDir: string
  tempDir: string
  /** resources 目录（内置 nginx/模板，打包后在 process.resourcesPath） */
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
