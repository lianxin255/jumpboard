import type { ElectronApplication, Page } from 'playwright-core';
import { existsSync, rmSync } from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { _electron } from 'playwright-core'

/**
 * 打包版真实 UI e2e 工具集。
 *
 * 思路（借鉴 markdown-viewer-extension 的"真实构建产物 e2e"）：
 * 不 mock、不抄近路——启动真实打包出的应用，驱动真实 DOM（导航/按钮/表单），
 * 业务效果用真实网络流量（fetch 过 nginx）和 API 状态双重断言。
 *
 * 实现注意：所有 UI 操作走 page.evaluate 派发 DOM 事件。
 * 熄屏/后台窗口 Chromium 不产帧，Playwright 的物理点击和 locator 管线
 * （boundingBox/稳定性检查）会永远超时；page.evaluate 不依赖渲染帧。
 */

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

export interface PackagedApp {
  dir: string
  exe: string
  dataDir: string
}

/** 定位当前平台的打包产物（build-desktop --current --dir 的输出） */
export function packagedAppInfo(): PackagedApp {
  const dir = path.join(ROOT, 'release', process.platform === 'win32' ? 'win-unpacked' : 'mac-arm64')
  const exe = process.platform === 'win32'
    ? path.join(dir, 'NginxJump.exe')
    : path.join(dir, 'NginxJump.app', 'Contents', 'MacOS', 'NginxJump')
  if (!existsSync(exe)) {
    throw new Error(`打包产物不存在：${exe}\n先运行 pnpm build:desktop:dir（或直接 pnpm test:e2e，会自动打包）`)
  }
  return { dir, exe, dataDir: path.join(dir, 'data') }
}

/** 启动打包版应用（每次全新数据目录，保证用例间隔离） */
export async function launchPackagedApp(): Promise<{ app: ElectronApplication, page: Page, info: PackagedApp }> {
  const info = packagedAppInfo()
  rmSync(info.dataDir, { recursive: true, force: true })
  const app = await _electron.launch({ executablePath: info.exe })
  const page = await app.firstWindow()
  // 等 Vue 应用挂载完成（导航/头部按钮出现）
  await page.waitForFunction(() => !!document.querySelector('.nav'))
  return { app, page, info }
}

/** 走 renderer 的 electronAPI 调 IPC（快速断言后端真实状态） */
export function api<T = any>(page: Page, channel: string, ...args: any[]): Promise<T> {
  return page.evaluate(({ channel, args }) => {
    return (window as any).electronAPI.invoke(channel, ...args)
  }, { channel, args })
}

/** 轮询等待页面出现指定文本 */
export async function waitForText(page: Page, text: string, timeoutMs = 15_000) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    if (await page.evaluate(t => document.body.textContent.includes(t), text))
      return
    if (Date.now() > deadline) {
      const ctx = await page.evaluate(() => document.body.textContent.slice(0, 400)).catch(() => '(evaluate failed)')
      throw new Error(`waitForText 超时(${timeoutMs}ms): ${text}\n页面上下文: ${JSON.stringify(ctx)}`)
    }
    await new Promise(r => setTimeout(r, 200))
  }
}

interface ClickOpts { css?: string, last?: boolean, contains?: boolean }

/** 派发 DOM click 到匹配文本的按钮；css 收窄选择范围，last 取最后一个（confirm 弹窗在 DOM 尾部） */
export async function clickButton(page: Page, text: string, opts: ClickOpts = {}) {
  const r = await page.evaluate(({ text, css, last, contains }) => {
    const scope = css ? [...document.querySelectorAll(css)] : [...document.querySelectorAll('button')]
    const list = scope.filter((el): el is HTMLElement => el instanceof HTMLElement && el.offsetParent !== null)
      .filter(el => contains ? el.textContent?.includes(text) : el.textContent?.trim() === text)
    const btn = last ? list[list.length - 1] : list[0]
    if (!btn)
      return { ok: false, buttons: [...document.querySelectorAll('button')].filter(b => b.offsetParent !== null).map(b => b.textContent?.trim().slice(0, 14)) }
    btn.click()
    return { ok: true }
  }, { text, css: opts.css ?? 'button', last: !!opts.last, contains: !!opts.contains })
  if (!r.ok)
    throw new Error(`clickButton("${text}") 未找到可点按钮。可见按钮: ${JSON.stringify(r.buttons)}`)
}

/** 点顶部导航 tab */
export async function clickNav(page: Page, text: string) {
  await clickButton(page, text, { css: '.nav button' })
}

/** 向 placeholder 定位的输入框填值（派发 input 事件驱动 Vue v-model） */
export async function fillInput(page: Page, placeholder: string, value: string) {
  const r = await page.evaluate(({ placeholder, value }) => {
    const el = [...document.querySelectorAll('input')].filter(i => i.offsetParent !== null).find(i => i.placeholder === placeholder)
    if (!el)
      return { ok: false, placeholders: [...document.querySelectorAll('input')].filter(i => i.offsetParent !== null).map(i => i.placeholder) }
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set
    setter.call(el, value)
    el.dispatchEvent(new Event('input', { bubbles: true }))
    return { ok: true }
  }, { placeholder, value })
  if (!r.ok)
    throw new Error(`fillInput("${placeholder}") 未找到输入框。可见输入框: ${JSON.stringify(r.placeholders)}`)
}

/** 读取页面全部可见文本（调试/断言用） */
export async function bodyText(page: Page): Promise<string> {
  return page.evaluate(() => document.body.textContent)
}

/** 经 nginx 代理取上游内容（19080 是 e2e 固定监听端口） */
export async function fetchViaProxy(): Promise<string> {
  const res = await fetch('http://127.0.0.1:19080/', { signal: AbortSignal.timeout(3000) })
  return res.text()
}

/** 轮询直到代理流量包含期望的上游标记（reload 生效有毫秒级延迟） */
export async function expectTrafficEventually(tag: string, timeoutMs = 12_000) {
  const deadline = Date.now() + timeoutMs
  let last = '(never)'
  for (;;) {
    try {
      last = await fetchViaProxy()
      if (last.includes(tag))
        return
    }
    catch { /* 引擎未就绪/已停止/规则已删 */ }
    if (Date.now() > deadline)
      throw new Error(`代理流量未命中 ${tag}，最后实际: ${JSON.stringify(last)}`)
    await new Promise(r => setTimeout(r, 300))
  }
}

/** 轮询直到连接被拒绝（引擎停止或该端口无规则） */
export async function expectRefused(url = 'http://127.0.0.1:19080/', timeoutMs = 12_000) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    try {
      await fetch(url, { signal: AbortSignal.timeout(3000) }).then(r => r.text())
    }
    catch {
      return
    }
    if (Date.now() > deadline)
      throw new Error(`期望连接被拒绝，但 ${url} 仍有响应`)
    await new Promise(r => setTimeout(r, 300))
  }
}

/** 起两个内容不同的上游 HTTP 服务（19099/19100），用于断言流量与规则切换 */
export function startUpstreams(): http.Server[] {
  const make = (port: number, tag: string) => {
    const s = http.createServer((_req, res) => {
      res.setHeader('Content-Type', 'text/plain')
      res.end(`E2E-UPSTREAM-${tag}\n`)
    })
    s.listen(port, '127.0.0.1')
    return s
  }
  return [make(19099, 'A-19099'), make(19100, 'B-19100')]
}

export async function stopUpstreams(servers: http.Server[]): Promise<void> {
  await Promise.all(servers.map(s => new Promise<void>(r => s.close(() => r()))))
}

/** 轮询直到代理返回指定状态码（如白名单开启后本机被 403 拦截） */
export async function expectProxyStatusEventually(status: number, timeoutMs = 12_000) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    try {
      const res = await fetch('http://127.0.0.1:19080/', { signal: AbortSignal.timeout(3000) })
      if (res.status === status)
        return
    }
    catch { /* 未就绪/已停止 */ }
    if (Date.now() > deadline)
      throw new Error(`代理未返回 ${status}（超时）`)
    await new Promise(r => setTimeout(r, 300))
  }
}

/** 轮询等待页面文本消失（删除/关闭后的负向断言） */
export async function waitForTextGone(page: Page, text: string, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    if (!(await page.evaluate(t => document.body.textContent.includes(t), text)))
      return
    if (Date.now() > deadline)
      throw new Error(`waitForTextGone 超时(${timeoutMs}ms): 文本仍在: ${text}`)
    await new Promise(r => setTimeout(r, 200))
  }
}
