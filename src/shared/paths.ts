/**
 * 路径工具（前后端共用）。
 *
 * nginx 在 Windows 上把反斜杠当转义符（\t→TAB, \n→换行），
 * Windows 路径 C:\Users\...\data 会被错误解析。
 * nginx 在 Windows 上完全支持正斜杠，所以统一转成 /。
 *
 * 非 Windows 平台原样返回（无副作用）。
 */
export function toFwdSlash(p: string): string {
  if (process.platform === 'win32') return p.replace(/\\/g, '/')
  return p
}
