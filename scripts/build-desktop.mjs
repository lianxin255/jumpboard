#!/usr/bin/env node
/**
 * desktop 打包脚本：解决 pnpm workspace + electron-builder 的符号链接问题。
 *
 * 流程：
 * 1. electron-vite build 构建 main/preload/renderer
 * 2. pnpm deploy --legacy 创建扁平化 staging 目录（无符号链接，workspace 依赖被复制为真实文件）
 * 3. electron-builder 从 staging 目录打包
 *
 * SQLite 已迁移到 Electron 内置的 node:sqlite（38+），无原生模块，
 * 早期为 better-sqlite3 服务的 prebuild-install/electron-rebuild 步骤已移除。
 *
 * 用法：
 *   node scripts/build-desktop.mjs --mac --dir      # macOS 绿色文件夹
 *   node scripts/build-desktop.mjs --mac dmg         # macOS dmg
 *   node scripts/build-desktop.mjs --win --dir       # Windows 绿色文件夹
 *   node scripts/build-desktop.mjs --win portable    # Windows portable exe
 */
import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, rmSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const DESKTOP = join(ROOT, 'apps/desktop')
const STAGE = join(DESKTOP, '.stage')

// 解析命令行参数
const args = process.argv.slice(2)
// --current：按当前平台打包（本机验证 / e2e 用），等价于 --mac 或 --win
if (args.includes('--current'))
  args.push(process.platform === 'win32' ? '--win' : '--mac')
const isMac = args.includes('--mac')
const isWin = args.includes('--win')
const isDir = args.includes('--dir')
const target = args.find(a => !a.startsWith('--') && a !== 'dir' && a !== 'mac' && a !== 'win')

if (!isMac && !isWin) {
  console.error('请指定平台：--mac 或 --win')
  process.exit(1)
}

const platformFlag = isMac ? '--mac' : '--win'
const targetFlag = isDir ? '--dir' : target || (isMac ? 'dmg' : 'portable')

function run(cmd, cmdArgs, cwd = ROOT) {
  console.log(`\n$ ${cmd} ${cmdArgs.join(' ')}`)
  // Windows 上 npx 等工具是 .cmd 脚本，spawnSync 必须经 shell 才能找到
  const r = spawnSync(cmd, cmdArgs, { stdio: 'inherit', cwd, shell: process.platform === 'win32' })
  if (r.status !== 0) {
    console.error(`✗ ${cmd} 退出码 ${r.status}`)
    process.exit(1)
  }
}

console.log('① electron-vite build')
run('npx', ['electron-vite', 'build'], DESKTOP)

console.log('② pnpm deploy 创建 staging 目录')
rmSync(STAGE, { recursive: true, force: true })
run('pnpm', ['--filter', '@ngj/desktop', 'deploy', '--legacy', STAGE], ROOT)

// 复制 electron-builder 配置和 icons 到 staging
cpSync(join(DESKTOP, 'electron-builder.config.cjs'), join(STAGE, 'electron-builder.config.cjs'))
if (existsSync(join(DESKTOP, 'icons'))) {
  cpSync(join(DESKTOP, 'icons'), join(STAGE, 'icons'), { recursive: true })
}

// 确保 out/ 目录在 staging 里（pnpm deploy 会复制 out/，因为它不是 devDependency）
// 检查并手动复制如果缺失
const stageOut = join(STAGE, 'out')
if (!existsSync(stageOut)) {
  console.log('③ 复制 out/ 构建产物到 staging')
  cpSync(join(DESKTOP, 'out'), stageOut, { recursive: true })
}

// node:sqlite（Electron 35+ 内置）替代了原生模块 better-sqlite3，
// 不再需要 prebuild-install 下载平台原生二进制，也没有 store 污染问题。

console.log(`④ electron-builder ${platformFlag} ${targetFlag}`)
const ebArgs = ['electron-builder', platformFlag, targetFlag]
// Windows 交叉编译时明确指定 x64（不跟随 macOS arm64 主机架构）
if (isWin)
  ebArgs.push('--x64')
ebArgs.push('--config', 'electron-builder.config.cjs')
run('npx', ebArgs, STAGE)

console.log('✓ 打包完成')
console.log(`  产物目录：${join(ROOT, 'release')}`)
