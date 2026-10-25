import { formToTemplateMapping } from '@renderer/utils/templateMapping'
import { describe, expect, it } from 'vitest'

function makeForm(over: Record<string, any> = {}) {
  return {
    name: '默认名',
    protocol: 'https',
    listenPort: 443,
    serverNames: 'a.local',
    listenPath: '/',
    upstreamHost: '192.168.1.100',
    upstreamPort: 8443,
    proxyPassUrl: '',
    tlsCertPath: '/c.pem',
    tlsKeyPath: '/k.pem',
    redirectHttp: true,
    proxyMode: 'standard',
    proxyHeaders: '',
    healthCheckEnabled: true,
    healthCheckIntervalSec: 60,
    healthCheckTargetPath: '/health',
    healthCheckExpectedStatus: 200,
    ...over,
  } as any
}

describe('formToTemplateMapping', () => {
  it('保留全部 16 个业务字段（修旧版漏 tls/healthCheck）', () => {
    const m = formToTemplateMapping(makeForm())
    expect(m.tlsCertPath).toBe('/c.pem')
    expect(m.tlsKeyPath).toBe('/k.pem')
    expect(m.redirectHttp).toBe(1)
    expect(m.healthCheckEnabled).toBe(1)
    expect(m.healthCheckIntervalSec).toBe(60)
    expect(m.healthCheckTargetPath).toBe('/health')
    expect(m.healthCheckExpectedStatus).toBe(200)
  })

  it('upstreamHost 不再硬编码为空（修数据丢失 bug）', () => {
    const m = formToTemplateMapping(makeForm({ upstreamHost: '10.0.0.5' }))
    expect(m.upstreamHost).toBe('10.0.0.5')
  })

  it('boolean → number 转换：redirectHttp/healthCheckEnabled', () => {
    const on = formToTemplateMapping(makeForm({ redirectHttp: true, healthCheckEnabled: true }))
    expect(on.redirectHttp).toBe(1)
    expect(on.healthCheckEnabled).toBe(1)
    const off = formToTemplateMapping(makeForm({ redirectHttp: false, healthCheckEnabled: false }))
    expect(off.redirectHttp).toBe(0)
    expect(off.healthCheckEnabled).toBe(0)
  })

  it('proxyHeaders 仅 custom 模式保留，其他模式置 null', () => {
    const custom = formToTemplateMapping(makeForm({ proxyMode: 'custom', proxyHeaders: 'proxy_set_header X 1;' }))
    expect(custom.proxyHeaders).toBe('proxy_set_header X 1;')
    const standard = formToTemplateMapping(makeForm({ proxyMode: 'standard', proxyHeaders: '应被忽略' }))
    expect(standard.proxyHeaders).toBeNull()
    const minimal = formToTemplateMapping(makeForm({ proxyMode: 'minimal', proxyHeaders: '应被忽略' }))
    expect(minimal.proxyHeaders).toBeNull()
  })

  it('空字符串字段转 null（serverNames/tls/healthCheckTargetPath）', () => {
    const m = formToTemplateMapping(makeForm({ serverNames: '', tlsCertPath: '', tlsKeyPath: '', healthCheckTargetPath: '' }))
    expect(m.serverNames).toBeNull()
    expect(m.tlsCertPath).toBeNull()
    expect(m.tlsKeyPath).toBeNull()
    expect(m.healthCheckTargetPath).toBeNull()
  })
})
