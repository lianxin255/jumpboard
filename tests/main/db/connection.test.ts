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

  it('删除 host 时级联删除关联 mappings（FK CASCADE 生效）', () => {
    const db = openDb()
    // 先建一个 host
    db.prepare(`INSERT INTO hosts (id, name, type, is_default, status, created_at)
                VALUES ('h1', 'test', 'local', 0, 'unknown', 0)`).run()
    // 建两个关联的 mapping
    db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, upstream_host, upstream_port, redirect_http, health_check_enabled, health_check_interval_sec, health_status, sort_order, created_at, updated_at)
                VALUES ('m1', 'h1', 'a', 1, 'tcp', '1.1.1.1', 80, 0, 0, 30, 'unknown', 0, 0, 0)`).run()
    db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, upstream_host, upstream_port, redirect_http, health_check_enabled, health_check_interval_sec, health_status, sort_order, created_at, updated_at)
                VALUES ('m2', 'h1', 'b', 1, 'tcp', '1.1.1.1', 81, 0, 0, 30, 'unknown', 1, 0, 0)`).run()
    // 删 host
    db.prepare('DELETE FROM hosts WHERE id = ?').run('h1')
    // mappings 应该被级联删除
    const remaining = db.prepare('SELECT COUNT(*) AS c FROM mappings WHERE host_id = ?').get('h1') as { c: number }
    expect(remaining.c).toBe(0)
  })
})
