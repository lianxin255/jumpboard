import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import { createSnapshot, listSnapshots, getSnapshot, trimSnapshots } from '@main/db/snapshotRepo'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('snapshotRepo', () => {
  let dbPath: string
  let hostId: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
    hostId = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null }).id
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('createSnapshot 写入并返回', () => {
    const s = createSnapshot({ hostId, content: 'worker_processes 1;', reason: 'reload' })
    expect(s.id).toBeTruthy()
    expect(s.content).toContain('worker_processes')
    expect(s.reason).toBe('reload')
  })

  it('listSnapshots 按 hostId 倒序', () => {
    createSnapshot({ hostId, content: 'v1', reason: 'r1' })
    createSnapshot({ hostId, content: 'v2', reason: 'r2' })
    const list = listSnapshots(hostId, 100)
    expect(list).toHaveLength(2)
    expect(list[0].content).toBe('v2')  // 最新在前
  })

  it('getSnapshot 按 id', () => {
    const s = createSnapshot({ hostId, content: 'conf', reason: 'manual' })
    expect(getSnapshot(s.id)?.content).toBe('conf')
  })

  it('trimSnapshots 每 host 保留 N 条', () => {
    for (let i = 0; i < 10; i++) createSnapshot({ hostId, content: 'v' + i, reason: 'r' })
    const removed = trimSnapshots(5)
    expect(removed).toBe(5)
    expect(listSnapshots(hostId, 100)).toHaveLength(5)
  })

  it('trimSnapshots 多 host 独立保留', () => {
    const host2 = createHost({ name: 'h2', type: 'local', isDefault: 0, localConfig: null, remoteConfig: null }).id
    for (let i = 0; i < 3; i++) createSnapshot({ hostId, content: 'a' + i, reason: 'r' })
    for (let i = 0; i < 3; i++) createSnapshot({ hostId: host2, content: 'b' + i, reason: 'r' })
    const removed = trimSnapshots(2)
    expect(removed).toBe(2)  // 每个 host 删 1 条，共 2 条
    expect(listSnapshots(hostId, 100)).toHaveLength(2)
    expect(listSnapshots(host2, 100)).toHaveLength(2)
  })
})
