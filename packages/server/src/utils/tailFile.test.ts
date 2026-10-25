import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { tailFile } from '../utils/tailFile'

describe('tailFile', () => {
  let dir: string

  beforeEach(() => {
    dir = join(tmpdir(), `tail-test-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
  })
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true })
  })

  it('小文件：返回全部行', () => {
    const f = join(dir, 'small.log')
    writeFileSync(f, 'line1\nline2\nline3\n')
    expect(tailFile(f, 100)).toEqual(['line1', 'line2', 'line3'])
  })

  it('取最后 N 行', () => {
    const f = join(dir, 'tail.log')
    writeFileSync(f, 'a\nb\nc\nd\ne\n')
    expect(tailFile(f, 2)).toEqual(['d', 'e'])
  })

  it('n 大于行数：返回全部', () => {
    const f = join(dir, 'few.log')
    writeFileSync(f, 'x\ny\n')
    expect(tailFile(f, 100)).toEqual(['x', 'y'])
  })

  it('大文件（跨多个 CHUNK）：正确取尾部', () => {
    const f = join(dir, 'big.log')
    // 生成 > 64KB 的内容（每行约 100 字节，1000 行 ≈ 100KB）
    const lines: string[] = []
    for (let i = 0; i < 1000; i++) lines.push(`line-${i}-`.padEnd(95, 'x'))
    writeFileSync(f, `${lines.join('\n')}\n`)
    const tail = tailFile(f, 5)
    expect(tail).toHaveLength(5)
    expect(tail[4]).toContain('line-999-')
    expect(tail[0]).toContain('line-995-')
  })

  it('文件不存在：返回空数组', () => {
    expect(tailFile(join(dir, 'nope.log'), 10)).toEqual([])
  })

  it('空文件：返回空数组', () => {
    const f = join(dir, 'empty.log')
    writeFileSync(f, '')
    expect(tailFile(f, 10)).toEqual([])
  })

  it('无尾换行：仍正确切分', () => {
    const f = join(dir, 'notrail.log')
    writeFileSync(f, 'a\nb\nc') // 无尾换行
    expect(tailFile(f, 10)).toEqual(['a', 'b', 'c'])
  })

  it('行尾 \\r\\n（Windows 日志）：剥离行尾 \\r', () => {
    const f = join(dir, 'crlf.log')
    writeFileSync(f, 'a\r\nb\r\nc\r\n')
    expect(tailFile(f, 10)).toEqual(['a', 'b', 'c'])
  })
})
