import type { Mapping } from '@shared/types'
import { renderTcpServer, renderUdpServer, renderHttpServer, renderHttpsServer } from './renderers'

export interface ConfGenOptions {
  nginxLogDir: string
  nginxTempDir: string
  /** 白名单 IP 列表（非空时在每个 server 块注入 allow/deny all） */
  whitelistIps?: string[]
}

/**
 * 规范化路径供 nginx conf 使用：Windows 反斜杠转正斜杠。
 * nginx conf 解析时反斜杠是转义字符（\t→TAB, \n→换行），
 * Windows 路径 C:\Users\...\data\temp 会被错误解析。
 * nginx 在 Windows 上完全支持正斜杠，所以统一转成 /。
 */
function confPath(p: string): string {
  return p.replace(/\\/g, '/')
}

export function generateFullConf(mappings: Mapping[], opts: ConfGenOptions): string {
  const enabled = mappings.filter(m => m.enabled)

  const tcpMappings = enabled.filter(m => m.protocol === 'tcp')
  const udpMappings = enabled.filter(m => m.protocol === 'udp')
  const httpMappings = enabled.filter(m => m.protocol === 'http')
  const httpsMappings = enabled.filter(m => m.protocol === 'https')

  const hasStream = tcpMappings.length > 0 || udpMappings.length > 0
  const hasHttp = httpMappings.length > 0 || httpsMappings.length > 0
  // 白名单 IP（非空时注入 allow/deny all 到每个 server 块）
  const wl = opts.whitelistIps && opts.whitelistIps.length > 0 ? opts.whitelistIps : undefined

  // 所有路径用相对 prefix 的相对路径（logs/ temp/），nginx 基于 -p prefix 解析。
  // 彻底避开绝对路径的反斜杠转义/中文路径问题（Windows nginx 用 ANSI 代码页读绝对路径）。
  const lines: string[] = []
  lines.push('# 由 nginx-jump-gui 自动生成，请勿手动编辑')
  lines.push(`# 生成时间: ${new Date().toISOString()}`)
  lines.push('worker_processes 1;')
  lines.push('error_log logs/error.log warn;')
  lines.push('pid temp/nginx.pid;')
  lines.push('')
  lines.push('events {')
  lines.push('  worker_connections 1024;')
  lines.push('}')
  lines.push('')

  if (hasHttp) {
    lines.push('http {')
    // WebSocket 支持：$connection_upgrade 变量（proxy_set_header Connection 引用）
    lines.push('  map $http_upgrade $connection_upgrade {')
    lines.push('    default upgrade;')
    lines.push('    ""      close;')
    lines.push('  }')
    // 公共代理参数放 http 顶层，所有 location 继承（standard/minimal/custom 通用）：
    // - proxy_http_version 1.1：nginx 默认用 HTTP/1.0 与上游通信，不认 git smart HTTP
    //   的 chunked 请求体（git push/receive-pack）会 502，放顶层后任何模式都不会漏
    // - client_max_body_size 0：不限上传大小（git 大包/大文件）
    // - proxy_read_timeout 300s：大仓库 push/pull 超时保护
    // - proxy_buffering off：git 实时进度回显
    // header 系列不能上移：proxy_set_header 是数组替换语义，location 出现一条就替换整组，
    // 上移会让 minimal/custom 模式被迫继承 Host 头且无法删除，故留在 location 按模式生成。
    lines.push('  proxy_http_version 1.1;')
    lines.push('  client_max_body_size 0;')
    lines.push('  proxy_read_timeout 300s;')
    lines.push('  proxy_buffering off;')
    lines.push('  access_log logs/access.log;')
    lines.push('  client_body_temp_path temp/client_body;')
    lines.push('  proxy_temp_path temp/proxy;')
    lines.push('  include mime.types;')
    if (httpMappings.length > 0) {
      lines.push('  # ---------- HTTP server 块 ----------')
      for (const m of httpMappings) lines.push(...renderHttpServer(m, wl))
    }
    if (httpsMappings.length > 0) {
      lines.push('  # ---------- HTTPS server 块 ----------')
      for (const m of httpsMappings) lines.push(...renderHttpsServer(m, wl))
    }
    lines.push('}')
    lines.push('')
  }

  if (hasStream) {
    lines.push('stream {')
    // TCP/UDP 转发的访问日志（stream 模块独立于 http，需单独配 access_log）
    lines.push('  access_log logs/stream_access.log;')
    for (const m of tcpMappings) lines.push(...renderTcpServer(m, wl))
    for (const m of udpMappings) lines.push(...renderUdpServer(m, wl))
    lines.push('}')
    lines.push('')
  }

  return lines.join('\n')
}
