import { resolve } from 'node:path'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const sharedDir = resolve(__dirname, '../../packages/shared/src')
const webDir = resolve(__dirname, 'src')

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@ngj/shared/types': resolve(sharedDir, 'types.ts'),
      '@ngj/shared/paths': resolve(sharedDir, 'paths.ts'),
      '@ngj/shared/validation': resolve(sharedDir, 'validation.ts'),
      '@ngj/shared': resolve(sharedDir, 'index.ts'),
      '@web': webDir,
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:47831',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
