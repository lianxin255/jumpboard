import type { Mapping } from '@shared/types'

/** 公共：渲染 extraDirectives，每行 trim + 缩进 */
function renderExtra(extra: string | null, indent = '    '): string[] {
  if (!extra || !extra.trim()) return []
  const out: string[] = []
  for (const line of extra.split('\n')) {
    if (line.trim()) out.push(indent + line.trim())
  }
  return out
}

/** 公共：HTTP/HTTPS 转发头 */
function proxyHeaders(indent = '      '): string[] {
  return [
    `${indent}proxy_set_header Host $host;`,
    `${indent}proxy_set_header X-Real-IP $remote_addr;`,
    `${indent}proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
    `${indent}proxy_set_header X-Forwarded-Proto $scheme;`
  ]
}

export function renderTcpServer(m: Mapping): string[] {
  if (m.listenPort == null) return []
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${m.listenPort};`,
    `    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderUdpServer(m: Mapping): string[] {
  if (m.listenPort == null) return []
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${m.listenPort} udp;`,
    `    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderHttpServer(m: Mapping): string[] {
  const port = m.listenPort ?? 80
  const path = m.listenPath ?? '/'
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)
  lines.push('  server {')
  lines.push(`    listen ${port};`)
  if (m.serverNames) lines.push(`    server_name ${m.serverNames};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${m.upstreamHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}

export function renderHttpsServer(m: Mapping): string[] {
  const path = m.listenPath ?? '/'
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)

  // HTTP → HTTPS 跳转块（可选）
  if (m.redirectHttp === 1 && m.serverNames) {
    lines.push('  server {')
    lines.push('    listen 80;')
    lines.push(`    server_name ${m.serverNames};`)
    lines.push('    return 301 https://$host$request_uri;')
    lines.push('  }')
    lines.push('')
  }

  // HTTPS 主块
  lines.push('  server {')
  lines.push('    listen 443 ssl;')
  if (m.serverNames) lines.push(`    server_name ${m.serverNames};`)
  if (m.tlsCertPath) lines.push(`    ssl_certificate ${m.tlsCertPath};`)
  if (m.tlsKeyPath) lines.push(`    ssl_certificate_key ${m.tlsKeyPath};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${m.upstreamHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}
