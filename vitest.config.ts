import { resolve } from 'node:path'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

const sharedDir = resolve(__dirname, 'packages/shared/src')
const serverDir = resolve(__dirname, 'packages/server/src')
const webDir = resolve(__dirname, 'apps/web/src')

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@ngj/shared/types': resolve(sharedDir, 'types.ts'),
      '@ngj/shared/paths': resolve(sharedDir, 'paths.ts'),
      '@ngj/shared/validation': resolve(sharedDir, 'validation.ts'),
      '@ngj/shared': resolve(sharedDir, 'index.ts'),
      '@ngj/server': resolve(serverDir, 'index.ts'),
      '@web': webDir,
    },
  },
  test: {
    environment: 'node',
    include: [
      'packages/server/src/**/*.test.ts',
      'packages/shared/src/**/*.test.ts',
      'apps/web/src/**/*.test.ts',
    ],
    pool: 'forks',
    poolOptions: { forks: { singleFork: true } },
  },
})
