import type { Mapping } from '@shared/types'
import { renderTcpServer, renderUdpServer, renderHttpServer, renderHttpsServer } from './renderers'

export interface ConfGenOptions {
  nginxLogDir: string
  nginxTempDir: string
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

  const lines: string[] = []
  lines.push('# 由 nginx-jump-gui 自动生成，请勿手动编辑')
  lines.push(`# 生成时间: ${new Date().toISOString()}`)
  lines.push('worker_processes 1;')
  lines.push('error_log ' + confPath(opts.nginxLogDir) + '/error.log warn;')
  lines.push('pid ' + confPath(opts.nginxTempDir) + '/nginx.pid;')
  lines.push('')
  lines.push('events {')
  lines.push('  worker_connections 1024;')
  lines.push('}')
  lines.push('')

  if (hasHttp) {
    lines.push('http {')
    lines.push('  access_log ' + confPath(opts.nginxLogDir) + '/access.log;')
    lines.push('  client_body_temp_path ' + confPath(opts.nginxTempDir) + '/client_body;')
    lines.push('  proxy_temp_path ' + confPath(opts.nginxTempDir) + '/proxy;')
    lines.push('  include mime.types;')
    if (httpMappings.length > 0) {
      lines.push('  # ---------- HTTP server 块 ----------')
      for (const m of httpMappings) lines.push(...renderHttpServer(m))
    }
    if (httpsMappings.length > 0) {
      lines.push('  # ---------- HTTPS server 块 ----------')
      for (const m of httpsMappings) lines.push(...renderHttpsServer(m))
    }
    lines.push('}')
    lines.push('')
  }

  if (hasStream) {
    lines.push('stream {')
    for (const m of tcpMappings) lines.push(...renderTcpServer(m))
    for (const m of udpMappings) lines.push(...renderUdpServer(m))
    lines.push('}')
    lines.push('')
  }

  return lines.join('\n')
}
