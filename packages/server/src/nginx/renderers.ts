import type { Mapping } from '@ngj/shared/types'

/**
 * 安全过滤：剔除 nginx.conf 注入危险字符（; { } # 换行）。
 * server_name / location path 是结构化的，不应含这些字符；
 * 用户若输入 `a.com; } server { ...` 会被压平成无害文本，无法逃逸出指令块。
 * extraDirectives 不过滤（那是给高级用户开后门，设计如此）。
 */
function sanitize(v: string | null | undefined): string {
  if (!v)
    return ''
  return v.replace(/[;{}#\n\r]/g, '').trim()
}

/**
 * mapping.name 消毒（用于 conf 注释行）。name 含 \n 可让注释提前终止、
 * 后续行成为真实指令（注入完整 server 块），必须剔除换行和指令字符。
 */
function safeName(name: string | null | undefined): string {
  return sanitize(name)
}

/** 校验端口合法（1-65535），非法返回 null */
function safePort(p: number | null | undefined): number | null {
  if (p == null)
    return null
  const n = Number(p)
  if (!Number.isInteger(n) || n < 1 || n > 65535)
    return null
  return n
}

/** 公共：渲染 extraDirectives，每行 trim + 缩进 */
function renderExtra(extra: string | null, indent = '    '): string[] {
  if (!extra || !extra.trim())
    return []
  const out: string[] = []
  for (const line of extra.split('\n')) {
    if (line.trim())
      out.push(indent + line.trim())
  }
  return out
}

/**
 * 标准反代的 proxy_set_header 系列。
 * 注意：HTTP/1.1、body 大小、超时、buffering 等公共参数在 confGenerator 的 http 顶层
 * 只写一次（所有模式继承），不在这里重复生成；header 是数组替换语义，若上移会让
 * minimal/custom 模式被迫继承 Host 头且无法删除，所以必须留在 location 按模式生成。
 */
function standardProxySetHeaders(indent = '      '): string[] {
  return [
    `${indent}proxy_set_header Host $host;`,
    `${indent}proxy_set_header X-Real-IP $remote_addr;`,
    `${indent}proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
    `${indent}proxy_set_header X-Forwarded-Proto $scheme;`,
    `${indent}proxy_set_header Upgrade $http_upgrade;`,
    `${indent}proxy_set_header Connection $connection_upgrade;`,
  ]
}

/**
 * 按 proxyMode 生成 location 内的代理指令（proxy_pass 之后的行）：
 * - standard：标准反代头（Host/X-Real-IP/WebSocket）
 * - minimal：不生成任何指令（HTTP/1.1、大 body、超时、buffering 继承 http 顶层公共参数）
 * - custom：用户在 proxyHeaders 字段填的自定义指令（替代自动生成的头）
 *   custom 模式下每行过滤掉 {}#（防注入逃逸），保留 ;（指令结尾需要）
 */
function renderProxyDirectives(m: Mapping, indent = '      '): string[] {
  const mode = m.proxyMode || 'standard'
  if (mode === 'minimal') {
    // 精简：不生成任何指令。proxy_http_version 1.1 / client_max_body_size 0 /
    // proxy_read_timeout 300s 已在 http 顶层，minimal 继承即可，正好是纯 proxy_pass。
    return []
  }
  if (mode === 'custom' && m.proxyHeaders && m.proxyHeaders.trim()) {
    // 过滤危险字符（{}# 和换行），保留分号（代理头指令需要）
    return m.proxyHeaders.split('\n')
      .map(l => l.replace(/[{}#\r]/g, '').trim())
      .filter(l => l)
      .map(l => indent + l)
  }
  // custom+空、standard、undefined 都走标准头
  return standardProxySetHeaders(indent)
}

/**
 * 生成白名单 allow/deny 指令行。whitelist 非空时返回 allow + deny all，空则返回空数组。
 *  关键：sanitize 后若无有效 allow，不注入 deny all（避免管理员自锁）。
 */
function renderWhitelist(wl: string[] | undefined, indent = '    '): string[] {
  if (!wl || wl.length === 0)
    return []
  const lines: string[] = []
  for (const ip of wl) {
    const clean = sanitize(ip)
    if (clean)
      lines.push(`${indent}allow ${clean};`)
  }
  // 只有存在至少一条有效 allow 时才 deny all，否则跳过（防止 sanitize 过滤掉全部后只剩 deny all 锁死）
  if (lines.length > 0)
    lines.push(`${indent}deny all;`)
  return lines
}

export function renderTcpServer(m: Mapping, wl?: string[]): string[] {
  const listen = safePort(m.listenPort)
  if (listen == null)
    return []
  const upHost = sanitize(m.upstreamHost)
  return [
    `  # mapping: ${safeName(m.name)}`,
    '  server {',
    `    listen ${listen};`,
    ...renderWhitelist(wl),
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }',
  ]
}

export function renderUdpServer(m: Mapping, wl?: string[]): string[] {
  const listen = safePort(m.listenPort)
  if (listen == null)
    return []
  const upHost = sanitize(m.upstreamHost)
  return [
    `  # mapping: ${safeName(m.name)}`,
    '  server {',
    `    listen ${listen} udp;`,
    ...renderWhitelist(wl),
    `    proxy_pass ${upHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }',
  ]
}

/** 生成 proxy_pass 指令值。proxyPassUrl 非空时原样用（用户自定义），否则按 host:port + 尾斜杠拼接。 */
function renderProxyPass(m: Mapping, path: string, indent = '      '): string {
  const upHost = sanitize(m.upstreamHost)
  if (m.proxyPassUrl && m.proxyPassUrl.trim()) {
    // 用户自定义：sanitize 后原样用（isValidProxyPassUrl 已在 handler 层校验，这里只防注入）
    return `${indent}proxy_pass ${sanitize(m.proxyPassUrl)};`
  }
  // 默认拼接：location /（根匹配）时加尾斜杠，让 GitLab 等后端路径规范化正确
  const trailingSlash = path === '/' ? '/' : ''
  return `${indent}proxy_pass http://${upHost}:${m.upstreamPort}${trailingSlash};`
}

export function renderHttpServer(m: Mapping, wl?: string[]): string[] {
  const port = safePort(m.listenPort) ?? 80
  const path = sanitize(m.listenPath) || '/'
  const serverNames = sanitize(m.serverNames)
  const lines: string[] = []
  lines.push(`  # mapping: ${safeName(m.name)}`)
  lines.push('  server {')
  lines.push(`    listen ${port};`)
  if (serverNames)
    lines.push(`    server_name ${serverNames};`)
  lines.push(`    location ${path} {`)
  lines.push(...renderWhitelist(wl, '      '))
  lines.push(renderProxyPass(m, path))
  lines.push(...renderProxyDirectives(m))
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}

export function renderHttpsServer(m: Mapping, wl?: string[]): string[] {
  const path = sanitize(m.listenPath) || '/'
  const serverNames = sanitize(m.serverNames)
  const lines: string[] = []
  lines.push(`  # mapping: ${safeName(m.name)}`)

  // HTTP → HTTPS 跳转块（可选）—— 白名单也要注入此块，否则 80 端口可绕过
  if (m.redirectHttp === 1 && serverNames) {
    lines.push('  server {')
    lines.push('    listen 80;')
    lines.push(`    server_name ${serverNames};`)
    lines.push(...renderWhitelist(wl))
    lines.push('    return 301 https://$host$request_uri;')
    lines.push('  }')
    lines.push('')
  }

  // HTTPS 主块
  lines.push('  server {')
  lines.push('    listen 443 ssl;')
  if (serverNames)
    lines.push(`    server_name ${serverNames};`)
  if (m.tlsCertPath)
    lines.push(`    ssl_certificate ${sanitize(m.tlsCertPath)};`)
  if (m.tlsKeyPath)
    lines.push(`    ssl_certificate_key ${sanitize(m.tlsKeyPath)};`)
  lines.push(`    location ${path} {`)
  lines.push(...renderWhitelist(wl, '      '))
  lines.push(renderProxyPass(m, path))
  lines.push(...renderProxyDirectives(m))
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}
