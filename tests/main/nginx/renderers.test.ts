import { describe, it, expect } from 'vitest'
import { renderTcpServer, renderUdpServer, renderHttpServer, renderHttpsServer } from '@main/nginx/renderers'
import type { Mapping } from '@shared/types'

function base(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1', hostId: 'h1', name: '测试', enabled: 1, protocol: 'tcp',
    listenPort: 9000, serverNames: null, listenPath: null,
    upstreamHost: '10.0.0.1', upstreamPort: 3306,
    tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null,
    healthCheckEnabled: 0, healthCheckIntervalSec: 30,
    healthCheckTargetPath: null, healthCheckExpectedStatus: null,
    healthStatus: 'unknown', lastCheckedAt: null, sortOrder: 0,
    createdAt: 0, updatedAt: 0, ...over
  } as Mapping
}

describe('renderTcpServer', () => {
  it('生成 listen + proxy_pass + extraDirectives', () => {
    const out = renderTcpServer(base({ protocol: 'tcp', listenPort: 9000, upstreamHost: '10.0.0.1', upstreamPort: 3306, extraDirectives: 'proxy_timeout 1h;' }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 9000;')
    expect(conf).toContain('proxy_pass 10.0.0.1:3306;')
    expect(conf).toContain('proxy_timeout 1h;')
  })
  it('listenPort 为 null 返回空数组', () => {
    expect(renderTcpServer(base({ listenPort: null }))).toEqual([])
  })
})

describe('renderUdpServer', () => {
  it('生成 listen udp + proxy_pass', () => {
    const out = renderUdpServer(base({ protocol: 'udp', listenPort: 5353, upstreamHost: '10.0.0.2', upstreamPort: 53 }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 5353 udp;')
    expect(conf).toContain('proxy_pass 10.0.0.2:53;')
  })
  it('listenPort 为 null 返回空数组', () => {
    expect(renderUdpServer(base({ protocol: 'udp', listenPort: null }))).toEqual([])
  })
})

describe('renderHttpServer', () => {
  it('生成 listen + server_name + location + proxy_pass + 转发头', () => {
    const out = renderHttpServer(base({ protocol: 'http', listenPort: 8080, serverNames: 'app1.gw.local', listenPath: '/', upstreamHost: '10.0.0.3', upstreamPort: 3000 }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 8080;')
    expect(conf).toContain('server_name app1.gw.local;')
    expect(conf).toContain('location /')
    expect(conf).toContain('proxy_pass http://10.0.0.3:3000;')
    expect(conf).toContain('proxy_set_header Host')
    expect(conf).toContain('proxy_set_header X-Real-IP')
    expect(conf).toContain('proxy_set_header X-Forwarded-For')
  })
  it('无 serverName 时省略 server_name 行', () => {
    const out = renderHttpServer(base({ protocol: 'http', listenPort: 8080, serverNames: null, upstreamHost: '10.0.0.3', upstreamPort: 3000 }))
    expect(out.join('\n')).not.toContain('server_name')
  })
  it('extraDirectives 在 location 块内正确缩进渲染', () => {
    const out = renderHttpServer(base({ protocol: 'http', listenPort: 8080, upstreamHost: '10.0.0.3', upstreamPort: 3000, extraDirectives: 'proxy_buffering off;' }))
    const conf = out.join('\n')
    expect(conf).toContain('proxy_buffering off;')
  })
})

describe('renderHttpsServer', () => {
  it('生成 443 ssl + 证书 + HTTP→HTTPS 跳转（redirectHttp=1）', () => {
    const out = renderHttpsServer(base({
      protocol: 'https', listenPort: null, serverNames: 'secure.gw.local', listenPath: '/',
      upstreamHost: '10.0.0.4', upstreamPort: 8443,
      tlsCertPath: '/data/cert.pem', tlsKeyPath: '/data/key.pem', redirectHttp: 1
    }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 80;')
    expect(conf).toContain('return 301 https://')
    expect(conf).toContain('listen 443 ssl;')
    expect(conf).toContain('ssl_certificate /data/cert.pem;')
    expect(conf).toContain('ssl_certificate_key /data/key.pem;')
    expect(conf).toContain('proxy_pass http://10.0.0.4:8443;')
  })
  it('redirectHttp=0 时不生成 80 跳转块', () => {
    const out = renderHttpsServer(base({
      protocol: 'https', serverNames: 's.gw.local',
      upstreamHost: '10.0.0.4', upstreamPort: 8443,
      tlsCertPath: '/c.pem', tlsKeyPath: '/k.pem', redirectHttp: 0
    }))
    expect(out.join('\n')).not.toContain('listen 80;')
  })
  it('tlsCertPath 为 null 时省略 ssl_certificate 行', () => {
    const out = renderHttpsServer(base({
      protocol: 'https', serverNames: 's.local',
      upstreamHost: '10.0.0.4', upstreamPort: 8443,
      tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0
    }))
    expect(out.join('\n')).not.toContain('ssl_certificate')
  })
})
