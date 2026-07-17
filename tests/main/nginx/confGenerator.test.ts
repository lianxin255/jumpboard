import { describe, it, expect } from 'vitest'
import { generateFullConf } from '@main/nginx/confGenerator'
import type { Mapping } from '@shared/types'

function tcpMapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1', hostId: 'h1', name: 'MySQL 内库', enabled: 1, protocol: 'tcp',
    listenPort: 9000, serverNames: null, listenPath: null,
    upstreamHost: '192.168.1.10', upstreamPort: 3306,
    tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: 'proxy_timeout 1h;',
    healthCheckEnabled: 0, healthCheckIntervalSec: 30,
    healthCheckTargetPath: null, healthCheckExpectedStatus: null,
    healthStatus: 'unknown', lastCheckedAt: null, sortOrder: 0,
    createdAt: 0, updatedAt: 0, ...over
  } as Mapping
}

describe('confGenerator TCP', () => {
  it('生成包含 stream 块的完整 conf', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('stream {')
    expect(conf).toContain('listen 9000;')
    expect(conf).toContain('proxy_pass 192.168.1.10:3306;')
    expect(conf).toContain('proxy_timeout 1h;')
  })

  it('disabled 的 mapping 不生成', () => {
    const conf = generateFullConf([tcpMapping({ enabled: 0 })], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).not.toContain('listen 9000;')
  })

  it('不含 http 块（没有 http/https mapping）', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).not.toMatch(/\bhttp\s*\{/)
  })

  it('注释里带 mapping name', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('MySQL 内库')
  })

  it('空 mappings 仍生成有效的最小 conf（含 events）', () => {
    const conf = generateFullConf([], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('events {')
    expect(conf).toContain('worker_connections 1024;')
    expect(conf).not.toContain('stream {')
    expect(conf).not.toContain('http {')
  })

  it('多个 TCP mapping 都被渲染', () => {
    const conf = generateFullConf([
      tcpMapping({ id: 'm1', name: '第一个', listenPort: 9001, upstreamPort: 3301 }),
      tcpMapping({ id: 'm2', name: '第二个', listenPort: 9002, upstreamPort: 3302 }),
    ], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('listen 9001;')
    expect(conf).toContain('listen 9002;')
    expect(conf).toContain('第一个')
    expect(conf).toContain('第二个')
  })

  it('多行 extraDirectives 逐行渲染', () => {
    const conf = generateFullConf([tcpMapping({
      extraDirectives: 'proxy_timeout 1h;\nproxy_connect_timeout 60s;\n  # 带缩进的注释'
    })], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('proxy_timeout 1h;')
    expect(conf).toContain('proxy_connect_timeout 60s;')
    expect(conf).toContain('# 带缩进的注释')
  })

  it('listenPort 为 null 的 mapping 不生成 server 块', () => {
    const conf = generateFullConf([tcpMapping({ listenPort: null })], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).not.toContain('listen null;')
    expect(conf).not.toContain('server {')
  })
})
