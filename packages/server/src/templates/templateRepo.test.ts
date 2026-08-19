import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { __setCustomDirForTest, deleteCustomTemplate, listAllTemplates, listBuiltinTemplates, listCustomTemplates, saveCustomTemplate } from '../templates/templateRepo'

describe('templateRepo', () => {
  let customDir: string

  beforeEach(() => {
    customDir = join(tmpdir(), `tpl-test-${Date.now()}`)
    mkdirSync(customDir, { recursive: true })
    __setCustomDirForTest(customDir)
  })

  afterEach(() => {
    rmSync(customDir, { recursive: true, force: true })
  })

  it('listBuiltinTemplates 返回 13 个', () => {
    const list = listBuiltinTemplates()
    expect(list).toHaveLength(13)
    expect(list.every(t => t.builtin)).toBe(true)
  })

  it('内置模板按 category 分组覆盖 4 类', () => {
    const list = listBuiltinTemplates()
    const cats = new Set(list.map(t => t.category))
    expect(cats.has('网络层')).toBe(true)
    expect(cats.has('Web')).toBe(true)
    expect(cats.has('数据库')).toBe(true)
    expect(cats.has('消息')).toBe(true)
  })

  it('saveCustomTemplate 写入并 listCustomTemplates 能读到', () => {
    saveCustomTemplate({
      id: 'my-tpl',
      name: '我的模板',
      icon: 'star',
      category: '自定义',
      description: '测试',
      builtin: false,
      mapping: { protocol: 'tcp', listenPort: 7000, upstreamHost: '10.0.0.1', upstreamPort: 22 },
    })
    const list = listCustomTemplates()
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('我的模板')
    expect(list[0].builtin).toBe(false)
  })

  it('listAllTemplates 合并内置 + 自定义', () => {
    saveCustomTemplate({
      id: 'c1',
      name: '自定义1',
      icon: 'x',
      category: '自定义',
      description: '',
      builtin: false,
      mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
    })
    const all = listAllTemplates()
    expect(all.length).toBe(13 + 1)
  })

  it('deleteCustomTemplate 删除', () => {
    saveCustomTemplate({
      id: 'del',
      name: '待删',
      icon: 'x',
      category: '自定义',
      description: '',
      builtin: false,
      mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
    })
    deleteCustomTemplate('del')
    expect(listCustomTemplates()).toHaveLength(0)
  })

  it('重复 id 的自定义模板覆盖旧版', () => {
    saveCustomTemplate({ id: 'same', name: 'v1', icon: 'x', category: 'c', description: '', builtin: false, mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 } })
    saveCustomTemplate({ id: 'same', name: 'v2', icon: 'x', category: 'c', description: '', builtin: false, mapping: { protocol: 'tcp', upstreamHost: '2.2.2.2', upstreamPort: 81 } })
    const list = listCustomTemplates()
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('v2')
  })

  it('saveCustomTemplate 拒绝路径穿越 id（防止写到模板目录外）', () => {
    const evil = '../../../etc/evil' as any
    expect(() => saveCustomTemplate({
      id: evil,
      name: 'x',
      icon: 'x',
      category: 'c',
      description: '',
      builtin: false,
      mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 },
    })).toThrow()
    // customDir 里没有越界文件
    expect(existsSync(join(customDir, '../../../etc/evil.yml'))).toBe(false)
  })

  it('deleteCustomTemplate 对路径穿越 id 静默拒绝（不拼路径）', () => {
    // 不抛错，也不应删 customDir 外的文件
    expect(() => deleteCustomTemplate('../../whatever')).not.toThrow()
  })
})
