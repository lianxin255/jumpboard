import { describe, it, expect } from 'vitest'
import { isValidIpOrCidr, isValidPort, clampInt, isValidProtocol, isValidProxyMode, isValidProxyPassUrl } from '@shared/validation'

describe('isValidIpOrCidr', () => {
  it('合法 IPv4', () => {
    expect(isValidIpOrCidr('192.168.1.10')).toBe(true)
    expect(isValidIpOrCidr('0.0.0.0')).toBe(true)
    expect(isValidIpOrCidr('255.255.255.255')).toBe(true)
  })
  it('合法 CIDR', () => {
    expect(isValidIpOrCidr('10.0.0.0/24')).toBe(true)
    expect(isValidIpOrCidr('10.0.0.0/32')).toBe(true)
  })
  it('L1: 0.0.0.0/0 被拒绝（= 允许所有，白名单静默失效）', () => {
    expect(isValidIpOrCidr('10.0.0.0/0')).toBe(false)
    expect(isValidIpOrCidr('0.0.0.0/0')).toBe(false)
  })
  it('L2: 前导零被拒绝（规范化分叉）', () => {
    expect(isValidIpOrCidr('01.2.3.4')).toBe(false)
    expect(isValidIpOrCidr('1.02.3.04')).toBe(false)
    expect(isValidIpOrCidr('10.0.0.01/24')).toBe(false)
  })
  it('非法：段数不对', () => {
    expect(isValidIpOrCidr('1.2.3')).toBe(false)
    expect(isValidIpOrCidr('1.2.3.4.5')).toBe(false)
  })
  it('非法：段超范围', () => {
    expect(isValidIpOrCidr('1.2.3.256')).toBe(false)
    expect(isValidIpOrCidr('1.2.3.-1')).toBe(false)
  })
  it('非法：掩码超范围', () => {
    expect(isValidIpOrCidr('10.0.0.0/33')).toBe(false)
    expect(isValidIpOrCidr('10.0.0.0/-1')).toBe(false)
  })
  it('非法：注入字符', () => {
    expect(isValidIpOrCidr('1.2.3.4 allow all')).toBe(false)
    expect(isValidIpOrCidr('1.2.3.4; deny all')).toBe(false)
  })
})

describe('isValidPort', () => {
  it('合法端口', () => {
    expect(isValidPort(1)).toBe(true)
    expect(isValidPort(80)).toBe(true)
    expect(isValidPort(65535)).toBe(true)
  })
  it('非法：超范围', () => {
    expect(isValidPort(0)).toBe(false)
    expect(isValidPort(65536)).toBe(false)
    expect(isValidPort(-1)).toBe(false)
  })
  it('非法：非整数', () => {
    expect(isValidPort(80.5)).toBe(false)
    expect(isValidPort(null)).toBe(false)
    expect(isValidPort(undefined)).toBe(false)
  })
  it('字符串数字接受（IPC 层不保证类型，Number("80")=80 合法）', () => {
    expect(isValidPort('80')).toBe(true)
    expect(isValidPort('abc')).toBe(false)
  })
})

describe('clampInt', () => {
  it('范围内不变', () => {
    expect(clampInt(100, 0, 5000)).toBe(100)
  })
  it('超上限夹紧', () => {
    expect(clampInt(99999, 0, 5000)).toBe(5000)
  })
  it('超下限夹紧', () => {
    expect(clampInt(-5, 0, 5000)).toBe(0)
  })
  it('非数字返回 min', () => {
    expect(clampInt('abc', 1, 100)).toBe(1)
    expect(clampInt(null, 1, 100)).toBe(1)
  })
  it('浮点截断', () => {
    expect(clampInt(100.9, 0, 5000)).toBe(100)
  })
})

describe('isValidProtocol / isValidProxyMode', () => {
  it('合法协议', () => {
    expect(isValidProtocol('tcp')).toBe(true)
    expect(isValidProtocol('udp')).toBe(true)
    expect(isValidProtocol('http')).toBe(true)
    expect(isValidProtocol('https')).toBe(true)
  })
  it('非法协议', () => {
    expect(isValidProtocol('ftp')).toBe(false)
    expect(isValidProtocol('')).toBe(false)
  })
  it('合法代理模式', () => {
    expect(isValidProxyMode('standard')).toBe(true)
    expect(isValidProxyMode('minimal')).toBe(true)
    expect(isValidProxyMode('custom')).toBe(true)
  })
  it('非法代理模式', () => {
    expect(isValidProxyMode('advanced')).toBe(false)
  })
})

describe('isValidProxyPassUrl', () => {
  it('合法 http/https', () => {
    expect(isValidProxyPassUrl('http://27.196.154.94/')).toBe(true)
    expect(isValidProxyPassUrl('https://gitlab.com')).toBe(true)
    expect(isValidProxyPassUrl('http://10.0.0.1:8080/path/')).toBe(true)
  })
  it('非法：不以 http:// 或 https:// 开头', () => {
    expect(isValidProxyPassUrl('27.196.154.94')).toBe(false)
    expect(isValidProxyPassUrl('ftp://x')).toBe(false)
    expect(isValidProxyPassUrl('')).toBe(false)
  })
  it('非法：含 nginx 注入字符', () => {
    expect(isValidProxyPassUrl('http://x/; }')).toBe(false)
    expect(isValidProxyPassUrl('http://x/{evil}')).toBe(false)
    expect(isValidProxyPassUrl('http://x/#comment')).toBe(false)
    expect(isValidProxyPassUrl('http://x/\n;')).toBe(false)
  })
  it('非法：超长', () => {
    expect(isValidProxyPassUrl('http://x/' + 'a'.repeat(500))).toBe(false)
  })
})
