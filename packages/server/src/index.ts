import type { IncomingMessage, ServerResponse } from 'node:http';
import type { RouteContext, RouteHandler } from './routes';
import { existsSync, readFileSync, statSync } from 'node:fs'
/**
 * Node sidecar 入口：HTTP server + 静态托管 web/dist。
 *
 * 环境变量（由 Tauri Rust 壳注入）：
 * - NGJ_DATA_ROOT：data 目录根（exe 旁 data/）
 * - NGJ_RESOURCES_DIR：resources 目录（内置 nginx/模板）
 * - NGJ_PORT：HTTP 端口（默认 47831）
 * - NGJ_WEB_DIST：前端静态文件目录（开发时指向 apps/web/dist，打包时由 sidecar 旁的 web-dist/）
 * - NGJ_SECRET：密码加密密钥（每次启动随机生成）
 */
import { createServer } from 'node:http'
import { dirname, extname, join, normalize } from 'node:path'

import { fileURLToPath } from 'node:url'
import { closeDb, openDb } from './db/connection'
import { createHost, getDefaultHost, listHosts, setDefaultHost } from './db/hostRepo'
import { getActiveEngine } from './nginx/engineRegistry'
import { switchEngineToHost } from './nginx/hostEngineSwitch'
import { resolvePaths } from './paths'
import { buildRoutes } from './routes'
import { startHealthCheckLoop, stopHealthCheckLoop } from './services/healthCheck'
import { logger } from './services/logger'

const PORT = Number(process.env.NGJ_PORT ?? 47831)
const HOST = '127.0.0.1'

let __dirname = '.'
try { __dirname = dirname(fileURLToPath(import.meta.url)) }
catch { /* CJS bundle：__dirname 已由 Node 注入 */ }

const WEB_DIST = process.env.NGJ_WEB_DIST ?? join(__dirname, '..', '..', '..', 'apps', 'web', 'dist')

const MIME: Record<string, string> = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.mjs': 'text/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.json': 'application/json',
  '.map': 'application/json',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
}

function send(res: ServerResponse, code: number, body: unknown, type = 'application/json'): void {
  // API 响应一律 JSON 序列化：字符串值（如 engine status 'stopped'）也需要引号包裹才是合法 JSON
  const payload = JSON.stringify(body)
  res.writeHead(code, { 'content-type': `${type}; charset=utf-8`, 'cache-control': 'no-store' })
  res.end(payload)
}

function sendError(res: ServerResponse, code: number, message: string): void {
  send(res, code, { error: message })
}

/** 读取请求体（JSON） */
function readBody(req: IncomingMessage): Promise<any> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', (chunk: Buffer) => {
      data += chunk.toString()
      if (data.length > 10 * 1024 * 1024) { // 10MB 上限
        reject(new Error('请求体过大'))
        req.destroy()
      }
    })
    req.on('end', () => {
      if (!data)
        return resolve({})
      try {
        resolve(JSON.parse(data))
      }
      catch {
        reject(new Error('JSON 解析失败'))
      }
    })
    req.on('error', reject)
  })
}

/** 静态文件托管（web/dist） */
function serveStatic(res: ServerResponse, relPath: string): boolean {
  const safePath = normalize(relPath).replace(/^(\.\.[/\\])+/, '')
  const filePath = join(WEB_DIST, safePath)
  if (!filePath.startsWith(WEB_DIST) || !existsSync(filePath) || !statSync(filePath).isFile()) {
    return false
  }
  const ext = extname(filePath)
  const mime = MIME[ext] ?? 'application/octet-stream'
  res.writeHead(200, { 'content-type': `${mime}; charset=utf-8` })
  res.end(readFileSync(filePath))
  return true
}

async function handleRoute(
  routes: Record<string, RouteHandler>,
  method: string,
  pathname: string,
  query: URLSearchParams,
  body: any,
  res: ServerResponse,
): Promise<void> {
  const key = `${method} ${pathname}`
  const handler = routes[key]
  if (!handler) {
    sendError(res, 404, `路由不存在: ${method} ${pathname}`)
    return
  }
  const ctx: RouteContext = { params: {}, query, body }
  try {
    const result = await handler(ctx)
    send(res, 200, result)
  }
  catch (err: any) {
    logger.error(null, 'app', `路由错误 ${key}: ${err?.message ?? String(err)}`)
    sendError(res, 500, err?.message ?? String(err))
  }
}

async function main(): Promise<void> {
  // 初始化数据目录 + 数据库
  const paths = resolvePaths()
  paths.ensureDataDirs()
  openDb()

  // 首次启动：如果没有 host，创建默认 local host
  if (listHosts().length === 0) {
    const h = createHost({
      name: '本机网关',
      type: 'local',
      isDefault: 1,
      localConfig: { nginxPath: '', confDir: '', logDir: '' },
      remoteConfig: null,
    })
    setDefaultHost(h.id)
  }

  // 切换到默认主机
  const def = getDefaultHost()
  if (def) {
    const r = await switchEngineToHost(def.id)
    if (!r.ok)
      logger.warn(def.id, 'app', `启动时切换默认主机失败：${r.error ?? '未知原因'}`)
  }

  // 启动健康检查循环
  startHealthCheckLoop()

  // 构建路由表
  const routes = buildRoutes()

  // 创建 HTTP server
  const server = createServer(async (req, res) => {
    // CORS + 安全头（仅 localhost 访问）
    res.setHeader('access-control-allow-origin', '*')
    res.setHeader('access-control-allow-methods', 'GET, POST, OPTIONS')
    res.setHeader('access-control-allow-headers', 'Content-Type')

    if (req.method === 'OPTIONS') {
      res.writeHead(204)
      res.end()
      return
    }

    const url = new URL(req.url ?? '/', `http://${HOST}:${PORT}`)
    const pathname = url.pathname
    const query = url.searchParams

    // API 路由（以 /api/ 开头）
    if (pathname.startsWith('/api/')) {
      let body: any = {}
      if (req.method === 'POST' || req.method === 'PUT' || req.method === 'PATCH') {
        try {
          body = await readBody(req)
        }
        catch (err: any) {
          sendError(res, 400, err?.message ?? '请求体解析失败')
          return
        }
      }
      await handleRoute(routes, req.method ?? 'GET', pathname, query, body, res)
      return
    }

    // 静态文件托管
    if (req.method === 'GET') {
      // SPA fallback：/xxx → index.html
      const relPath = pathname === '/' ? 'index.html' : pathname.slice(1)
      if (serveStatic(res, relPath))
        return
      // SPA history fallback
      if (!pathname.includes('.') && serveStatic(res, 'index.html'))
        return
    }

    sendError(res, 404, 'Not Found')
  })

  server.listen(PORT, HOST, () => {
    logger.info(null, 'app', `sidecar HTTP server listening on http://${HOST}:${PORT}`)
    // 通知 Rust 壳端口就绪（Rust 壳轮询端口，不需要 stdout 信号）
    if (process.send) {
      process.send({ type: 'ready', port: PORT })
    }
  })

  // 优雅退出：收到 SIGTERM/SIGINT 时停止 nginx + 关闭数据库
  let shuttingDown = false
  async function shutdown(signal: string): Promise<void> {
    if (shuttingDown)
      return
    shuttingDown = true
    logger.info(null, 'app', `收到 ${signal}，正在优雅退出...`)
    try {
      const eng = getActiveEngine()
      if (eng && eng.status() === 'running') {
        await eng.stop()
        logger.info(null, 'app', 'nginx 已停止')
      }
    }
    catch (err) {
      logger.error(null, 'app', `退出时停止 nginx 失败: ${err instanceof Error ? err.message : String(err)}`)
    }
    stopHealthCheckLoop()
    closeDb()
    server.close()
    process.exit(0)
  }

  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
}

main().catch((err) => {
  console.error('[startup] fatal:', err)
  process.exit(1)
})
