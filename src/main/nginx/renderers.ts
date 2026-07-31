import type { Mapping } from '@shared/types'

/**
 * 安全过滤：剔除 nginx.conf 注入危险字符（; { } # 换行）。
 * server_name / location path 是结构化的，不应含这些字符；
 * 用户若输入 `a.com; } server { ...` 会被压平成无害文本，无法逃逸出指令块。
 * extraDirectives 不过滤（那是给高级用户开后门，设计如此）。
 */
function sanitize(v: string | null | undefined): string {
  if (!v) return ''
  return v.replace(/[;{}#\n\r]/g, '').trim()
}

/** 校验端口合法（1-65535），非法返回 null */
function safePort(p: number | null | undefined): number | null {
  if (p == null) return null
  const n = Number(p)
  if (!Number.isInteger(n) || n < 1 || n > 65535) return null
  return n
}

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
  const listen = safePort(m.listenPort)
  if (listen == null) return []
  const upHost = sanitize(m.upstreamHost)
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${listen};`,
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderUdpServer(m: Mapping): string[] {
  const listen = safePort(m.listenPort)
  if (listen == null) return []
  const upHost = sanitize(m.upstreamHost)
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${listen} udp;`,
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderHttpServer(m: Mapping): string[] {
  const port = safePort(m.listenPort) ?? 80
  const path = sanitize(m.listenPath) || '/'
  const serverNames = sanitize(m.serverNames)
  const upHost = sanitize(m.upstreamHost)
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)
  lines.push('  server {')
  lines.push(`    listen ${port};`)
  if (serverNames) lines.push(`    server_name ${serverNames};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${upHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}

export function renderHttpsServer(m: Mapping): string[] {
  const path = sanitize(m.listenPath) || '/'
  const serverNames = sanitize(m.serverNames)
  const upHost = sanitize(m.upstreamHost)
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)

  // HTTP → HTTPS 跳转块（可选）
  if (m.redirectHttp === 1 && serverNames) {
    lines.push('  server {')
    lines.push('    listen 80;')
    lines.push(`    server_name ${serverNames};`)
    lines.push('    return 301 https://$host$request_uri;')
    lines.push('  }')
    lines.push('')
  }

  // HTTPS 主块
  lines.push('  server {')
  lines.push('    listen 443 ssl;')
  if (serverNames) lines.push(`    server_name ${serverNames};`)
  if (m.tlsCertPath) lines.push(`    ssl_certificate ${sanitize(m.tlsCertPath)};`)
  if (m.tlsKeyPath) lines.push(`    ssl_certificate_key ${sanitize(m.tlsKeyPath)};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${upHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}
