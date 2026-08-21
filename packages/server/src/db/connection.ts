import type { Database as DB, Options as DbOptions } from 'better-sqlite3'
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import Database from 'better-sqlite3'
import { resolvePaths } from '../paths'
import { runMigrations } from './migrations'

let dbInstance: DB | null = null
let dbPathForTest: string | null = null

export function __setDbPathForTest(path: string): void {
  dbPathForTest = path
}

function resolveDbPath(): string {
  if (dbPathForTest)
    return dbPathForTest
  const paths = resolvePaths()
  paths.ensureDataDirs()
  return paths.dbPath
}

/**
 * 打包环境（esbuild bundle + resources 分发）下，better-sqlite3 的 JS 已打进
 * server.cjs，但原生 .node 文件在 sidecar 旁，需要用 nativeBinding 显式指定。
 * build-app.mjs 保证 build/app/better_sqlite3.node 是当前平台的二进制：
 *   - 本地构建/测试：从 node_modules 复制当前平台（如 darwin-arm64）的 .node
 *   - Windows 打包：fetch-better-sqlite3.sh 下载 win32-x64 prebuilt 覆盖
 * 开发环境（未打包，__dirname 在 src/ 下找不到 .node）走 better-sqlite3 默认解析。
 */
function resolveNativeBinding(): DbOptions | undefined {
  const candidate = join(__dirname, 'better_sqlite3.node')
  if (existsSync(candidate))
    return { nativeBinding: candidate }
  return undefined
}

export function openDb(): DB {
  if (dbInstance)
    return dbInstance
  const path = resolveDbPath()
  const db = new Database(path, resolveNativeBinding())
  try {
    runMigrations(db)
  }
  catch (e) {
    db.close()
    throw e
  }
  dbInstance = db
  return db
}

export function closeDb(): void {
  if (dbInstance) {
    dbInstance.close()
    dbInstance = null
  }
}

export function getDb(): DB {
  if (!dbInstance)
    throw new Error('DB not opened. Call openDb() first.')
  return dbInstance
}
