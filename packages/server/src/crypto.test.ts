import { describe, expect, it } from 'vitest'
import { decryptPassword, encryptPassword, isEncryptionAvailable } from './crypto'

describe('crypto', () => {
  it('isEncryptionAvailable 在测试环境返回布尔值', () => {
    expect(typeof isEncryptionAvailable()).toBe('boolean')
  })

  it('encryptPassword 返回字符串', () => {
    const enc = encryptPassword('my-secret')
    expect(typeof enc).toBe('string')
    expect(enc.length).toBeGreaterThan(0)
  })

  it('decryptPassword(encryptPassword(x)) === x（往返一致）', () => {
    const original = 'my-secret-password-123'
    const encrypted = encryptPassword(original)
    const decrypted = decryptPassword(encrypted)
    expect(decrypted).toBe(original)
  })

  it('不同明文加密结果不同', () => {
    expect(encryptPassword('a')).not.toBe(encryptPassword('b'))
  })

  it('空字符串也能处理', () => {
    expect(decryptPassword(encryptPassword(''))).toBe('')
  })
})
