import type http from 'node:http'
import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync, writeFileSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  api,
  clickButton,
  clickNav,
  launchPackagedApp,
  packagedAppInfo,
  startUpstreams,
  stopUpstreams,
  waitForText,
} from './helpers'

/**
 * 数据管理 e2e：导出 → UI 导入回环（file input）→ 清空数据自愈。
 */

let app: ElectronApplication
let page: Page
let dataDir: string
let upstreams: http.Server[]

beforeAll(async () => {
  upstreams = startUpstreams()
  const info = packagedAppInfo()
  dataDir = info.dataDir
  ;({ app, page } = await launchPackagedApp())
  await clickButton(page, '启动')
  try {
    await waitForText(page, '运行中', 15_000)
  }
  catch (e: any) {
    const logs = await api<any[]>(page, '/api/log/list', null, 20).catch(() => null)
    throw new Error(`启动引擎失败: ${e.message}\n应用日志: ${JSON.stringify(logs?.map(l => l.message))}`)
  }
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  await stopUpstreams(upstreams)
  if (dataDir)
    rmSync(dataDir, { recursive: true, force: true })
})

async function createMappingViaUi(name: string) {
  await clickNav(page, '映射')
  await clickButton(page, '+ 新建映射')
  await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await clickButton(page, '下一步：填写参数')
  await page.getByPlaceholder('如：公司 MySQL 对外').fill(name)
  await page.evaluate(() => {
    const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="http"]'))
    lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
  await page.getByPlaceholder('80', { exact: true }).fill('19081')
  await page.getByPlaceholder('192.168.1.10').fill('127.0.0.1')
  await page.getByPlaceholder('3306').fill('19099')
  await clickButton(page, '保存映射')
  await waitForText(page, name)
}

describe('数据管理 e2e', () => {
  it('导出 → 制造偏移 → UI 导入回环', async () => {
    await createMappingViaUi('导入回环')
    const exported = await api<any>(page, '/api/data/export')
    expect(exported.hosts).toHaveLength(1)
    const file = path.join(os.tmpdir(), `ngj-e2e-backup-${Date.now()}.json`)
    writeFileSync(file, JSON.stringify(exported))

    // 制造偏移：多建一条映射
    await createMappingViaUi('多余映射')
    const hosts = await api<any[]>(page, '/api/host/list')
    expect(await api<any[]>(page, '/api/mapping/list', hosts[0].id)).toHaveLength(2)

    // UI 导入：file input + 确认框
    await clickNav(page, '设置')
    await page.locator('input[type=file]').setInputFiles(file)
    await clickButton(page, '导入', { last: true }) // 确认框的导入（页面里还有设置页的同名按钮）
    // toast 文案带动态计数，失败时 dump 页面帮助定位
    try {
      await waitForText(page, '已导入 1 主机 / 1 映射', 8000)
    }
    catch {
      const ctx = await page.evaluate(() => document.body.textContent.slice(0, 500))
      throw new Error(`导入 toast 未出现。页面: ${JSON.stringify(ctx)}`)
    }

    const hostsAfter = await api<any[]>(page, '/api/host/list')
    expect(hostsAfter).toHaveLength(1)
    expect(await api<any[]>(page, '/api/mapping/list', hostsAfter[0].id)).toHaveLength(1)
    rmSync(file, { force: true })
  })

  it('uI 清空数据 → 自愈为全新状态（默认主机重建，无需重启）', async () => {
    await clickNav(page, '设置')
    await clickButton(page, '清空所有数据')
    await clickButton(page, '清空') // 确认框
    await waitForText(page, '已重置为初始状态')
    // 前端自动刷新，等待自愈完成
    await waitForText(page, '本机网关')
    const hosts = await api<any[]>(page, '/api/host/list')
    expect(hosts).toHaveLength(1)
    expect(hosts[0].isDefault).toBe(1)
    expect(await api<any[]>(page, '/api/mapping/list', hosts[0].id)).toHaveLength(0)
  })
})
