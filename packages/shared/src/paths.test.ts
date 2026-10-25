import { afterEach, describe, expect, it } from 'vitest'
import { toFwdSlash } from './paths'

/**
 * toFwdSlash 的 win32 分支在 macOS/Linux CI 上无法被真实平台触达，
 * 这里 mock process.platform 验证两个分支（nginx 在 Windows 把反斜杠当转义符）。
 */

const platformDesc = Object.getOwnPropertyDescriptor(process, 'platform')!

function setPlatform(value: string): void {
  Object.defineProperty(process, 'platform', { value })
}

afterEach(() => {
  Object.defineProperty(process, 'platform', platformDesc)
})

describe('toFwdSlash', () => {
  it('win32：反斜杠路径全部转为正斜杠', () => {
    setPlatform('win32')
    expect(toFwdSlash('C:\\Users\\app\\data\\nginx-home')).toBe('C:/Users/app/data/nginx-home')
    expect(toFwdSlash('D:\\nginx jump\\conf\\nginx.conf')).toBe('D:/nginx jump/conf/nginx.conf')
    expect(toFwdSlash('/already/forward')).toBe('/already/forward')
  })

  it('非 win32：原样返回（无副作用）', () => {
    setPlatform('darwin')
    expect(toFwdSlash('/var/folders/tmp/nginx-home')).toBe('/var/folders/tmp/nginx-home')
    expect(toFwdSlash('C:\\keep\\backslash')).toBe('C:\\keep\\backslash')

    setPlatform('linux')
    expect(toFwdSlash('/etc/nginx/nginx.conf')).toBe('/etc/nginx/nginx.conf')
  })
})
