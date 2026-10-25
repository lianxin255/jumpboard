import type http from 'node:http'
import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  api,
  clickButton,
  clickNav,
  expectProxyStatusEventually,
  expectTrafficEventually,
  fillInput,
  launchPackagedApp,
  packagedAppInfo,
  startUpstreams,
  stopUpstreams,
  waitForText,
} from './helpers'

/**
 * IP 白名单 e2e：设置页添加 IP → 开启白名单 → 应用 → conf 含 allow 指令 → 关闭移除。
 * 交叉验证「应用配置」把白名单写进生成的 nginx conf（与生成器的契约）。
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

  // 建一条映射并应用（有 server 块，白名单 allow 才有附着点）
  await clickNav(page, '映射')
  await clickButton(page, '+ 新建映射')
  await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await clickButton(page, '下一步：填写参数')
  await fillInput(page, '如：公司 MySQL 对外', 'wl 验证')
  await page.evaluate(() => {
    const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="http"]'))
    lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
  await fillInput(page, '80', '19080')
  await fillInput(page, '192.168.1.10', '127.0.0.1')
  await fillInput(page, '3306', '19099')
  await clickButton(page, '保存映射')
  await waitForText(page, 'wl 验证')
  hostId = (await api<any[]>(page, '/api/host/list'))[0].id
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  await stopUpstreams(upstreams)
  if (dataDir)
    rmSync(dataDir, { recursive: true, force: true })
})

async function applyConfig() {
  await clickButton(page, '应用配置')
  await clickButton(page, '应用')
}

describe('iP 白名单 e2e', () => {
  it('设置页添加 IP（非法输入被拦）', async () => {
    await clickNav(page, '设置')
    await fillInput(page, '192.168.1.10 或 10.0.0.0/24', 'not-an-ip')
    await clickButton(page, '+ 添加')
    await waitForText(page, '格式不正确')
    await fillInput(page, '192.168.1.10 或 10.0.0.0/24', '10.0.0.0/24')
    await clickButton(page, '+ 添加')
    await waitForText(page, '10.0.0.0/24')
    const wl = await api<any[]>(page, '/api/whitelist/list')
    expect(wl.some(w => w.ip === '10.0.0.0/24')).toBe(true)
  })

  it('开启白名单 → 应用 → 生成的 conf 含 allow 10.0.0.0/24', async () => {
    await page.evaluate(() => {
      const row = [...document.querySelectorAll('.row')].find(r => r.textContent?.includes('启用 IP 白名单'))
      row?.querySelector<HTMLInputElement>('input[type=checkbox]')?.click()
    })
    await waitForText(page, '白名单已开启')
    await clickNav(page, '映射')
    await applyConfig()

    // 快照写入在主进程 reload 完成后的微任务里，与 nginx 进程级生效存在毫秒级竞态，轮询等待
    let snaps: any[] = []
    const deadline = Date.now() + 8000
    while (Date.now() < deadline) {
      snaps = await api<any[]>(page, '/api/snapshot/list', hostId)
      if (snaps?.length)
        break
      await new Promise(r => setTimeout(r, 200))
    }
    expect(snaps.length).toBeGreaterThan(0)
    expect(snaps[0].content).toContain('allow 10.0.0.0/24')
    expect(snaps[0].content).toContain('deny all')

    // 白名单真实拦截：127.0.0.1 不在名单 → nginx 返回 403（本机回环被拒是预期行为）
    await expectProxyStatusEventually(403)
  })

  it('移除 IP 并关闭白名单 → 应用后 conf 不再含 allow', async () => {
    await clickNav(page, '设置')
    await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button.icon-btn')].find(b => b.title === '移除' && b.offsetParent !== null)
      btn?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await clickButton(page, '移除') // 确认框
    await waitForText(page, '已移除')
    await page.evaluate(() => {
      const row = [...document.querySelectorAll('.row')].find(r => r.textContent?.includes('启用 IP 白名单'))
      row?.querySelector<HTMLInputElement>('input[type=checkbox]')?.click()
    })
    await waitForText(page, '白名单已关闭')
    await clickNav(page, '映射')
    await applyConfig()
    await expectTrafficEventually('A-19099')
    const snaps = await api<any[]>(page, '/api/snapshot/list', hostId)
    expect(snaps[0].content).not.toContain('allow ')
  })
})
