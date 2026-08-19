import type { Mapping } from '@ngj/shared/types'
import { describe, expect, it } from 'vitest'
import { generateFullConf } from '../nginx/confGenerator'

function tcpMapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1',
    hostId: 'h1',
    name: 'MySQL 内库',
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
    extraDirectives: 'proxy_timeout 1h;',
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    healthStatus: 'unknown',
    lastCheckedAt: null,
    sortOrder: 0,
    createdAt: 0,
    updatedAt: 0,
    ...over,
  } as Mapping
}

describe('confGenerator TCP', () => {
  it('生成包含 stream 块的完整 conf', () => {
    const conf = generateFullConf([tcpMapping()], {})
    expect(conf).toContain('stream {')
    expect(conf).toContain('listen 9000;')
    expect(conf).toContain('proxy_pass 192.168.1.10:3306;')
    expect(conf).toContain('proxy_timeout 1h;')
  })

  it('disabled 的 mapping 不生成', () => {
    const conf = generateFullConf([tcpMapping({ enabled: 0 })], {})
    expect(conf).not.toContain('listen 9000;')
  })

  it('不含 http 块（没有 http/https mapping）', () => {
    const conf = generateFullConf([tcpMapping()], {})
    expect(conf).not.toMatch(/\bhttp\s*\{/)
  })

  it('注释里带 mapping name', () => {
    const conf = generateFullConf([tcpMapping()], {})
    expect(conf).toContain('MySQL 内库')
  })

  it('空 mappings 仍生成有效的最小 conf（含 events）', () => {
    const conf = generateFullConf([], {})
    expect(conf).toContain('events {')
    expect(conf).toContain('worker_connections 1024;')
    expect(conf).not.toContain('stream {')
    expect(conf).not.toContain('http {')
  })

  it('多个 TCP mapping 都被渲染', () => {
    const conf = generateFullConf([
      tcpMapping({ id: 'm1', name: '第一个', listenPort: 9001, upstreamPort: 3301 }),
      tcpMapping({ id: 'm2', name: '第二个', listenPort: 9002, upstreamPort: 3302 }),
    ], {})
    expect(conf).toContain('listen 9001;')
    expect(conf).toContain('listen 9002;')
    expect(conf).toContain('第一个')
    expect(conf).toContain('第二个')
  })

  it('多行 extraDirectives 逐行渲染', () => {
    const conf = generateFullConf([tcpMapping({
      extraDirectives: 'proxy_timeout 1h;\nproxy_connect_timeout 60s;\n  # 带缩进的注释',
    })], {})
    expect(conf).toContain('proxy_timeout 1h;')
    expect(conf).toContain('proxy_connect_timeout 60s;')
    expect(conf).toContain('# 带缩进的注释')
  })

  it('listenPort 为 null 的 mapping 不生成 server 块', () => {
    const conf = generateFullConf([tcpMapping({ listenPort: null })], {})
    expect(conf).not.toContain('listen null;')
    expect(conf).not.toContain('server {')
  })

  it('uDP only 时生成 stream 块含 listen udp', () => {
    const conf = generateFullConf([tcpMapping({ protocol: 'udp', listenPort: 5353, upstreamPort: 53 })], {})
    expect(conf).toContain('stream {')
    expect(conf).toContain('listen 5353 udp;')
    expect(conf).not.toContain('\nhttp {')
  })

  it('hTTP 映射生成 http 块', () => {
    const conf = generateFullConf([tcpMapping({ protocol: 'http', listenPort: 8080, serverNames: 'a.local', listenPath: '/', upstreamPort: 3000 })], {})
    expect(conf).toContain('http {')
    expect(conf).toContain('server_name a.local;')
    expect(conf).toContain('proxy_pass http://')
    expect(conf).not.toContain('stream {')
  })

  it('混合 TCP + HTTPS 同时生成 stream 和 http 块', () => {
    const conf = generateFullConf([
      tcpMapping({ protocol: 'tcp', listenPort: 9000, upstreamPort: 3306 }),
      tcpMapping({ protocol: 'https', listenPort: null, serverNames: 's.local', tlsCertPath: '/c.pem', tlsKeyPath: '/k.pem', upstreamPort: 8443, redirectHttp: 1 }),
    ], {})
    expect(conf).toContain('stream {')
    expect(conf).toContain('http {')
    expect(conf).toContain('listen 443 ssl;')
  })

  it('公共代理参数在 http 顶层只写一次（所有模式继承）', () => {
    const conf = generateFullConf([
      tcpMapping({ id: 's', protocol: 'http', listenPort: 8080, upstreamPort: 3000 }),
      tcpMapping({ id: 'mi', protocol: 'http', listenPort: 8081, upstreamPort: 3001, proxyMode: 'minimal' }),
    ], {})
    // 顶层出现一次
    expect(conf).toContain('proxy_http_version 1.1;')
    expect(conf).toContain('client_max_body_size 0;')
    expect(conf).toContain('proxy_read_timeout 300s;')
    expect(conf).toContain('proxy_buffering off;')
    // 不重复出现在 location 里
    const locationSections = conf.split('location ')
    expect(locationSections.length - 1).toBe(2)
    for (const sec of locationSections.slice(1)) {
      expect(sec).not.toContain('proxy_http_version')
      expect(sec).not.toContain('client_max_body_size')
    }
  })

  it('standard 模式生成标准头，minimal 模式 location 是纯 proxy_pass', () => {
    const conf = generateFullConf([
      tcpMapping({ id: 's', protocol: 'http', listenPort: 8080, upstreamPort: 3000 }),
      tcpMapping({ id: 'mi', protocol: 'http', listenPort: 8081, upstreamPort: 3001, proxyMode: 'minimal' }),
    ], {})
    expect(conf).toContain('proxy_set_header Host $host;')
    expect(conf).toContain('proxy_set_header X-Real-IP $remote_addr;')
    // minimal 的 location 块里没有任何 proxy_set_header
    const locStart = conf.indexOf('listen 8081;')
    const nextServer = conf.indexOf('# mapping:', locStart)
    const minimalSection = conf.slice(locStart, nextServer === -1 ? undefined : nextServer)
    expect(minimalSection).not.toContain('proxy_set_header')
  })
})
