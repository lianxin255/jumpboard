import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { build } from 'esbuild'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const RESOURCES_ONLY = process.argv.includes('--resources')

function run(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, { stdio: 'inherit', cwd: ROOT, ...opts })
  if (r.status !== 0) {
    console.error(`✗ ${cmd} ${args.join(' ')} 退出码 ${r.status}`)
    process.exit(1)
  }
}

console.log('① web 构建')
run('pnpm', ['-F', '@ngj/web', 'build'])

console.log('② esbuild 单文件 server.cjs（自包含，app 内零 node_modules）')
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

console.log('②b 复制内置模板到 build/app/templates/builtin')
cpSync(
  join(ROOT, 'packages/server/src/templates/builtin'),
  join(ROOT, 'build/app/templates/builtin'),
  { recursive: true },
)

console.log('③ 下载 win32-x64 node 运行时')
run('bash', ['scripts/fetch-node.sh'])

console.log('④ 下载 win32-x64 better-sqlite3 prebuilt')
run('bash', ['scripts/fetch-better-sqlite3.sh'])

if (RESOURCES_ONLY) {
  console.log('✓ Resources 素材就绪')
  process.exit(0)
}

console.log('⑤ tauri build')
run('pnpm', ['tauri', 'build', '--config', 'apps/desktop/tauri.conf.json'])

console.log('✓ 完成')
