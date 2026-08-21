import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'

/**
 * 密码加密封装。
 * 生产环境（Electron）用 safeStorage（系统钥匙串）；
 * 测试/开发环境（无 Electron）用 AES-256-GCM fallback。
 * fallback 的 key 是进程级随机，重启失效——仅测试可用，真正安全靠生产 safeStorage。
 *
 * 格式：
 * - 'ss1:' + base64(safeStorage.encryptString) — 生产
 * - 'fb1:' + base64(iv[12] + tag[16] + ciphertext) — fallback
 */

const FALLBACK_KEY = randomBytes(32)

function trySafeStorage(): any | null {
  try {
    const electron = require('electron')
    if (electron?.safeStorage && electron.safeStorage.isEncryptionAvailable())
      return electron.safeStorage
  }
  catch {
    // 无 electron
  }
  return null
}

export function isEncryptionAvailable(): boolean {
  return trySafeStorage() !== null
}

export function encryptPassword(plain: string): string {
  const safe = trySafeStorage()
  if (safe) {
    const buf = safe.encryptString(plain)
    return `ss1:${buf.toString('base64')}`
  }
  const iv = randomBytes(12)
  const cipher = createCipheriv('aes-256-gcm', FALLBACK_KEY, iv)
  const enc = Buffer.concat([cipher.update(plain, 'utf8'), cipher.final()])
  const tag = cipher.getAuthTag()
  return `fb1:${Buffer.concat([iv, tag, enc]).toString('base64')}`
}

export function decryptPassword(stored: string): string {
  const safe = trySafeStorage()
  if (stored.startsWith('ss1:')) {
    if (!safe)
      throw new Error('密码用 safeStorage 加密，但当前环境不可用')
    const buf = Buffer.from(stored.slice(4), 'base64')
    return safe.decryptString(buf)
  }
  if (stored.startsWith('fb1:')) {
    const raw = Buffer.from(stored.slice(4), 'base64')
    const iv = raw.subarray(0, 12)
    const tag = raw.subarray(12, 28)
    const enc = raw.subarray(28)
    const decipher = createDecipheriv('aes-256-gcm', FALLBACK_KEY, iv)
    decipher.setAuthTag(tag)
    return Buffer.concat([decipher.update(enc), decipher.final()]).toString('utf8')
  }
  throw new Error('无法识别的密码加密格式')
}
