import { closeSync, openSync, readSync, statSync } from 'node:fs'

/**
 * 从文件尾部读取最后 N 行（避免全量 readFileSync 打爆内存）。
 *
 * 策略：从文件末尾向前按块（CHUNK）读取，累积到拿到 N+1 个换行符或到达文件头。
 * 只在内存里保留最近读取的块，大文件也不会 OOM。
 *
 * 返回的行不含尾部空行；文件不存在或读取失败返回空数组。
 */
export function tailFile(filePath: string, maxLines: number): string[] {
  let fd: number | null = null
  try {
    const stat = statSync(filePath)
    if (!stat.isFile() || stat.size === 0)
      return []

    fd = openSync(filePath, 'r')
    const fileSize = stat.size
    const CHUNK = 64 * 1024 // 64KB 块
    const blocks: Buffer[] = []
    let position = fileSize
    let newlineCount = 0

    // 从尾部向前读，直到攒够 maxLines+1 个换行（+1 保证切出来的行完整）
    while (position > 0 && newlineCount <= maxLines) {
      const readSize = Math.min(CHUNK, position)
      position -= readSize
      const buf = Buffer.alloc(readSize)
      readSync(fd, buf, 0, readSize, position)
      blocks.unshift(buf)
      // 统计这块里的换行符
      for (let i = 0; i < readSize; i++) {
        if (buf[i] === 0x0A)
          newlineCount++
      }
    }

    const content = Buffer.concat(blocks).toString('utf-8')
    // split + 剥离行尾 \r（Windows 日志为 \r\n）+ filter 空行，取最后 maxLines 行
    const lines = content.split('\n')
      .map(l => (l.endsWith('\r') ? l.slice(0, -1) : l))
      .filter(l => l.length > 0)
    return lines.slice(-maxLines)
  }
  catch {
    return []
  }
  finally {
    if (fd !== null) {
      try { closeSync(fd) }
      catch { /* 忽略 */ }
    }
  }
}
