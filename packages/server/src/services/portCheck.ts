import { createServer } from 'node:net'

/**
 * 检测本机端口是否可用（未被占用）。
 * 实现：尝试 listen，成功即空闲，失败（EADDRINUSE 等）即被占。
 * null/undefined 返回 true（无需检测，例如 HTTPS 不收集 listenPort）。
 */
export function isPortAvailable(port: number | null | undefined): Promise<boolean> {
  if (port == null)
    return Promise.resolve(true)
  return new Promise((resolve) => {
    const server = createServer()
    server.once('error', () => resolve(false))
    server.once('listening', () => {
      server.close(() => resolve(true))
    })
    server.listen(port)
  })
}
