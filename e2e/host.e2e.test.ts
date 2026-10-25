import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { api, clickButton, launchPackagedApp, packagedAppInfo, waitForText } from './helpers'

/**
 * 主机管理 e2e：新增本机主机 → 设为默认 → 切换 → 编辑（回归 bug#1：编辑不得丢默认标志）→ 删除。
 */

let app: ElectronApplication
let page: Page
let dataDir: string

beforeAll(async () => {
  const info = packagedAppInfo()
  dataDir = info.dataDir
  ;({ app, page } = await launchPackagedApp())
}, 150_000)

afterAll(async () => {
  await app?.close().catch(() => {})
  if (dataDir)
    rmSync(dataDir, { recursive: true, force: true })
})

function hostsViaApi() {
  return api<any[]>(page, '/api/host/list')
}

describe('主机管理 e2e', () => {
  it('新增本机主机（表单：本地主机 → 名称 → 保存）', async () => {
    await clickButton(page, '主机')
    await clickButton(page, '+ 新增主机')
    await page.evaluate(() => {
      const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="local"]'))
      lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await page.getByPlaceholder('如：生产跳板 prod-jump-01').fill('e2e 第二台')
    await clickButton(page, '保存主机')
    await waitForText(page, '主机已保存')
    const hosts = await hostsViaApi()
    expect(hosts).toHaveLength(2)
    expect(hosts.find(h => h.name === 'e2e 第二台')?.type).toBe('local')
  })

  it('编辑默认主机改名称 → isDefault 保留（回归：编辑不得清默认标志）', async () => {
    // 编辑默认主机「本机网关」（列表第一张卡的编辑按钮）
    await page.evaluate(() => document.querySelector('button.icon-btn[title="编辑"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await page.getByPlaceholder('如：生产跳板 prod-jump-01').fill('本机网关改')
    await clickButton(page, '保存主机')
    await waitForText(page, '主机已保存')
    const hosts = await hostsViaApi()
    const def = hosts.find(h => h.name === '本机网关改')
    expect(def).toBeTruthy()
    expect(def.isDefault).toBe(1)
  })

  it('设第二台为默认 → getDefaultHost 切换', async () => {
    await page.evaluate(() => document.querySelector('button.icon-btn[title="设为默认"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await waitForText(page, '已将「e2e 第二台」设为默认主机')
    const hosts = await hostsViaApi()
    expect(hosts.find(h => h.name === 'e2e 第二台')?.isDefault).toBe(1)
    expect(hosts.find(h => h.name === '本机网关改')?.isDefault).toBe(0)
  })

  it('切回默认主机并删除第二台 → 列表剩一台', async () => {
    // 删除第二台（第二张卡上的删除按钮）
    await page.evaluate(() => {
      const cards = [...document.querySelectorAll('button.icon-btn.danger[title="删除"]')]
      cards[cards.length - 1]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await clickButton(page, '删除') // 确认框
    await waitForText(page, '本机网关改')
    const hosts = await hostsViaApi()
    expect(hosts).toHaveLength(1)
    expect(hosts[0].isDefault).toBe(1)
  })
})
