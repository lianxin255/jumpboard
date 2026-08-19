import { createCipheriv, createDecipheriv, randomBytes, scryptSync } from 'node:crypto'

/**
 * 密码加密封装。
 *
 * Tauri sidecar 无 safeStorage（Electron 专有），改用 AES-256-GCM + 派生密钥。
 * 密钥从 NGJ_SECRET 环境变量派生（Tauri 壳启动时注入，每次启动随机生成）。
 * 无 NGJ_SECRET 时用进程级随机密钥（重启失效——仅开发/测试可用）。
 *
 * 格式：
 * - 'fb1:' + base64(iv[12] + tag[16] + ciphertext)  —— AES-256-GCM
 * - 'ss1:' 旧 safeStorage 格式保留兼容（解密时识别但不再产生）
 */

function deriveKey(): Buffer {
  const secret = process.env.NGJ_SECRET
  if (secret) {
    return scryptSync(secret, 'nginx-jump-gui-salt', 32)
  }
  // 进程级随机密钥（重启失效，仅开发/测试）
  return FALLBACK_KEY
}

const FALLBACK_KEY = randomBytes(32)

export function isEncryptionAvailable(): boolean {
  return true
}

export function encryptPassword(plain: string): string {
  const key = deriveKey()
  const iv = randomBytes(12)
  const cipher = createCipheriv('aes-256-gcm', key, iv)
  const enc = Buffer.concat([cipher.update(plain, 'utf8'), cipher.final()])
  const tag = cipher.getAuthTag()
  return `fb1:${Buffer.concat([iv, tag, enc]).toString('base64')}`
}

export function decryptPassword(stored: string): string {
  if (stored.startsWith('ss1:')) {
    throw new Error('密码用 safeStorage 加密（Electron 旧格式），Tauri 环境不可用，请重新输入密码')
  }
  if (stored.startsWith('fb1:')) {
    const key = deriveKey()
    const raw = Buffer.from(stored.slice(4), 'base64')
    const iv = raw.subarray(0, 12)
    const tag = raw.subarray(12, 28)
    const enc = raw.subarray(28)
    const decipher = createDecipheriv('aes-256-gcm', key, iv)
    decipher.setAuthTag(tag)
    return Buffer.concat([decipher.update(enc), decipher.final()]).toString('utf8')
  }
  throw new Error('无法识别的密码加密格式')
}
