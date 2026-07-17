import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import { createMapping } from '@main/db/mappingRepo'
import { exportData, importData } from '@main/services/dataTransfer'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('dataTransfer', () => {
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

  it('exportData 返回含 version + hosts + mappings', () => {
    const h = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    createMapping({
      hostId: h.id, name: 'm', enabled: 1, protocol: 'tcp', listenPort: 9000,
      serverNames: null, listenPath: null, upstreamHost: '1.1.1.1', upstreamPort: 80,
      tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null,
      healthCheckEnabled: 0, healthCheckIntervalSec: 30, healthCheckTargetPath: null, healthCheckExpectedStatus: null
    })
    const data = exportData()
    expect(data.version).toBe(1)
    expect(data.hosts).toHaveLength(1)
    expect(data.mappings).toHaveLength(1)
    expect(data.exportedAt).toBeGreaterThan(0)
  })

  it('importData 空库时导入 host + mapping', () => {
    const payload = {
      version: 1, exportedAt: 0,
      hosts: [{ id: 'h1', name: '导入', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [{ id: 'm1', hostId: 'h1', name: '导入映射', enabled: 1, protocol: 'tcp', listenPort: 8000, serverNames: null, listenPath: null, upstreamHost: '2.2.2.2', upstreamPort: 22, tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null, healthCheckEnabled: 0, healthCheckIntervalSec: 30, healthCheckTargetPath: null, healthCheckExpectedStatus: null, healthStatus: 'unknown', lastCheckedAt: null, sortOrder: 0, createdAt: 0, updatedAt: 0 }]
    }
    const result = importData(payload)
    expect(result.hostsImported).toBe(1)
    expect(result.mappingsImported).toBe(1)
  })

  it('importData 覆盖式（先清空）', () => {
    createHost({ name: 'old', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    const payload = { version: 1, exportedAt: 0, hosts: [{ id: 'new', name: 'new', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }], mappings: [] }
    importData(payload)
    const data = exportData()
    expect(data.hosts).toHaveLength(1)
    expect(data.hosts[0].name).toBe('new')
  })

  it('importData 版本不匹配抛错', () => {
    expect(() => importData({ version: 99, exportedAt: 0, hosts: [], mappings: [] } as any)).toThrow(/版本/)
  })

  it('importData 缺字段抛错', () => {
    expect(() => importData({ version: 1 } as any)).toThrow(/格式/)
  })
})
