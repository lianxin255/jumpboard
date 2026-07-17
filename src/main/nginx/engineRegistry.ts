import type { NginxEngine } from './types'

let current: NginxEngine | null = null

export function setActiveEngine(eng: NginxEngine | null): void {
  current = eng
}

export function getActiveEngine(): NginxEngine | null {
  return current
}
