import type { NginxEngine } from './types'

let current: NginxEngine | null = null
// 活动 engine 对应的 hostId。ENGINE_RELOAD 用它（而非 getDefaultHost）取 mappings，
// 避免切到非默认主机后把默认主机的规则写到当前主机（#1 数据错配 bug）。
let currentHostId: string | null = null

// 待应用改动标记（内存态，不入库）。改 mapping/导入数据 → 置脏；reload 成功 → 清脏。
// 用内存变量而非数据库列：healthCheck 定时器直接写 mappingRepo 绕过 IPC，
// 若 dirty 落库会被健康检查误触发。
let dirty = false

export function setActiveEngine(eng: NginxEngine | null, hostId?: string): void {
  // 切换 engine 时，关闭旧 engine 的资源（SSH 连接等）
  if (current?.close) {
    try { current.close() } catch { /* 忽略关闭错误 */ }
  }
  current = eng
  currentHostId = eng ? (hostId ?? null) : null
  // 切换主机后旧 dirty 无意义，重置为干净
  dirty = false
}

export function getActiveEngine(): NginxEngine | null {
  return current
}

/** 活动 engine 对应的 hostId（用于取 mappings，避免用错默认 host） */
export function getActiveHostId(): string | null {
  return currentHostId
}

export function isEngineDirty(): boolean {
  return dirty
}

export function setEngineDirty(v: boolean): void {
  dirty = v
}

/** 刷新 active engine 的状态（remote 用 SSH 探测，local 是 no-op 返回当前 status） */
export async function refreshActiveStatus(): Promise<void> {
  const eng = current
  if (!eng) return
  await eng.refreshStatus()
}
