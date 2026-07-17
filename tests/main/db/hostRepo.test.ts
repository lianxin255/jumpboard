import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost, listHosts, getHost, updateHost, deleteHost, setDefaultHost, getDefaultHost } from '@main/db/hostRepo'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('hostRepo', () => {
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

  it('createHost 创建并返回 local host', () => {
    const h = createHost({ name: '本机', type: 'local', isDefault: 1, localConfig: { nginxPath: '', confDir: '', logDir: '' }, remoteConfig: null })
    expect(h.id).toBeTruthy()
    expect(h.name).toBe('本机')
    expect(h.type).toBe('local')
    expect(h.localConfig?.nginxPath).toBe('')
  })

  it('listHosts 返回所有', () => {
    createHost({ name: 'a', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    createHost({ name: 'b', type: 'remote', isDefault: 0, localConfig: null, remoteConfig: { host: '1.1.1.1', port: 22, username: 'root', authType: 'password', passwordEncrypted: null, keyPath: null, nginxPath: '/usr/sbin/nginx', confPath: '/etc/nginx/nginx.conf', reloadCmd: 'nginx -s reload', testCmd: 'nginx -t' } })
    const list = listHosts()
    expect(list).toHaveLength(2)
  })

  it('getHost 按 id 查', () => {
    const h = createHost({ name: 'x', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    const got = getHost(h.id)
    expect(got?.name).toBe('x')
  })

  it('updateHost 更新名字', () => {
    const h = createHost({ name: 'old', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    updateHost(h.id, { name: 'new' })
    expect(getHost(h.id)?.name).toBe('new')
  })

  it('deleteHost 删除', () => {
    const h = createHost({ name: 'del', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    deleteHost(h.id)
    expect(getHost(h.id)).toBeUndefined()
  })

  it('setDefaultHost 保证只有一个默认', () => {
    const a = createHost({ name: 'a', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    const b = createHost({ name: 'b', type: 'local', isDefault: 0, localConfig: null, remoteConfig: null })
    setDefaultHost(b.id)
    expect(getDefaultHost()?.id).toBe(b.id)
    expect(getHost(a.id)?.isDefault).toBe(0)
  })
})
