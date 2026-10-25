import { resolve } from 'node:path'
import vue from '@vitejs/plugin-vue'
import { defineConfig, externalizeDepsPlugin } from 'electron-vite'

const sharedDir = resolve(__dirname, '../../packages/shared/src')

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin({ exclude: ['@ngj/server', '@ngj/shared'] })],
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/main/index.ts') },
      },
    },
    resolve: {
      alias: {
        '@ngj/shared/types': resolve(sharedDir, 'types.ts'),
        '@ngj/shared/paths': resolve(sharedDir, 'paths.ts'),
        '@ngj/shared/validation': resolve(sharedDir, 'validation.ts'),
        '@ngj/shared': resolve(sharedDir, 'index.ts'),
        '@ngj/server': resolve(__dirname, '../../packages/server/src'),
      },
    },
  },
  preload: {
    plugins: [externalizeDepsPlugin({ exclude: ['@ngj/shared'] })],
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/preload/index.ts') },
      },
    },
    resolve: {
      alias: {
        '@ngj/shared/types': resolve(sharedDir, 'types.ts'),
        '@ngj/shared': resolve(sharedDir, 'index.ts'),
      },
    },
  },
  renderer: {
    root: 'src/renderer',
    build: {
      rollupOptions: {
        input: resolve(__dirname, 'src/renderer/index.html'),
      },
    },
    resolve: {
      alias: {
        '@ngj/shared/types': resolve(sharedDir, 'types.ts'),
        '@ngj/shared/paths': resolve(sharedDir, 'paths.ts'),
        '@ngj/shared/validation': resolve(sharedDir, 'validation.ts'),
        '@ngj/shared': resolve(sharedDir, 'index.ts'),
        '@renderer': resolve(__dirname, 'src/renderer'),
      },
    },
    plugins: [vue()],
  },
})
