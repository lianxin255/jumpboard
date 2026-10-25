import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import {
  api,
  clickButton,
  clickNav,
  fillInput,
  launchPackagedApp,
  packagedAppInfo,
  waitForText,
  waitForTextGone,
} from './helpers'

/**
 * 模板 e2e：映射表单「保存为模板」（promptBox 命名）→ 模板选择器出现自定义模板 → 删除。
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

async function openTemplatePicker() {
  await clickNav(page, '映射')
  await clickButton(page, '+ 新建映射')
  await waitForText(page, '选择模板')
}

describe('模板 e2e', () => {
  it('表单另存为自定义模板（promptBox 命名）', async () => {
    await openTemplatePicker()
    await page.evaluate(() => document.querySelector('.tpl-card.custom')?.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await clickButton(page, '下一步：填写参数')
    await fillInput(page, '如：公司 MySQL 对外', '模板源映射')
    await page.evaluate(() => {
      const lbl = [...document.querySelectorAll('label')].find(l => l.querySelector('input[type=radio][value="https"]'))
      lbl?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await clickButton(page, '另存为模板')
    // promptBox：输入框已预填默认值，直接确认
    await clickButton(page, '保存模板')
    await waitForText(page, '模板已保存')
    const tpls = await api<any[]>(page, '/api/template/list')
    expect(tpls.some(t => t.name === '模板源映射' && !t.builtin)).toBe(true)
  })

  it('模板选择器出现自定义模板并可选中', async () => {
    await clickButton(page, '取消')
    await openTemplatePicker()
    await waitForText(page, '模板源映射')
    await page.evaluate(() => {
      const card = [...document.querySelectorAll('.tpl-card')].find(c => c.textContent?.includes('模板源映射'))
      card?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await clickButton(page, '下一步：填写参数')
    await waitForText(page, '保存映射') // 进入表单即成功
    await clickButton(page, '取消')
  })

  it('删除自定义模板 → 选择器不再出现', async () => {
    await openTemplatePicker()
    await page.evaluate(() => {
      const del = [...document.querySelectorAll('.tpl-card .tpl-del')].find(b => b.offsetParent !== null)
      del?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await clickButton(page, '删除') // 确认框
    await waitForTextGone(page, '模板源映射')
    const tpls = await api<any[]>(page, '/api/template/list')
    expect(tpls.find(t => t.name === '模板源映射')).toBeUndefined()
    await clickButton(page, '取消')
  })
})
