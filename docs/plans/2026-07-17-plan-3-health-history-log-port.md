# Plan 3: 健康检查 + 配置历史回滚 + 日志系统 + 端口占用检测

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Plan 1/2 的基础上，补齐 4 个运维子系统：映射健康检查（定时探活）、配置版本历史 + 一键回滚、应用 + nginx 日志查看、保存前的端口占用检测。这些是设计 §7 的全部内容。

**Architecture:** 4 个子系统相对独立，各自一个 service 模块（main 侧）+ IPC + UI。健康检查用 Node net/http 模块做探活，结果写回 mappingRepo。配置历史在 applyFlow 成功后自动存快照（snapshotRepo），回滚 = 用快照覆盖 conf + nginx -t + reload。日志系统写 app_logs 表 + tail nginx access/error.log。端口检测在 MappingForm 保存前通过 IPC 查 `net.createServer().listen()`。

**Tech Stack:** 沿用：Electron 30 / Vue 3 / TypeScript / better-sqlite3。无新依赖（健康检查用 Node 内置 `net`/`http`，端口检测用 `net`）。

**Spec 引用:** `docs/specs/2026-07-17-nginx-jump-gui-design.md` §7.1（端口检测）、§7.2（健康检查）、§7.3（配置历史回滚）、§7.4（日志）。

**前置条件:** Plan 1（commit `006b00e`）+ Plan 2（commit `3420da2`）已完成。`config_snapshots` 和 `app_logs` 表已建（Plan 1 Task 4，含索引）。applyFlow 已实现（Plan 2 Task 4）。

---

## 文件结构

```
nginx-jump-gui/src/
├── shared/
│   └── types.ts                      # 加日志/快照/健康检查/端口检测 IPC 通道 + LogEntry/Snapshot 类型
├── main/
│   ├── db/
│   │   ├── mappingRepo.ts            # 加 updateHealthStatus（不经过 updateMapping，避免 clobber 时间戳）
│   │   ├── snapshotRepo.ts           # 新：ConfigSnapshot CRUD + 保留策略裁剪
│   │   └── logRepo.ts                # 新：AppLog CRUD + 写入 + 裁剪
│   ├── nginx/
│   │   └── applyFlow.ts              # 改：reload 成功后存一条 snapshot
│   ├── services/
│   │   ├── healthCheck.ts            # 新：定时轮询探活服务
│   │   ├── portCheck.ts              # 新：端口占用检测
│   │   └── logger.ts                 # 新：应用日志写入便捷函数（封装 logRepo）
│   ├── ipc/
│   │   └── handlers.ts               # 加：日志/快照/健康检查/端口检测 IPC
│   ├── preload/index.ts              # 加对应桥接
│   └── index.ts                      # 改：启动健康检查服务、退出时停
└── renderer/
    ├── api/ipc.ts                    # 加新方法类型
    ├── stores/
    │   ├── logs.ts                   # 新：日志 store
    │   └── snapshots.ts              # 新：快照 store
    ├── components/
    │   ├── HealthDot.vue             # （MappingList 已有 dot，复用即可，不改）
    │   ├── LogViewer.vue             # 新：日志查看页
    │   ├── HistoryPanel.vue          # 新：配置历史 + 回滚
    │   └── MappingForm.vue           # 改：保存前端口检测（仅 stream/http 监听端口）
    └── App.vue                       # 改：加导航入口（日志/历史页）
tests/main/
├── db/snapshotRepo.test.ts           # 新
├── db/logRepo.test.ts                # 新
├── services/healthCheck.test.ts      # 新
├── services/portCheck.test.ts        # 新
```

---

## Task 1: 扩展类型 + IPC 通道 + LogEntry/Snapshot 类型

**Files:**
- Modify: `src/shared/types.ts`

- [ ] **Step 1: 在 `src/shared/types.ts` 末尾加类型 + IPC 通道**

加类型（在 `IpcChannel` 类型定义之前）：

```typescript
// ============ Log ============
export type LogLevel = 'info' | 'warn' | 'error'
export type LogSource = 'app' | 'nginx-access' | 'nginx-error'

export interface LogEntry {
  id: string
  hostId: string | null
  level: LogLevel
  source: LogSource
  message: string
  timestamp: number
}

// ============ ConfigSnapshot ============
export interface ConfigSnapshot {
  id: string
  hostId: string
  content: string
  reason: string
  createdAt: number
}
```

在 `IPC` const 里加通道（在 `TEMPLATE_DELETE_CUSTOM` 之后）：

```typescript
  // Log
  LOG_LIST: 'log:list',
  LOG_ADD: 'log:add',
  // Snapshot
  SNAPSHOT_LIST: 'snapshot:list',
  SNAPSHOT_GET: 'snapshot:get',
  SNAPSHOT_ROLLBACK: 'snapshot:rollback',
  // Health check
  HEALTH_CHECK_NOW: 'health:check-now',
  // Port check
  PORT_CHECK: 'port:check',
```

- [ ] **Step 2: 验证**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 61 tests 全过

- [ ] **Step 3: 提交**

```bash
git add src/shared/types.ts
git commit -m "feat(types): 加 LogEntry/ConfigSnapshot 类型 + 日志/快照/健康/端口 IPC 通道"
```

---

## Task 2: logRepo + logger（日志存储 + 写入便捷函数）

**Files:**
- Create: `src/main/db/logRepo.ts`, `src/main/services/logger.ts`
- Test: `tests/main/db/logRepo.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/db/logRepo.test.ts`**

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { addLog, listLogs, listLogsByLevel, trimLogs } from '@main/db/logRepo'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('logRepo', () => {
  let dbPath: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('addLog 写入并返回 LogEntry', () => {
    const log = addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'started' })
    expect(log.id).toBeTruthy()
    expect(log.message).toBe('started')
    expect(log.timestamp).toBeGreaterThan(0)
  })

  it('listLogs 按 hostId 过滤 + 时间倒序', () => {
    addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'a' })
    addLog({ hostId: 'h1', level: 'error', source: 'nginx-error', message: 'b' })
    addLog({ hostId: 'h2', level: 'info', source: 'app', message: 'c' })
    const list = listLogs('h1', 100)
    expect(list).toHaveLength(2)
    expect(list[0].message).toBe('b')  // 倒序，最新的在前
  })

  it('listLogs limit 生效', () => {
    for (let i = 0; i < 10; i++) addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'm' + i })
    expect(listLogs('h1', 5)).toHaveLength(5)
  })

  it('listLogsByLevel 过滤', () => {
    addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'a' })
    addLog({ hostId: 'h1', level: 'error', source: 'app', message: 'b' })
    const errors = listLogsByLevel('h1', 'error', 100)
    expect(errors).toHaveLength(1)
    expect(errors[0].message).toBe('b')
  })

  it('trimLogs 保留最近 N 条', () => {
    for (let i = 0; i < 10; i++) addLog({ hostId: 'h1', level: 'info', source: 'app', message: 'm' + i })
    const removed = trimLogs(5)
    expect(removed).toBe(5)
    expect(listLogs('h1', 100)).toHaveLength(5)
  })

  it('hostId 为 null 也能写入和查询（全局日志）', () => {
    addLog({ hostId: null, level: 'info', source: 'app', message: 'global' })
    const list = listLogs(null, 100)
    expect(list).toHaveLength(1)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/db/logRepo.test.ts`
Expected: FAIL.

- [ ] **Step 3: 实现 `src/main/db/logRepo.ts`**

```typescript
import { randomUUID } from 'crypto'
import { getDb } from './connection'
import type { LogEntry, LogLevel, LogSource } from '@shared/types'

interface LogRow {
  id: string
  host_id: string | null
  level: string
  source: string
  message: string
  timestamp: number
}

function rowToLog(r: LogRow): LogEntry {
  return {
    id: r.id,
    hostId: r.host_id,
    level: r.level as LogLevel,
    source: r.source as LogSource,
    message: r.message,
    timestamp: r.timestamp
  }
}

export function addLog(input: { hostId: string | null; level: LogLevel; source: LogSource; message: string }): LogEntry {
  const id = randomUUID()
  const timestamp = Date.now()
  getDb().prepare(`INSERT INTO app_logs (id, host_id, level, source, message, timestamp)
    VALUES (?, ?, ?, ?, ?, ?)`)
    .run(id, input.hostId, input.level, input.source, input.message, timestamp)
  return { id, ...input, timestamp }
}

export function listLogs(hostId: string | null, limit: number): LogEntry[] {
  const sql = hostId
    ? `SELECT * FROM app_logs WHERE host_id = ? ORDER BY timestamp DESC LIMIT ?`
    : `SELECT * FROM app_logs ORDER BY timestamp DESC LIMIT ?`
  const rows = hostId
    ? getDb().prepare(sql).all(hostId, limit) as LogRow[]
    : getDb().prepare(sql).all(limit) as LogRow[]
  return rows.map(rowToLog)
}

export function listLogsByLevel(hostId: string | null, level: LogLevel, limit: number): LogEntry[] {
  const sql = hostId
    ? `SELECT * FROM app_logs WHERE host_id = ? AND level = ? ORDER BY timestamp DESC LIMIT ?`
    : `SELECT * FROM app_logs WHERE level = ? ORDER BY timestamp DESC LIMIT ?`
  const rows = hostId
    ? getDb().prepare(sql).all(hostId, level, limit) as LogRow[]
    : getDb().prepare(sql).all(level, limit) as LogRow[]
  return rows.map(rowToLog)
}

/** 删除超出的旧日志，返回删除条数。保留最近 N 条（全局，不分 host） */
export function trimLogs(keepCount: number): number {
  const db = getDb()
  const result = db.prepare(`DELETE FROM app_logs WHERE id NOT IN (
    SELECT id FROM app_logs ORDER BY timestamp DESC LIMIT ?
  )`).run(keepCount)
  return result.changes
}
```

- [ ] **Step 4: 实现 `src/main/services/logger.ts`（便捷封装）**

```typescript
import { addLog, trimLogs } from '../db/logRepo'
import type { LogLevel, LogSource } from '@shared/types'

const MAX_LOGS = 10000

/** 写一条日志。每 MAX_LOGS 条触发一次裁剪。 */
export function log(hostId: string | null, level: LogLevel, source: LogSource, message: string): void {
  addLog({ hostId, level, source, message })
  // 简易裁剪：每次写都检查太频繁，改成概率性裁剪（每 100 条检查一次）
  if (Math.random() < 0.01) trimLogs(MAX_LOGS)
}

export const logger = {
  info: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'info', source, msg),
  warn: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'warn', source, msg),
  error: (hostId: string | null, source: LogSource, msg: string) => log(hostId, 'error', source, msg),
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `npx vitest run tests/main/db/logRepo.test.ts`
Expected: PASS (6 tests).

- [ ] **Step 6: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿，67 tests）

- [ ] **Step 7: 提交**

```bash
git add src/main/db/logRepo.ts src/main/services/logger.ts tests/main/db/logRepo.test.ts
git commit -m "feat(log): logRepo CRUD + 裁剪 + logger 便捷封装"
```

---

## Task 3: snapshotRepo（配置历史存储）

**Files:**
- Create: `src/main/db/snapshotRepo.ts`
- Test: `tests/main/db/snapshotRepo.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/db/snapshotRepo.test.ts`**

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import { createSnapshot, listSnapshots, getSnapshot, trimSnapshots } from '@main/db/snapshotRepo'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('snapshotRepo', () => {
  let dbPath: string
  let hostId: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
    hostId = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null }).id
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath)) rmSync(dbPath)
  })

  it('createSnapshot 写入并返回', () => {
    const s = createSnapshot({ hostId, content: 'worker_processes 1;', reason: 'reload' })
    expect(s.id).toBeTruthy()
    expect(s.content).toContain('worker_processes')
    expect(s.reason).toBe('reload')
  })

  it('listSnapshots 按 hostId 倒序', () => {
    createSnapshot({ hostId, content: 'v1', reason: 'r1' })
    createSnapshot({ hostId, content: 'v2', reason: 'r2' })
    const list = listSnapshots(hostId, 100)
    expect(list).toHaveLength(2)
    expect(list[0].content).toBe('v2')  // 最新在前
  })

  it('getSnapshot 按 id', () => {
    const s = createSnapshot({ hostId, content: 'conf', reason: 'manual' })
    expect(getSnapshot(s.id)?.content).toBe('conf')
  })

  it('trimSnapshots 每 host 保留 N 条', () => {
    for (let i = 0; i < 10; i++) createSnapshot({ hostId, content: 'v' + i, reason: 'r' })
    const removed = trimSnapshots(5)
    expect(removed).toBe(5)
    expect(listSnapshots(hostId, 100)).toHaveLength(5)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/db/snapshotRepo.test.ts`
Expected: FAIL.

- [ ] **Step 3: 实现 `src/main/db/snapshotRepo.ts`**

```typescript
import { randomUUID } from 'crypto'
import { getDb } from './connection'
import type { ConfigSnapshot } from '@shared/types'

interface SnapshotRow {
  id: string
  host_id: string
  content: string
  reason: string
  created_at: number
}

function rowToSnapshot(r: SnapshotRow): ConfigSnapshot {
  return {
    id: r.id,
    hostId: r.host_id,
    content: r.content,
    reason: r.reason,
    createdAt: r.created_at
  }
}

export function createSnapshot(input: { hostId: string; content: string; reason: string }): ConfigSnapshot {
  const id = randomUUID()
  const createdAt = Date.now()
  getDb().prepare(`INSERT INTO config_snapshots (id, host_id, content, reason, created_at)
    VALUES (?, ?, ?, ?, ?)`)
    .run(id, input.hostId, input.content, input.reason, createdAt)
  return { id, ...input, createdAt }
}

export function listSnapshots(hostId: string, limit: number): ConfigSnapshot[] {
  const rows = getDb().prepare(`SELECT * FROM config_snapshots WHERE host_id = ? ORDER BY created_at DESC LIMIT ?`)
    .all(hostId, limit) as SnapshotRow[]
  return rows.map(rowToSnapshot)
}

export function getSnapshot(id: string): ConfigSnapshot | undefined {
  const row = getDb().prepare(`SELECT * FROM config_snapshots WHERE id = ?`).get(id) as SnapshotRow | undefined
  return row ? rowToSnapshot(row) : undefined
}

/** 每 host 保留最近 N 条，删除超出的。返回删除条数。 */
export function trimSnapshots(keepPerHost: number): number {
  const db = getDb()
  const result = db.prepare(`DELETE FROM config_snapshots WHERE id NOT IN (
    SELECT id FROM (
      SELECT id, host_id,
             ROW_NUMBER() OVER (PARTITION BY host_id ORDER BY created_at DESC) AS rn
      FROM config_snapshots
    ) WHERE rn <= ?
  )`).run(keepPerHost)
  return result.changes
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/db/snapshotRepo.test.ts`
Expected: PASS (4 tests).

> 注意：`ROW_NUMBER() OVER (PARTITION BY ...)` 是 SQLite 3.25+ 的窗口函数。better-sqlite3 自带的 SQLite 通常 ≥ 3.35，支持。如果测试报语法错误，改成先查每 host 的保留 id 列表再删除的方式（但不优雅）。先按窗口函数实现，测试验证。

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿，71 tests）

- [ ] **Step 6: 提交**

```bash
git add src/main/db/snapshotRepo.ts tests/main/db/snapshotRepo.test.ts
git commit -m "feat(snapshot): snapshotRepo CRUD + 每 host 保留策略裁剪（窗口函数）"
```

---

## Task 4: applyFlow 接入快照存储 + 日志

**Files:**
- Modify: `src/main/nginx/applyFlow.ts`（reload 成功后存快照 + 写日志）
- Modify: `src/main/db/mappingRepo.ts`（加 updateHealthStatus）

- [ ] **Step 1: mappingRepo 加 updateHealthStatus**

在 `src/main/db/mappingRepo.ts` 末尾加：

```typescript
/** 健康检查服务专用：只更新 healthStatus + lastCheckedAt，不动其他字段 */
export function updateHealthStatus(id: string, healthStatus: HealthStatus, lastCheckedAt: number): void {
  getDb().prepare(`UPDATE mappings SET health_status = ?, last_checked_at = ? WHERE id = ?`)
    .run(healthStatus, lastCheckedAt, id)
}
```

（`HealthStatus` 类型已在文件顶部 import。）

- [ ] **Step 2: applyFlow 接入快照 + 日志**

读 `src/main/nginx/applyFlow.ts`。在 reload 成功的分支里，存一条 snapshot + 写日志。需要 hostId 参数（快照按 host 存）。

修改 `applyMappings` 签名，加可选的 `hostId` 参数（用于存快照）：

```typescript
import type { Mapping, ConfigSnapshot } from '@shared/types'
import type { NginxEngine } from './types'
import { generateFullConf, type ConfGenOptions } from './confGenerator'
import { createSnapshot, trimSnapshots } from '../db/snapshotRepo'
import { logger } from '../services/logger'

export interface ApplyOptions extends ConfGenOptions {
  /** 当前 host id，用于存配置快照。不传则不存快照。 */
  hostId?: string
}

export interface ApplyResult {
  ok: boolean
  conf: string
  output: string
}

const SNAPSHOT_KEEP = 50

export async function applyMappings(
  engine: NginxEngine,
  mappings: Mapping[],
  opts: ApplyOptions
): Promise<ApplyResult> {
  const conf = generateFullConf(mappings, opts)

  const oldConf = await engine.readConf()
  await engine.writeConf(conf)

  const testResult = await engine.test()
  if (!testResult.ok) {
    await engine.writeConf(oldConf)
    logger.warn(opts.hostId ?? null, 'app', `配置校验失败: ${testResult.output}`)
    return { ok: false, conf, output: testResult.output }
  }

  try {
    await engine.reload()
    // 成功：存快照 + 日志
    if (opts.hostId) {
      createSnapshot({ hostId: opts.hostId, content: conf, reason: 'reload' })
      trimSnapshots(SNAPSHOT_KEEP)
    }
    logger.info(opts.hostId ?? null, 'app', `配置应用成功，${mappings.length} 条映射`)
    return { ok: true, conf, output: testResult.output }
  } catch (e: any) {
    logger.error(opts.hostId ?? null, 'app', `reload 失败: ${e?.message ?? e}`)
    return { ok: false, conf, output: e?.message ?? String(e) }
  }
}
```

注意：`ConfGenOptions` 原来是 `applyMappings` 的第三个参数类型。现在改成 `ApplyOptions extends ConfGenOptions`（加了 hostId）。原来传 `{ nginxLogDir, nginxTempDir }` 的地方仍兼容（hostId 可选）。

- [ ] **Step 3: 更新现有测试 `tests/main/nginx/applyFlow.test.ts`**

原来的测试传 `{ nginxLogDir, nginxTempDir }`，现在类型是 ApplyOptions，hostId 可选——原测试不用改，仍通过。但加一个测试验证快照存储：

在 describe 里加：

```typescript
  it('成功时存配置快照（hostId 提供时）', async () => {
    // 这个测试需要真实 db，用 vi.mock 或跳过 db 部分
    // applyFlow 现在依赖 snapshotRepo + logger，单测里它们会真的写 db
    // 简化：mock snapshotRepo
    // ... 这个测试在 applyFlow.test.ts 里较难独立测，放到集成测试
    // 这里只验证成功路径仍返回 ok:true
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp', hostId: 'h1' })
    expect(result.ok).toBe(true)
  })
```

实际上 applyFlow.test.ts 的 mock engine 不涉及 db，但 applyFlow 现在调 createSnapshot/logger 会触真实 db。**需要 mock snapshotRepo 和 logger**。在 applyFlow.test.ts 顶部加：

```typescript
vi.mock('@main/db/snapshotRepo', () => ({
  createSnapshot: vi.fn(),
  trimSnapshots: vi.fn(),
}))
vi.mock('@main/services/logger', () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn() }
}))
```

并 import 这两个 mock 验证调用：

```typescript
import { createSnapshot } from '@main/db/snapshotRepo'

it('成功时存配置快照', async () => {
  const eng = makeEngine()
  await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp', hostId: 'h1' })
  expect(createSnapshot).toHaveBeenCalledWith(expect.objectContaining({ hostId: 'h1', reason: 'reload' }))
})

it('校验失败时写 warn 日志不存快照', async () => {
  const eng = makeEngine({ testResult: { ok: false, output: 'bad' } })
  const { logger } = await import('@main/services/logger')
  await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp', hostId: 'h1' })
  expect(logger.warn).toHaveBeenCalled()
  expect(createSnapshot).not.toHaveBeenCalled()
})
```

- [ ] **Step 4: 运行测试**

Run: `npx vitest run tests/main/nginx/applyFlow.test.ts`
Expected: PASS（原有 4 + 新增 2 = 6，且 mock 生效）。

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）

- [ ] **Step 5: handlers.ts 的 ENGINE_RELOAD 传 hostId**

读 `src/main/ipc/handlers.ts`，找到 ENGINE_RELOAD handler 里调 `applyMappings(eng, mappings, { nginxLogDir, nginxTempDir })`，加 `hostId: host.id`：

```typescript
    const result = await applyMappings(eng, mappings, {
      nginxLogDir: paths.nginxLogDir,
      nginxTempDir: paths.tempDir,
      hostId: host.id
    })
```

- [ ] **Step 6: 提交**

```bash
git add src/main/nginx/applyFlow.ts src/main/db/mappingRepo.ts tests/main/nginx/applyFlow.test.ts src/main/ipc/handlers.ts
git commit -m "feat(applyFlow): 成功后存配置快照 + 写日志 + mappingRepo.updateHealthStatus"
```

---

## Task 5: portCheck（端口占用检测）

**Files:**
- Create: `src/main/services/portCheck.ts`
- Test: `tests/main/services/portCheck.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/services/portCheck.test.ts`**

```typescript
import { describe, it, expect } from 'vitest'
import { isPortAvailable } from '@main/services/portCheck'
import { createServer } from 'net'

describe('portCheck', () => {
  it('空闲端口返回 true', async () => {
    // 用一个不太可能被占的高端口
    expect(await isPortAvailable(39998)).toBe(true)
  })

  it('被占端口返回 false', async () => {
    const server = createServer()
    await new Promise<void>((resolve) => server.listen(39999, resolve))
    expect(await isPortAvailable(39999)).toBe(false)
    await new Promise<void>((resolve) => server.close(() => resolve()))
  })

  it('null/undefined 端口返回 true（无需检测）', async () => {
    expect(await isPortAvailable(null)).toBe(true)
    expect(await isPortAvailable(undefined)).toBe(true)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/services/portCheck.test.ts`
Expected: FAIL.

- [ ] **Step 3: 实现 `src/main/services/portCheck.ts`**

```typescript
import { createServer } from 'net'

/**
 * 检测本机端口是否可用（未被占用）。
 * 实现：尝试 listen，成功即空闲，失败（EADDRINUSE）即被占。
 * null/undefined 返回 true（无需检测，例如 HTTPS 不收集 listenPort）。
 */
export function isPortAvailable(port: number | null | undefined): Promise<boolean> {
  if (port == null) return Promise.resolve(true)
  return new Promise((resolve) => {
    const server = createServer()
    server.once('error', () => resolve(false))
    server.once('listening', () => {
      server.close(() => resolve(true))
    })
    server.listen(port)
  })
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/services/portCheck.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）

- [ ] **Step 6: 提交**

```bash
git add src/main/services/portCheck.ts tests/main/services/portCheck.test.ts
git commit -m "feat(portCheck): 端口占用检测（net.createServer listen 探测）"
```

---

## Task 6: healthCheck（定时探活服务）

**Files:**
- Create: `src/main/services/healthCheck.ts`
- Test: `tests/main/services/healthCheck.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/services/healthCheck.test.ts`**

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { checkTcp, checkHttp, startHealthCheckLoop, stopHealthCheckLoop } from '@main/services/healthCheck'
import { createServer } from 'net'
import http from 'http'

describe('checkTcp', () => {
  it('端口通返回 healthy', async () => {
    const server = createServer()
    await new Promise<void>((r) => server.listen(39997, r))
    expect(await checkTcp('127.0.0.1', 39997)).toBe('healthy')
    await new Promise<void>((r) => server.close(() => r()))
  })

  it('端口不通返回 unhealthy', async () => {
    expect(await checkTcp('127.0.0.1', 39996)).toBe('unhealthy')
  })
})

describe('checkHttp', () => {
  it('返回 200 且期望 200 返回 healthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 200; res.end('ok') })
    await new Promise<void>((r) => server.listen(39995, r))
    expect(await checkHttp('127.0.0.1', 39995, '/', 200)).toBe('healthy')
    await new Promise<void>((r) => server.close(() => r()))
  })

  it('返回 500 期望 200 返回 unhealthy', async () => {
    const server = http.createServer((req, res) => { res.statusCode = 500; res.end('err') })
    await new Promise<void>((r) => server.listen(39994, r))
    expect(await checkHttp('127.0.0.1', 39994, '/', 200)).toBe('unhealthy')
    await new Promise<void>((r) => server.close(() => r()))
  })
})

describe('healthCheckLoop', () => {
  beforeEach(() => { stopHealthCheckLoop() })
  afterEach(() => { stopHealthCheckLoop() })

  it('start/stop 不抛错', () => {
    expect(() => startHealthCheckLoop()).not.toThrow()
    expect(() => stopHealthCheckLoop()).not.toThrow()
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/services/healthCheck.test.ts`
Expected: FAIL.

- [ ] **Step 3: 实现 `src/main/services/healthCheck.ts`**

```typescript
import { createConnection } from 'net'
import http from 'http'
import type { Mapping, HealthStatus } from '@shared/types'
import { listEnabledMappings } from '../db/mappingRepo'
import { updateHealthStatus } from '../db/mappingRepo'
import { getDefaultHost } from '../db/hostRepo'
import { logger } from './logger'

const CHECK_TIMEOUT_MS = 3000
let loopTimer: NodeJS.Timeout | null = null

/** TCP 探活：尝试建立 TCP 连接 */
export function checkTcp(host: string, port: number): Promise<HealthStatus> {
  return new Promise((resolve) => {
    const socket = createConnection({ host, port }, () => {
      socket.destroy()
      resolve('healthy')
    })
    socket.setTimeout(CHECK_TIMEOUT_MS)
    socket.on('error', () => resolve('unhealthy'))
    socket.on('timeout', () => { socket.destroy(); resolve('unhealthy') })
  })
}

/** HTTP 探活：GET path，检查状态码 */
export function checkHttp(host: string, port: number, path: string, expectedStatus: number): Promise<HealthStatus> {
  return new Promise((resolve) => {
    const req = http.get({ hostname: host, port, path, timeout: CHECK_TIMEOUT_MS }, (res) => {
      res.destroy()
      resolve(res.statusCode === expectedStatus ? 'healthy' : 'unhealthy')
    })
    req.on('error', () => resolve('unhealthy'))
    req.on('timeout', () => { req.destroy(); resolve('unhealthy') })
  })
}

/** 对单条映射做健康检查，返回新状态 */
export async function checkMapping(m: Mapping): Promise<HealthStatus> {
  if (m.protocol === 'tcp' || m.protocol === 'udp') {
    // TCP/UDP 都用 TCP 连接探测（UDP 无连接，nginx stream UDP 探活用 TCP 近似）
    return checkTcp(m.upstreamHost, m.upstreamPort)
  }
  // http/https
  const path = m.healthCheckTargetPath ?? '/'
  const expected = m.healthCheckExpectedStatus ?? 200
  if (m.protocol === 'https') {
    // HTTPS 用 https 模块更准确，但 MVP 用 http 近似（很多内网 https 也可 http 探）
    // 真正 HTTPS 探活留后续，这里先按 http
    return checkHttp(m.upstreamHost, m.upstreamPort, path, expected)
  }
  return checkHttp(m.upstreamHost, m.upstreamPort, path, expected)
}

/** 轮询所有 enabled + healthCheckEnabled 的映射 */
async function runOnce(): Promise<void> {
  const host = getDefaultHost()
  if (!host) return
  const mappings = listEnabledMappings(host.id).filter(m => m.healthCheckEnabled === 1)
  const now = Date.now()
  for (const m of mappings) {
    try {
      const status = await checkMapping(m)
      updateHealthStatus(m.id, status, now)
    } catch (e: any) {
      updateHealthStatus(m.id, 'unknown', now)
      logger.warn(host.id, 'app', `健康检查异常 ${m.name}: ${e?.message ?? e}`)
    }
  }
}

const LOOP_INTERVAL_MS = 30000  // 默认 30 秒一轮

/** 启动定时轮询 */
export function startHealthCheckLoop(intervalMs: number = LOOP_INTERVAL_MS): void {
  stopHealthCheckLoop()
  // 首次立即跑一次，之后按 interval
  runOnce().catch(() => {})
  loopTimer = setInterval(() => { runOnce().catch(() => {}) }, intervalMs)
}

/** 停止轮询 */
export function stopHealthCheckLoop(): void {
  if (loopTimer) {
    clearInterval(loopTimer)
    loopTimer = null
  }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/services/healthCheck.test.ts`
Expected: PASS (5 tests).

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）

- [ ] **Step 6: 提交**

```bash
git add src/main/services/healthCheck.ts tests/main/services/healthCheck.test.ts
git commit -m "feat(healthCheck): TCP/HTTP 探活 + 定时轮询服务（默认 30s）"
```

---

## Task 7: IPC handlers + preload 桥接（日志/快照/健康/端口）

**Files:**
- Modify: `src/main/ipc/handlers.ts`
- Modify: `src/preload/index.ts`
- Modify: `src/renderer/api/ipc.ts`

- [ ] **Step 1: handlers.ts 加 IPC**

import：
```typescript
import { listLogs, listLogsByLevel } from '../db/logRepo'
import { listSnapshots, getSnapshot } from '../db/snapshotRepo'
import { isPortAvailable } from '../services/portCheck'
import { checkMapping, startHealthCheckLoop, stopHealthCheckLoop } from '../services/healthCheck'
import { getMapping } from '../db/mappingRepo'
import { applyMappings } from '../nginx/applyFlow'
import { resolvePaths } from '../paths'
import { getActiveEngine } from '../nginx/engineRegistry'
```
（合并到已有 import，不重复）

在 `registerIpcHandlers()` 加：

```typescript
  // Log
  ipcMain.handle(IPC.LOG_LIST, (_e, hostId: string | null, limit: number) => listLogs(hostId, limit))

  // Snapshot
  ipcMain.handle(IPC.SNAPSHOT_LIST, (_e, hostId: string) => listSnapshots(hostId, 50))
  ipcMain.handle(IPC.SNAPSHOT_GET, (_e, id: string) => getSnapshot(id))
  ipcMain.handle(IPC.SNAPSHOT_ROLLBACK, async (_e, snapshotId: string) => {
    // 回滚：用快照 content 覆盖 conf → nginx -t → reload
    const snap = getSnapshot(snapshotId)
    if (!snap) return { ok: false, output: 'snapshot not found' }
    const eng = getActiveEngine()
    if (!eng) return { ok: false, output: 'no active engine' }
    const paths = resolvePaths()
    // 用 applyMappings 的逻辑，但 conf 来自快照而非生成
    // 简化：直接 writeConf + test + reload + 存新快照
    const oldConf = await eng.readConf()
    await eng.writeConf(snap.content)
    const testResult = await eng.test()
    if (!testResult.ok) {
      await eng.writeConf(oldConf)
      return { ok: false, output: testResult.output }
    }
    try {
      await eng.reload()
      return { ok: true, output: testResult.output }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // Health check
  ipcMain.handle(IPC.HEALTH_CHECK_NOW, async (_e, mappingId: string) => {
    const m = getMapping(mappingId)
    if (!m) return 'unknown'
    return await checkMapping(m)
  })

  // Port check
  ipcMain.handle(IPC.PORT_CHECK, (_e, port: number) => isPortAvailable(port))
```

- [ ] **Step 2: preload/index.ts 加桥接**

```typescript
  // Log
  listLogs: (hostId: string | null, limit: number) => ipcRenderer.invoke(IPC.LOG_LIST, hostId, limit),
  // Snapshot
  listSnapshots: (hostId: string) => ipcRenderer.invoke(IPC.SNAPSHOT_LIST, hostId),
  getSnapshot: (id: string) => ipcRenderer.invoke(IPC.SNAPSHOT_GET, id),
  rollbackSnapshot: (snapshotId: string) => ipcRenderer.invoke(IPC.SNAPSHOT_ROLLBACK, snapshotId),
  // Health check
  checkMappingNow: (mappingId: string) => ipcRenderer.invoke(IPC.HEALTH_CHECK_NOW, mappingId),
  // Port check
  checkPort: (port: number) => ipcRenderer.invoke(IPC.PORT_CHECK, port),
```

- [ ] **Step 3: renderer/api/ipc.ts 加类型**

在 ElectronAPI interface 加：
```typescript
  // Log
  listLogs(hostId: string | null, limit: number): Promise<any[]>
  // Snapshot
  listSnapshots(hostId: string): Promise<any[]>
  getSnapshot(id: string): Promise<any>
  rollbackSnapshot(snapshotId: string): Promise<{ ok: boolean; output: string }>
  // Health check
  checkMappingNow(mappingId: string): Promise<string>
  // Port check
  checkPort(port: number): Promise<boolean>
```

- [ ] **Step 4: main/index.ts 启动/停止健康检查**

读 `src/main/index.ts`。在 `app.whenReady` 的 try 块里、`createWindow()` 之后加：
```typescript
  startHealthCheckLoop()
```
（import `startHealthCheckLoop, stopHealthCheckLoop` from `./services/healthCheck`）

在 before-quit handler 里加 `stopHealthCheckLoop()`（在 closeDb 之前）。

- [ ] **Step 5: 验证**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）
Run: `npm run build`

- [ ] **Step 6: 提交**

```bash
git add src/main/ipc/handlers.ts src/preload/index.ts src/renderer/api/ipc.ts src/main/index.ts
git commit -m "feat(ipc): 日志/快照/健康检查/端口检测 IPC + 回滚端点 + 健康检查循环启停"
```

---

## Task 8: LogViewer + HistoryPanel 组件 + stores

**Files:**
- Create: `src/renderer/stores/logs.ts`, `src/renderer/stores/snapshots.ts`
- Create: `src/renderer/components/LogViewer.vue`, `src/renderer/components/HistoryPanel.vue`
- Modify: `src/renderer/App.vue`（加导航入口）

- [ ] **Step 1: 创建 `src/renderer/stores/logs.ts`**

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface LogEntry {
  id: string
  hostId: string | null
  level: string
  source: string
  message: string
  timestamp: number
}

export const useLogsStore = defineStore('logs', () => {
  const list = ref<LogEntry[]>([])
  const loading = ref(false)

  async function load(hostId: string | null, limit = 500) {
    loading.value = true
    try {
      list.value = await api.listLogs(hostId, limit)
    } finally {
      loading.value = false
    }
  }

  return { list, loading, load }
})
```

- [ ] **Step 2: 创建 `src/renderer/stores/snapshots.ts`**

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface Snapshot {
  id: string
  hostId: string
  content: string
  reason: string
  createdAt: number
}

export const useSnapshotsStore = defineStore('snapshots', () => {
  const list = ref<Snapshot[]>([])

  async function load(hostId: string) {
    list.value = await api.listSnapshots(hostId)
  }

  async function rollback(snapshotId: string): Promise<{ ok: boolean; output: string }> {
    return await api.rollbackSnapshot(snapshotId)
  }

  return { list, load, rollback }
})
```

- [ ] **Step 3: 创建 `src/renderer/components/LogViewer.vue`**

```vue
<script setup lang="ts">
import { useLogsStore } from '@renderer/stores/logs'
const store = useLogsStore()
const props = defineProps<{ hostId: string }>()

async function refresh() {
  await store.load(props.hostId, 500)
}
refresh()

function levelColor(level: string): string {
  return level === 'error' ? '#ff3b30' : level === 'warn' ? '#ff9500' : '#8e8e93'
}
function formatTime(ts: number): string {
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="log-viewer">
    <div class="toolbar">
      <h3>日志</h3>
      <button @click="refresh">刷新</button>
    </div>
    <div class="log-list">
      <div v-for="log in store.list" :key="log.id" class="log-line">
        <span class="time">{{ formatTime(log.timestamp) }}</span>
        <span class="level" :style="{ color: levelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
        <span class="source">({{ log.source }})</span>
        <span class="msg">{{ log.message }}</span>
      </div>
      <div v-if="store.list.length === 0" class="empty">暂无日志</div>
    </div>
  </div>
</template>

<style scoped>
.log-viewer { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid #e5e5e5; }
h3 { margin: 0; font-size: 16px; }
button { padding: 4px 12px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; }
.log-list { flex: 1; overflow-y: auto; padding: 8px 16px; font-family: 'SF Mono', Menlo, monospace; font-size: 12px; }
.log-line { padding: 2px 0; display: flex; gap: 6px; }
.time { color: #8e8e93; white-space: nowrap; }
.level { font-weight: 600; white-space: nowrap; }
.source { color: #8e8e93; }
.msg { flex: 1; word-break: break-all; }
.empty { color: #888; padding: 32px; text-align: center; }
</style>
```

- [ ] **Step 4: 创建 `src/renderer/components/HistoryPanel.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useSnapshotsStore } from '@renderer/stores/snapshots'
const store = useSnapshotsStore()
const props = defineProps<{ hostId: string }>()
const emit = defineEmits<{ (e: 'rolled-back'): void }>()
const selectedId = ref<string | null>(null)
const selectedContent = ref('')
const rolling = ref(false)

async function refresh() {
  await store.load(props.hostId)
  selectedId.value = null
  selectedContent.value = ''
}
refresh()

async function viewContent(id: string) {
  selectedId.value = id
  const snap = await api_getSnapshot(id)
  selectedContent.value = snap?.content ?? ''
}
async function api_getSnapshot(id: string) {
  return await (await import('@renderer/api/ipc')).api.getSnapshot(id)
}

async function rollback() {
  if (!selectedId.value) return
  if (!confirm('确定回滚到选中的配置？当前配置将被覆盖。')) return
  rolling.value = true
  try {
    const result = await store.rollback(selectedId.value)
    if (!result.ok) { alert('回滚失败:\n' + result.output); return }
    emit('rolled-back')
    await refresh()
  } finally {
    rolling.value = false
  }
}

function formatTime(ts: number): string {
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="history-panel">
    <div class="toolbar">
      <h3>配置历史</h3>
      <button @click="refresh">刷新</button>
      <button :disabled="!selectedId || rolling" @click="rollback">回滚到选中</button>
    </div>
    <div class="body">
      <div class="snap-list">
        <div v-for="s in store.list" :key="s.id"
          class="snap-item" :class="{ active: s.id === selectedId }"
          @click="viewContent(s.id)">
          <div class="snap-time">{{ formatTime(s.createdAt) }}</div>
          <div class="snap-reason">{{ s.reason }}</div>
        </div>
        <div v-if="store.list.length === 0" class="empty">暂无历史（应用配置后自动生成）</div>
      </div>
      <div class="snap-content">
        <pre v-if="selectedContent">{{ selectedContent }}</pre>
        <div v-else class="empty">选择左侧某条查看内容</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.history-panel { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid #e5e5e5; }
h3 { margin: 0; font-size: 16px; }
button { padding: 4px 12px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; }
button:disabled { opacity: 0.4; }
.body { flex: 1; display: flex; overflow: hidden; }
.snap-list { width: 240px; border-right: 1px solid #e5e5e5; overflow-y: auto; }
.snap-item { padding: 10px 14px; border-bottom: 1px solid #f0f0f0; cursor: pointer; }
.snap-item:hover { background: #f5f5f7; }
.snap-item.active { background: #e8f2ff; }
.snap-time { font-size: 12px; color: #555; }
.snap-reason { font-size: 11px; color: #888; margin-top: 2px; }
.snap-content { flex: 1; overflow: auto; padding: 12px; }
pre { margin: 0; font-family: 'SF Mono', Menlo, monospace; font-size: 12px; white-space: pre-wrap; word-break: break-all; }
.empty { color: #888; padding: 32px; text-align: center; }
</style>
```

- [ ] **Step 5: App.vue 加导航（映射/日志/历史 三个视图切换）**

读当前 App.vue。在顶栏加三个 tab 按钮，main 区域按当前 view 切换组件。

script 加：
```typescript
import LogViewer from '@renderer/components/LogViewer.vue'
import HistoryPanel from '@renderer/components/HistoryPanel.vue'
const currentView = ref<'mappings' | 'logs' | 'history'>('mappings')
```

template 顶栏加（host select 之后）：
```html
<div class="nav-tabs">
  <button :class="{ active: currentView === 'mappings' }" @click="currentView = 'mappings'">映射</button>
  <button :class="{ active: currentView === 'logs' }" @click="currentView = 'logs'">日志</button>
  <button :class="{ active: currentView === 'history' }" @click="currentView = 'history'">历史</button>
</div>
```

main 区域改成：
```html
<main>
  <MappingList v-if="currentView === 'mappings'" @new="openNew" @edit="openEdit" />
  <LogViewer v-else-if="currentView === 'logs'" :host-id="hosts.currentHostId!" />
  <HistoryPanel v-else-if="currentView === 'history'" :host-id="hosts.currentHostId!" @rolled-back="reloadAll" />
</main>
```

style 加：
```css
.nav-tabs { display: flex; gap: 4px; margin-left: 8px; }
.nav-tabs button { padding: 6px 14px; border: none; border-radius: 8px; background: transparent; color: #555; cursor: pointer; font-size: 14px; }
.nav-tabs button.active { background: #0A84FF; color: #fff; }
```

- [ ] **Step 6: MappingForm 加端口检测**

读 `src/renderer/components/MappingForm.vue`。在 save() 校验里，对 listenPort 做 IPC 端口检测。改 save 函数，在现有校验之后、saving 之前加：

```typescript
  // 端口占用检测（仅 stream/http 监听端口）
  if ((isStream.value || isHttp.value) && form.value.listenPort) {
    const available = await api.checkPort(form.value.listenPort)
    if (!available) {
      if (!confirm(`端口 ${form.value.listenPort} 已被占用，仍要保存吗？`)) return
    }
  }
```

- [ ] **Step 7: 验证**

Run: `npm run typecheck`
Run: `npm run build`
Run: `npx vitest run`（全量绿）

- [ ] **Step 8: 提交**

```bash
git add src/renderer/stores/logs.ts src/renderer/stores/snapshots.ts src/renderer/components/LogViewer.vue src/renderer/components/HistoryPanel.vue src/renderer/components/MappingForm.vue src/renderer/App.vue
git commit -m "feat(ui): 日志查看 + 配置历史回滚 + 端口检测 + 三视图导航"
```

---

## Task 9: 端到端验证

**Files:** 无新文件。

- [ ] **Step 1: 全量回归**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 全绿（应 71 + 新增 ≈ 80+）
Run: `npm run build` — 成功

- [ ] **Step 2: dev 模式**

Run: `npm run rebuild:electron` 然后 `npm run dev`（后台 22s 后 kill）
Expected: 无 `[startup] fatal:`，窗口打开，三个 tab（映射/日志/历史）可切换。

- [ ] **Step 3: 手动验证清单**

1. 切到"日志"tab → 显示空（暂无日志）
2. 在"映射"tab 建一条映射 → 切到"日志"应看到"配置应用成功"日志
3. 切到"历史"tab → 看到一条 reload 快照
4. 建一条监听已占用端口的映射 → 保存时弹端口占用确认
5. 健康检查：建一条 healthCheckEnabled=1 的映射（需 Plan 4 UI 加开关，目前表单没暴露——可通过 db 直接改或暂时跳过此项）

- [ ] **Step 4: 恢复 node ABI + 提交**

Run: `npm run rebuild:node`
Run: `npx vitest run`（确认恢复）

提交最终状态（如有未提交改动）：
```bash
git add -A
git commit -m "test: Plan 3 全量验证通过" --allow-empty
```

---

## Plan 3 完成标准

- [ ] 9 个 task 完成
- [ ] `npm run typecheck` 无错误
- [ ] `npx vitest run` 全绿
- [ ] `npm run build` 成功
- [ ] dev 模式：三 tab 导航、日志显示、历史回滚、端口检测弹窗

## 后续 Plan 预告

- **Plan 4**: 毛玻璃 iOS 风格 UI 设计系统（届时日志/历史/表单全部视觉重构，并暴露健康检查开关）
- **Plan 5**: 远程 nginx SSH 管理
- **Plan 6**: electron-builder portable 打包 + 一键清除 + 导入导出
