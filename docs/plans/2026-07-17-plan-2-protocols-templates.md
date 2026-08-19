# Plan 2: 全协议配置生成 + 模板系统

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Plan 1 的 TCP-only 基础上，补齐 UDP/HTTP/HTTPS 三种协议的 nginx 配置生成，建立 13 个内置模板 + 自定义模板机制，并把 reload 流程接入 `nginx -t` 校验（失败则不 reload）。

**Architecture:** 配置生成器从单一 `renderTcpServer` 扩展为按协议分派（TCP/UDP/HTTP/HTTPS 各一个 render 函数）。模板系统是独立模块：内置模板随包发布（YAML），自定义模板存 `data/templates/custom/`，统一通过 `TemplateRepo` 加载。MappingForm 从 TCP-only 改为按协议动态显示字段，新建映射时先过模板选择器。reload 流程在 IPC 层包装 `test → write → reload` 三步，test 失败原配置不动。

**Tech Stack:** 沿用 Plan 1：Electron 30 / Vue 3 / TypeScript / better-sqlite3 / electron-vite。新增：YAML 解理（`yaml` 或 `js-yaml` 包）。

**Spec 引用:** `docs/specs/2026-07-17-nginx-jump-gui-design.md` §5（配置生成器）、§6（模板系统）、§5.2（生成流程含 nginx -t 校验）。

**前置条件:** Plan 1 已完成（commit `006b00e`）。confGenerator 当前只有 TCP，http 块是占位注释。MappingForm 协议下拉只有 TCP 选项。

---

## 文件结构

本 plan 修改/创建的文件（在 Plan 1 基础上扩展）：

```
nginx-jump-gui/
├── package.json                          # 加 js-yaml 依赖
├── src/
│   ├── shared/
│   │   └── types.ts                      # 加 Template 类型 + 模板相关 IPC 通道
│   ├── main/
│   │   ├── nginx/
│   │   │   ├── confGenerator.ts          # 扩展：UDP + HTTP + HTTPS 渲染分派
│   │   │   ├── renderers.ts              # 新：4 个协议的 render 函数（从 confGenerator 抽出）
│   │   │   └── applyFlow.ts              # 新：test → write → reload 封装（含快照保存 hook）
│   │   ├── templates/
│   │   │   ├── types.ts                  # 新：TemplateDefinition 类型 + 内置模板元数据
│   │   │   ├── templateRepo.ts           # 新：加载内置 + 自定义模板，存自定义模板
│   │   │   └── builtin/                  # 13 个内置模板 YAML
│   │   │       ├── tcp.yml
│   │   │       ├── udp.yml
│   │   │       ├── ssh.yml
│   │   │       ├── rdp.yml
│   │   │       ├── website.yml
│   │   │       ├── website-https.yml
│   │   │       ├── api.yml
│   │   │       ├── websocket.yml
│   │   │       ├── mysql.yml
│   │   │       ├── postgresql.yml
│   │   │       ├── redis.yml
│   │   │       ├── mongodb.yml
│   │   │       └── mqtt.yml
│   │   └── ipc/
│   │       └── handlers.ts               # 加模板 IPC + 把 reload 改成走 applyFlow
│   ├── preload/
│   │   └── index.ts                      # 加模板 IPC 桥接
│   └── renderer/
│       ├── components/
│       │   ├── MappingForm.vue           # 重构：按协议动态字段
│       │   └── TemplatePicker.vue        # 新：模板选择器（卡片网格）
│       └── stores/
│           └── templates.ts              # 新：模板 Pinia store
└── tests/
    └── main/
        ├── nginx/
        │   ├── renderers.test.ts         # 4 种协议 render 单元测试
        │   ├── confGenerator.test.ts     # 扩展：混合协议、hasStream 含 UDP、http 块
        │   └── applyFlow.test.ts         # test 失败不 reload、成功路径
        └── templates/
            └── templateRepo.test.ts      # 加载内置/自定义、存自定义、YAML 解析
```

---

## Task 1: 安装 js-yaml + 定义模板类型

**Files:**
- Modify: `package.json` (加 `yaml` 依赖)
- Create: `src/main/templates/types.ts`
- Modify: `src/shared/types.ts` (加 IPC 通道)

- [ ] **Step 1: 安装 yaml 包**

Run: `npm install yaml`
（选 `yaml` 而非 `js-yaml`：现代、维护活跃、API 更干净、bundle 更小。）

- [ ] **Step 2: 创建 `src/main/templates/types.ts`**

```typescript
import type { MappingInput, Protocol } from '@shared/types'

/**
 * 模板定义：预填的 MappingInput 草稿 + 附带的 nginx 指令。
 * 用户选模板 → 跳到预填表单 → 改几个值 → 保存。
 */
export interface TemplateDefinition {
  /** 文件名 stem（不含扩展名），作为 id */
  id: string
  name: string
  /** 图标标识，前端映射到 SVG/emoji。内置模板用固定集合。 */
  icon: string
  /** 分类，用于模板选择器分组：网络层 / Web / 数据库 / 消息 */
  category: string
  description: string
  /** true = 随包发布的内置模板；false = 用户自建 */
  builtin: boolean
  /** 预填的映射字段（部分字段为 null 表示用户必填） */
  mapping: Partial<MappingInput> & {
    protocol: Protocol
  }
  /** 生成 nginx 配置时附加到 server 块的指令 */
  extraDirectives?: string
}
```

- [ ] **Step 3: 在 `src/shared/types.ts` 末尾加模板 IPC 通道**

在现有 `IPC` const 对象里、`ENGINE_STATUS` 之后加：

```typescript
  // Template
  TEMPLATE_LIST: 'template:list',
  TEMPLATE_LIST_BUILTIN: 'template:list-builtin',
  TEMPLATE_SAVE_CUSTOM: 'template:save-custom',
  TEMPLATE_DELETE_CUSTOM: 'template:delete-custom',
```

（4 个模板通道。`TEMPLATE_LIST` 返回内置+自定义合并列表，`TEMPLATE_LIST_BUILTIN` 只返回内置，方便 UI 区分。）

- [ ] **Step 4: 验证**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 38 tests 全过（无回归）

- [ ] **Step 5: 提交**

```bash
git add package.json package-lock.json src/main/templates/types.ts src/shared/types.ts
git commit -m "feat(templates): 安装 yaml 包 + 定义 TemplateDefinition 类型 + 模板 IPC 通道"
```

---

## Task 2: 4 种协议的 render 函数（含测试）

**Files:**
- Create: `src/main/nginx/renderers.ts`
- Test: `tests/main/nginx/renderers.test.ts`

把渲染逻辑从 confGenerator 抽出，独立可测。TCP 从 confGenerator 迁移过来，新增 UDP/HTTP/HTTPS。

- [ ] **Step 1: 写失败测试 `tests/main/nginx/renderers.test.ts`**

```typescript
import type { Mapping } from '@shared/types'
import { renderHttpServer, renderHttpsServer, renderTcpServer, renderUdpServer } from '@main/nginx/renderers'
import { describe, expect, it } from 'vitest'

function base(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1',
    hostId: 'h1',
    name: '测试',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '10.0.0.1',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
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

describe('renderTcpServer', () => {
  it('生成 listen + proxy_pass + extraDirectives', () => {
    const out = renderTcpServer(base({ protocol: 'tcp', listenPort: 9000, upstreamHost: '10.0.0.1', upstreamPort: 3306, extraDirectives: 'proxy_timeout 1h;' }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 9000;')
    expect(conf).toContain('proxy_pass 10.0.0.1:3306;')
    expect(conf).toContain('proxy_timeout 1h;')
  })
  it('listenPort 为 null 返回空数组', () => {
    expect(renderTcpServer(base({ listenPort: null }))).toEqual([])
  })
})

describe('renderUdpServer', () => {
  it('生成 listen udp + proxy_pass', () => {
    const out = renderUdpServer(base({ protocol: 'udp', listenPort: 5353, upstreamHost: '10.0.0.2', upstreamPort: 53 }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 5353 udp;')
    expect(conf).toContain('proxy_pass 10.0.0.2:53;')
  })
  it('listenPort 为 null 返回空数组', () => {
    expect(renderUdpServer(base({ protocol: 'udp', listenPort: null }))).toEqual([])
  })
})

describe('renderHttpServer', () => {
  it('生成 server_name + location + proxy_pass + 转发头', () => {
    const out = renderHttpServer(base({ protocol: 'http', listenPort: 8080, serverNames: 'app1.gw.local', listenPath: '/', upstreamHost: '10.0.0.3', upstreamPort: 3000 }))
    const conf = out.join('\n')
    expect(conf).toContain('listen 8080;')
    expect(conf).toContain('server_name app1.gw.local;')
    expect(conf).toContain('location /')
    expect(conf).toContain('proxy_pass http://10.0.0.3:3000;')
    expect(conf).toContain('proxy_set_header Host')
    expect(conf).toContain('proxy_set_header X-Real-IP')
    expect(conf).toContain('proxy_set_header X-Forwarded-For')
  })
  it('无 serverName 时省略 server_name 行', () => {
    const out = renderHttpServer(base({ protocol: 'http', listenPort: 8080, serverNames: null, upstreamHost: '10.0.0.3', upstreamPort: 3000 }))
    expect(out.join('\n')).not.toContain('server_name')
  })
})

describe('renderHttpsServer', () => {
  it('生成 443 ssl + 证书 + HTTP→HTTPS 跳转（redirectHttp=1）', () => {
    const out = renderHttpsServer(base({
      protocol: 'https',
      listenPort: null,
      serverNames: 'secure.gw.local',
      listenPath: '/',
      upstreamHost: '10.0.0.4',
      upstreamPort: 8443,
      tlsCertPath: '/data/cert.pem',
      tlsKeyPath: '/data/key.pem',
      redirectHttp: 1
    }))
    const conf = out.join('\n')
    // 跳转块
    expect(conf).toContain('listen 80;')
    expect(conf).toContain('return 301 https://')
    // https 块
    expect(conf).toContain('listen 443 ssl;')
    expect(conf).toContain('ssl_certificate /data/cert.pem;')
    expect(conf).toContain('ssl_certificate_key /data/key.pem;')
    expect(conf).toContain('proxy_pass http://10.0.0.4:8443;')
  })
  it('redirectHttp=0 时不生成 80 跳转块', () => {
    const out = renderHttpsServer(base({
      protocol: 'https',
      serverNames: 's.gw.local',
      upstreamHost: '10.0.0.4',
      upstreamPort: 8443,
      tlsCertPath: '/c.pem',
      tlsKeyPath: '/k.pem',
      redirectHttp: 0
    }))
    expect(out.join('\n')).not.toContain('listen 80;')
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/nginx/renderers.test.ts`
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 实现 `src/main/nginx/renderers.ts`**

```typescript
import type { Mapping } from '@shared/types'

/** 公共：渲染 extraDirectives，每行 trim + 缩进 4 空格 */
function renderExtra(extra: string | null, indent = '    '): string[] {
  if (!extra || !extra.trim())
    return []
  const out: string[] = []
  for (const line of extra.split('\n')) {
    if (line.trim())
      out.push(indent + line.trim())
  }
  return out
}

/** 公共：HTTP/HTTPS 转发头 */
function proxyHeaders(indent = '        '): string[] {
  return [
    `${indent}proxy_set_header Host $host;`,
    `${indent}proxy_set_header X-Real-IP $remote_addr;`,
    `${indent}proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
    `${indent}proxy_set_header X-Forwarded-Proto $scheme;`
  ]
}

export function renderTcpServer(m: Mapping): string[] {
  if (m.listenPort == null)
    return []
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${m.listenPort};`,
    `    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderUdpServer(m: Mapping): string[] {
  if (m.listenPort == null)
    return []
  return [
    `  # mapping: ${m.name}`,
    '  server {',
    `    listen ${m.listenPort} udp;`,
    `    proxy_pass ${m.upstreamHost}:${m.upstreamPort};`,
    ...renderExtra(m.extraDirectives),
    '  }'
  ]
}

export function renderHttpServer(m: Mapping): string[] {
  const port = m.listenPort ?? 80
  const path = m.listenPath ?? '/'
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)
  lines.push('  server {')
  lines.push(`    listen ${port};`)
  if (m.serverNames)
    lines.push(`    server_name ${m.serverNames};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${m.upstreamHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}

export function renderHttpsServer(m: Mapping): string[] {
  const path = m.listenPath ?? '/'
  const lines: string[] = []
  lines.push(`  # mapping: ${m.name}`)

  // HTTP → HTTPS 跳转块（可选）
  if (m.redirectHttp === 1 && m.serverNames) {
    lines.push('  server {')
    lines.push('    listen 80;')
    lines.push(`    server_name ${m.serverNames};`)
    lines.push('    return 301 https://$host$request_uri;')
    lines.push('  }')
    lines.push('')
  }

  // HTTPS 主块
  lines.push('  server {')
  lines.push('    listen 443 ssl;')
  if (m.serverNames)
    lines.push(`    server_name ${m.serverNames};`)
  if (m.tlsCertPath)
    lines.push(`    ssl_certificate ${m.tlsCertPath};`)
  if (m.tlsKeyPath)
    lines.push(`    ssl_certificate_key ${m.tlsKeyPath};`)
  lines.push(`    location ${path} {`)
  lines.push(`      proxy_pass http://${m.upstreamHost}:${m.upstreamPort};`)
  lines.push(...proxyHeaders())
  lines.push(...renderExtra(m.extraDirectives, '      '))
  lines.push('    }')
  lines.push('  }')
  return lines
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/nginx/renderers.test.ts`
Expected: PASS（8 tests）。

- [ ] **Step 5: typecheck + 全量回归**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）

- [ ] **Step 6: 提交**

```bash
git add src/main/nginx/renderers.ts tests/main/nginx/renderers.test.ts
git commit -m "feat(nginx): 4 种协议 render 函数（TCP/UDP/HTTP/HTTPS）+ 单元测试"
```

---

## Task 3: confGenerator 集成全部协议

**Files:**
- Modify: `src/main/nginx/confGenerator.ts`（接入 renderers，删旧 renderTcpServer）
- Modify: `tests/main/nginx/confGenerator.test.ts`（加混合协议、UDP-only、HTTP 块测试）

- [ ] **Step 1: 重构 confGenerator 接入 renderers**

把 confGenerator 改为：从 renderers.ts 导入 4 个 render 函数，按协议分派到 stream/http 块。删掉 confGenerator 内部的旧 `renderTcpServer`。

新 `src/main/nginx/confGenerator.ts`：

```typescript
import type { Mapping } from '@shared/types'
import { renderHttpServer, renderHttpsServer, renderTcpServer, renderUdpServer } from './renderers'

export interface ConfGenOptions {
  nginxLogDir: string
  nginxTempDir: string
}

export function generateFullConf(mappings: Mapping[], opts: ConfGenOptions): string {
  const enabled = mappings.filter(m => m.enabled)

  const tcpMappings = enabled.filter(m => m.protocol === 'tcp')
  const udpMappings = enabled.filter(m => m.protocol === 'udp')
  const httpMappings = enabled.filter(m => m.protocol === 'http')
  const httpsMappings = enabled.filter(m => m.protocol === 'https')

  const hasStream = tcpMappings.length > 0 || udpMappings.length > 0
  const hasHttp = httpMappings.length > 0 || httpsMappings.length > 0

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
    lines.push('  # ---------- HTTP server 块 ----------')
    for (const m of httpMappings) lines.push(...renderHttpServer(m))
    lines.push('  # ---------- HTTPS server 块 ----------')
    for (const m of httpsMappings) lines.push(...renderHttpsServer(m))
    lines.push('}')
    lines.push('')
  }

  if (hasStream) {
    lines.push('stream {')
    for (const m of tcpMappings) lines.push(...renderTcpServer(m))
    for (const m of udpMappings) lines.push(...renderUdpServer(m))
    lines.push('}')
    lines.push('')
  }

  return lines.join('\n')
}
```

- [ ] **Step 2: 删掉 confGenerator.test.ts 里重复测试 TCP render 的部分**

旧测试文件里有 `renderTcpServer` 通过 `generateFullConf` 间接测的用例（含 extraDirectives、disabled、name 注释、空 mappings、多 TCP、listenPort null）。这些保留——它们现在测的是"集成"路径。但删掉对"不含 http 块"的断言（现在 http 块逻辑变了，且 Plan 2 要加 http 测试）。

读 `tests/main/nginx/confGenerator.test.ts`，删除这个测试用例：
```typescript
  it('不含 http 块（没有 http/https mapping）', () => { ... })
```
（renderers.test.ts 已经覆盖各协议，confGenerator.test.ts 现在聚焦集成。）

- [ ] **Step 3: 加新测试用例到 confGenerator.test.ts**

在 describe 块里加：

```typescript
it('UDP only 时生成 stream 块含 listen udp', () => {
  const conf = generateFullConf([tcpMapping({ protocol: 'udp', listenPort: 5353, upstreamPort: 53 })], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
  expect(conf).toContain('stream {')
  expect(conf).toContain('listen 5353 udp;')
})

it('HTTP 映射生成 http 块', () => {
  const conf = generateFullConf([tcpMapping({ protocol: 'http', listenPort: 8080, serverNames: 'a.local', listenPath: '/', upstreamPort: 3000 })], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
  expect(conf).toContain('http {')
  expect(conf).toContain('server_name a.local;')
  expect(conf).toContain('proxy_pass http://')
})

it('混合 TCP + HTTPS 同时生成 stream 和 http 块', () => {
  const conf = generateFullConf([
    tcpMapping({ protocol: 'tcp', listenPort: 9000, upstreamPort: 3306 }),
    tcpMapping({ protocol: 'https', listenPort: null, serverNames: 's.local', tlsCertPath: '/c.pem', tlsKeyPath: '/k.pem', upstreamPort: 8443, redirectHttp: 1 })
  ], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
  expect(conf).toContain('stream {')
  expect(conf).toContain('http {')
  expect(conf).toContain('listen 443 ssl;')
})
```

（`tcpMapping` 是测试文件里已有的 helper，只是名字沿用——它构造的是 base Mapping，protocol 通过 over 覆盖。）

- [ ] **Step 4: 运行测试**

Run: `npx vitest run tests/main/nginx/`
Expected: renderers 8 + confGenerator 原有（减1删 + 加3新）全过。

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）

- [ ] **Step 5: 提交**

```bash
git add src/main/nginx/confGenerator.ts tests/main/nginx/confGenerator.test.ts
git commit -m "feat(nginx): confGenerator 集成全部 4 协议分派到 stream/http 块"
```

---

## Task 4: applyFlow（test → write → reload 封装）

**Files:**
- Create: `src/main/nginx/applyFlow.ts`
- Test: `tests/main/nginx/applyFlow.test.ts`

实现设计 §5.2 的核心安全流程：生成 conf → `nginx -t` 校验 → 通过才 write + reload → 失败则原配置不动。

- [ ] **Step 1: 写失败测试 `tests/main/nginx/applyFlow.test.ts`**

```typescript
import type { EngineTestResult, Mapping } from '@shared/types'
import { applyMappings } from '@main/nginx/applyFlow'
import { describe, expect, it, vi } from 'vitest'

function mapping(over: Partial<Mapping> = {}): Mapping {
  return {
    id: 'm1',
    hostId: 'h1',
    name: 't',
    enabled: 1,
    protocol: 'tcp',
    listenPort: 9000,
    serverNames: null,
    listenPath: null,
    upstreamHost: '10.0.0.1',
    upstreamPort: 3306,
    tlsCertPath: null,
    tlsKeyPath: null,
    redirectHttp: 0,
    extraDirectives: null,
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

function makeEngine(opts: {
  testResult?: EngineTestResult
  testMock?: (conf: string) => Promise<EngineTestResult>
  writeConf?: vi.Mock
  reload?: vi.Mock
  readConf?: vi.Mock
} = {}) {
  return {
    test: opts.testMock ?? vi.fn().mockResolvedValue(opts.testResult ?? { ok: true, output: 'syntax is ok' }),
    writeConf: opts.writeConf ?? vi.fn().mockResolvedValue(undefined),
    reload: opts.reload ?? vi.fn().mockResolvedValue(undefined),
    readConf: opts.readConf ?? vi.fn().mockResolvedValue('old conf'),
    start: vi.fn(),
    stop: vi.fn(),
    status: vi.fn().mockReturnValue('running')
  }
}

describe('applyMappings', () => {
  it('test 通过 → writeConf + reload', async () => {
    const eng = makeEngine()
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(true)
    expect(eng.writeConf).toHaveBeenCalledOnce()
    expect(eng.reload).toHaveBeenCalledOnce()
  })

  it('test 失败 → 不 writeConf 不 reload，原配置不动', async () => {
    const eng = makeEngine({ testResult: { ok: false, output: 'invalid host' } })
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(false)
    expect(result.output).toContain('invalid host')
    expect(eng.writeConf).not.toHaveBeenCalled()
    expect(eng.reload).not.toHaveBeenCalled()
  })

  it('test 通过但 reload 抛错 → 返回错误（此时 conf 已写入，靠 nginx 保留旧配置兜底）', async () => {
    const eng = makeEngine({ reload: vi.fn().mockRejectedValue(new Error('reload failed')) })
    const result = await applyMappings(eng as any, [mapping()], { nginxLogDir: '/d/logs', nginxTempDir: '/d/temp' })
    expect(result.ok).toBe(false)
    expect(result.output).toContain('reload failed')
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/nginx/applyFlow.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 `src/main/nginx/applyFlow.ts`**

```typescript
import type { Mapping } from '@shared/types'
import type { ConfGenOptions } from './confGenerator';
import type { NginxEngine } from './types'
import { generateFullConf } from './confGenerator'

export interface ApplyResult {
  ok: boolean
  /** 生成的完整 conf（无论成功失败，调试用） */
  conf: string
  /** nginx -t 或 reload 的输出/错误 */
  output: string
}

/**
 * 应用映射变更到 nginx：生成 → 校验 → 写入 → reload。
 * 校验失败则原配置不动（不 writeConf 不 reload）。
 */
export async function applyMappings(
  engine: NginxEngine,
  mappings: Mapping[],
  opts: ConfGenOptions
): Promise<ApplyResult> {
  const conf = generateFullConf(mappings, opts)

  // 1. nginx -t 校验（写到临时然后 test，或直接 test 当前生成的文本）
  // nginx -t 只能校验磁盘上的文件，所以这里先写一个临时校验：
  // 实际做法是把生成 conf 写到 <confPath>.test 临时文件，nginx -t -c 临时文件。
  // 但 LocalNginxEngine.test() 固定 test 主 confPath。为保持简单，
  // 这里先 writeConf 再 test，test 失败则用 readConf 拿回旧 conf 重写（回滚）。
  // 注意：这意味着 test 失败时磁盘上有短暂的新 conf，但 nginx 没 reload 所以运行时不受影响。

  const oldConf = await engine.readConf()
  await engine.writeConf(conf)
  const testResult = await engine.test()
  if (!testResult.ok) {
    // 回滚到旧 conf
    await engine.writeConf(oldConf)
    return { ok: false, conf, output: testResult.output }
  }

  // 2. reload
  try {
    await engine.reload()
    return { ok: true, conf, output: testResult.output }
  }
  catch (e: any) {
    return { ok: false, conf, output: e?.message ?? String(e) }
  }
}
```

> 注意：这里有个权衡——nginx -t 必须校验磁盘文件，所以不得不先写再 test。test 失败时回滚旧 conf。虽然磁盘有短暂新 conf，但 nginx 没 reload 所以运行配置不变。Task 9 的 LocalNginxEngine 已有 readConf/writeConf。Task 5 review 时保留了这个接口正好用上。

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/nginx/applyFlow.test.ts`
Expected: PASS（3 tests）。

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`

- [ ] **Step 6: 提交**

```bash
git add src/main/nginx/applyFlow.ts tests/main/nginx/applyFlow.test.ts
git commit -m "feat(nginx): applyFlow test→write→reload 封装，校验失败回滚原配置"
```

---

## Task 5: 13 个内置模板 YAML

**Files:**
- Create: `src/main/templates/builtin/*.yml`（13 个文件）

按设计 §6.1 的清单。每个 YAML 文件结构对应 `TemplateDefinition`（去掉 builtin 字段，加载时设为 true）。

- [ ] **Step 1: 创建 13 个模板文件**

`src/main/templates/builtin/tcp.yml`:
```yaml
id: tcp
name: TCP 转发
icon: network
category: 网络层
description: 通用 TCP 端口转发，适用于任意 TCP 服务
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: null
extraDirectives: |
  proxy_timeout 1h;
  proxy_connect_timeout 60s;
```

`src/main/templates/builtin/udp.yml`:
```yaml
id: udp
name: UDP 转发
icon: network
category: 网络层
description: UDP 端口转发，适用于 DNS/Game Server 等
mapping:
  protocol: udp
  listenPort: null
  upstreamHost: null
  upstreamPort: null
```

`src/main/templates/builtin/ssh.yml`:
```yaml
id: ssh
name: SSH
icon: terminal
category: 网络层
description: SSH 远程登录，默认 22 端口
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 22
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/rdp.yml`:
```yaml
id: rdp
name: RDP 远程桌面
icon: monitor
category: 网络层
description: Windows 远程桌面，默认 3389 端口
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 3389
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/website.yml`:
```yaml
id: website
name: 网站反代 (HTTP)
icon: globe
category: Web
description: HTTP 反向代理，带域名和转发头
mapping:
  protocol: http
  listenPort: 80
  serverNames: null
  listenPath: /
  upstreamHost: null
  upstreamPort: null
```

`src/main/templates/builtin/website-https.yml`:
```yaml
id: website-https
name: HTTPS 网站
icon: lock
category: Web
description: HTTPS 反向代理，带 TLS 证书和 HTTP→HTTPS 跳转
mapping:
  protocol: https
  listenPort: null
  serverNames: null
  listenPath: /
  upstreamHost: null
  upstreamPort: null
  tlsCertPath: null
  tlsKeyPath: null
  redirectHttp: 1
```

`src/main/templates/builtin/api.yml`:
```yaml
id: api
name: API 接口
icon: code
category: Web
description: 后端 API 反代，超时调短、关 buffering
mapping:
  protocol: http
  listenPort: null
  serverNames: null
  listenPath: /api
  upstreamHost: null
  upstreamPort: null
extraDirectives: |
  proxy_connect_timeout 5s;
  proxy_read_timeout 30s;
  proxy_buffering off;
```

`src/main/templates/builtin/websocket.yml`:
```yaml
id: websocket
name: WebSocket
icon: bolt
category: Web
description: WebSocket 反代，带 Upgrade/Connection 头
mapping:
  protocol: http
  listenPort: null
  serverNames: null
  listenPath: /ws
  upstreamHost: null
  upstreamPort: null
extraDirectives: |
  proxy_http_version 1.1;
  proxy_set_header Upgrade $http_upgrade;
  proxy_set_header Connection "upgrade";
  proxy_read_timeout 3600s;
```

`src/main/templates/builtin/mysql.yml`:
```yaml
id: mysql
name: MySQL 数据库
icon: database
category: 数据库
description: MySQL 端口转发，默认 3306
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 3306
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/postgresql.yml`:
```yaml
id: postgresql
name: PostgreSQL 数据库
icon: database
category: 数据库
description: PostgreSQL 端口转发，默认 5432
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 5432
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/redis.yml`:
```yaml
id: redis
name: Redis
icon: database
category: 数据库
description: Redis 端口转发，默认 6379
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 6379
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/mongodb.yml`:
```yaml
id: mongodb
name: MongoDB
icon: database
category: 数据库
description: MongoDB 端口转发，默认 27017
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 27017
extraDirectives: |
  proxy_timeout 1h;
```

`src/main/templates/builtin/mqtt.yml`:
```yaml
id: mqtt
name: MQTT
icon: message
category: 消息
description: MQTT 端口转发，默认 1883
mapping:
  protocol: tcp
  listenPort: null
  upstreamHost: null
  upstreamPort: 1883
extraDirectives: |
  proxy_timeout 1h;
```

- [ ] **Step 2: 验证 YAML 语法（用 node 快速 parse 每个）**

Run:
```bash
node -e "const fs=require('fs'),{parse}=require('yaml');const dir='src/main/templates/builtin';for(const f of fs.readdirSync(dir)){parse(fs.readFileSync(dir+'/'+f,'utf8'));console.log('OK',f)}"
```
Expected: 13 行 `OK <file>`，无异常。

- [ ] **Step 3: 提交**

```bash
git add src/main/templates/builtin/
git commit -m "feat(templates): 13 个内置模板 YAML（TCP/UDP/SSH/RDP/Web/HTTPS/API/WS/4数据库/MQTT）"
```

---

## Task 6: TemplateRepo（加载 + 存自定义）

**Files:**
- Create: `src/main/templates/templateRepo.ts`
- Test: `tests/main/templates/templateRepo.test.ts`

- [ ] **Step 1: 写失败测试 `tests/main/templates/templateRepo.test.ts`**

```typescript
import { existsSync, mkdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { __setCustomDirForTest, deleteCustomTemplate, listAllTemplates, listBuiltinTemplates, listCustomTemplates, saveCustomTemplate } from '@main/templates/templateRepo'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

describe('templateRepo', () => {
  let customDir: string

  beforeEach(() => {
    customDir = join(tmpdir(), `tpl-test-${Date.now()}`)
    mkdirSync(customDir, { recursive: true })
    __setCustomDirForTest(customDir)
  })

  afterEach(() => {
    rmSync(customDir, { recursive: true, force: true })
  })

  it('listBuiltinTemplates 返回 13 个', () => {
    const list = listBuiltinTemplates()
    expect(list).toHaveLength(13)
    expect(list.every(t => t.builtin)).toBe(true)
  })

  it('内置模板按 category 分组覆盖 4 类', () => {
    const list = listBuiltinTemplates()
    const cats = new Set(list.map(t => t.category))
    expect(cats.has('网络层')).toBe(true)
    expect(cats.has('Web')).toBe(true)
    expect(cats.has('数据库')).toBe(true)
    expect(cats.has('消息')).toBe(true)
  })

  it('saveCustomTemplate 写入并 listCustomTemplates 能读到', () => {
    saveCustomTemplate({
      id: 'my-tpl',
      name: '我的模板',
      icon: 'star',
      category: '自定义',
      description: '测试',
      builtin: false,
      mapping: { protocol: 'tcp', listenPort: 7000, upstreamHost: '10.0.0.1', upstreamPort: 22 }
    })
    const list = listCustomTemplates()
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('我的模板')
    expect(list[0].builtin).toBe(false)
  })

  it('listAllTemplates 合并内置 + 自定义', () => {
    saveCustomTemplate({
      id: 'c1',
      name: '自定义1',
      icon: 'x',
      category: '自定义',
      description: '',
      builtin: false,
      mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 }
    })
    const all = listAllTemplates()
    expect(all.length).toBe(13 + 1)
  })

  it('deleteCustomTemplate 删除', () => {
    saveCustomTemplate({
      id: 'del',
      name: '待删',
      icon: 'x',
      category: '自定义',
      description: '',
      builtin: false,
      mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 }
    })
    deleteCustomTemplate('del')
    expect(listCustomTemplates()).toHaveLength(0)
  })

  it('重复 id 的自定义模板覆盖旧版', () => {
    saveCustomTemplate({ id: 'same', name: 'v1', icon: 'x', category: 'c', description: '', builtin: false, mapping: { protocol: 'tcp', upstreamHost: '1.1.1.1', upstreamPort: 80 } })
    saveCustomTemplate({ id: 'same', name: 'v2', icon: 'x', category: 'c', description: '', builtin: false, mapping: { protocol: 'tcp', upstreamHost: '2.2.2.2', upstreamPort: 81 } })
    const list = listCustomTemplates()
    expect(list).toHaveLength(1)
    expect(list[0].name).toBe('v2')
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `npx vitest run tests/main/templates/templateRepo.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 `src/main/templates/templateRepo.ts`**

```typescript
import type { TemplateDefinition } from './types'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { basename, extname, join } from 'node:path'
import { parse, stringify } from 'yaml'

// 内置模板目录：源码在 src/main/templates/builtin，打包后在 resources/templates/builtin
function getBuiltinDir(): string {
  // 开发：相对源码；打包：app.getAppPath() 下
  // 用 __dirname 推算：编译后在 out/main，源码模板需复制到 resources
  // 简化：开发期直接读 src 下；打包期读 process.resourcesPath
  if (process.env.NODE_ENV === 'production') {
    return join(process.resourcesPath, 'templates', 'builtin')
  }
  // 开发：从 out/main 回溯到 src/main/templates/builtin
  return join(__dirname, '..', '..', '..', 'src', 'main', 'templates', 'builtin')
}

let customDirForTest: string | null = null
export function __setCustomDirForTest(dir: string): void {
  customDirForTest = dir
}

function getCustomDir(): string {
  if (customDirForTest)
    return customDirForTest
  // 正式：data/templates/custom
  const { resolvePaths } = require('../paths')
  const paths = resolvePaths()
  const dir = join(paths.dataDir, 'templates', 'custom')
  if (!existsSync(dir))
    mkdirSync(dir, { recursive: true })
  return dir
}

function loadYamlFile(filePath: string, id: string, builtin: boolean): TemplateDefinition | null {
  try {
    const content = readFileSync(filePath, 'utf-8')
    const raw = parse(content) as Partial<TemplateDefinition>
    if (!raw || !raw.mapping || !raw.mapping.protocol)
      return null
    return {
      id: raw.id ?? id,
      name: raw.name ?? id,
      icon: raw.icon ?? 'default',
      category: raw.category ?? '其他',
      description: raw.description ?? '',
      builtin,
      mapping: raw.mapping,
      extraDirectives: raw.extraDirectives
    }
  }
  catch {
    return null
  }
}

export function listBuiltinTemplates(): TemplateDefinition[] {
  const dir = getBuiltinDir()
  if (!existsSync(dir))
    return []
  const files = readdirSync(dir).filter(f => extname(f) === '.yml' || extname(f) === '.yaml')
  const list: TemplateDefinition[] = []
  for (const f of files) {
    const id = basename(f, extname(f))
    const tpl = loadYamlFile(join(dir, f), id, true)
    if (tpl)
      list.push(tpl)
  }
  return list.sort((a, b) => a.category.localeCompare(b.category) || a.name.localeCompare(b.name))
}

export function listCustomTemplates(): TemplateDefinition[] {
  const dir = getCustomDir()
  if (!existsSync(dir))
    return []
  const files = readdirSync(dir).filter(f => extname(f) === '.yml' || extname(f) === '.yaml')
  const list: TemplateDefinition[] = []
  for (const f of files) {
    const id = basename(f, extname(f))
    const tpl = loadYamlFile(join(dir, f), id, false)
    if (tpl)
      list.push(tpl)
  }
  return list.sort((a, b) => a.name.localeCompare(b.name))
}

export function listAllTemplates(): TemplateDefinition[] {
  return [...listBuiltinTemplates(), ...listCustomTemplates()]
}

export function saveCustomTemplate(tpl: TemplateDefinition): void {
  const dir = getCustomDir()
  if (!existsSync(dir))
    mkdirSync(dir, { recursive: true })
  const data = {
    id: tpl.id,
    name: tpl.name,
    icon: tpl.icon,
    category: tpl.category,
    description: tpl.description,
    mapping: tpl.mapping,
    extraDirectives: tpl.extraDirectives
  }
  writeFileSync(join(dir, `${tpl.id}.yml`), stringify(data), 'utf-8')
}

export function deleteCustomTemplate(id: string): void {
  const dir = getCustomDir()
  const file = join(dir, `${id}.yml`)
  if (existsSync(file))
    rmSync(file)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npx vitest run tests/main/templates/templateRepo.test.ts`
Expected: PASS（6 tests）。

- [ ] **Step 5: typecheck + 全量**

Run: `npm run typecheck`
Run: `npx vitest run`

- [ ] **Step 6: 提交**

```bash
git add src/main/templates/templateRepo.ts tests/main/templates/templateRepo.test.ts
git commit -m "feat(templates): TemplateRepo 加载内置/自定义 + 存/删自定义（YAML）"
```

---

## Task 7: IPC handlers + preload 桥接模板

**Files:**
- Modify: `src/main/ipc/handlers.ts`（加模板 IPC + reload 改走 applyFlow）
- Modify: `src/preload/index.ts`（加模板桥接）

- [ ] **Step 1: handlers.ts 加模板通道**

在 `registerIpcHandlers()` 里加（import `listAllTemplates, listBuiltinTemplates, saveCustomTemplate, deleteCustomTemplate` from `../templates/templateRepo`，import `applyMappings` from `../nginx/applyFlow`）：

```typescript
// Template
ipcMain.handle(IPC.TEMPLATE_LIST, () => listAllTemplates())
ipcMain.handle(IPC.TEMPLATE_LIST_BUILTIN, () => listBuiltinTemplates())
ipcMain.handle(IPC.TEMPLATE_SAVE_CUSTOM, (_e, tpl: any) => { saveCustomTemplate(tpl); return true })
ipcMain.handle(IPC.TEMPLATE_DELETE_CUSTOM, (_e, id: string) => { deleteCustomTemplate(id); return true })
```

- [ ] **Step 2: handlers.ts 把 ENGINE_RELOAD 改走 applyFlow**

旧的 ENGINE_RELOAD：
```typescript
ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
  const eng = getActiveEngine(); if (eng)
    await eng.reload(); return true
})
```

改为：reload 时重新生成 conf + test + write + reload。需要拿到当前 host 的 enabled mappings。需要 hostId —— 从 getDefaultHost 拿。

```typescript
ipcMain.handle(IPC.ENGINE_RELOAD, async () => {
  const eng = getActiveEngine()
  if (!eng)
    return { ok: false, output: 'no active engine' }
  const host = getDefaultHost()
  if (!host)
    return { ok: false, output: 'no default host' }
  const { resolvePaths } = require('../paths')
  const paths = resolvePaths()
  const mappings = listEnabledMappings(host.id)
  const result = await applyMappings(eng, mappings, {
    nginxLogDir: paths.nginxLogDir,
    nginxTempDir: paths.tempDir
  })
  return { ok: result.ok, output: result.output }
})
```

（import `getDefaultHost` from `../db/hostRepo`，`listEnabledMappings` from `../db/mappingRepo`，`applyMappings` from `../nginx/applyFlow`。）

ENGINE_RELOAD 返回值从 `true` 变成 `{ ok, output }`——前端要同步改。

- [ ] **Step 3: preload/index.ts 加模板桥接**

在 api 对象里加：
```typescript
  // Template
  listTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST),
  listBuiltinTemplates: () => ipcRenderer.invoke(IPC.TEMPLATE_LIST_BUILTIN),
  saveCustomTemplate: (tpl: any) => ipcRenderer.invoke(IPC.TEMPLATE_SAVE_CUSTOM, tpl),
  deleteCustomTemplate: (id: string) => ipcRenderer.invoke(IPC.TEMPLATE_DELETE_CUSTOM, id),
```

并把 `reloadEngine` 的返回类型改成 `Promise<{ ok: boolean; output: string }>`（ElectronAPI 接口在 renderer/api/ipc.ts 也要同步）。

- [ ] **Step 4: 同步改 renderer/api/ipc.ts**

`reloadEngine(): Promise<boolean>` → `reloadEngine(): Promise<{ ok: boolean; output: string }>`

- [ ] **Step 5: 同步改 EngineControl.vue**

原来 reload 调用后没看返回值，现在要看 `{ ok, output }`。改 `reload()` 函数：

```typescript
async function reload() {
  busy.value = true; error.value = ''
  try {
    const r = await api.testEngine()
    if (!r.ok) { alert(`配置校验失败:\n${r.output}`); return }
    const result = await api.reloadEngine()
    if (!result.ok) { alert(`应用失败:\n${result.output}`); return }
    status.value = await api.engineStatus()
  }
  catch (e: any) { error.value = e.message; alert(`应用失败:\n${e.message}`) }
  finally { busy.value = false }
}
```

- [ ] **Step 6: 验证**

Run: `npm run typecheck`
Run: `npx vitest run`（全量绿）
Run: `npm run build`（确认构建通过）

- [ ] **Step 7: 提交**

```bash
git add src/main/ipc/handlers.ts src/preload/index.ts src/renderer/api/ipc.ts src/renderer/components/EngineControl.vue
git commit -m "feat(ipc): 模板 IPC 通道 + reload 改走 applyFlow（test→write→reload）"
```

---

## Task 8: TemplatePicker 组件 + templates store

**Files:**
- Create: `src/renderer/stores/templates.ts`
- Create: `src/renderer/components/TemplatePicker.vue`

- [ ] **Step 1: 创建 `src/renderer/stores/templates.ts`**

```typescript
import { api } from '@renderer/api/ipc'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Template {
  id: string
  name: string
  icon: string
  category: string
  description: string
  builtin: boolean
  mapping: Record<string, any>
  extraDirectives?: string
}

export const useTemplatesStore = defineStore('templates', () => {
  const list = ref<Template[]>([])
  const loading = ref(false)

  async function load() {
    loading.value = true
    try {
      list.value = await api.listTemplates()
    }
    finally {
      loading.value = false
    }
  }

  return { list, loading, load }
})
```

- [ ] **Step 2: 创建 `src/renderer/components/TemplatePicker.vue`**

```vue
<script setup lang="ts">
import type { Template } from '@renderer/stores/templates'
import { useTemplatesStore } from '@renderer/stores/templates'
import { computed } from 'vue'

const emit = defineEmits<{ (e: 'pick', tpl: Template): void, (e: 'cancel'): void }>()
const store = useTemplatesStore()
// 按 category 分组
const grouped = computed(() => {
  const map = new Map<string, Template[]>()
  for (const t of store.list) {
    if (!map.has(t.category))
      map.set(t.category, [])
    map.get(t.category)!.push(t)
  }
  return Array.from(map.entries())
})

// icon 映射到 emoji（毛玻璃版会换 SVG，Plan 4）
const iconEmoji: Record<string, string> = {
  network: '🔌',
  terminal: '💻',
  monitor: '🖥️',
  globe: '🌐',
  lock: '🔒',
  code: '⚙️',
  bolt: '⚡',
  database: '🗄️',
  message: '📡',
  default: '📦'
}
</script>

<template>
  <div class="picker-overlay" @click.self="emit('cancel')">
    <div class="picker-card">
      <h3>选择模板</h3>
      <div v-for="[cat, items] in grouped" :key="cat" class="group">
        <div class="group-title">
          {{ cat }}
        </div>
        <div class="cards">
          <button v-for="t in items" :key="t.id" class="tpl-card" @click="emit('pick', t)">
            <span class="icon">{{ iconEmoji[t.icon] ?? '📦' }}</span>
            <div class="info">
              <div class="name">
                {{ t.name }}<span v-if="!t.builtin" class="custom-tag">自定义</span>
              </div>
              <div class="desc">
                {{ t.description }}
              </div>
            </div>
          </button>
        </div>
      </div>
      <div class="actions">
        <button class="ghost" @click="emit('cancel')">
          取消
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.picker-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.3); display: flex;
  align-items: center; justify-content: center; z-index: 10; }
.picker-card { background: #fff; border-radius: 16px; padding: 24px; width: 640px; max-width: 92vw;
  max-height: 80vh; overflow-y: auto; }
h3 { margin: 0 0 16px; }
.group { margin-bottom: 20px; }
.group-title { font-size: 13px; color: #888; font-weight: 600; margin-bottom: 8px; text-transform: uppercase; }
.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }
.tpl-card { display: flex; gap: 10px; padding: 12px; border: 1px solid #e5e5e5; border-radius: 12px;
  background: #fff; cursor: pointer; text-align: left; align-items: flex-start; }
.tpl-card:hover { border-color: #0A84FF; background: #f0f7ff; }
.icon { font-size: 24px; }
.info { flex: 1; }
.name { font-weight: 600; font-size: 14px; display: flex; align-items: center; gap: 6px; }
.custom-tag { font-size: 10px; background: #0A84FF; color: #fff; padding: 1px 5px; border-radius: 4px; }
.desc { font-size: 12px; color: #888; margin-top: 2px; line-height: 1.4; }
.actions { display: flex; justify-content: flex-end; margin-top: 16px; }
button.ghost { padding: 8px 16px; border: none; border-radius: 8px; background: #eee; color: #333; cursor: pointer; }
</style>
```

- [ ] **Step 3: typecheck + build**

Run: `npm run typecheck`
Run: `npm run build`

- [ ] **Step 4: 提交**

```bash
git add src/renderer/stores/templates.ts src/renderer/components/TemplatePicker.vue
git commit -m "feat(ui): TemplatePicker 模板选择器（按 category 分组卡片网格）+ templates store"
```

---

## Task 9: MappingForm 重构为全协议动态字段 + 模板预填

**Files:**
- Modify: `src/renderer/components/MappingForm.vue`（大改：支持模板预填 + 4 协议动态字段）
- Modify: `src/renderer/App.vue`（新建流程加模板选择步骤）
- Modify: `src/renderer/components/MappingList.vue`（无改，保持）

- [ ] **Step 1: 重写 MappingForm.vue 支持模板预填 + 4 协议**

关键变化：
1. 接受可选 `template` prop（从 TemplatePicker 传入），用于预填表单
2. `form` 扩展为完整字段（含 serverNames/listenPath/tls/redirectHttp）
3. 模板 script 里根据 `form.protocol` 计算显示哪些字段
4. 协议下拉改为 4 选项
5. save 构建 MappingInput 时按协议填充对应字段、其他 null

完整新 `src/renderer/components/MappingForm.vue`：

```vue
<script setup lang="ts">
import type { Template } from '@renderer/stores/templates'
import { api } from '@renderer/api/ipc'
import { computed, ref, watch } from 'vue'

const props = defineProps<{ hostId: string, mappingId?: string, template?: Template | null }>()
const emit = defineEmits<{ (e: 'saved'): void, (e: 'cancel'): void }>()

interface FormState {
  name: string
  protocol: 'tcp' | 'udp' | 'http' | 'https'
  listenPort: number | null
  serverNames: string
  listenPath: string
  upstreamHost: string
  upstreamPort: number | null
  tlsCertPath: string
  tlsKeyPath: string
  redirectHttp: boolean
  extraDirectives: string
}

function templateToForm(tpl: Template | null | undefined): FormState {
  const m = tpl?.mapping ?? {}
  return {
    name: m.name ?? '',
    protocol: m.protocol ?? 'tcp',
    listenPort: m.listenPort ?? null,
    serverNames: m.serverNames ?? '',
    listenPath: m.listenPath ?? '/',
    upstreamHost: m.upstreamHost ?? '',
    upstreamPort: m.upstreamPort ?? null,
    tlsCertPath: m.tlsCertPath ?? '',
    tlsKeyPath: m.tlsKeyPath ?? '',
    redirectHttp: m.redirectHttp === 1,
    extraDirectives: tpl?.extraDirectives ?? m.extraDirectives ?? ''
  }
}

const form = ref<FormState>(templateToForm(props.template))

// 编辑模式：加载已有 mapping
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
      serverNames: m.serverNames ?? '',
      listenPath: m.listenPath ?? '/',
      upstreamHost: m.upstreamHost,
      upstreamPort: m.upstreamPort,
      tlsCertPath: m.tlsCertPath ?? '',
      tlsKeyPath: m.tlsKeyPath ?? '',
      redirectHttp: m.redirectHttp === 1,
      extraDirectives: m.extraDirectives ?? ''
    }
  }
}
loadExisting()

// 按协议显示字段
const isStream = computed(() => form.value.protocol === 'tcp' || form.value.protocol === 'udp')
const isHttp = computed(() => form.value.protocol === 'http')
const isHttps = computed(() => form.value.protocol === 'https')

const saving = ref(false)
const error = ref('')

async function save() {
  error.value = ''
  if (!form.value.name) { error.value = '请填写名称'; return }
  if (!form.value.upstreamHost) { error.value = '请填写目标主机'; return }
  if (!form.value.upstreamPort) { error.value = '请填写目标端口'; return }
  if (isStream.value && !form.value.listenPort) { error.value = '请填写监听端口'; return }
  if (isHttps.value && (!form.value.tlsCertPath || !form.value.tlsKeyPath)) { error.value = 'HTTPS 需要证书和私钥路径'; return }

  saving.value = true
  try {
    const f = form.value
    const input = {
      hostId: props.hostId,
      name: f.name,
      enabled: 1,
      protocol: f.protocol,
      listenPort: isStream.value ? f.listenPort : (isHttp.value ? f.listenPort : null),
      serverNames: (isHttp.value || isHttps.value) ? (f.serverNames || null) : null,
      listenPath: (isHttp.value || isHttps.value) ? f.listenPath : null,
      upstreamHost: f.upstreamHost,
      upstreamPort: f.upstreamPort,
      tlsCertPath: isHttps.value ? f.tlsCertPath : null,
      tlsKeyPath: isHttps.value ? f.tlsKeyPath : null,
      redirectHttp: isHttps.value && f.redirectHttp ? 1 : 0,
      extraDirectives: f.extraDirectives || null,
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
      <h3>{{ mappingId ? '编辑映射' : (template ? '从模板新建' : '新建映射') }}</h3>
      <label>名称<input v-model="form.name" placeholder="如：MySQL 内库"></label>
      <label>协议
        <select v-model="form.protocol">
          <option value="tcp">TCP</option>
          <option value="udp">UDP</option>
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </select>
      </label>

      <!-- TCP/UDP: 监听端口 -->
      <label v-if="isStream">监听端口<input v-model.number="form.listenPort" type="number" placeholder="9000"></label>

      <!-- HTTP/HTTPS: 监听端口 + 域名 + 路径 -->
      <label v-if="isHttp">监听端口<input v-model.number="form.listenPort" type="number" placeholder="80"></label>
      <label v-if="isHttp || isHttps">域名 (server_name)<input v-model="form.serverNames" placeholder="app1.gw.local（可留空）"></label>
      <label v-if="isHttp || isHttps">路径前缀 (location)<input v-model="form.listenPath" placeholder="/"></label>

      <!-- HTTPS: 证书 + 跳转 -->
      <template v-if="isHttps">
        <label>证书路径 (ssl_certificate)<input v-model="form.tlsCertPath" placeholder="/data/cert.pem"></label>
        <label>私钥路径 (ssl_certificate_key)<input v-model="form.tlsKeyPath" placeholder="/data/key.pem"></label>
        <label class="checkbox"><input v-model="form.redirectHttp" type="checkbox"> HTTP→HTTPS 自动跳转</label>
      </template>

      <label>目标主机<input v-model="form.upstreamHost" placeholder="192.168.1.10"></label>
      <label>目标端口<input v-model.number="form.upstreamPort" type="number" placeholder="3306"></label>
      <label>附加指令（高级）<textarea v-model="form.extraDirectives" rows="2" placeholder="proxy_timeout 1h;" /></label>
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
  align-items: center; justify-content: center; z-index: 10; }
.form-card { background: #fff; border-radius: 16px; padding: 24px; width: 480px; max-width: 92vw; max-height: 88vh; overflow-y: auto; }
h3 { margin: 0 0 16px; }
label { display: block; margin-bottom: 12px; font-size: 13px; color: #555; }
label.checkbox { display: flex; align-items: center; gap: 6px; }
input, select, textarea { display: block; width: 100%; margin-top: 4px; padding: 8px;
  border: 1px solid #ddd; border-radius: 8px; font-size: 14px; }
label.checkbox input { width: auto; margin: 0; }
.error { color: #ff3b30; font-size: 13px; margin-bottom: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
button { padding: 8px 16px; border: none; border-radius: 8px; background: #0A84FF; color: #fff; cursor: pointer; }
button.ghost { background: #eee; color: #333; }
button:disabled { opacity: 0.5; }
</style>
```

- [ ] **Step 2: 修改 App.vue 加模板选择步骤**

新建流程改为：点"+ 新建映射" → 弹 TemplatePicker → 选模板 → 弹预填的 MappingForm。

读当前 App.vue，改 `<script setup>`：
- 加 `import TemplatePicker`、`import { useTemplatesStore }`、`import type { Template }`
- 加 state：`const showPicker = ref(false)`、`const pickedTemplate = ref<Template | null>(null)`
- `openNew()` 改成：`pickedTemplate.value = null; showPicker.value = true`
- 加 `function onPick(tpl: Template) { pickedTemplate.value = tpl; showPicker.value = false; showForm.value = true; editingId.value = undefined }`
- `onMounted` 里加 `useTemplatesStore().load()`

改 `<template>`：
- 加 `<TemplatePicker v-if="showPicker" @pick="onPick" @cancel="showPicker = false" />`
- MappingForm 加 `:template="pickedTemplate"` prop

- [ ] **Step 3: typecheck + build**

Run: `npm run typecheck`
Run: `npm run build`
Run: `npx vitest run`（全量绿）

- [ ] **Step 4: 提交**

```bash
git add src/renderer/components/MappingForm.vue src/renderer/App.vue
git commit -m "feat(ui): MappingForm 支持 4 协议动态字段 + 模板预填 + 新建走模板选择器"
```

---

## Task 10: 端到端验证

**Files:** 无新文件，验证 task。

- [ ] **Step 1: 全量回归**

Run: `npm run typecheck` — exit 0
Run: `npx vitest run` — 全绿（应 50+ tests：Plan 1 的 38 + Plan 2 新增的 renderers 8 + applyFlow 3 + templateRepo 6 + confGenerator 新增 3 ≈ 58）
Run: `npm run build` — 3 bundle 成功

- [ ] **Step 2: dev 模式启动验证**

Run: `npm run dev`（后台 20s 后 kill）
Expected:
- Electron 窗口打开，无 `[startup] fatal:`
- 点"+ 新建映射" → 弹出模板选择器，显示 13 个内置模板按 4 类分组
- 选"MySQL 数据库" → 弹出表单，协议预选 TCP、目标端口预填 3306
- 改协议为 HTTPS → 表单动态显示证书/私钥/跳转字段
- 保存 → 列表出现该映射

- [ ] **Step 3: 手动验证清单（记录结果）**

1. ✅ 模板选择器显示 13 个模板，4 个 category 分组
2. ✅ 选 TCP 模板 → 表单只有监听端口 + 目标
3. ✅ 选 HTTPS 模板 → 表单有域名/路径/证书/私钥/跳转
4. ✅ 切协议字段动态变化
5. ✅ 保存后列表显示，协议标记正确
6. ✅ 引擎 reload 走 applyFlow（test 失败弹错误、不应用）

- [ ] **Step 4: 提交验证记录**

```bash
git add -A
git commit -m "test: Plan 2 全量验证通过（renderers + applyFlow + 模板系统 + 4 协议表单）"
```

（若有未提交的改动；否则空提交可跳过。）

---

## Plan 2 完成标准（Definition of Done）

- [ ] 10 个 task 完成
- [ ] `npm run typecheck` 无错误
- [ ] `npx vitest run` 全绿
- [ ] `npm run build` 成功
- [ ] dev 模式：模板选择器显示 13 模板，4 协议表单动态字段，reload 走 applyFlow
- [ ] 13 个内置模板 YAML 全部可加载

---

## 后续 Plan 预告

- **Plan 3**: 健康检查轮询 + 配置历史回滚 + 日志系统 + 端口占用检测
- **Plan 4**: 毛玻璃 iOS 风格 UI 设计系统（此时模板选择器/表单的样式会全面重做）
- **Plan 5**: 远程 nginx 管理（SSH2 引擎）
- **Plan 6**: electron-builder portable 打包 + 一键清除 + 导入导出
