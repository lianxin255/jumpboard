import type http from 'node:http'
import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  api,
  clickButton,
  clickNav,
  expectTrafficEventually,
  fillInput,
  launchPackagedApp,
  packagedAppInfo,
  startUpstreams,
  stopUpstreams,
  waitForText,
} from './helpers'

/**
 * 历史快照 e2e：两次应用产生两份快照 → 回滚到旧版 → 真实流量随 conf 回滚 → dirty 提示。
 */

let app: ElectronApplication
let page: Page
let dataDir: string
let upstreams: http.Server[]
let hostId: string

beforeAll(async () => {
  upstreams = startUpstreams()
  const info = packagedAppInfo()
  dataDir = info.dataDir
  ;({ app, page } = await launchPackagedApp())
  await clickButton(page, '启动')
  await waitForText(page, '运行中')

  await clickNav(page, '映射')
  await clickButton(page, '+ 新建映射')
  await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await clickButton(page, '下一步：填写参数')
  await fillInput(page, '如：公司 MySQL 对外', '回滚验证')
  await page.evaluate(() => {
    const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="http"]'))
    lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
  await fillInput(page, '80', '19080')
  await fillInput(page, '192.168.1.10', '127.0.0.1')
  await fillInput(page, '3306', '19099')
  await clickButton(page, '保存映射')
  await waitForText(page, '回滚验证')
  hostId = (await api<any[]>(page, '/api/host/list'))[0].id

  await clickButton(page, '应用配置')
  await clickButton(page, '应用')
  await expectTrafficEventually('A-19099')

  // 改上游到 19100 再应用 → conf 当前为 B，历史里有 A 版本
  await page.evaluate(() => document.querySelector('button.icon-btn[title="编辑"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await fillInput(page, '3306', '19100')
  await clickButton(page, '保存映射')
  await clickButton(page, '应用配置')
  await clickButton(page, '应用')
  await expectTrafficEventually('B-19100')
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  await stopUpstreams(upstreams)
  if (dataDir)
    rmSync(dataDir, { recursive: true, force: true })
})

describe('历史快照 e2e', () => {
  it('两次应用产生两份快照', async () => {
    const snaps = await api<any[]>(page, '/api/snapshot/list', hostId)
    expect(snaps.length).toBe(2)
    expect(snaps[0].content).toContain('19100')
    expect(snaps[1].content).toContain('19099')
  })

  it('历史页回滚到旧版 → dirty 提示 + 真实流量回到 A', async () => {
    await clickNav(page, '历史')
    await waitForText(page, '快照')
    // 点选最旧一条（列表最新在前）
    await page.evaluate(() => {
      const items = [...document.querySelectorAll('.hist-item')]
      items[items.length - 1]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await waitForText(page, '回滚到此版本')
    await clickButton(page, '回滚到此版本')
    await clickButton(page, '回滚') // 确认框
    await waitForText(page, '已回滚')

    // 回滚真实生效：conf 回到 19099 → 流量回到 A
    await expectTrafficEventually('A-19099')
    // 回滚后 conf 与 DB 偏离，dirty 置位（点应用会覆盖回滚结果——设计语义）
    expect(await api(page, '/api/engine/dirty-get')).toBe(true)
  })
})
