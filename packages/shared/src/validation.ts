/**
 * 前后端共用的输入校验工具。
 *
 * 渲染进程的校验可被 DevTools / 直接调 window.electronAPI 绕过，
 * 因此主进程（handlers.ts）必须用同一份校验逻辑兜底。
 * 放 shared/ 让 main + renderer 都能 import，避免双轨。
 */

/**
 * IPv4 或 CIDR（各段 0-255，掩码 0-32）。
 * 拒绝：前导零（01.2.3.04，规范化分叉）；0.0.0.0/0（= 允许所有，会让白名单静默失效）。
 */
export function isValidIpOrCidr(s: string): boolean {
  if (typeof s !== 'string')
    return false
  const [ipPart, cidr] = s.split('/')
  const segs = ipPart.split('.')
  if (segs.length !== 4)
    return false
  // 每段：0 或非零开头的 1-3 位数字，值 0-255（拒绝 01/007 等前导零）
  if (!segs.every(n => /^(0|[1-9]\d{0,2})$/.test(n) && +n >= 0 && +n <= 255))
    return false
  if (cidr !== undefined && (!/^\d+$/.test(cidr) || +cidr < 0 || +cidr > 32))
    return false
  // 0.0.0.0/0 匹配一切，作为白名单条目等于关闭访问控制
  if (cidr !== undefined && +cidr === 0)
    return false
  return true
}

/** 端口合法：1-65535 整数。迁移自 renderers.ts 的 safePort 的判断部分。 */
export function isValidPort(p: unknown): p is number {
  const n = Number(p)
  return Number.isInteger(n) && n >= 1 && n <= 65535
}

/** 夹紧数值到 [min, max]，非数字返回 min。用于 limit/lines 等防超大值。 */
export function clampInt(n: unknown, min: number, max: number): number {
  const v = Number(n)
  if (!Number.isFinite(v))
    return min
  return Math.max(min, Math.min(max, Math.trunc(v)))
}

/** 协议合法：tcp / udp / http / https */
export function isValidProtocol(v: unknown): v is 'tcp' | 'udp' | 'http' | 'https' {
  return v === 'tcp' || v === 'udp' || v === 'http' || v === 'https'
}

/** 代理模式合法：standard / minimal / custom */
export function isValidProxyMode(v: unknown): v is 'standard' | 'minimal' | 'custom' {
  return v === 'standard' || v === 'minimal' || v === 'custom'
}

/**
 * proxy_pass 自定义值合法：必须 http:// 或 https:// 开头，不含 nginx 注入字符（;{}# 换行）。
 * 用于 proxyPassUrl 字段（用户直接控制 proxy_pass 的值）。
 * 允许的示例：http://1.2.3.4/、https://gitlab.com、http://10.0.0.1:8080/path/
 */
export function isValidProxyPassUrl(v: string): boolean {
  if (typeof v !== 'string' || v.length === 0 || v.length > 500)
    return false
  if (!/^https?:\/\//.test(v))
    return false
  // nginx.conf 注入字符：; { } # 换行（和 renderers.ts 的 sanitize 一致）
  if (/[;{}#\r\n]/.test(v))
    return false
  return true
}
