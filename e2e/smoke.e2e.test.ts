import type { ElectronApplication, Page } from 'playwright-core'
import { rmSync } from 'node:fs'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { api, clickButton, launchPackagedApp, packagedAppInfo, waitForText } from './helpers'

/**
 * 小示例（smoke）：打包版能启动、node:sqlite 建库迁移生效、引擎能通过 UI 开关。
 * 跑通后按业务模块逐个补全 e2e 覆盖。
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

describe('smoke：打包版启动与引擎开关', () => {
  it('应用启动：默认主机已自动创建（node:sqlite 建库迁移生效）', async () => {
    const hosts = await api<any[]>(page, '/api/host/list')
    expect(hosts).toHaveLength(1)
    expect(hosts[0].name).toBe('本机网关')
    expect(hosts[0].isDefault).toBe(1)
  })

  it('uI 启动引擎 → 运行中 + PID', async () => {
    await clickButton(page, '启动')
    await waitForText(page, '运行中')
    expect(await api(page, '/api/engine/status')).toBe('running')
    expect(await api(page, '/api/engine/pid-get')).toBeTypeOf('number')
  })

  it('uI 停止引擎（确认框）→ 已停止', async () => {
    await clickButton(page, '停止')
    await clickButton(page, '停止', { css: 'button:not(.btn-sm)', last: true })
    await waitForText(page, '已停止')
    expect(await api(page, '/api/engine/pid-get')).toBeNull()
  })
})
