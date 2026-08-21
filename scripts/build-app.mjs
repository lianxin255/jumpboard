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
  const sqliteNode = join(ROOT, 'build/app/better_sqlite3.win32.node')
  const missing = []
  if (!existsSync(nodeExe))
    missing.push(`  build/node/node.exe       — Node.js win-x64 运行时`)
  if (!existsSync(sqliteNode))
    missing.push(`  build/app/better_sqlite3.win32.node — better-sqlite3 win32 原生模块`)
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
// 清理 build/app/，但保留 win32 prebuilt（离线打包时不能重新下载）
const SQLITE_WIN32 = join(ROOT, 'build/app/better_sqlite3.win32.node')
const SQLITE_WIN32_BAK = join(ROOT, 'build/.better_sqlite3.win32.node.bak')
if (existsSync(SQLITE_WIN32)) {
  cpSync(SQLITE_WIN32, SQLITE_WIN32_BAK)
}
rmSync(join(ROOT, 'build/app'), { recursive: true, force: true })
mkdirSync(join(ROOT, 'build/app'), { recursive: true })

// better-sqlite3 只留原生 .node 为 external（运行时经 nativeBinding 显式加载），
// 其 JS 部分与 ssh2 全部打进 bundle——ssh2 的 cpu-features 是 try-catch 可选依赖，
// 缺失时静默走纯 JS crypto，无需 external。
const BSQLITE = join(ROOT, 'node_modules/.pnpm/better-sqlite3@12.11.1/node_modules/better-sqlite3')
await build({
  entryPoints: [join(ROOT, 'packages/server/src/index.ts')],
  bundle: true,
  platform: 'node',
  format: 'cjs',
  target: 'node22',
  outfile: join(ROOT, 'build/app/server.cjs'),
  // cpu-features 是 ssh2 的可选依赖（try-catch 加载），external 后运行时
  // require 失败被 ssh2 静默吞掉，走纯 JS crypto fallback
  external: ['cpu-features'],
  alias: {
    // 把 better-sqlite3 指到它的 JS 入口：bundle JS，.node 由 nativeBinding 加载
    'better-sqlite3': join(BSQLITE, 'lib/index.js'),
    '@ngj/shared/types': join(ROOT, 'packages/shared/src/types.ts'),
    '@ngj/shared/paths': join(ROOT, 'packages/shared/src/paths.ts'),
    '@ngj/shared/validation': join(ROOT, 'packages/shared/src/validation.ts'),
    '@ngj/shared': join(ROOT, 'packages/shared/src/index.ts'),
  },
})

// 恢复 win32 prebuilt（本次备份的，或上次构建中断遗留的 .bak）
if (existsSync(SQLITE_WIN32_BAK)) {
  cpSync(SQLITE_WIN32_BAK, SQLITE_WIN32)
  rmSync(SQLITE_WIN32_BAK)
}

// 复制当前平台的 better_sqlite3.node 到 build/app/（供本地 e2e 测试用）
// win32 prebuilt 在 build/app/better_sqlite3.win32.node，两者互不干扰
const SQLITE_NATIVE = join(BSQLITE, 'build/Release/better_sqlite3.node')
if (existsSync(SQLITE_NATIVE)) {
  cpSync(SQLITE_NATIVE, join(ROOT, 'build/app/better_sqlite3.node'))
  console.log(`  复制当前平台 better_sqlite3.node (${process.platform}-${process.arch})`)
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
