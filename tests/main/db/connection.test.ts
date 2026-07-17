import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('db connection', () => {
  let dbPath: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('openDb 创建数据库文件', () => {
    openDb()
    expect(existsSync(dbPath)).toBe(true)
  })

  it('建表后表存在', () => {
    const db = openDb()
    const tables = db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all() as { name: string }[]
    const names = tables.map(t => t.name)
    expect(names).toContain('hosts')
    expect(names).toContain('mappings')
    expect(names).toContain('config_snapshots')
    expect(names).toContain('app_logs')
  })

  it('重复 openDb 不报错（幂等）', () => {
    openDb()
    expect(() => openDb()).not.toThrow()
  })
})
