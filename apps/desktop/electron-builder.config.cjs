const path = require('node:path')

// 从 staging 目录运行时，monorepo 根在 staging 的上三级（.stage → desktop → apps → root）
// 从 apps/desktop 直接运行时，monorepo 根在上两级
// 检测：如果当前目录名是 .stage，则为 staging 模式
const cwd = process.cwd()
const isStage = path.basename(cwd) === '.stage'
const root = isStage
  ? path.resolve(cwd, '..', '..', '..')
  : path.resolve(cwd, '..', '..')

/** @type {import('electron-builder').Configuration} */
module.exports = {
  appId: 'com.nginx-jump-gui.app',
  productName: 'NginxJump',
  directories: {
    output: path.resolve(root, 'release'),
  },
  asarUnpack: [
    '**/node_modules/better-sqlite3/**',
    '**/node_modules/ssh2/**',
  ],
  files: [
    'out/**/*',
    'package.json',
    '!**/.DS_Store',
  ],
  win: {
    target: [{ target: 'portable', arch: ['x64'] }],
    // eslint-disable-next-line no-template-curly-in-string -- electron-builder 占位符，非 JS 模板
    artifactName: 'NginxJump-${version}-win-x64.exe',
    icon: 'icons/icon.ico',
  },
  mac: {
    target: [{ target: 'dmg', arch: ['arm64', 'x64'] }],
    // eslint-disable-next-line no-template-curly-in-string -- electron-builder 占位符，非 JS 模板
    artifactName: 'NginxJump-${version}-${arch}.dmg',
    icon: 'icons/icon.icns',
    category: 'public.app-category.developer-tools',
  },
  // 不让 electron-builder 自动 rebuild（cpu-features 会因 Python distutils 缺失失败）
  // native module 在 pnpm install 阶段已编译
  npmRebuild: false,
  extraResources: [
    { from: path.resolve(root, 'resources/nginx'), to: 'bundled/nginx', filter: ['**/*'] },
    {
      from: path.resolve(root, 'packages/server/src/templates/builtin'),
      to: 'bundled/templates',
      filter: ['**/*.yml'],
    },
  ],
}
