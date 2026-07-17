# Plan 6: electron-builder portable 打包 + 一键清除 + 导入导出

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 产出可分发的绿色版 Windows exe（单文件、免安装、data 目录跟 exe 走、退出无痕），加"一键清除所有痕迹"按钮和配置导入导出（备份/迁移）。这是整个软件的收尾 plan。

**Architecture:**
1. **打包**：electron-builder `portable` target + extraResources 把内置 nginx + 模板打进 exe 旁边的 resources。修正 paths.ts 用 `PORTABLE_EXECUTABLE_DIR` 环境变量（portable 模式专属）定位 exe 旁目录，保证 data/ 真正跟 exe 走（而不是临时解压目录）。
2. **一键清除**：停 nginx → 删 data/ 全部 → 重置内存状态 → 提示重启。对应设计 §8.4 的"跑路"场景。
3. **导入导出**：把 hosts + mappings + 自定义模板导出成一个 JSON 文件（含版本号），导入时校验 + 合并/覆盖。

**Tech Stack:** electron-builder（已在 devDependencies）。无新依赖。导入导出用 JSON + Electron dialog（文件选择）。

**Spec 引用:** `docs/specs/2026-07-17-nginx-jump-gui-design.md` §8.4（一键清除）、§9（绿色版打包 + 目录结构）。

**前置条件:** Plan 1-5 完成（98 tests 绿）。paths.ts 已有 production 分支（但用 `dirname(process.execPath)`，portable 模式下指向临时解压目录，需修正）。`resources/nginx/` 已占位（Task 11，无真实 nginx.exe）。`build:portable` 脚本已存在但无 electron-builder 配置文件。

---

## 文件结构

```
nginx-jump-gui/
├── electron-builder.yml          # 新：portable 打包配置 + extraResources
├── src/
│   ├── main/
│   │   ├── paths.ts              # 改：PORTABLE_EXECUTABLE_DIR 优先
│   │   ├── ipc/handlers.ts       # 加：DATA_WIPE / DATA_EXPORT / DATA_IMPORT
│   │   └── services/
│   │       └── dataTransfer.ts   # 新：导出/导入 JSON 序列化
│   ├── shared/types.ts           # 加 IPC 通道 + ExportPayload 类型
│   ├── preload/index.ts          # 加桥接
│   └── renderer/
│       ├── api/ipc.ts            # 加类型
│       └── components/SettingsPanel.vue  # 新：设置页（清除/导入/导出/关于）
└── tests/main/services/dataTransfer.test.ts  # 新
```

---

## Task 1: electron-builder 配置 + paths portable 修正

**Files:**
- Create: `electron-builder.yml`
- Modify: `src/main/paths.ts`

- [ ] **Step 1: 创建 `electron-builder.yml`**

```yaml
appId: com.nginx-jump-gui.app
productName: Nginx跳板机
directories:
  output: release
  buildResources: build
files:
  - out/**/*
  - package.json
  - "!**/.DS_Store"
# portable：单文件自解压 exe，免安装
win:
  target:
    - target: portable
      arch:
        - x64
  artifactName: NginxJump-${version}-portable.exe
# 把内置 nginx 二进制 + 模板打进 exe 旁边的 resources
extraResources:
  - from: resources/nginx
    to: nginx
    filter: ['**/*']
  - from: src/main/templates/builtin
    to: templates/builtin
    filter: ['**/*.yml']
# portable 运行时设置 PORTABLE_EXECUTABLE_DIR 环境变量（指向 exe 所在目录）
# electron-builder portable 默认提供此变量
```

> 注意：portable target 解压到 `%TEMP%` 运行，`process.execPath` 指向临时解压的 exe。但 portable 模式下 electron-builder 注入 `PORTABLE_EXECUTABLE_DIR` 环境变量指向真正的 exe 位置。Step 2 用它。

- [ ] **Step 2: 修正 `src/main/paths.ts` 的 getAppRoot**

读当前 paths.ts。`getAppRoot()` 的 production 分支改成优先用 `PORTABLE_EXECUTABLE_DIR`：

```typescript
function getAppRoot(): string {
  if (appRootForTest) return appRootForTest
  // portable 模式：electron-builder 注入此环境变量，指向真正 exe 所在目录
  if (process.env.PORTABLE_EXECUTABLE_DIR) {
    return process.env.PORTABLE_EXECUTABLE_DIR
  }
  // 生产环境（非 portable，如 nsis 安装版）：exe 所在目录
  if (process.env.NODE_ENV === 'production') {
    return dirname(process.execPath)
  }
  // 开发环境：项目根目录
  return process.cwd()
}
```

- [ ] **Step 3: 更新 paths.test.ts 加 portable 测试**

读 `tests/main/paths.test.ts`。加一个测试用例验证 `PORTABLE_EXECUTABLE_DIR` 优先级：

```typescript
  it('PORTABLE_EXECUTABLE_DIR 环境变量优先于其他', () => {
    process.env.PORTABLE_EXECUTABLE_DIR = '/fake/portable/dir'
    __setAppRootForTest(null)  // 清除 test override
    const p = resolvePaths()
    expect(p.appRoot).toBe('/fake/portable/dir')
    expect(p.dataDir).toBe('/fake/portable/dir/data')
    delete process.env.PORTABLE_EXECUTABLE_DIR
  })
```

（注意：这个测试要放在其他测试之后或用 afterEach 清理环境变量。如果 `__setAppRootForTest` 优先级高于环境变量，测试需要调整——当前实现 `appRootForTest` 最优先，所以测试里要 `__setAppRootForTest(null)` 清除。）

- [ ] **Step 4: 验证**

Run: `npx vitest run tests/main/paths.test.ts` — 全过（原 5 + 新 1 = 6）
Run: `npm run typecheck`
Run: `npx vitest run` — 99 tests

- [ ] **Step 5: 提交**

```bash
git add electron-builder.yml src/main/paths.ts tests/main/paths.test.ts
git commit -m "feat(build): electron-builder portable 配置 + paths 用 PORTABLE_EXECUTABLE_DIR 定位 exe 旁目录"
```

---

## Task 2: dataTransfer（导出/导入序列化）

**Files:**
- Create: `src/main/services/dataTransfer.ts`
- Test: `tests/main/services/dataTransfer.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/services/dataTransfer.test.ts`**

```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { openDb, closeDb, __setDbPathForTest } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import { createMapping } from '@main/db/mappingRepo'
import { exportData, importData } from '@main/services/dataTransfer'
import { existsSync, rmSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

describe('dataTransfer', () => {
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

  it('exportData 返回含 version + hosts + mappings 的对象', () => {
    const h = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    createMapping({
      hostId: h.id, name: 'm', enabled: 1, protocol: 'tcp', listenPort: 9000,
      serverNames: null, listenPath: null, upstreamHost: '1.1.1.1', upstreamPort: 80,
      tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null,
      healthCheckEnabled: 0, healthCheckIntervalSec: 30, healthCheckTargetPath: null, healthCheckExpectedStatus: null
    })
    const data = exportData()
    expect(data.version).toBe(1)
    expect(data.hosts).toHaveLength(1)
    expect(data.mappings).toHaveLength(1)
    expect(data.exportedAt).toBeGreaterThan(0)
  })

  it('importData 空库时导入 host + mapping', () => {
    const payload = {
      version: 1,
      exportedAt: 0,
      hosts: [{ id: 'h-import', name: '导入的', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null, status: 'unknown', createdAt: 0 }],
      mappings: [{ id: 'm-import', hostId: 'h-import', name: '导入映射', enabled: 1, protocol: 'tcp', listenPort: 8000, serverNames: null, listenPath: null, upstreamHost: '2.2.2.2', upstreamPort: 22, tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0, extraDirectives: null, healthCheckEnabled: 0, healthCheckIntervalSec: 30, healthCheckTargetPath: null, healthCheckExpectedStatus: null, healthStatus: 'unknown', lastCheckedAt: null, sortOrder: 0, createdAt: 0, updatedAt: 0 }]
    }
    const result = importData(payload)
    expect(result.hostsImported).toBe(1)
    expect(result.mappingsImported).toBe(1)
  })

  it('importData 版本不匹配抛错', () => {
    expect(() => importData({ version: 99, exportedAt: 0, hosts: [], mappings: [] } as any))
      .toThrow(/版本/)
  })

  it('importData 缺字段抛错', () => {
    expect(() => importData({ version: 1 } as any)).toThrow(/格式/)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/services/dataTransfer.test.ts`
Expected: FAIL.

- [ ] **Step 3: 实现 `src/main/services/dataTransfer.ts`**

```typescript
import { getDb } from '../db/connection'
import type { Host, Mapping } from '@shared/types'

const EXPORT_VERSION = 1

export interface ExportPayload {
  version: number
  exportedAt: number
  hosts: Host[]
  mappings: Mapping[]
}

/** 导出全部 hosts + mappings 为可序列化对象 */
export function exportData(): ExportPayload {
  const db = getDb()
  const hosts = db.prepare('SELECT * FROM hosts').all() as any[]
  const mappings = db.prepare('SELECT * FROM mappings').all() as any[]
  // 转成应用层格式（snake→camel）
  return {
    version: EXPORT_VERSION,
    exportedAt: Date.now(),
    hosts: hosts.map(rowToHost),
    mappings: mappings.map(rowToMapping)
  }
}

export interface ImportResult {
  hostsImported: number
  mappingsImported: number
}

/** 校验 + 导入。覆盖式（先清空再导入），不支持合并（MVP）。 */
export function importData(payload: Partial<ExportPayload>): ImportResult {
  if (!payload || typeof payload !== 'object') throw new Error('导入格式错误：非对象')
  if (payload.version !== EXPORT_VERSION) throw new Error(`导入版本不匹配：期望 ${EXPORT_VERSION}，得到 ${payload.version}`)
  if (!Array.isArray(payload.hosts) || !Array.isArray(payload.mappings)) throw new Error('导入格式错误：缺少 hosts/mappings 数组')

  const db = getDb()
  // 覆盖式：清空再导入（事务）
  let hostsImported = 0
  let mappingsImported = 0
  db.transaction(() => {
    db.prepare('DELETE FROM mappings').run()
    db.prepare('DELETE FROM hosts').run()
    for (const h of payload.hosts!) {
      db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
        .run(h.id, h.name, h.type, h.isDefault ?? 0,
          h.localConfig ? JSON.stringify(h.localConfig) : null,
          h.remoteConfig ? JSON.stringify(h.remoteConfig) : null,
          h.status ?? 'unknown', h.createdAt ?? Date.now())
      hostsImported++
    }
    for (const m of payload.mappings!) {
      db.prepare(`INSERT INTO mappings (id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
        upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
        health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
        health_status, last_checked_at, sort_order, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
        .run(m.id, m.hostId, m.name, m.enabled ?? 1, m.protocol, m.listenPort, m.serverNames, m.listenPath,
          m.upstreamHost, m.upstreamPort, m.tlsCertPath, m.tlsKeyPath, m.redirectHttp ?? 0, m.extraDirectives,
          m.healthCheckEnabled ?? 0, m.healthCheckIntervalSec ?? 30, m.healthCheckTargetPath, m.healthCheckExpectedStatus,
          m.healthStatus ?? 'unknown', m.lastCheckedAt, m.sortOrder ?? 0, m.createdAt ?? Date.now(), m.updatedAt ?? Date.now())
      mappingsImported++
    }
  })()
  return { hostsImported, mappingsImported }
}

// 行转换（snake→camel，复用 hostRepo/mappingRepo 的逻辑但独立避免循环依赖）
function rowToHost(r: any): Host {
  return {
    id: r.id, name: r.name, type: r.type, isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status, createdAt: r.created_at
  }
}

function rowToMapping(r: any): Mapping {
  return {
    id: r.id, hostId: r.host_id, name: r.name, enabled: r.enabled, protocol: r.protocol,
    listenPort: r.listen_port, serverNames: r.server_names, listenPath: r.listen_path,
    upstreamHost: r.upstream_host, upstreamPort: r.upstream_port,
    tlsCertPath: r.tls_cert_path, tlsKeyPath: r.tls_key_path, redirectHttp: r.redirect_http,
    extraDirectives: r.extra_directives,
    healthCheckEnabled: r.health_check_enabled, healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path, healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status, lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order, createdAt: r.created_at, updatedAt: r.updated_at
  }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/services/dataTransfer.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿，103 tests）

- [ ] **Step 6: 提交**

```bash
git add src/main/services/dataTransfer.ts tests/main/services/dataTransfer.test.ts
git commit -m "feat(data): exportData/importData 配置序列化（覆盖式导入 + 版本校验）"
```

---

## Task 3: 一键清除 + 导入导出 IPC

**Files:**
- Modify: `src/shared/types.ts`（加 IPC + ExportPayload）
- Modify: `src/main/ipc/handlers.ts`（加 wipe/export/import handler）
- Modify: `src/preload/index.ts`
- Modify: `src/renderer/api/ipc.ts`

- [ ] **Step 1: types.ts 加类型 + IPC**

加 ExportPayload 类型（或从 dataTransfer re-export）。简化：直接在 types.ts 定义最小接口。

在 IPC const 加：
```typescript
  // 数据管理
  DATA_WIPE: 'data:wipe',
  DATA_EXPORT: 'data:export',
  DATA_IMPORT: 'data:import',
```

- [ ] **Step 2: handlers.ts 加 handler**

import：
```typescript
import { exportData, importData } from '../services/dataTransfer'
import { closeDb } from '../db/connection'
import { getActiveEngine } from '../nginx/engineRegistry'
```

加 handler：
```typescript
  // 一键清除所有痕迹：停 nginx + 删 data 目录内容
  ipcMain.handle(IPC.DATA_WIPE, async () => {
    try {
      const eng = getActiveEngine()
      if (eng && eng.status() === 'running') {
        await eng.stop()
      }
      // 删除 data 目录所有内容（db/conf/logs/snapshots/templates/custom）
      const paths = resolvePaths()
      closeDb()
      const fs = require('fs')
      for (const sub of ['data.db', 'data.db-shm', 'data.db-wal', 'nginx-conf', 'logs', 'snapshots', 'temp', 'templates']) {
        const p = joinPath(paths.dataDir, sub)
        if (fs.existsSync(p)) {
          fs.rmSync(p, { recursive: true, force: true })
        }
      }
      return { ok: true }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })

  // 导出
  ipcMain.handle(IPC.DATA_EXPORT, () => {
    return exportData()
  })

  // 导入
  ipcMain.handle(IPC.DATA_IMPORT, (_e, payload: any) => {
    try {
      const result = importData(payload)
      return { ok: true, ...result }
    } catch (e: any) {
      return { ok: false, output: e?.message ?? String(e) }
    }
  })
```

（`joinPath` = `join` from 'path'，已 import 或加 import。`require('fs')` 用 dynamic 避免 ESM 问题——或顶部 import fs。优先顶部 import。）

- [ ] **Step 3: preload + renderer api 加桥接**

preload api 加：
```typescript
  wipeAllData: () => ipcRenderer.invoke(IPC.DATA_WIPE),
  exportData: () => ipcRenderer.invoke(IPC.DATA_EXPORT),
  importData: (payload: any) => ipcRenderer.invoke(IPC.DATA_IMPORT, payload),
```

renderer ElectronAPI 加：
```typescript
  wipeAllData(): Promise<{ ok: boolean; output?: string }>
  exportData(): Promise<any>
  importData(payload: any): Promise<{ ok: boolean; output?: string; hostsImported?: number; mappingsImported?: number }>
```

- [ ] **Step 4: 验证 + 提交**

Run: typecheck + tests + build
```bash
git add src/shared/types.ts src/main/ipc/handlers.ts src/preload/index.ts src/renderer/api/ipc.ts
git commit -m "feat(ipc): DATA_WIPE + DATA_EXPORT + DATA_IMPORT（清除/导出/导入）"
```

---

## Task 4: SettingsPanel UI

**Files:**
- Create: `src/renderer/components/SettingsPanel.vue`
- Modify: `src/renderer/App.vue`（加"设置"tab）
- Modify: `src/renderer/stores/hosts.ts`（wipe 后重新 load）

- [ ] **Step 1: 创建 `src/renderer/components/SettingsPanel.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const message = ref('')
const messageType = ref<'info' | 'error'>('info')

async function wipeAll() {
  if (!confirm('⚠️ 确定清除所有痕迹？\n\n这将删除所有主机、映射、日志、历史配置。\n程序将重启到空状态。\n\n此操作不可撤销！')) return
  if (!confirm('再次确认：真的要删除所有数据吗？')) return
  const r = await api.wipeAllData()
  if (!r.ok) {
    message.value = '清除失败：' + (r.output ?? '未知错误')
    messageType.value = 'error'
    return
  }
  message.value = '已清除所有痕迹，请手动重启程序。'
  messageType.value = 'info'
}

async function doExport() {
  const data = await api.exportData()
  // 用 dialog 选保存位置（通过 IPC），或直接下载。简化：用 Blob 下载
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `nginx-jump-backup-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
  message.value = `已导出 ${data.hosts.length} 个主机、${data.mappings.length} 条映射`
  messageType.value = 'info'
}

async function doImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!confirm('导入会覆盖当前所有配置，确定继续？')) { input.value = ''; return }
  try {
    const text = await file.text()
    const payload = JSON.parse(text)
    const r = await api.importData(payload)
    if (!r.ok) {
      message.value = '导入失败：' + (r.output ?? '未知错误')
      messageType.value = 'error'
    } else {
      message.value = `已导入 ${r.hostsImported} 个主机、${r.mappingsImported} 条映射，请重启程序`
      messageType.value = 'info'
      await hosts.load()
      if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
    }
  } catch (e: any) {
    message.value = '导入失败：' + e.message
    messageType.value = 'error'
  }
  input.value = ''
}
</script>

<template>
  <div class="settings-panel">
    <div class="section glass-card">
      <h3>数据管理</h3>
      <p class="desc">备份当前配置到文件，或从备份恢复。</p>
      <div class="actions">
        <button class="btn btn-primary" @click="doExport">导出配置</button>
        <label class="btn btn-ghost">
          导入配置
          <input type="file" accept=".json" @change="doImport" hidden />
        </label>
      </div>
    </div>

    <div class="section glass-card danger-zone">
      <h3>⚠️ 危险区域</h3>
      <p class="desc">一键清除所有数据（主机、映射、日志、历史）。用于彻底抹除痕迹。</p>
      <button class="btn btn-danger" @click="wipeAll">清除所有痕迹</button>
    </div>

    <div class="section glass-card">
      <h3>关于</h3>
      <p class="desc">Nginx 跳板机管理软件 v0.1.0</p>
      <p class="desc">绿色版 · 免安装 · 退出无痕</p>
    </div>

    <div v-if="message" class="message" :class="messageType">{{ message }}</div>
  </div>
</template>

<style scoped>
.settings-panel { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-4); max-width: 600px; }
.section { padding: var(--space-4); }
.section h3 { font-size: var(--font-size-md); margin-bottom: var(--space-2); }
.desc { font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-3); line-height: 1.5; }
.actions { display: flex; gap: var(--space-2); }
.btn-danger { background: var(--color-danger); color: #fff; }
.btn-danger:hover { opacity: 0.85; }
.danger-zone { border-color: var(--color-danger); }
.message { padding: var(--space-3); border-radius: var(--radius-md); font-size: var(--font-size-sm); }
.message.info { background: var(--color-accent-soft); color: var(--text-primary); }
.message.error { background: rgba(255, 59, 48, 0.12); color: var(--color-danger); }
</style>
```

- [ ] **Step 2: App.vue 加"设置"tab**

currentView 类型加 `'settings'`：
```typescript
const currentView = ref<'mappings' | 'logs' | 'history' | 'hosts' | 'settings'>('mappings')
```

nav-tabs 加：
```html
<button :class="{ active: currentView === 'settings' }" @click="currentView = 'settings'">设置</button>
```

main 加：
```html
<SettingsPanel v-else-if="currentView === 'settings'" />
```

import SettingsPanel。

- [ ] **Step 3: 验证 + 提交**

Run: typecheck + build + tests
```bash
git add src/renderer/components/SettingsPanel.vue src/renderer/App.vue
git commit -m "feat(ui): SettingsPanel 设置页（导出/导入/一键清除/关于）"
```

---

## Task 5: 端到端验证

- [ ] **Step 1: 全量回归**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 全绿（103+ tests）
Run: `npm run build` — 成功

- [ ] **Step 2: dev 模式**

Run: `npm run rebuild:electron && npm run dev`（后台 22s）
Expected: 窗口打开，"设置"tab 出现，能导出配置（下载 JSON）、导入（选文件）、清除（双确认）

- [ ] **Step 3: 打包验证（Windows 上做）**

由于 macOS 无法打 Windows portable，此处只验证 electron-builder 配置加载：
Run: `npx electron-builder --win portable --config electron-builder.yml 2>&1 | head -20`
Expected: 开始打包流程（会因无 Windows 工具链在 macOS 上失败，但能验证配置语法正确、extraResources 路径解析）

实际 Windows 打包验证留到交付时。

- [ ] **Step 4: 恢复 node ABI + 提交 plan 文档**

Run: `npm run rebuild:node`
```bash
git add docs/plans/2026-07-17-plan-6-packaging-wipe-transfer.md
git commit -m "docs: Plan 6 实施计划（portable 打包 + 一键清除 + 导入导出）"
```

---

## Plan 6 完成标准

- [ ] 5 个 task 完成
- [ ] typecheck/build/全量 tests 绿
- [ ] electron-builder.yml 配置正确（portable + extraResources）
- [ ] paths.ts 用 PORTABLE_EXECUTABLE_DIR 定位
- [ ] SettingsPanel：导出/导入/一键清除/关于
- [ ] dataTransfer 单元测试覆盖

## 完成后整个项目状态

Plan 1-6 全部完成 = 可交付的绿色版软件：
- 功能：4 协议映射、13 模板、健康检查、历史回滚、日志、端口检测、远程 SSH 管理
- UI：毛玻璃 iOS 风格 + 深浅色主题
- 分发：portable exe（绿色版）+ 导入导出 + 一键清除
- 测试：100+ 单元测试覆盖核心逻辑
