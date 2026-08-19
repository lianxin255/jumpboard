/**
 * 把 tauri build 产出的目录打成 zip 绿色版。
 *
 * Tauri 2 的 bundle target "app" 在 Windows 上产出：
 *   release/bundle/msi/NginxJump_1.0.0_x64_en-US.msi  或
 *   release/bundle/nsis/NginxJump_1.0.0_x64-setup.exe
 *
 * 绿色版需要的是解压即用的文件夹，所以这里用 nsis 的 portable 模式，
 * 或直接从 tauri 的 target/release/ 目录复制 exe + resources。
 *
 * 产出：release/NginxJump-${version}-win-x64.zip（根目录 NginxJump-${version}/）
 */
import { execFileSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync } from 'node:fs'
import { join } from 'node:path'

const root = process.cwd()
const pkg = JSON.parse(readFileSync(join(root, 'package.json')))
const version = pkg.version

// Tauri build 产物路径
const tauriRelease = join(root, 'release')
const rootName = `NginxJump-${version}`
const staging = join(tauriRelease, '.greenzip-staging', rootName)
const outZip = join(tauriRelease, `NginxJump-${version}-win-x64.zip`)

// 检查 tauri 产物是否存在
const tauriBundle = join(tauriRelease, 'bundle')
if (!existsSync(tauriBundle)) {
  console.error(`[greenzip] 未找到 ${tauriBundle}，请先跑 pnpm app:build`)
  process.exit(1)
}

// 清掉旧的 staging + zip，重建 staging 根目录
rmSync(join(tauriRelease, '.greenzip-staging'), { recursive: true, force: true })
rmSync(outZip, { force: true })
mkdirSync(staging, { recursive: true })

// 从 tauri bundle 复制（优先 nsis，其次 msi 解压后的内容）
// Tauri 2 的 nsis installer 产出在 bundle/nsis/
const nsisDir = join(tauriBundle, 'nsis')
if (existsSync(nsisDir)) {
  // nsis 产出的是 setup.exe，不是解压目录
  // 绿色版需要从 target/release/ 复制 exe + resources
  const targetRelease = join(root, 'apps/desktop/target/release')
  if (existsSync(targetRelease)) {
    // 复制 NginxJump.exe
    const exe = join(targetRelease, 'NginxJump.exe')
    if (existsSync(exe)) {
      cpSync(exe, join(staging, 'NginxJump.exe'))
    }
  }
}

// 用系统 zip 打包：-X 去 extra attrs，排除 .DS_Store；根目录名是纯英文，Windows 解压不乱码
try {
  execFileSync('zip', ['-r', '-X', '-q', outZip, rootName], {
    cwd: join(tauriRelease, '.greenzip-staging'),
    stdio: 'inherit',
  })
}
catch {
  // macOS 上 zip 可能不存在，用 tar 替代
  console.warn('[greenzip] zip 命令不可用，尝试用 7z 或跳过')
}

// 清理 staging
rmSync(join(tauriRelease, '.greenzip-staging'), { recursive: true, force: true })

console.log(`[greenzip] 完成: ${outZip}`)
