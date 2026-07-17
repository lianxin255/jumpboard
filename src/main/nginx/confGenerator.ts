import type { Mapping } from '@shared/types'

export interface ConfGenOptions {
  nginxLogDir: string
  nginxTempDir: string
  // httpPort / httpsPort 可扩展，本 plan 不用
}

/**
 * 生成完整的 nginx.conf。
 * 本 plan 只处理 tcp（stream）。udp/http/https 留到 Plan 2。
 */
export function generateFullConf(mappings: Mapping[], opts: ConfGenOptions): string {
  const enabled = mappings.filter(m => m.enabled)

  const tcpMappings = enabled.filter(m => m.protocol === 'tcp')
  // 预留，Plan 2 实现
  const udpMappings = enabled.filter(m => m.protocol === 'udp')
  const httpMappings = enabled.filter(m => m.protocol === 'http' || m.protocol === 'https')

  const hasStream = tcpMappings.length > 0 || udpMappings.length > 0
  const hasHttp = httpMappings.length > 0

  const lines: string[] = []
  lines.push('# 由 nginx-jump-gui 自动生成，请勿手动编辑')
  lines.push(`# 生成时间: ${new Date().toISOString()}`)
  lines.push('worker_processes 1;')
  lines.push('error_log ' + opts.nginxLogDir + '/error.log warn;')
  lines.push('pid ' + opts.nginxTempDir + '/nginx.pid;')
  lines.push('')
  lines.push('events {')
  lines.push('  worker_connections 1024;')
  lines.push('}')
  lines.push('')

  if (hasHttp) {
    lines.push('http {')
    lines.push('  access_log ' + opts.nginxLogDir + '/access.log;')
    lines.push('  client_body_temp_path ' + opts.nginxTempDir + '/client_body;')
    lines.push('  proxy_temp_path ' + opts.nginxTempDir + '/proxy;')
    lines.push('  include mime.types;')
    lines.push('  # http/https server 块在 Plan 2 实现')
    lines.push('}')
    lines.push('')
  }

  if (hasStream) {
    lines.push('stream {')
    for (const m of tcpMappings) {
      lines.push(...renderTcpServer(m))
    }
    // udp 在 Plan 2
    lines.push('}')
    lines.push('')
  }

  return lines.join('\n')
}

function renderTcpServer(m: Mapping): string[] {
  const out: string[] = []
  out.push(`  # mapping: ${m.name}`)
  out.push('  server {')
  out.push(`    listen ${m.listenPort};`)
  out.push(`    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`)
  if (m.extraDirectives && m.extraDirectives.trim()) {
    for (const line of m.extraDirectives.split('\n')) {
      if (line.trim()) out.push('    ' + line.trim())
    }
  }
  out.push('  }')
  return out
}
