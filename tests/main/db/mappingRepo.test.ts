import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import {
  createMapping, listMappings, getMapping, updateMapping, deleteMapping, listEnabledMappings
} from '@main/db/mappingRepo'
import type { MappingInput } from '@shared/types'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

function tcpInput(hostId: string, over: Partial<MappingInput> = {}): MappingInput {
  return {
    hostId,
    name: 'test',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '192.168.1.10',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    ...over
  }
}

describe('mappingRepo', () => {
  let dbPath: string
  let hostId: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
    const h = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    hostId = h.id
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('createMapping 创建 TCP 映射', () => {
    const m = createMapping(tcpInput(hostId))
    expect(m.id).toBeTruthy()
    expect(m.protocol).toBe('tcp')
    expect(m.healthStatus).toBe('unknown')
    expect(m.sortOrder).toBe(0)
  })

  it('listMappings 按 hostId 过滤', () => {
    createMapping(tcpInput(hostId, { name: 'a' }))
    createMapping(tcpInput(hostId, { name: 'b' }))
    expect(listMappings(hostId)).toHaveLength(2)
    expect(listMappings('other-id')).toHaveLength(0)
  })

  it('listEnabledMappings 只返回 enabled=1', () => {
    createMapping(tcpInput(hostId, { enabled: 1, name: 'on' }))
    createMapping(tcpInput(hostId, { enabled: 0, name: 'off' }))
    const list = listEnabledMappings(hostId)
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('on')
  })

  it('updateMapping 改端口', () => {
    const m = createMapping(tcpInput(hostId))
    updateMapping(m.id, { listenPort: 8888 })
    expect(getMapping(m.id)?.listenPort).toBe(8888)
  })

  it('deleteMapping 删除', () => {
    const m = createMapping(tcpInput(hostId))
    deleteMapping(m.id)
    expect(getMapping(m.id)).toBeUndefined()
  })

  it('sortOrder 自增', () => {
    const a = createMapping(tcpInput(hostId, { name: 'a' }))
    const b = createMapping(tcpInput(hostId, { name: 'b' }))
    expect(b.sortOrder).toBeGreaterThan(a.sortOrder)
  })

  it('updateMapping 不影响 sortOrder/healthStatus/createdAt', () => {
    const m = createMapping(tcpInput(hostId))
    const origSort = m.sortOrder
    const origCreated = m.createdAt
    updateMapping(m.id, { listenPort: 7777, name: 'changed' })
    const got = getMapping(m.id)
    expect(got?.sortOrder).toBe(origSort)
    expect(got?.healthStatus).toBe('unknown')
    expect(got?.createdAt).toBe(origCreated)
    expect(got?.listenPort).toBe(7777)
    expect(got?.name).toBe('changed')
  })

  it('全字段往返：createMapping 的所有字段都能通过 getMapping 读回', () => {
    const input = tcpInput(hostId, {
      name: '全字段',
      enabled: 1,
      protocol: 'tcp',
      listenPort: 12345,
      serverNames: null,
      listenPath: null,
      upstreamHost: '10.0.0.5',
      upstreamPort: 5432,
      tlsCertPath: null,
      tlsKeyPath: null,
      redirectHttp: 0,
      extraDirectives: 'proxy_timeout 2h;',
      healthCheckEnabled: 1,
      healthCheckIntervalSec: 60,
      healthCheckTargetPath: null,
      healthCheckExpectedStatus: null,
    })
    const m = createMapping(input)
    const got = getMapping(m.id)
    expect(got).toBeDefined()
    expect(got?.name).toBe('全字段')
    expect(got?.protocol).toBe('tcp')
    expect(got?.listenPort).toBe(12345)
    expect(got?.upstreamHost).toBe('10.0.0.5')
    expect(got?.upstreamPort).toBe(5432)
    expect(got?.extraDirectives).toBe('proxy_timeout 2h;')
    expect(got?.healthCheckEnabled).toBe(1)
    expect(got?.healthCheckIntervalSec).toBe(60)
    expect(got?.hostId).toBe(hostId)
  })
})
