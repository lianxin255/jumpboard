import { defineConfig } from 'vitest/config'

/**
 * 打包版真实 UI e2e（Playwright Electron）。
 * 前置：先产出打包产物（pnpm test:e2e 会自动执行 build-desktop --current --dir）。
 * 与单测（pnpm test）分离：e2e 依赖打包，较慢，CI 单独 job 跑。
 */
export default defineConfig({
  test: {
    environment: 'node',
    include: ['e2e/**/*.e2e.test.ts'],
    testTimeout: 90_000,
    hookTimeout: 180_000,
    pool: 'forks',
    poolOptions: { forks: { singleFork: true } },
  },
})
