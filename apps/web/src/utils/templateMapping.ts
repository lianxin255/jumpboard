/**
 * Mapping ↔ Template 的字段映射工具。
 *
 * 解决两个问题：
 * 1. Template.mapping 之前是 Record<string,any> 弱类型，saveAsTemplate 漏存 8 个字段
 *    （tlsCertPath/healthCheck 全丢）且 upstreamHost 被硬编码成 ''，TS 都没抓到。
 *    现在用 TemplateMapping 强类型，漏字段会编译报错。
 * 2. FormState 用 boolean（redirectHttp/healthCheckEnabled），DB/模板用 number（0/1）。
 *    集中转换，避免散落。
 */

/** 模板里存的业务字段（对应 FormState，但 boolean→number 对齐 DB 语义） */
export interface TemplateMapping {
  name: string
  protocol: 'tcp' | 'udp' | 'http' | 'https'
  listenPort: number | null
  serverNames: string | null
  listenPath: string
  upstreamHost: string
  upstreamPort: number | null
  proxyPassUrl: string | null
  tlsCertPath: string | null
  tlsKeyPath: string | null
  redirectHttp: number // 0/1
  proxyMode: 'standard' | 'minimal' | 'custom'
  proxyHeaders: string | null
  healthCheckEnabled: number // 0/1
  healthCheckIntervalSec: number
  healthCheckTargetPath: string | null
  healthCheckExpectedStatus: number | null
}

/** FormState 的结构约定（避免直接 import MappingForm 的内部类型，用结构兼容） */
interface FormStateLike {
  name: string
  protocol: 'tcp' | 'udp' | 'http' | 'https'
  listenPort: number | null
  serverNames: string
  listenPath: string
  upstreamHost: string
  upstreamPort: number | null
  proxyPassUrl: string
  tlsCertPath: string
  tlsKeyPath: string
  redirectHttp: boolean
  proxyMode: 'standard' | 'minimal' | 'custom'
  proxyHeaders: string
  healthCheckEnabled: boolean
  healthCheckIntervalSec: number
  healthCheckTargetPath: string
  healthCheckExpectedStatus: number | null
}

/**
 * 表单 → 模板 mapping 字段。补齐全部 16 个业务字段（修复旧版漏 8 字段 + upstreamHost 硬编码 '' 的 bug）。
 * proxyHeaders 仅 custom 模式保留，其他模式置 null（避免存无关配置）。
 */
export function formToTemplateMapping(form: FormStateLike): TemplateMapping {
  return {
    name: form.name,
    protocol: form.protocol,
    listenPort: form.listenPort,
    serverNames: form.serverNames || null,
    listenPath: form.listenPath || '/',
    upstreamHost: form.upstreamHost,
    upstreamPort: form.upstreamPort,
    proxyPassUrl: form.proxyPassUrl.trim() || null,
    tlsCertPath: form.tlsCertPath || null,
    tlsKeyPath: form.tlsKeyPath || null,
    redirectHttp: form.redirectHttp ? 1 : 0,
    proxyMode: form.proxyMode,
    proxyHeaders: form.proxyMode === 'custom' ? (form.proxyHeaders || null) : null,
    healthCheckEnabled: form.healthCheckEnabled ? 1 : 0,
    healthCheckIntervalSec: form.healthCheckIntervalSec,
    healthCheckTargetPath: form.healthCheckTargetPath || null,
    healthCheckExpectedStatus: form.healthCheckExpectedStatus,
  }
}
