import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { addLog, listLogs, listLogsByLevel, trimLogs } from '@main/db/logRepo'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('logRepo', () => {
  let dbPath: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('addLog 写入并返回 LogEntry', () => {
    const log = addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'started' })
    expect(log.id).toBeTruthy()
    expect(log.message).toBe('started')
    expect(log.timestamp).toBeGreaterThan(0)
  })

  it('listLogs 按 hostId 过滤 + 时间倒序', () => {
    addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'a' })
    addLog({ hostId: 'h1', level: 'error', source: 'nginx-error', message: 'b' })
    addLog({ hostId: 'h2', level: 'info', source: 'app', message: 'c' })
    const list = listLogs('h1', 100)
    expect(list).toHaveLength(2)
    expect(list[0].message).toBe('b')
  })

  it('listLogs limit 生效', () => {
    for (let i = 0; i < 10; i++) addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'm' + i })
    expect(listLogs('h1', 5)).toHaveLength(5)
  })

  it('listLogsByLevel 过滤', () => {
    addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'a' })
    addLog({ hostId: 'h1', level: 'error', source: 'app', message: 'b' })
    const errors = listLogsByLevel('h1', 'error', 100)
    expect(errors).toHaveLength(1)
    expect(errors[0].message).toBe('b')
  })

  it('trimLogs 保留最近 N 条', () => {
    for (let i = 0; i < 10; i++) addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'm' + i })
    const removed = trimLogs(5)
    expect(removed).toBe(5)
    expect(listLogs('h1', 100)).toHaveLength(5)
  })

  it('hostId 为 null 也能写入和查询（全局日志）', () => {
    addLog({ hostId: null, level: 'info', source: 'app', message: 'global' })
    const list = listLogs(null, 100)
    expect(list).toHaveLength(1)
  })
})
