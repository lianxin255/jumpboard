import { join, dirname } from 'path'
import { mkdirSync } from 'fs'

let appRootForTest: string | null = null

export function __setAppRootForTest(root: string): void {
  appRootForTest = root
}

function getAppRoot(): string {
  if (appRootForTest) return appRootForTest
  // 生产环境：exe 所在目录；开发环境：项目根目录
  if (process.env.NODE_ENV === 'production') {
    // electron-builder portable 下，process.execPath 指向 exe
    return dirname(process.execPath)
  }
  return process.cwd()
}

export interface AppPaths {
  appRoot: string
  dataDir: string
  dbPath: string
  nginxConfDir: string
  nginxLogDir: string
  snapshotDir: string
  tempDir: string
  resourcesDir: string
  bundledNginxDir: string
  ensureDataDirs: () => void
}

export function resolvePaths(): AppPaths {
  const appRoot = getAppRoot()
  const dataDir = join(appRoot, 'data')
  const resourcesDir = join(appRoot, 'resources')

  return {
    appRoot,
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
