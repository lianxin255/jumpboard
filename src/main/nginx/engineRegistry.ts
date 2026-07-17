import type { NginxEngine } from './types'

let current: NginxEngine | null = null

export function setActiveEngine(eng: NginxEngine | null): void {
  // 切换 engine 时，关闭旧 engine 的资源（SSH 连接等）
  if (current?.close) {
    try { current.close() } catch { /* 忽略关闭错误 */ }
  }
  current = eng
}

export function getActiveEngine(): NginxEngine | null {
  return current
}

/** 刷新 active engine 的状态（remote 用 SSH 探测，local 是 no-op） */
export async function refreshActiveStatus(): Promise<void> {
  const eng = current
  if (!eng) return
  // RemoteNginxEngine 有 refreshStatus，LocalNginxEngine 没有
  const anyEng = eng as any
  if (typeof anyEng.refreshStatus === 'function') {
    await anyEng.refreshStatus()
  }
}
