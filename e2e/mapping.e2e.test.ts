import type http from 'node:http'
import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  api,
  clickButton,
  clickNav,
  expectRefused,
  expectTrafficEventually,
  fillInput,
  launchPackagedApp,
  packagedAppInfo,
  startUpstreams,
  stopUpstreams,
  waitForText,
} from './helpers'

/**
 * 映射全生命周期 e2e：新建（两步模板流）→ 应用 → 真实流量 → 编辑切换上游 → 删除 → 流量断开。
 * 附带引擎启动/停止（前置与收尾），与白名单/快照等模块的交叉验证在各自文件里。
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
  await waitForText(page, '运行中')
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  await stopUpstreams(upstreams)
  if (dataDir)
    rmSync(dataDir, { recursive: true, force: true })
})

async function applyConfig() {
  await clickButton(page, '应用配置')
  await clickButton(page, '应用') // 确认框
}

describe('映射全生命周期 e2e', () => {
  it('新建映射：模板选择 → 自定义 → 表单填写 → 保存', async () => {
    await clickNav(page, '映射')
    await clickButton(page, '+ 新建映射')
    await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await clickButton(page, '下一步：填写参数')
    await fillInput(page, '如：公司 MySQL 对外', 'e2e 反代')
    await page.evaluate(() => {
      const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="http"]'))
      lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await fillInput(page, '80', '19080')
    await fillInput(page, 'api.example.com', 'e2e.local')
    await fillInput(page, '192.168.1.10', '127.0.0.1')
    await fillInput(page, '3306', '19099')
    await clickButton(page, '保存映射')
    await waitForText(page, 'e2e 反代')
    const hosts = await api<any[]>(page, '/api/host/list')
    expect(await api<any[]>(page, '/api/mapping/list', hosts[0].id)).toHaveLength(1)
  })

  it('应用配置 → 真实流量经 nginx 命中上游 A', async () => {
    await applyConfig()
    await expectTrafficEventually('A-19099')
  })

  it('编辑映射上游 19099 → 19100 → 重应用 → 流量切到 B', async () => {
    await page.evaluate(() => document.querySelector('button.icon-btn[title="编辑"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await fillInput(page, '3306', '19100')
    await clickButton(page, '保存映射')
    await applyConfig()
    await expectTrafficEventually('B-19100')
  })

  it('删除映射 → 重应用 → 19080 不再监听', async () => {
    await page.evaluate(() => document.querySelector('button.icon-btn.danger[title="删除"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await clickButton(page, '删除') // 确认框
    await waitForText(page, '还没有任何映射规则')
    await applyConfig()
    await expectRefused('http://127.0.0.1:19080/')
  })
})
