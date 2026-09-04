import { readFileSync, unlinkSync } from 'node:fs'
import { join } from 'node:path'
import { pathExistsSync } from 'fs-extra'
import { isPidAlive, killTree } from './processKill'

/**
 * 清理孤儿 nginx：应用崩溃/强杀后，上次启动的 nginx 仍在运行并占用端口，
 * 新实例 start 会 bind 失败且用户无从排查。
 *
 * nginx 的 pid 默认写在 <prefix>/logs/nginx.pid（本应用 conf 全部相对 -p prefix）。
 * 重建引擎前读取该文件：pid 存活 → 杀掉整个进程树并移除 pid 文件；
 * pid 不存活或文件不存在 → 无操作。只动自己 prefix 下的 pid，不碰别的实例。
 */
export async function stopStaleNginx(prefixDir: string): Promise<void> {
  const pidFile = join(prefixDir, 'logs', 'nginx.pid')
  if (!pathExistsSync(pidFile))
    return
  let pid: number | null = null
  try {
    const n = Number.parseInt(readFileSync(pidFile, 'utf-8').trim(), 10)
    if (Number.isFinite(n) && n > 0)
      pid = n
  }
  catch { /* 读取失败按无 pid 处理 */ }
  // pid 文件此刻必然是陈旧的（nginx 正常退出会自己删），无论内容是否可解析都移除
  try { unlinkSync(pidFile) }
  catch { /* 已被 nginx 自身清理 */ }
  if (pid === null)
    return

  if (isPidAlive(pid))
    await killTree(pid)
}
