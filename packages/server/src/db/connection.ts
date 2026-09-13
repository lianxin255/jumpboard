import { createRequire } from 'node:module'
import { resolvePaths } from '../paths'
import { runMigrations } from './migrations'

/**
 * SQLite 连接：用 Node/Electron 内置的 node:sqlite（Electron 35+ 自带），
 * 替代原生模块 better-sqlite3 —— 无 ABI、无跨平台编译、打包链路无需 prebuild。
 * API 与 better-sqlite3 同形（prepare().run/get/all），差异：
 * - 无 .pragma() helper，用 db.exec('PRAGMA ...')
 * - 无 .transaction() helper，用 withTransaction()
 */

// vitest 1.x（vite-node）的内置模块清单不认识 node:sqlite，静态 import 会被当
// 第三方包解析报 "Failed to load url sqlite"（vitest#7177）。用 createRequire
// 走 Node 原生 require 加载，绕开 vite 模块管线；Electron 主进程 CJS bundle 同样可用。
const requireNative = createRequire(import.meta.url)
const { DatabaseSync } = requireNative('node:sqlite') as typeof import('node:sqlite')

export type SqliteDb = InstanceType<typeof DatabaseSync>

let dbInstance: SqliteDb | null = null
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

export function openDb(): SqliteDb {
  if (dbInstance)
    return dbInstance
  const path = resolveDbPath()
  const db = new DatabaseSync(path)
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

export function getDb(): SqliteDb {
  if (!dbInstance)
    throw new Error('DB not opened. Call openDb() first.')
  return dbInstance
}

/** 事务包装（node:sqlite 无 .transaction helper）。fn 内抛错则回滚并重抛。 */
export function withTransaction<T>(db: SqliteDb, fn: () => T): T {
  db.exec('BEGIN')
  try {
    const out = fn()
    db.exec('COMMIT')
    return out
  }
  catch (e) {
    try { db.exec('ROLLBACK') }
    catch { /* BEGIN 已被回滚/连接异常时忽略 */ }
    throw e
  }
}
