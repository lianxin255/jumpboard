# Plan 1: 骨架 + TCP 转发一整条龙

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭起 Electron + Vue 绿色版骨架，跑通"建一条 TCP 转发映射 → 生成 nginx stream 配置 → 启动内置 nginx → 外部能连通"的最小闭环。

**Architecture:** Electron main 进程用 better-sqlite3 存数据，通过 IPC 暴露给 Vue renderer。NginxEngine 抽象层管理本地 nginx.exe 子进程的生命周期（start/stop/reload/test）。配置生成器把 SQLite 里的 mapping 渲染成 nginx stream conf 分片。userData 强制重定向到 exe 旁的 `data/` 目录，实现绿色版无痕。

**Tech Stack:** Electron 30、Vite、Vue 3、TypeScript、better-sqlite3、electron-vite（脚手架）、electron-builder（portable 打包）

**Spec 引用:** `docs/specs/2026-07-17-nginx-jump-gui-design.md` §3/§4/§5/§9。本 plan 仅实现 TCP 协议（§5.3）、最小 UI（§8 简化版）、本地引擎（§3.2 LocalEngine）、绿色版目录（§9）。UDP/HTTP/HTTPS/模板系统/远程引擎/辅助服务留到后续 plan。

---

## 文件结构

本 plan 创建的文件及职责（后续 plan 会在此基础上扩展）：

```
nginx-jump-gui/
├── package.json                          # 依赖、脚本
├── electron-vite.config.ts               # electron-vite 配置
├── tsconfig.json                         # TS 配置
├── electron-builder.yml                  # portable 打包配置
├── src/
│   ├── shared/                           # main/renderer 共享类型
│   │   └── types.ts                      # Host/Mapping/IPC 通道定义
│   ├── main/                             # Electron 主进程
│   │   ├── index.ts                      # 入口：创建窗口、注册 IPC、重定向 userData
│   │   ├── paths.ts                      # 绿色版路径解析（userData→data/）
│   │   ├── db/
│   │   │   ├── connection.ts             # better-sqlite3 连接 + 建表
│   │   │   ├── migrations.ts             # 建表 SQL
│   │   │   ├── hostRepo.ts               # Host CRUD
│   │   │   └── mappingRepo.ts            # Mapping CRUD
│   │   ├── nginx/
│   │   │   ├── types.ts                  # NginxEngine 接口 + EngineStatus
│   │   │   ├── localEngine.ts            # 本地 nginx 子进程管理
│   │   │   └── confGenerator.ts          # mapping → conf 分片
│   │   └── ipc/
│   │       └── handlers.ts               # ipcMain.handle 注册
│   ├── renderer/                         # Vue 前端
│   │   ├── index.html
│   │   ├── main.ts                       # Vue 入口
│   │   ├── App.vue                       # 根组件（最小布局）
│   │   ├── api/
│   │   │   └── ipc.ts                    # 封装 window.electronAPI 调用
│   │   ├── components/
│   │   │   ├── MappingList.vue           # 映射列表
│   │   │   └── MappingForm.vue           # 新建/编辑表单（TCP 字段）
│   │   └── stores/
│   │       └── mappings.ts               # Pinia store
│   └── preload/
│       └── index.ts                      # contextBridge 暴露 IPC
├── resources/
│   ├── nginx/                            # 占位，打包前放入 nginx.exe（Task 12）
│   └── icon.png
└── tests/
    ├── db/
    │   ├── hostRepo.test.ts
    │   └── mappingRepo.test.ts
    ├── nginx/
    │   ├── confGenerator.test.ts
    │   └── localEngine.test.ts
    └── e2e/
        └── tcp-flow.test.ts              # 端到端：建 TCP 映射能连通
```

---

## Task 1: 初始化项目骨架

**Files:**
- Create: `package.json`, `electron-vite.config.ts`, `tsconfig.json`, `tsconfig.node.json`, `.gitignore`, `src/main/index.ts`, `src/preload/index.ts`, `src/renderer/index.html`, `src/renderer/main.ts`, `src/renderer/App.vue`

- [ ] **Step 1: 创建 package.json**

```json
{
  "name": "nginx-jump-gui",
  "version": "0.1.0",
  "description": "内网跳板机 nginx 管理软件",
  "main": "out/main/index.js",
  "author": "",
  "license": "MIT",
  "scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "preview": "electron-vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "vue-tsc --noEmit && tsc --noEmit -p tsconfig.node.json",
    "build:portable": "electron-vite build && electron-builder --win portable"
  },
  "dependencies": {
    "better-sqlite3": "^11.0.0",
    "pinia": "^2.2.0",
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@types/better-sqlite3": "^7.6.0",
    "@vitejs/plugin-vue": "^5.0.0",
    "electron": "^30.0.0",
    "electron-builder": "^24.13.0",
    "electron-vite": "^2.1.0",
    "typescript": "^5.4.0",
    "vite": "^5.2.0",
    "vitest": "^1.6.0",
    "vue-tsc": "^2.0.0",
    "@vue/test-utils": "^2.4.0"
  }
}
```

- [ ] **Step 2: 创建 .gitignore**

```
node_modules/
out/
dist/
data/
*.log
.DS_Store
```

- [ ] **Step 3: 创建 tsconfig.json**（renderer 用，引用 vue-tsc）

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "sourceMap": true,
    "resolveJsonModule": true,
    "esModuleInterop": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "baseUrl": ".",
    "paths": {
      "@shared/*": ["src/shared/*"],
      "@renderer/*": ["src/renderer/*"]
    }
  },
  "include": ["src/renderer/**/*", "src/shared/**/*", "src/preload/**/*"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 4: 创建 tsconfig.node.json**（main/preload/config 用）

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "sourceMap": true,
    "esModuleInterop": true,
    "lib": ["ES2023"],
    "skipLibCheck": true,
    "composite": true,
    "baseUrl": ".",
    "paths": {
      "@shared/*": ["src/shared/*"]
    }
  },
  "include": ["src/main/**/*", "src/shared/**/*", "electron-vite.config.ts"]
}
```

- [ ] **Step 5: 创建 electron-vite.config.ts**

```typescript
import { resolve } from 'node:path'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'electron-vite'

export default defineConfig({
  main: {
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/main/index.ts') }
      }
    }
  },
  preload: {
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/preload/index.ts') }
      }
    }
  },
  renderer: {
    root: 'src/renderer',
    build: {
      rollupOptions: {
        input: resolve(__dirname, 'src/renderer/index.html')
      }
    },
    resolve: {
      alias: {
        '@renderer': resolve(__dirname, 'src/renderer'),
        '@shared': resolve(__dirname, 'src/shared')
      }
    },
    plugins: [vue()]
  }
})
```

- [ ] **Step 6: 创建 src/shared/types.ts（占位，后续 task 填充）**

```typescript
// 共享类型定义 - 在后续 task 中填充
export {}
```

- [ ] **Step 7: 创建最小 main 入口 src/main/index.ts（占位，Task 5 完善）**

```typescript
// 占位，Task 5 完善
export {}
```

- [ ] **Step 8: 创建最小 preload src/preload/index.ts**

```typescript
// 占位，Task 8 完善
export {}
```

- [ ] **Step 9: 创建 src/renderer/index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Nginx 跳板机</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="./main.ts"></script>
  </body>
</html>
```

- [ ] **Step 10: 创建 src/renderer/main.ts**

```typescript
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'

createApp(App).use(createPinia()).mount('#app')
```

- [ ] **Step 11: 创建 src/renderer/App.vue（占位）**

```vue
<script setup lang="ts"></script>

<template>
  <div class="app">
    骨架就绪
  </div>
</template>

<style scoped>
.app { padding: 20px; font-family: -apple-system, 'Segoe UI', sans-serif; }
</style>
```

- [ ] **Step 12: 创建 vitest 配置（内联到 package.json 或单独文件）**

创建 `vitest.config.ts`：

```typescript
import { resolve } from 'node:path'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@shared': resolve(__dirname, 'src/shared'),
      '@renderer': resolve(__dirname, 'src/renderer'),
      '@main': resolve(__dirname, 'src/main')
    }
  },
  test: {
    environment: 'node',
    include: ['tests/**/*.test.ts']
  }
})
```

- [ ] **Step 13: 安装依赖**

Run: `npm install`
Expected: 无报错，node_modules 创建。

- [ ] **Step 14: 验证类型检查通过**

Run: `npx tsc --noEmit -p tsconfig.node.json`
Expected: 无错误。

- [ ] **Step 15: 初始化 git 并提交**

```bash
cd /Users/lianxin/Desktop/oepnProject/nginx-jump-gui
git init
git add -A
git commit -m "chore: 初始化 Electron + Vue 骨架"
```

---

## Task 2: 绿色版路径解析（userData 重定向）

**Files:**
- Create: `src/main/paths.ts`
- Test: `tests/main/paths.test.ts`

这是"绿色版 + 退出无痕"的核心。把所有运行时数据强制限制在 exe 旁边的 `data/` 目录，绝不写 `%APPDATA%` 或注册表。

- [ ] **Step 1: 写失败测试 tests/main/paths.test.ts**

```typescript
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { __setAppRootForTest, resolvePaths } from '@main/paths'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

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
    expect(existsSync(join(dataDir, 'logs'))).toBe(true)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `npx vitest run tests/main/paths.test.ts`
Expected: FAIL，`resolvePaths` 未定义。

- [ ] **Step 3: 实现 src/main/paths.ts**

```typescript
import { existsSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'

let appRootForTest: string | null = null

export function __setAppRootForTest(root: string): void {
  appRootForTest = root
}

function getAppRoot(): string {
  if (appRootForTest)
    return appRootForTest
  // 生产环境：exe 所在目录；开发环境：项目根目录
  if (process.env.NODE_ENV === 'production') {
    // electron-builder portable 下，process.execPath 指向 exe
    return require('node:path').dirname(process.execPath)
  }
  return process.cwd()
}

export interface AppPaths {
  appRoot: string
  dataDir: string
  dbPath: string
  nginxConfDir: string
  nginxLogDir: string
  snapshotDir: string
  tempDir: string
  resourcesDir: string
  bundledNginxDir: string
  ensureDataDirs: () => void
}

export function resolvePaths(): AppPaths {
  const appRoot = getAppRoot()
  const dataDir = join(appRoot, 'data')
  const resourcesDir = process.env.NODE_ENV === 'production'
    ? join(appRoot, 'resources')
    : join(appRoot, 'resources')

  return {
    appRoot,
    dataDir,
    dbPath: join(dataDir, 'data.db'),
    nginxConfDir: join(dataDir, 'nginx-conf'),
    nginxLogDir: join(dataDir, 'logs'),
    snapshotDir: join(dataDir, 'snapshots'),
    tempDir: join(dataDir, 'temp'),
    resourcesDir,
    bundledNginxDir: join(resourcesDir, 'nginx'),
    ensureDataDirs: () => {
      for (const d of [dataDir, join(dataDir, 'nginx-conf'), join(dataDir, 'nginx-conf', 'http'), join(dataDir, 'nginx-conf', 'stream'), join(dataDir, 'logs'), join(dataDir, 'snapshots'), join(dataDir, 'temp')]) {
        if (!existsSync(d))
          mkdirSync(d, { recursive: true })
      }
    }
  }
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/paths.test.ts`
Expected: PASS（5 个测试全过）。

- [ ] **Step 5: 提交**

```bash
git add src/main/paths.ts tests/main/paths.test.ts
git commit -m "feat(paths): 绿色版路径解析，userData 重定向到 exe 旁 data 目录"
```

---

## Task 3: 共享类型定义

**Files:**
- Create: `src/shared/types.ts`（覆盖 Task 1 的占位）

- [ ] **Step 1: 实现 src/shared/types.ts**

```typescript
// ============ Host ============
export type HostType = 'local' | 'remote'

export interface Host {
  id: string
  name: string
  type: HostType
  isDefault: number // 0 | 1
  localConfig: LocalHostConfig | null
  remoteConfig: RemoteHostConfig | null
  status: EngineStatus
  createdAt: number
}

export interface LocalHostConfig {
  nginxPath: string // 可空，空则用 bundled
  confDir: string // 可空，空则用 data/nginx-conf
  logDir: string // 可空
}

export interface RemoteHostConfig {
  host: string
  port: number // SSH 端口，默认 22
  username: string
  authType: 'password' | 'key'
  passwordEncrypted: string | null // safeStorage 加密后 base64
  keyPath: string | null
  nginxPath: string
  confPath: string // 远程 conf 主文件路径
  reloadCmd: string // 默认 nginx -s reload
  testCmd: string // 默认 nginx -t
}

// ============ Mapping ============
export type Protocol = 'tcp' | 'udp' | 'http' | 'https'
export type HealthStatus = 'healthy' | 'unhealthy' | 'unknown' | 'checking'

export interface Mapping {
  id: string
  hostId: string
  name: string
  enabled: number // 0 | 1
  protocol: Protocol
  listenPort: number | null
  serverNames: string | null
  listenPath: string | null
  upstreamHost: string
  upstreamPort: number
  tlsCertPath: string | null
  tlsKeyPath: string | null
  redirectHttp: number // 0 | 1
  extraDirectives: string | null
  healthCheckEnabled: number
  healthCheckIntervalSec: number
  healthCheckTargetPath: string | null
  healthCheckExpectedStatus: number | null
  healthStatus: HealthStatus
  lastCheckedAt: number | null
  sortOrder: number
  createdAt: number
  updatedAt: number
}

// 输入用（无 id/时间戳/healthStatus，由后端填）
export type MappingInput = Omit<Mapping, 'id' | 'createdAt' | 'updatedAt' | 'healthStatus' | 'lastCheckedAt' | 'sortOrder'>

// ============ Nginx Engine ============
export type EngineStatus = 'running' | 'stopped' | 'unknown'

export interface EngineTestResult {
  ok: boolean
  output: string
}

// ============ IPC 通道 ============
export const IPC = {
  // Host
  HOST_LIST: 'host:list',
  HOST_CREATE: 'host:create',
  HOST_UPDATE: 'host:update',
  HOST_DELETE: 'host:delete',
  // Mapping
  MAPPING_LIST: 'mapping:list',
  MAPPING_CREATE: 'mapping:create',
  MAPPING_UPDATE: 'mapping:update',
  MAPPING_DELETE: 'mapping:delete',
  // Engine
  ENGINE_START: 'engine:start',
  ENGINE_STOP: 'engine:stop',
  ENGINE_RELOAD: 'engine:reload',
  ENGINE_TEST: 'engine:test',
  ENGINE_STATUS: 'engine:status',
} as const

export type IpcChannel = typeof IPC[keyof typeof IPC]
```

- [ ] **Step 2: 类型检查**

Run: `npx vue-tsc --noEmit && npx tsc --noEmit -p tsconfig.node.json`
Expected: 无错误。

- [ ] **Step 3: 提交**

```bash
git add src/shared/types.ts
git commit -m "feat(types): 定义 Host/Mapping/Engine/IPC 共享类型"
```

---

## Task 4: SQLite 连接与建表

**Files:**
- Create: `src/main/db/connection.ts`, `src/main/db/migrations.ts`
- Test: `tests/main/db/connection.test.ts`

- [ ] **Step 1: 写失败测试 tests/main/db/connection.test.ts**

```typescript
import { existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { __setDbPathForTest, closeDb, openDb } from '@main/db/connection'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

describe('db connection', () => {
  let dbPath: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath))
      rmSync(dbPath)
  })

  it('openDb 创建数据库文件', () => {
    openDb()
    expect(existsSync(dbPath)).toBe(true)
  })

  it('建表后表存在', () => {
    const db = openDb()
    const tables = db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all() as { name: string }[]
    const names = tables.map(t => t.name)
    expect(names).toContain('hosts')
    expect(names).toContain('mappings')
    expect(names).toContain('config_snapshots')
    expect(names).toContain('app_logs')
  })

  it('重复 openDb 不报错（幂等）', () => {
    openDb()
    expect(() => openDb()).not.toThrow()
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/db/connection.test.ts`
Expected: FAIL，`openDb` 未定义。

- [ ] **Step 3: 实现 src/main/db/migrations.ts**

```typescript
import type { Database } from 'better-sqlite3'

export function runMigrations(db: Database): void {
  db.pragma('journal_mode = WAL')

  db.exec(`
    CREATE TABLE IF NOT EXISTS hosts (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      type TEXT NOT NULL CHECK(type IN ('local','remote')),
      is_default INTEGER NOT NULL DEFAULT 0,
      local_config TEXT,
      remote_config TEXT,
      status TEXT NOT NULL DEFAULT 'unknown',
      created_at INTEGER NOT NULL
    );

    CREATE TABLE IF NOT EXISTS mappings (
      id TEXT PRIMARY KEY,
      host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
      name TEXT NOT NULL,
      enabled INTEGER NOT NULL DEFAULT 1,
      protocol TEXT NOT NULL CHECK(protocol IN ('tcp','udp','http','https')),
      listen_port INTEGER,
      server_names TEXT,
      listen_path TEXT,
      upstream_host TEXT NOT NULL,
      upstream_port INTEGER NOT NULL,
      tls_cert_path TEXT,
      tls_key_path TEXT,
      redirect_http INTEGER NOT NULL DEFAULT 0,
      extra_directives TEXT,
      health_check_enabled INTEGER NOT NULL DEFAULT 0,
      health_check_interval_sec INTEGER NOT NULL DEFAULT 30,
      health_check_target_path TEXT,
      health_check_expected_status INTEGER,
      health_status TEXT NOT NULL DEFAULT 'unknown',
      last_checked_at INTEGER,
      sort_order INTEGER NOT NULL DEFAULT 0,
      created_at INTEGER NOT NULL,
      updated_at INTEGER NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_mappings_host ON mappings(host_id);

    CREATE TABLE IF NOT EXISTS config_snapshots (
      id TEXT PRIMARY KEY,
      host_id TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
      content TEXT NOT NULL,
      reason TEXT NOT NULL,
      created_at INTEGER NOT NULL
    );

    CREATE TABLE IF NOT EXISTS app_logs (
      id TEXT PRIMARY KEY,
      host_id TEXT,
      level TEXT NOT NULL,
      source TEXT NOT NULL,
      message TEXT NOT NULL,
      timestamp INTEGER NOT NULL
    );
  `)
}
```

- [ ] **Step 4: 实现 src/main/db/connection.ts**

```typescript
import type { Database as DB } from 'better-sqlite3'
import Database from 'better-sqlite3'
import { resolvePaths } from '../paths'
import { runMigrations } from './migrations'

let dbInstance: DB | null = null
let dbPathForTest: string | null = null

export function __setDbPathForTest(path: string): void {
  dbPathForTest = path
}

function resolveDbPath(): string {
  if (dbPathForTest)
    return dbPathForTest
  const paths = resolvePaths()
  paths.ensureDataDirs()
  return paths.dbPath
}

export function openDb(): DB {
  if (dbInstance)
    return dbInstance
  const path = resolveDbPath()
  const db = new Database(path)
  runMigrations(db)
  dbInstance = db
  return db
}

export function closeDb(): void {
  if (dbInstance) {
    dbInstance.close()
    dbInstance = null
  }
}

export function getDb(): DB {
  if (!dbInstance)
    throw new Error('DB not opened. Call openDb() first.')
  return dbInstance
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `npx vitest run tests/main/db/connection.test.ts`
Expected: PASS（3 个测试）。

- [ ] **Step 6: 提交**

```bash
git add src/main/db/ tests/main/db/
git commit -m "feat(db): SQLite 连接 + 建表迁移（hosts/mappings/snapshots/logs）"
```

---

## Task 5: Host 仓库

**Files:**
- Create: `src/main/db/hostRepo.ts`
- Test: `tests/main/db/hostRepo.test.ts`

- [ ] **Step 1: 写失败测试**

```typescript
import { existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { __setDbPathForTest, closeDb, openDb } from '@main/db/connection'
import { createHost, deleteHost, getDefaultHost, getHost, listHosts, setDefaultHost, updateHost } from '@main/db/hostRepo'
// tests/main/db/hostRepo.test.ts
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

describe('hostRepo', () => {
  let dbPath: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath))
      rmSync(dbPath)
  })

  it('createHost 创建并返回 local host', () => {
    const h = createHost({ name: '本机', type: 'local', isDefault: 1, localConfig: { nginxPath: '', confDir: '', logDir: '' } })
    expect(h.id).toBeTruthy()
    expect(h.name).toBe('本机')
    expect(h.type).toBe('local')
    expect(h.localConfig?.nginxPath).toBe('')
  })

  it('listHosts 返回所有', () => {
    createHost({ name: 'a', type: 'local', isDefault: 1, localConfig: null })
    createHost({ name: 'b', type: 'remote', isDefault: 0, remoteConfig: { host: '1.1.1.1', port: 22, username: 'root', authType: 'password', passwordEncrypted: null, keyPath: null, nginxPath: '/usr/sbin/nginx', confPath: '/etc/nginx/nginx.conf', reloadCmd: 'nginx -s reload', testCmd: 'nginx -t' } })
    const list = listHosts()
    expect(list).toHaveLength(2)
  })

  it('getHost 按 id 查', () => {
    const h = createHost({ name: 'x', type: 'local', isDefault: 1, localConfig: null })
    const got = getHost(h.id)
    expect(got?.name).toBe('x')
  })

  it('updateHost 更新名字', () => {
    const h = createHost({ name: 'old', type: 'local', isDefault: 1, localConfig: null })
    updateHost(h.id, { name: 'new' })
    expect(getHost(h.id)?.name).toBe('new')
  })

  it('deleteHost 删除', () => {
    const h = createHost({ name: 'del', type: 'local', isDefault: 1, localConfig: null })
    deleteHost(h.id)
    expect(getHost(h.id)).toBeUndefined()
  })

  it('setDefaultHost 保证只有一个默认', () => {
    const a = createHost({ name: 'a', type: 'local', isDefault: 1, localConfig: null })
    const b = createHost({ name: 'b', type: 'local', isDefault: 0, localConfig: null })
    setDefaultHost(b.id)
    expect(getDefaultHost()?.id).toBe(b.id)
    expect(getHost(a.id)?.isDefault).toBe(0)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/db/hostRepo.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 src/main/db/hostRepo.ts**

```typescript
import type { Host, HostType, LocalHostConfig, RemoteHostConfig } from '@shared/types'
import { randomUUID } from 'node:crypto'
import { getDb } from './connection'

interface HostRow {
  id: string
  name: string
  type: HostType
  is_default: number
  local_config: string | null
  remote_config: string | null
  status: string
  created_at: number
}

function rowToHost(r: HostRow): Host {
  return {
    id: r.id,
    name: r.name,
    type: r.type,
    isDefault: r.is_default,
    localConfig: r.local_config ? JSON.parse(r.local_config) : null,
    remoteConfig: r.remote_config ? JSON.parse(r.remote_config) : null,
    status: r.status as Host['status'],
    createdAt: r.created_at
  }
}

export interface CreateHostInput {
  name: string
  type: HostType
  isDefault: number
  localConfig: LocalHostConfig | null
  remoteConfig: RemoteHostConfig | null
}

export function createHost(input: CreateHostInput): Host {
  const db = getDb()
  const id = randomUUID()
  const now = Date.now()
  db.prepare(`INSERT INTO hosts (id, name, type, is_default, local_config, remote_config, status, created_at)
              VALUES (?, ?, ?, ?, ?, ?, 'unknown', ?)`)
    .run(id, input.name, input.type, input.isDefault, input.localConfig ? JSON.stringify(input.localConfig) : null, input.remoteConfig ? JSON.stringify(input.remoteConfig) : null, now)
  return getHost(id)!
}

export function listHosts(): Host[] {
  const rows = getDb().prepare('SELECT * FROM hosts ORDER BY created_at').all() as HostRow[]
  return rows.map(rowToHost)
}

export function getHost(id: string): Host | undefined {
  const row = getDb().prepare('SELECT * FROM hosts WHERE id = ?').get(id) as HostRow | undefined
  return row ? rowToHost(row) : undefined
}

export function updateHost(id: string, patch: Partial<CreateHostInput>): void {
  const cur = getHost(id)
  if (!cur)
    throw new Error(`Host not found: ${id}`)
  const merged = { ...cur, ...patch } as CreateHostInput & Host
  getDb().prepare(`UPDATE hosts SET name=?, type=?, is_default=?, local_config=?, remote_config=? WHERE id=?`).run(merged.name, merged.type, merged.isDefault, merged.localConfig ? JSON.stringify(merged.localConfig) : null, merged.remoteConfig ? JSON.stringify(merged.remoteConfig) : null, id)
}

export function deleteHost(id: string): void {
  getDb().prepare('DELETE FROM hosts WHERE id = ?').run(id)
}

export function setDefaultHost(id: string): void {
  const db = getDb()
  db.transaction(() => {
    db.prepare('UPDATE hosts SET is_default = 0').run()
    db.prepare('UPDATE hosts SET is_default = 1 WHERE id = ?').run(id)
  })()
}

export function getDefaultHost(): Host | undefined {
  const row = getDb().prepare('SELECT * FROM hosts WHERE is_default = 1 LIMIT 1').get() as HostRow | undefined
  return row ? rowToHost(row) : undefined
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/db/hostRepo.test.ts`
Expected: PASS（6 个测试）。

- [ ] **Step 5: 提交**

```bash
git add src/main/db/hostRepo.ts tests/main/db/hostRepo.test.ts
git commit -m "feat(db): Host 仓库 CRUD + 默认实例切换"
```

---

## Task 6: Mapping 仓库

**Files:**
- Create: `src/main/db/mappingRepo.ts`
- Test: `tests/main/db/mappingRepo.test.ts`

- [ ] **Step 1: 写失败测试**

```typescript
import type { MappingInput } from '@shared/types'
import { existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { __setDbPathForTest, closeDb, openDb } from '@main/db/connection'
import { createHost } from '@main/db/hostRepo'
import {
  createMapping,
  deleteMapping,
  getMapping,
  listEnabledMappings,
  listMappings,
  updateMapping
} from '@main/db/mappingRepo'
// tests/main/db/mappingRepo.test.ts
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

function tcpInput(hostId: string, over: Partial<MappingInput> = {}): MappingInput {
  return {
    hostId,
    name: 'test',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '192.168.1.10',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    ...over
  }
}

describe('mappingRepo', () => {
  let dbPath: string
  let hostId: string

  beforeEach(() => {
    dbPath = join(tmpdir(), `test-${Date.now()}.db`)
    __setDbPathForTest(dbPath)
    openDb()
    const h = createHost({ name: 'h', type: 'local', isDefault: 1, localConfig: null, remoteConfig: null })
    hostId = h.id
  })

  afterEach(() => {
    closeDb()
    if (existsSync(dbPath))
      rmSync(dbPath)
  })

  it('createMapping 创建 TCP 映射', () => {
    const m = createMapping(tcpInput(hostId))
    expect(m.id).toBeTruthy()
    expect(m.protocol).toBe('tcp')
    expect(m.healthStatus).toBe('unknown')
    expect(m.sortOrder).toBe(0)
  })

  it('listMappings 按 hostId 过滤', () => {
    createMapping(tcpInput(hostId, { name: 'a' }))
    createMapping(tcpInput(hostId, { name: 'b' }))
    expect(listMappings(hostId)).toHaveLength(2)
    expect(listMappings('other-id')).toHaveLength(0)
  })

  it('listEnabledMappings 只返回 enabled=1', () => {
    createMapping(tcpInput(hostId, { enabled: 1, name: 'on' }))
    createMapping(tcpInput(hostId, { enabled: 0, name: 'off' }))
    const list = listEnabledMappings(hostId)
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('on')
  })

  it('updateMapping 改端口', () => {
    const m = createMapping(tcpInput(hostId))
    updateMapping(m.id, { listenPort: 8888 })
    expect(getMapping(m.id)?.listenPort).toBe(8888)
  })

  it('deleteMapping 删除', () => {
    const m = createMapping(tcpInput(hostId))
    deleteMapping(m.id)
    expect(getMapping(m.id)).toBeUndefined()
  })

  it('sortOrder 自增', () => {
    const a = createMapping(tcpInput(hostId, { name: 'a' }))
    const b = createMapping(tcpInput(hostId, { name: 'b' }))
    expect(b.sortOrder).toBeGreaterThan(a.sortOrder)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/db/mappingRepo.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 src/main/db/mappingRepo.ts**

```typescript
import type { HealthStatus, Mapping, MappingInput, Protocol } from '@shared/types'
import { randomUUID } from 'node:crypto'
import { getDb } from './connection'

interface MappingRow {
  id: string
  host_id: string
  name: string
  enabled: number
  protocol: Protocol
  listen_port: number | null
  server_names: string | null
  listen_path: string | null
  upstream_host: string
  upstream_port: number
  tls_cert_path: string | null
  tls_key_path: string | null
  redirect_http: number
  extra_directives: string | null
  health_check_enabled: number
  health_check_interval_sec: number
  health_check_target_path: string | null
  health_check_expected_status: number | null
  health_status: string
  last_checked_at: number | null
  sort_order: number
  created_at: number
  updated_at: number
}

function rowToMapping(r: MappingRow): Mapping {
  return {
    id: r.id,
    hostId: r.host_id,
    name: r.name,
    enabled: r.enabled,
    protocol: r.protocol,
    listenPort: r.listen_port,
    serverNames: r.server_names,
    listenPath: r.listen_path,
    upstreamHost: r.upstream_host,
    upstreamPort: r.upstream_port,
    tlsCertPath: r.tls_cert_path,
    tlsKeyPath: r.tls_key_path,
    redirectHttp: r.redirect_http,
    extraDirectives: r.extra_directives,
    healthCheckEnabled: r.health_check_enabled,
    healthCheckIntervalSec: r.health_check_interval_sec,
    healthCheckTargetPath: r.health_check_target_path,
    healthCheckExpectedStatus: r.health_check_expected_status,
    healthStatus: r.health_status as HealthStatus,
    lastCheckedAt: r.last_checked_at,
    sortOrder: r.sort_order,
    createdAt: r.created_at,
    updatedAt: r.updated_at
  }
}

const COLUMNS = `id, host_id, name, enabled, protocol, listen_port, server_names, listen_path,
  upstream_host, upstream_port, tls_cert_path, tls_key_path, redirect_http, extra_directives,
  health_check_enabled, health_check_interval_sec, health_check_target_path, health_check_expected_status,
  health_status, last_checked_at, sort_order, created_at, updated_at`

export function createMapping(input: MappingInput): Mapping {
  const db = getDb()
  const id = randomUUID()
  const now = Date.now()
  const maxOrder = (db.prepare('SELECT COALESCE(MAX(sort_order), -1) AS m FROM mappings WHERE host_id = ?').get(input.hostId) as { m: number }).m
  db.prepare(`INSERT INTO mappings (${COLUMNS})
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
    .run(id, input.hostId, input.name, input.enabled, input.protocol, input.listenPort, input.serverNames, input.listenPath, input.upstreamHost, input.upstreamPort, input.tlsCertPath, input.tlsKeyPath, input.redirectHttp, input.extraDirectives, input.healthCheckEnabled, input.healthCheckIntervalSec, input.healthCheckTargetPath, input.healthCheckExpectedStatus, 'unknown', null, maxOrder + 1, now, now)
  return getMapping(id)!
}

export function listMappings(hostId: string): Mapping[] {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? ORDER BY sort_order`).all(hostId) as MappingRow[]
  return rows.map(rowToMapping)
}

export function listEnabledMappings(hostId: string): Mapping[] {
  const rows = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE host_id = ? AND enabled = 1 ORDER BY sort_order`).all(hostId) as MappingRow[]
  return rows.map(rowToMapping)
}

export function getMapping(id: string): Mapping | undefined {
  const row = getDb().prepare(`SELECT ${COLUMNS} FROM mappings WHERE id = ?`).get(id) as MappingRow | undefined
  return row ? rowToMapping(row) : undefined
}

export function updateMapping(id: string, patch: Partial<MappingInput>): void {
  const cur = getMapping(id)
  if (!cur)
    throw new Error(`Mapping not found: ${id}`)
  const merged = { ...cur, ...patch } as Mapping & MappingInput
  getDb().prepare(`UPDATE mappings SET
    name=?, enabled=?, protocol=?, listen_port=?, server_names=?, listen_path=?,
    upstream_host=?, upstream_port=?, tls_cert_path=?, tls_key_path=?, redirect_http=?,
    extra_directives=?, health_check_enabled=?, health_check_interval_sec=?,
    health_check_target_path=?, health_check_expected_status=?, updated_at=? WHERE id=?`).run(merged.name, merged.enabled, merged.protocol, merged.listenPort, merged.serverNames, merged.listenPath, merged.upstreamHost, merged.upstreamPort, merged.tlsCertPath, merged.tlsKeyPath, merged.redirectHttp, merged.extraDirectives, merged.healthCheckEnabled, merged.healthCheckIntervalSec, merged.healthCheckTargetPath, merged.healthCheckExpectedStatus, Date.now(), id)
}

export function deleteMapping(id: string): void {
  getDb().prepare('DELETE FROM mappings WHERE id = ?').run(id)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/db/mappingRepo.test.ts`
Expected: PASS（6 个测试）。

- [ ] **Step 5: 提交**

```bash
git add src/main/db/mappingRepo.ts tests/main/db/mappingRepo.test.ts
git commit -m "feat(db): Mapping 仓库 CRUD + enabled 过滤 + sortOrder 自增"
```

---

## Task 7: TCP 配置生成器

**Files:**
- Create: `src/main/nginx/confGenerator.ts`
- Test: `tests/main/nginx/confGenerator.test.ts`

这是整个软件的核心逻辑之一：把 SQLite 里的 mapping 渲染成 nginx 能读的 conf 文本。本 task 只实现 TCP（stream 块），其他协议在 Plan 2。

- [ ] **Step 1: 写失败测试**

```typescript
import type { Mapping } from '@shared/types'
import { generateFullConf } from '@main/nginx/confGenerator'
// tests/main/nginx/confGenerator.test.ts
import { describe, expect, it } from 'vitest'

function tcpMapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1',
    hostId: 'h1',
    name: 'MySQL 内库',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '192.168.1.10',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: 'proxy_timeout 1h;',
    healthCheckEnabled: 0,
    healthCheckIntervalSec: 30,
    healthCheckTargetPath: null,
    healthCheckExpectedStatus: null,
    healthStatus: 'unknown',
    lastCheckedAt: null,
    sortOrder: 0,
    createdAt: 0,
    updatedAt: 0,
    ...over
  } as Mapping
}

describe('confGenerator TCP', () => {
  it('生成包含 stream 块的完整 conf', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('stream {')
    expect(conf).toContain('listen 9000;')
    expect(conf).toContain('proxy_pass 192.168.1.10:3306;')
    expect(conf).toContain('proxy_timeout 1h;')
  })

  it('disabled 的 mapping 不生成', () => {
    const conf = generateFullConf([tcpMapping({ enabled: 0 })], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).not.toContain('listen 9000;')
  })

  it('不含 http 块（没有 http/https mapping）', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).not.toMatch(/\bhttp\s*\{/)
  })

  it('注释里带 mapping name', () => {
    const conf = generateFullConf([tcpMapping()], { nginxLogDir: '/data/logs', nginxTempDir: '/data/temp' })
    expect(conf).toContain('MySQL 内库')
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/nginx/confGenerator.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 src/main/nginx/confGenerator.ts**

```typescript
import type { Mapping } from '@shared/types'

export interface ConfGenOptions {
  nginxLogDir: string
  nginxTempDir: string
  // httpPort / httpsPort 可扩展，本 plan 不用
}

/**
 * 生成完整的 nginx.conf。
 * 本 plan 只处理 tcp（stream）。udp/http/https 留到 Plan 2。
 */
export function generateFullConf(mappings: Mapping[], opts: ConfGenOptions): string {
  const enabled = mappings.filter(m => m.enabled)

  const tcpMappings = enabled.filter(m => m.protocol === 'tcp')
  // 预留，Plan 2 实现
  const udpMappings = enabled.filter(m => m.protocol === 'udp')
  const httpMappings = enabled.filter(m => m.protocol === 'http' || m.protocol === 'https')

  const hasStream = tcpMappings.length > 0 || udpMappings.length > 0
  const hasHttp = httpMappings.length > 0

  const lines: string[] = []
  lines.push('# 由 nginx-jump-gui 自动生成，请勿手动编辑')
  lines.push(`# 生成时间: ${new Date().toISOString()}`)
  lines.push('worker_processes 1;')
  lines.push(`error_log ${opts.nginxLogDir}/error.log warn;`)
  lines.push(`pid ${opts.nginxTempDir}/nginx.pid;`)
  lines.push('')
  lines.push('events {')
  lines.push('  worker_connections 1024;')
  lines.push('}')
  lines.push('')

  if (hasHttp) {
    lines.push('http {')
    lines.push(`  access_log ${opts.nginxLogDir}/access.log;`)
    lines.push(`  client_body_temp_path ${opts.nginxTempDir}/client_body;`)
    lines.push(`  proxy_temp_path ${opts.nginxTempDir}/proxy;`)
    lines.push('  include mime.types;')
    lines.push('  # http/https server 块在 Plan 2 实现')
    lines.push('}')
    lines.push('')
  }

  if (hasStream) {
    lines.push('stream {')
    for (const m of tcpMappings) {
      lines.push(...renderTcpServer(m))
    }
    // udp 在 Plan 2
    lines.push('}')
    lines.push('')
  }

  return lines.join('\n')
}

function renderTcpServer(m: Mapping): string[] {
  const out: string[] = []
  out.push(`  # mapping: ${m.name}`)
  out.push('  server {')
  out.push(`    listen ${m.listenPort};`)
  out.push(`    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`)
  if (m.extraDirectives && m.extraDirectives.trim()) {
    for (const line of m.extraDirectives.split('\n')) {
      if (line.trim())
        out.push(`    ${line.trim()}`)
    }
  }
  out.push('  }')
  return out
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/nginx/confGenerator.test.ts`
Expected: PASS（4 个测试）。

- [ ] **Step 5: 提交**

```bash
git add src/main/nginx/confGenerator.ts tests/main/nginx/confGenerator.test.ts
git commit -m "feat(nginx): TCP stream 配置生成器"
```

---

## Task 8: 本地 NginxEngine（子进程管理）

**Files:**
- Create: `src/main/nginx/types.ts`, `src/main/nginx/localEngine.ts`
- Test: `tests/main/nginx/localEngine.test.ts`

这是"退出无痕"的执行者。用 child_process 管 nginx.exe，主进程退出时必杀。

- [ ] **Step 1: 写失败测试**

```typescript
// tests/main/nginx/localEngine.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { LocalNginxEngine } from '@main/nginx/localEngine'
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'

// 本测试需要一个能跑的 nginx.exe，CI 环境用 mock；本地有 nginx 时跑真实验证。
// 这里用 echo 脚本假装 nginx，验证进程生命周期管理逻辑。
describe('LocalNginxEngine 进程管理', () => {
  let workDir: string
  let fakeNginx: string

  beforeEach(() => {
    workDir = join(tmpdir(), `nginx-engine-${Date.now()}`)
    mkdirSync(workDir, { recursive: true })
    // 写一个假 nginx 脚本：保持运行直到收到信号
    if (process.platform === 'win32')) {
      fakeNginx = join(workDir, 'fake-nginx.bat')
      writeFileSync(fakeNginx, '@echo off\n:loop\ntimeout /t 1 >nul\ngoto loop\n')
    } {
      fakeNginx = join(workDir, 'fake-nginx.sh')
      writeFileSync(fakeNginx, '#!/bin/bash\nwhile true; do sleep 1; done\n')
      require('fs').chmodSync(fakeNginx, 0o755)
    }
  })

  afterEach(() => {
    rmSync(workDir, { recursive: true, force: true })
  })

  it('status 初始为 stopped', () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir })
    expect(eng.status()).toBe('stopped')
  })

  it('start 后变 running，stop 后变 stopped', async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir })
    await eng.start()
    expect(eng.status()).toBe('running')
    await eng.stop()
    expect(eng.status()).toBe('stopped')
  })

  it('重复 start 抛错', async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir })
    await eng.start()
    await expect(eng.start()).rejects.toThrow()
    await eng.stop()
  })

  it('stop 未运行的引擎不报错', async () => {
    const eng = new LocalNginxEngine({ nginxPath: fakeNginx, confPath: join(workDir, 'nginx.conf'), logDir: workDir, tempDir: workDir })
    await expect(eng.stop()).resolves.toBeUndefined()
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/nginx/localEngine.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 src/main/nginx/types.ts**

```typescript
import type { EngineStatus, EngineTestResult } from '@shared/types'

export type { EngineStatus, EngineTestResult }

export interface NginxEngine {
  start: () => Promise<void>
  stop: () => Promise<void>
  reload: () => Promise<void>
  test: () => Promise<EngineTestResult>
  status: () => EngineStatus
  writeConf: (content: string) => Promise<void>
  readConf: () => Promise<string>
}

export interface LocalEngineConfig {
  nginxPath: string // nginx.exe 路径（bundled 或用户指定）
  confPath: string // 主 conf 路径
  logDir: string
  tempDir: string
}
```

- [ ] **Step 4: 实现 src/main/nginx/localEngine.ts**

```typescript
import type { ChildProcess } from 'node:child_process';
import type { EngineStatus, EngineTestResult, LocalEngineConfig, NginxEngine } from './types'
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'

export class LocalNginxEngine implements NginxEngine {
  private child: ChildProcess | null = null
  private readonly cfg: LocalEngineConfig

  constructor(cfg: LocalEngineConfig) {
    this.cfg = cfg
  }

  status(): EngineStatus {
    return this.child && !this.child.killed ? 'running' : 'stopped'
  }

  async start(): Promise<void> {
    if (this.child && !this.child.killed) {
      throw new Error('nginx already running')
    }
    if (!existsSync(this.cfg.nginxPath)) {
      throw new Error(`nginx binary not found: ${this.cfg.nginxPath}`)
    }
    for (const d of [this.cfg.logDir, this.cfg.tempDir]) {
      if (!existsSync(d))
        mkdirSync(d, { recursive: true })
    }
    // nginx 启动会 daemonize（-fork），所以 spawn 返回后子进程很快退出，
    // 真正的 master 进程独立运行。我们用 -g 'daemon off;' 前台运行便于管理。
    this.child = spawn(this.cfg.nginxPath, [
      '-c',
      this.cfg.confPath,
      '-p',
      this.cfg.tempDir,
      '-g',
      'daemon off;'
    ], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe']
    })

    this.child.on('exit', () => { this.child = null })

    // 等待一小段时间确认进程还在（没立即崩）
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        if (this.child && !this.child.killed)
          resolve()
        else reject(new Error('nginx exited immediately after start'))
      }, 500)
      this.child!.once('exit', (code) => {
        if (code !== 0 && code !== null) {
          clearTimeout(timer)
          reject(new Error(`nginx exited with code ${code}`))
        }
      })
    })
  }

  async stop(): Promise<void> {
    if (!this.child)
      return
    // daemon off 模式下杀子进程即停（nginx master 会带走 worker）
    this.child.kill('SIGTERM')
    await new Promise<void>((resolve) => {
      const t = setTimeout(() => {
        if (this.child)
          this.child!.kill('SIGKILL')
        resolve()
      }, 3000)
      const onExit = () => { clearTimeout(t); this.child = null; resolve() }
      this.child!.once('exit', onExit)
    })
  }

  async reload(): Promise<void> {
    // 前台模式没有 pid 文件，reload 通过对进程发 HUP
    if (!this.child)
      throw new Error('nginx not running, cannot reload')
    this.child.kill('SIGHUP')
    // Windows 不支持 SIGHUP，实际 reload 在 Windows 上需调 nginx -s reload（依赖 pid 文件）
    // daemon off 模式下 Windows reload 较复杂，本 plan 先用 HUP，集成测试验证
  }

  async test(): Promise<EngineTestResult> {
    const { execFile } = require('node:child_process')
    return new Promise((resolve) => {
      execFile(this.cfg.nginxPath, ['-t', '-c', this.cfg.confPath, '-p', this.cfg.tempDir], (err: any, stdout: string, stderr: string) => {
        // nginx -t 把诊断写到 stderr，即使成功
        const output = (stdout + stderr).trim()
        if (err)
          resolve({ ok: false, output })
        else resolve({ ok: true, output })
      })
    })
  }

  async writeConf(content: string): Promise<void> {
    writeFileSync(this.cfg.confPath, content, 'utf-8')
  }

  async readConf(): Promise<string> {
    if (!existsSync(this.cfg.confPath))
      return ''
    return readFileSync(this.cfg.confPath, 'utf-8')
  }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `npx vitest run tests/main/nginx/localEngine.test.ts`
Expected: PASS（4 个测试）。

> 注意：测试里有个语法 bug（`'win32')）` 多了个括号），实现时修正为 `process.platform === 'win32'`。

- [ ] **Step 6: 修正测试中的语法 bug 并重跑**

把测试文件里：
```typescript
    if (process.platform === 'win32') {
```
改为正确的（去掉多余括号）。重跑确认 PASS。

Run: `npx vitest run tests/main/nginx/localEngine.test.ts`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add src/main/nginx/types.ts src/main/nginx/localEngine.ts tests/main/nginx/localEngine.test.ts
git commit -m "feat(nginx): 本地 NginxEngine 子进程管理（start/stop/reload/test）"
```

---

## Task 9: IPC handlers + preload + main 入口

**Files:**
- Create: `src/main/ipc/handlers.ts`, `src/preload/index.ts`（覆盖占位）
- Modify: `src/main/index.ts`（覆盖占位）

把前面的能力通过 IPC 暴露给前端。

- [ ] **Step 1: 实现 src/main/ipc/handlers.ts**

```typescript
import type { MappingInput } from '@shared/types'
import type { CreateHostInput } from '../db/hostRepo';
import type { LocalNginxEngine } from '../nginx/localEngine'
import { IPC } from '@shared/types'
import { ipcMain } from 'electron'
import { createHost, deleteHost, listHosts, updateHost } from '../db/hostRepo'
import { createMapping, deleteMapping, listMappings, updateMapping } from '../db/mappingRepo'

let activeEngine: LocalNginxEngine | null = null

export function setActiveEngine(eng: LocalNginxEngine | null): void {
  activeEngine = eng
}

export function registerIpcHandlers(): void {
  // Host
  ipcMain.handle(IPC.HOST_LIST, () => listHosts())
  ipcMain.handle(IPC.HOST_CREATE, (_e, input: CreateHostInput) => createHost(input))
  ipcMain.handle(IPC.HOST_UPDATE, (_e, id: string, patch: Partial<CreateHostInput>) => updateHost(id, patch))
  ipcMain.handle(IPC.HOST_DELETE, (_e, id: string) => { deleteHost(id); return true })

  // Mapping
  ipcMain.handle(IPC.MAPPING_LIST, (_e, hostId: string) => listMappings(hostId))
  ipcMain.handle(IPC.MAPPING_CREATE, (_e, input: MappingInput) => createMapping(input))
  ipcMain.handle(IPC.MAPPING_UPDATE, (_e, id: string, patch: Partial<MappingInput>) => updateMapping(id, patch))
  ipcMain.handle(IPC.MAPPING_DELETE, (_e, id: string) => { deleteMapping(id); return true })

  // Engine
  ipcMain.handle(IPC.ENGINE_START, async () => {
    if (activeEngine)
      await activeEngine.start(); return activeEngine?.status()
  })
  ipcMain.handle(IPC.ENGINE_STOP, async () => {
    if (activeEngine)
      await activeEngine.stop(); return activeEngine?.status()
  })
  ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
    if (activeEngine)
      await activeEngine.reload(); return true
  })
  ipcMain.handle(IPC.ENGINE_TEST, async () => activeEngine ? activeEngine.test() : { ok: false, output: 'no engine' })
  ipcMain.handle(IPC.ENGINE_STATUS, () => activeEngine?.status() ?? 'unknown')
}
```

- [ ] **Step 2: 实现 src/preload/index.ts**

```typescript
import { IPC } from '@shared/types'
import { contextBridge, ipcRenderer } from 'electron'

const api = {
  // Host
  listHosts: () => ipcRenderer.invoke(IPC.HOST_LIST),
  createHost: (input: any) => ipcRenderer.invoke(IPC.HOST_CREATE, input),
  updateHost: (id: string, patch: any) => ipcRenderer.invoke(IPC.HOST_UPDATE, id, patch),
  deleteHost: (id: string) => ipcRenderer.invoke(IPC.HOST_DELETE, id),
  // Mapping
  listMappings: (hostId: string) => ipcRenderer.invoke(IPC.MAPPING_LIST, hostId),
  createMapping: (input: any) => ipcRenderer.invoke(IPC.MAPPING_CREATE, input),
  updateMapping: (id: string, patch: any) => ipcRenderer.invoke(IPC.MAPPING_UPDATE, id, patch),
  deleteMapping: (id: string) => ipcRenderer.invoke(IPC.MAPPING_DELETE, id),
  // Engine
  startEngine: () => ipcRenderer.invoke(IPC.ENGINE_START),
  stopEngine: () => ipcRenderer.invoke(IPC.ENGINE_STOP),
  reloadEngine: () => ipcRenderer.invoke(IPC.ENGINE_RELOAD),
  testEngine: () => ipcRenderer.invoke(IPC.ENGINE_TEST),
  engineStatus: () => ipcRenderer.invoke(IPC.ENGINE_STATUS),
}

contextBridge.exposeInMainWorld('electronAPI', api)

export type ElectronAPI = typeof api
```

- [ ] **Step 3: 实现 src/main/index.ts**

```typescript
import { join } from 'node:path'
import { app, BrowserWindow } from 'electron'
import { closeDb, openDb } from './db/connection'
import { getDefaultHost } from './db/hostRepo'
import { listEnabledMappings } from './db/mappingRepo'
import { registerIpcHandlers, setActiveEngine } from './ipc/handlers'
import { generateFullConf } from './nginx/confGenerator'
import { LocalNginxEngine } from './nginx/localEngine'
import { resolvePaths } from './paths'

// 关键：绿色版 - 禁用 userData 默认位置，全部用 exe 旁的 data/
// Electron 仍会创建默认 userData 目录（存放扩展/缓存），但我们自己的数据不进那里。
function createWindow() {
  const win = new BrowserWindow({
    width: 1100,
    height: 720,
    minWidth: 900,
    minHeight: 600,
    autoHideMenuBar: true,
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })

  if (process.env.ELECTRON_RENDERER_URL) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  }
  else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

async function initEngineForDefaultHost() {
  const host = getDefaultHost()
  if (!host || host.type !== 'local')
    return
  const paths = resolvePaths()
  const nginxPath = host.localConfig?.nginxPath
    || join(paths.bundledNginxDir, process.platform === 'win32' ? 'nginx.exe' : 'nginx')
  const confPath = join(paths.nginxConfDir, 'nginx.conf')
  const engine = new LocalNginxEngine({
    nginxPath,
    confPath,
    logDir: paths.nginxLogDir,
    tempDir: paths.tempDir
  })
  // 生成初始 conf（即使没 mapping，也要有最小 conf 让 nginx 能启动）
  const mappings = listEnabledMappings(host.id)
  const conf = generateFullConf(mappings, {
    nginxLogDir: paths.nginxLogDir,
    nginxTempDir: paths.tempDir
  })
  await engine.writeConf(conf)
  setActiveEngine(engine)
}

app.whenReady().then(async () => {
  const paths = resolvePaths()
  paths.ensureDataDirs()
  openDb()
  registerIpcHandlers()
  await initEngineForDefaultHost()
  createWindow()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0)
      createWindow()
  })
})

// 退出无痕：所有窗口关闭时退出 app（nginx 子进程在 before-quit 里杀）
app.on('window-all-closed', () => {
  app.quit()
})

// 关键：退出时确保 nginx 子进程被杀掉
app.on('before-quit', async (e) => {
  const eng = activeEngineRef
  if (eng && eng.status() === 'running') {
    e.preventDefault()
    await eng.stop()
    app.quit()
  }
  closeDb()
})

// 引用 activeEngine 用于退出钩子（handlers.ts 里的 setActiveEngine 同步设置这里）
let activeEngineRef: LocalNginxEngine | null = null
const _origSetEngine = setActiveEngine
// 覆盖：让 activeEngineRef 同步
;(setActiveEngine as any) = (eng: LocalNginxEngine | null) => {
  activeEngineRef = eng
  _origSetEngine(eng)
}
```

- [ ] **Step 4: 修正 main/index.ts 里的引擎引用（上面那段 hack 不好，重构）**

上面 Step 3 末尾用了个 hack 同步 `activeEngineRef`，不优雅。重构：把 activeEngine 的引用统一放在一个模块。

创建 `src/main/nginx/engineRegistry.ts`：

```typescript
import type { LocalNginxEngine } from './localEngine'
import type { NginxEngine } from './types'

let current: NginxEngine | null = null

export function setActiveEngine(eng: NginxEngine | null): void {
  current = eng
}

export function getActiveEngine(): NginxEngine | null {
  return current
}
```

然后修改 `src/main/ipc/handlers.ts`：删掉文件内的 `activeEngine` 变量和 `setActiveEngine`，改为从 `engineRegistry` 导入。

修改 `src/main/index.ts`：删掉末尾的 hack 段落，改用：
```typescript
import { getActiveEngine, setActiveEngine } from './nginx/engineRegistry'
// ...
app.on('before-quit', async (e) => {
  const eng = getActiveEngine()
  if (eng && eng.status() === 'running') {
    e.preventDefault()
    await eng.stop()
    app.quit()
  }
  closeDb()
})
```

- [ ] **Step 5: 同步修改 handlers.ts 使用 engineRegistry**

`src/main/ipc/handlers.ts` 顶部改为：
```typescript
import { getActiveEngine, setActiveEngine } from '../nginx/engineRegistry'

export { setActiveEngine }
```
所有 `activeEngine` 改为 `getActiveEngine()`。

- [ ] **Step 6: 类型检查**

Run: `npx tsc --noEmit -p tsconfig.node.json`
Expected: 无错误。

- [ ] **Step 7: 提交**

```bash
git add src/main/ src/preload/
git commit -m "feat(ipc): IPC handlers + preload bridge + main 入口 + 退出无痕钩子"
```

---

## Task 10: 前端最小 UI（TCP 列表 + 表单）

**Files:**
- Create: `src/renderer/api/ipc.ts`, `src/renderer/stores/mappings.ts`, `src/renderer/stores/hosts.ts`, `src/renderer/components/MappingList.vue`, `src/renderer/components/MappingForm.vue`, `src/renderer/components/EngineControl.vue`
- Modify: `src/renderer/App.vue`

最小能用的 UI，毛玻璃风格留到 Plan 4，这里先功能可用。

- [ ] **Step 1: 创建 src/renderer/api/ipc.ts**

```typescript
export interface ElectronAPI {
  listHosts: () => Promise<any[]>
  createHost: (input: any) => Promise<any>
  updateHost: (id: string, patch: any) => Promise<any>
  deleteHost: (id: string) => Promise<boolean>
  listMappings: (hostId: string) => Promise<any[]>
  createMapping: (input: any) => Promise<any>
  updateMapping: (id: string, patch: any) => Promise<any>
  deleteMapping: (id: string) => Promise<boolean>
  startEngine: () => Promise<string>
  stopEngine: () => Promise<string>
  reloadEngine: () => Promise<boolean>
  testEngine: () => Promise<{ ok: boolean, output: string }>
  engineStatus: () => Promise<string>
}

declare global {
  interface Window { electronAPI: ElectronAPI }
}

export const api: ElectronAPI = window.electronAPI
```

- [ ] **Step 2: 创建 src/renderer/stores/hosts.ts**

```typescript
import { api } from '@renderer/api/ipc'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Host {
  id: string
  name: string
  type: 'local' | 'remote'
  isDefault: number
}

export const useHostsStore = defineStore('hosts', () => {
  const hosts = ref<Host[]>([])
  const currentHostId = ref<string | null>(null)

  async function load() {
    hosts.value = await api.listHosts()
    const def = hosts.value.find(h => h.isDefault)
    currentHostId.value = def?.id ?? hosts.value[0]?.id ?? null
  }

  function setCurrent(id: string) { currentHostId.value = id }

  return { hosts, currentHostId, load, setCurrent }
})
```

- [ ] **Step 3: 创建 src/renderer/stores/mappings.ts**

```typescript
import { api } from '@renderer/api/ipc'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Mapping {
  id: string
  name: string
  enabled: number
  protocol: string
  listenPort: number | null
  upstreamHost: string
  upstreamPort: number
  healthStatus: string
}

export const useMappingsStore = defineStore('mappings', () => {
  const list = ref<Mapping[]>([])
  const loading = ref(false)

  async function load(hostId: string) {
    loading.value = true
    try {
      list.value = await api.listMappings(hostId)
    }
    finally {
      loading.value = false
    }
  }

  async function create(input: any) {
    await api.createMapping(input)
  }

  async function remove(id: string) {
    await api.deleteMapping(id)
  }

  async function toggle(id: string, enabled: number) {
    await api.updateMapping(id, { enabled })
  }

  return { list, loading, load, create, remove, toggle }
})
```

- [ ] **Step 4: 创建 src/renderer/components/MappingList.vue**

```vue
<script setup lang="ts">
import { useMappingsStore } from '@renderer/stores/mappings'

const emit = defineEmits<{ (e: 'new'): void, (e: 'edit', id: string): void }>()
const store = useMappingsStore()
</script>

<template>
  <div class="mapping-list">
    <div v-if="store.list.length === 0" class="empty">
      还没有映射规则，点下方按钮新建一条 TCP 转发试试
    </div>
    <div v-for="m in store.list" :key="m.id" class="card" @click="emit('edit', m.id)">
      <span class="dot" :class="m.healthStatus" />
      <div class="info">
        <div class="name">
          {{ m.name }}
        </div>
        <div class="meta">
          {{ m.protocol.toUpperCase() }} · {{ m.listenPort }} → {{ m.upstreamHost }}:{{ m.upstreamPort }}
        </div>
      </div>
      <input
        type="checkbox" :checked="m.enabled === 1"
        @click.stop @change="store.toggle(m.id, m.enabled === 1 ? 0 : 1)"
      >
    </div>
    <button class="new-btn" @click="emit('new')">
      + 新建映射
    </button>
  </div>
</template>

<style scoped>
.mapping-list { display: flex; flex-direction: column; gap: 8px; padding: 16px; }
.empty { color: #888; padding: 32px; text-align: center; }
.card { display: flex; align-items: center; gap: 12px; padding: 12px 16px;
  background: #fff; border: 1px solid #e5e5e5; border-radius: 12px; cursor: pointer; }
.card:hover { border-color: #0A84FF; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; }
.dot.healthy { background: #34c759; }
.dot.unhealthy { background: #ff3b30; }
.info { flex: 1; }
.name { font-weight: 600; }
.meta { font-size: 13px; color: #888; margin-top: 2px; }
.new-btn { margin-top: 8px; padding: 10px; border: none; border-radius: 12px;
  background: #0A84FF; color: #fff; font-size: 15px; cursor: pointer; }
</style>
```

- [ ] **Step 5: 创建 src/renderer/components/MappingForm.vue**

```vue
<script setup lang="ts">
import { api } from '@renderer/api/ipc'
import { ref, watch } from 'vue'

const props = defineProps<{ hostId: string, mappingId?: string }>()
const emit = defineEmits<{ (e: 'saved'): void, (e: 'cancel'): void }>()

const form = ref({
  name: '',
  protocol: 'tcp' as 'tcp' | 'udp' | 'http' | 'https',
  listenPort: null as number | null,
  upstreamHost: '',
  upstreamPort: null as number | null,
  extraDirectives: 'proxy_timeout 1h;',
})

// 仅 TCP 字段，其余协议 Plan 2 完善
const saving = ref(false)
const error = ref('')

async function loadExisting() {
  if (!props.mappingId)
    return
  const list = await api.listMappings(props.hostId)
  const m = list.find((x: any) => x.id === props.mappingId)
  if (m) {
    form.value = {
      name: m.name,
      protocol: m.protocol,
      listenPort: m.listenPort,
      upstreamHost: m.upstreamHost,
      upstreamPort: m.upstreamPort,
      extraDirectives: m.extraDirectives ?? 'proxy_timeout 1h;',
    }
  }
}
loadExisting()

async function save() {
  error.value = ''
  if (!form.value.name || !form.value.upstreamHost || !form.value.listenPort || !form.value.upstreamPort) {
    error.value = '请填写名称、监听端口、目标地址、目标端口'
    return
  }
  saving.value = true
  try {
    const input = {
      hostId: props.hostId,
      name: form.value.name,
      enabled: 1,
      protocol: form.value.protocol,
      listenPort: form.value.listenPort,
      serverNames: null,
      listenPath: null,
      upstreamHost: form.value.upstreamHost,
      upstreamPort: form.value.upstreamPort,
      tlsCertPath: null,
      tlsKeyPath: null,
      redirectHttp: 0,
      extraDirectives: form.value.extraDirectives,
      healthCheckEnabled: 0,
      healthCheckIntervalSec: 30,
      healthCheckTargetPath: null,
      healthCheckExpectedStatus: null,
    }
    if (props.mappingId)
      await api.updateMapping(props.mappingId, input)
    else await api.createMapping(input)
    emit('saved')
  }
  catch (e: any) {
    error.value = e.message
  }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="form-overlay" @click.self="emit('cancel')">
    <div class="form-card">
      <h3>{{ mappingId ? '编辑映射' : '新建映射（TCP）' }}</h3>
      <label>名称<input v-model="form.name" placeholder="如：MySQL 内库"></label>
      <label>协议
        <select v-model="form.protocol">
          <option value="tcp">TCP</option>
        </select>
        <small>UDP/HTTP/HTTPS 将在后续版本支持</small>
      </label>
      <label>监听端口<input v-model.number="form.listenPort" type="number" placeholder="9000"></label>
      <label>目标主机<input v-model="form.upstreamHost" placeholder="192.168.1.10"></label>
      <label>目标端口<input v-model.number="form.upstreamPort" type="number" placeholder="3306"></label>
      <label>附加指令（高级）<textarea v-model="form.extraDirectives" rows="2" /></label>
      <div v-if="error" class="error">
        {{ error }}
      </div>
      <div class="actions">
        <button class="ghost" @click="emit('cancel')">
          取消
        </button>
        <button :disabled="saving" @click="save">
          保存
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.3); display: flex;
  align-items: center; justify-content: center; }
.form-card { background: #fff; border-radius: 16px; padding: 24px; width: 420px; max-width: 90vw; }
h3 { margin: 0 0 16px; }
label { display: block; margin-bottom: 12px; font-size: 13px; color: #555; }
input, select, textarea { display: block; width: 100%; margin-top: 4px; padding: 8px;
  border: 1px solid #ddd; border-radius: 8px; font-size: 14px; }
small { color: #999; }
.error { color: #ff3b30; font-size: 13px; margin-bottom: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
button { padding: 8px 16px; border: none; border-radius: 8px; background: #0A84FF; color: #fff; cursor: pointer; }
button.ghost { background: #eee; color: #333; }
button:disabled { opacity: 0.5; }
</style>
```

- [ ] **Step 6: 创建 src/renderer/components/EngineControl.vue**

```vue
<script setup lang="ts">
import { api } from '@renderer/api/ipc'
import { onMounted, ref } from 'vue'

const status = ref('unknown')
const busy = ref(false)

async function refresh() { status.value = await api.engineStatus() }
async function start() {
  busy.value = true; try { status.value = await api.startEngine() }
  finally { busy.value = false }
}
async function stop() {
  busy.value = true; try { status.value = await api.stopEngine() }
  finally { busy.value = false }
}
async function reload() {
  busy.value = true
  try {
    const r = await api.testEngine()
    if (!r.ok) { alert(`配置校验失败:\n${r.output}`); return }
    await api.reloadEngine()
    status.value = await api.engineStatus()
  }
  finally { busy.value = false }
}

onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <div class="engine-ctrl">
    <span class="status-dot" :class="status" />
    <span class="status-text">{{ status }}</span>
    <button :disabled="busy || status === 'running'" @click="start">
      启动
    </button>
    <button :disabled="busy || status !== 'running'" @click="reload">
      应用
    </button>
    <button :disabled="busy || status !== 'running'" @click="stop">
      停止
    </button>
  </div>
</template>

<style scoped>
.engine-ctrl { display: flex; align-items: center; gap: 8px; padding: 8px 16px;
  background: #f5f5f7; border-radius: 12px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; }
.status-dot.running { background: #34c759; }
.status-dot.stopped { background: #ff3b30; }
.status-text { font-size: 13px; color: #555; margin-right: 8px; }
button { padding: 6px 12px; border: none; border-radius: 6px; background: #0A84FF;
  color: #fff; font-size: 13px; cursor: pointer; }
button:disabled { opacity: 0.4; }
</style>
```

- [ ] **Step 7: 重写 src/renderer/App.vue**

```vue
<script setup lang="ts">
import EngineControl from '@renderer/components/EngineControl.vue'
import MappingForm from '@renderer/components/MappingForm.vue'
import MappingList from '@renderer/components/MappingList.vue'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'
import { onMounted, ref, watch } from 'vue'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const showForm = ref(false)
const editingId = ref<string | undefined>()
const engineRef = ref()

async function reloadAll() {
  await hosts.load()
  if (hosts.currentHostId)
    await mappings.load(hosts.currentHostId)
}

onMounted(reloadAll)

watch(() => hosts.currentHostId, async (id) => {
  if (id)
    await mappings.load(id)
})

function openNew() { editingId.value = undefined; showForm.value = true }
function openEdit(id: string) { editingId.value = id; showForm.value = true }
async function onSaved() {
  showForm.value = false
  if (hosts.currentHostId)
    await mappings.load(hosts.currentHostId)
}
</script>

<template>
  <div class="layout">
    <header class="topbar">
      <h1>跳板机</h1>
      <select v-model="hosts.currentHostId" @change="mappings.load(hosts.currentHostId!)">
        <option v-for="h in hosts.hosts" :key="h.id" :value="h.id">
          {{ h.name }}
        </option>
      </select>
      <EngineControl ref="engineRef" />
    </header>
    <main>
      <MappingList @new="openNew" @edit="openEdit" />
    </main>
    <MappingForm
      v-if="showForm"
      :host-id="hosts.currentHostId!"
      :mapping-id="editingId"
      @saved="onSaved"
      @cancel="showForm = false"
    />
  </div>
</template>

<style scoped>
.layout { display: flex; flex-direction: column; height: 100vh; background: #f5f5f7; }
.topbar { display: flex; align-items: center; gap: 16px; padding: 12px 24px;
  background: rgba(255,255,255,0.7); backdrop-filter: blur(20px); border-bottom: 1px solid rgba(0,0,0,0.06); }
h1 { font-size: 18px; margin: 0; font-weight: 600; }
select { padding: 6px 10px; border-radius: 8px; border: 1px solid #ddd; }
main { flex: 1; overflow-y: auto; }
</style>
```

- [ ] **Step 8: 类型检查**

Run: `npx vue-tsc --noEmit`
Expected: 无错误（可能有若干 any 警告，本 plan 接受）。

- [ ] **Step 9: 提交**

```bash
git add src/renderer/
git commit -m "feat(ui): 最小 UI - 映射列表 + TCP 表单 + 引擎控制（毛玻璃留 Plan 4）"
```

---

## Task 11: 内置 nginx 资源准备 + 默认 Host 初始化

**Files:**
- Create: `resources/nginx/.gitkeep`, `resources/icon.png`
- Modify: `src/main/index.ts`（首次启动初始化默认 local host）

内置 nginx.exe 是二进制文件，不放进 git。本 task 只建占位 + 首次启动逻辑，真正放 nginx.exe 在 Plan 6 打包前。

- [ ] **Step 1: 创建占位文件**

```
resources/nginx/.gitkeep   （空文件）
resources/icon.png          （放一个临时图标）
```

- [ ] **Step 2: 修改 src/main/index.ts 增加首次启动初始化**

在 `app.whenReady` 回调里、`openDb()` 之后，加：

```typescript
import { listHosts } from './db/hostRepo'
// ...
// 首次启动：如果没有 host，创建默认 local host
if (listHosts().length === 0) {
  const { createHost, setDefaultHost } = require('./db/hostRepo')
  const h = createHost({
    name: '本机网关',
    type: 'local',
    isDefault: 1,
    localConfig: { nginxPath: '', confDir: '', logDir: '' },
    remoteConfig: null
  })
  setDefaultHost(h.id)
}
```

- [ ] **Step 3: 类型检查**

Run: `npx tsc --noEmit -p tsconfig.node.json`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add resources/ src/main/index.ts
git commit -m "feat: nginx 资源占位 + 首次启动默认 host 初始化"
```

---

## Task 12: 端到端手动验证（TCP 闭环）

**Files:** 无新文件，验证性 task。

这个 task 验证整个 Plan 1 的核心闭环：建一条 TCP 映射 → nginx 启动 → 外部能连通目标服务。

> 前置：需要在 `resources/nginx/` 放入 Windows 版 nginx.exe（从 nginx.org 下载稳定版解压）。开发环境在 macOS，真实验证需在 Windows 上做；此处用单元测试 + 手动步骤说明。

- [ ] **Step 1: 全量单元测试通过**

Run: `npx vitest run`
Expected: 所有测试 PASS（paths / db connection / hostRepo / mappingRepo / confGenerator / localEngine）。

- [ ] **Step 2: 类型检查全通过**

Run: `npm run typecheck`
Expected: 无错误。

- [ ] **Step 3: 开发模式启动验证（macOS 本地无 nginx.exe 会失败，属正常）**

Run: `npm run dev`
Expected: Electron 窗口打开，显示空映射列表 + 引擎控制条 + "新建映射"按钮。点"启动"因无 nginx.exe 会报错（正常）。

- [ ] **Step 4: 手动验证清单（在 Windows 上做）**

把项目拷到 Windows 机器，在 `resources/nginx/` 放入 nginx.exe，运行 `npm run dev`：

1. ✅ 窗口打开，顶部显示"本机网关"
2. ✅ 点"新建映射"，填：名称=测试、监听端口=9000、目标主机=127.0.0.1、目标端口=80、保存
3. ✅ 列表出现该映射
4. ✅ 点"启动"，引擎状态变 running
5. ✅ 浏览器访问 `http://localhost:9000`，能看到 nginx 默认页（转发到本机 80）
6. ✅ 点"停止"，再访问 9000 不通
7. ✅ 关闭窗口，任务管理器无 nginx 进程残留（退出无痕验证）

- [ ] **Step 5: 提交最终状态**

```bash
git add -A
git commit -m "test: Plan 1 全量验证通过（单元测试 + 手动 TCP 闭环）"
```

---

## Plan 1 完成标准（Definition of Done）

- [ ] 所有 12 个 task 完成
- [ ] `npm run typecheck` 无错误
- [ ] `npx vitest run` 全部 PASS
- [ ] 在 Windows 上能完成 Task 12 Step 4 的 7 项手动验证
- [ ] 绿色版验证：程序退出后 `data/` 之外无写入，无 nginx 进程残留

---

## 后续 Plan 预告（本 plan 不实现）

- **Plan 2**: UDP/HTTP/HTTPS 配置生成 + 13 个模板系统 + nginx -t 校验流程
- **Plan 3**: 健康检查轮询 + 配置历史回滚 + 日志系统 + 端口检测
- **Plan 4**: 毛玻璃 iOS 风格 UI 设计系统 + 全页面视觉重构
- **Plan 5**: 远程 nginx 管理（SSH2 引擎 + SFTP）
- **Plan 6**: electron-builder portable 打包 + 一键清除 + 导入导出
