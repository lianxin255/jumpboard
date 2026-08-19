import { join } from 'node:path'
import { ensureDirSync } from 'fs-extra'

let appRootForTest: string | null = null

export function __setAppRootForTest(root: string | null): void {
  appRootForTest = root
}

/**
 * data 目录根：用户数据跟 app 走（绿色版核心要求）。
 *
 * Tauri sidecar 启动时由 Rust 壳注入 NGJ_DATA_ROOT 环境变量（exe 旁 data/ 目录）。
 * 开发时 fallback 到 process.cwd()。
 * 测试时 appRootForTest 覆盖。
 */
function getDataRoot(): string {
  if (appRootForTest)
    return appRootForTest
  if (process.env.NGJ_DATA_ROOT)
    return process.env.NGJ_DATA_ROOT
  return process.cwd()
}

/**
 * resources 目录：内置 nginx + 模板所在。
 * Tauri sidecar 启动时由 Rust 壳注入 NGJ_RESOURCES_DIR（resource_dir()）。
 * 开发时在项目根的 resources/ 下。
 */
function getResourcesDir(): string {
  if (process.env.NGJ_RESOURCES_DIR)
    return process.env.NGJ_RESOURCES_DIR
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
        join(dataDir, 'temp'),
      ]) {
        ensureDirSync(d)
      }
    },
  }
}
