import Database from 'better-sqlite3'
import type { Database as DB } from 'better-sqlite3'
import { runMigrations } from './migrations'
import { resolvePaths } from '../paths'

let dbInstance: DB | null = null
let dbPathForTest: string | null = null

export function __setDbPathForTest(path: string): void {
  dbPathForTest = path
}

function resolveDbPath(): string {
  if (dbPathForTest) return dbPathForTest
  const paths = resolvePaths()
  paths.ensureDataDirs()
  return paths.dbPath
}

export function openDb(): DB {
  if (dbInstance) return dbInstance
  const path = resolveDbPath()
  const db = new Database(path)
  try {
    runMigrations(db)
  } catch (e) {
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
  if (!dbInstance) throw new Error('DB not opened. Call openDb() first.')
  return dbInstance
}
