import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { resolvePaths, __setAppRootForTest } from '@main/paths'
import { existsSync, mkdirSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('paths', () => {
  let testRoot: string

  beforeEach(() => {
    testRoot = join(tmpdir(), `nginx-gui-test-${Date.now()}`)
    mkdirSync(testRoot, { recursive: true })
    __setAppRootForTest(testRoot)
  })

  afterEach(() => {
    rmSync(testRoot, { recursive: true, force: true })
  })

  it('dataDir 位于 appRoot/data', () => {
    const p = resolvePaths()
    expect(p.dataDir).toBe(join(testRoot, 'data'))
  })

  it('dbPath 位于 dataDir/data.db', () => {
    const p = resolvePaths()
    expect(p.dbPath).toBe(join(testRoot, 'data', 'data.db'))
  })

  it('nginxConfDir 位于 dataDir/nginx-conf', () => {
    const p = resolvePaths()
    expect(p.nginxConfDir).toBe(join(testRoot, 'data', 'nginx-conf'))
  })

  it('nginxLogDir 位于 dataDir/logs', () => {
    const p = resolvePaths()
    expect(p.nginxLogDir).toBe(join(testRoot, 'data', 'logs'))
  })

  it('ensureDataDirs 创建所有必要目录', () => {
    const { dataDir, ensureDataDirs } = resolvePaths()
    ensureDataDirs()
    expect(existsSync(dataDir)).toBe(true)
    expect(existsSync(join(dataDir, 'nginx-conf'))).toBe(true)
    expect(existsSync(join(dataDir, 'nginx-conf', 'http'))).toBe(true)
    expect(existsSync(join(dataDir, 'nginx-conf', 'stream'))).toBe(true)
    expect(existsSync(join(dataDir, 'logs'))).toBe(true)
    expect(existsSync(join(dataDir, 'snapshots'))).toBe(true)
    expect(existsSync(join(dataDir, 'temp'))).toBe(true)
  })
})
