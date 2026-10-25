import type http from 'node:http'
import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, it } from 'vitest'
import {
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
 * 日志 e2e：操作日志（应用级）与 nginx 日志（文件级）的展示与清空。
 */

let app: ElectronApplication
let page: Page
let upstreams: http.Server[]

beforeAll(async () => {
  upstreams = startUpstreams()
  ;({ app, page } = await launchPackagedApp())
  await clickButton(page, '启动')
  await waitForText(page, '运行中')

  // 造操作日志 + access 日志
  await clickNav(page, '映射')
  await clickButton(page, '+ 新建映射')
  await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await clickButton(page, '下一步：填写参数')
  await fillInput(page, '如：公司 MySQL 对外', '日志验证')
  await page.evaluate(() => {
    const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="http"]'))
    lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
  await fillInput(page, '80', '19080')
  await fillInput(page, '192.168.1.10', '127.0.0.1')
  await fillInput(page, '3306', '19099')
  await clickButton(page, '保存映射')
  await waitForText(page, '日志验证')
  await clickButton(page, '应用配置')
  await clickButton(page, '应用')
  await waitForText(page, '运行中')
  // 轮询等 reload 生效并命中流量（产生 access 日志；worker 就绪有毫秒级延迟）
  await expectTrafficEventually('A-19099')
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  await stopUpstreams(upstreams)
  rmSync(packagedAppInfo().dataDir, { recursive: true, force: true })
})

describe('日志 e2e', () => {
  it('操作日志：业务操作有记录，清空后为空', async () => {
    await clickNav(page, '日志')
    await waitForText(page, '配置应用成功')
    await waitForText(page, '新建映射')
    // 清空两步：工具栏「清空」开确认框 → confirm 的「清空」确认；清空后只剩 clear 自身日志
    await clickButton(page, '清空', { last: true })
    await clickButton(page, '清空', { css: 'button:not(.btn-sm)', last: true })
    await waitForText(page, '操作日志已清空')
  })

  it('nginx Access 日志：真实请求可见，清空后消失', async () => {
    await clickButton(page, 'Nginx 日志', { contains: true })
    await clickButton(page, '刷新') // 主动重拉日志文件
    await clickButton(page, 'Access 日志')
    await waitForText(page, 'GET / HTTP/1.1')
    await clickButton(page, '清空', { last: true })
    await clickButton(page, '清空', { last: true }) // confirm（文案同为清空，取最后一个）
    await waitForText(page, '暂无 nginx 运行日志')
  })
})
