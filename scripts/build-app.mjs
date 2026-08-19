import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { build } from 'esbuild'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const RESOURCES_ONLY = process.argv.includes('--resources')
const OFFLINE = process.argv.includes('--offline')

function run(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, { stdio: 'inherit', cwd: ROOT, ...opts })
  if (r.status !== 0) {
    console.error(`✗ ${cmd} ${args.join(' ')} 退出码 ${r.status}`)
    process.exit(1)
  }
}

// 离线打包前置检查：确保已预下载的 Windows 二进制就位
function checkOfflineDeps() {
  const nodeExe = join(ROOT, 'build/node/node.exe')
  const sqliteNode = join(ROOT, 'build/app/better_sqlite3.node')
  const missing = []
  if (!existsSync(nodeExe))
    missing.push(`  build/node/node.exe       — Node.js win-x64 运行时`)
  if (!existsSync(sqliteNode))
    missing.push(`  build/app/better_sqlite3.node — better-sqlite3 原生模块`)
  if (missing.length > 0) {
    console.error('\n✗ 离线模式：缺少 Windows 预构建文件：\n')
    console.error(missing.join('\n'))
    console.error('\n请在有网的机器上先运行：')
    console.error('  pnpm app:build:resources')
    console.error('\n然后将整个项目目录（含 build/）拷贝到内网机器，再运行：')
    console.error('  pnpm app:build -- --offline\n')
    process.exit(1)
  }
  console.log('✓ 离线模式：预构建文件检查通过')
}

console.log('① web 构建')
run('pnpm', ['-F', '@ngj/web', 'build'])

console.log('② esbuild 单文件 server.cjs（自包含，app 内零 node_modules）')
// 清理 build/app/ 但保留 better_sqlite3.node（离线打包时不能重新下载）
// 先备份 .node 文件到 build/ 根目录，清理后恢复
const SQLITE_NODE = join(ROOT, 'build/app/better_sqlite3.node')
const SQLITE_BACKUP = join(ROOT, 'build/.better_sqlite3.node.bak')
let hadSqlite = false
if (existsSync(SQLITE_NODE)) {
  cpSync(SQLITE_NODE, SQLITE_BACKUP)
  hadSqlite = true
}
rmSync(join(ROOT, 'build/app'), { recursive: true, force: true })
mkdirSync(join(ROOT, 'build/app'), { recursive: true })

// better-sqlite3 + ssh2 + cpu-features 是 native 模块，不能 bundle，标记为 external
// esbuild 会保留 require()，运行时从 sidecar 旁的 node_modules 或 .node 文件加载
await build({
  entryPoints: [join(ROOT, 'packages/server/src/index.ts')],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  target: 'node22',
  outfile: join(ROOT, 'build/app/server.cjs'),
  external: ['better-sqlite3', 'ssh2', 'cpu-features'],
  alias: {
    '@ngj/shared/types': join(ROOT, 'packages/shared/src/types.ts'),
    '@ngj/shared/paths': join(ROOT, 'packages/shared/src/paths.ts'),
    '@ngj/shared/validation': join(ROOT, 'packages/shared/src/validation.ts'),
    '@ngj/shared': join(ROOT, 'packages/shared/src/index.ts'),
  },
})

// 恢复 better_sqlite3.node（如果清理前存在）
if (hadSqlite) {
  cpSync(SQLITE_BACKUP, SQLITE_NODE)
  rmSync(SQLITE_BACKUP)
}

console.log('②b 复制内置模板到 build/app/templates/builtin')
cpSync(
  join(ROOT, 'packages/server/src/templates/builtin'),
  join(ROOT, 'build/app/templates/builtin'),
  { recursive: true },
)

if (OFFLINE) {
  checkOfflineDeps()
  console.log('③④ 离线模式：跳过下载（使用已预构建的文件）')
}
else {
  console.log('③ 下载 win32-x64 node 运行时')
  run('bash', ['scripts/fetch-node.sh'])

  console.log('④ 下载 win32-x64 better-sqlite3 prebuilt')
  run('bash', ['scripts/fetch-better-sqlite3.sh'])
}

if (RESOURCES_ONLY) {
  console.log('✓ Resources 素材就绪')
  process.exit(0)
}

console.log('⑤ tauri build')
run('pnpm', ['tauri', 'build', '--config', 'apps/desktop/tauri.conf.json'])

console.log('✓ 完成')
