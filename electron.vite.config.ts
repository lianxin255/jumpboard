import { defineConfig } from 'electron-vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

const sharedDir = resolve(__dirname, 'src/shared')
const rendererDir = resolve(__dirname, 'src/renderer')

// alias 对象 key 不带尾斜杠时，Vite（@rollup/plugin-alias）会自动做前缀匹配，
// 因此 '@shared' 能匹配 '@shared/types'，'@renderer' 能匹配 '@renderer/...'。
const sharedAlias = {
  '@shared': sharedDir
}

export default defineConfig({
  main: {
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/main/index.ts') }
      }
    },
    resolve: { alias: sharedAlias }
  },
  preload: {
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/preload/index.ts') }
      }
    },
    resolve: { alias: sharedAlias }
  },
  renderer: {
    root: 'src/renderer',
    build: {
      rollupOptions: {
        input: resolve(__dirname, 'src/renderer/index.html')
      }
    },
    resolve: {
      alias: {
        ...sharedAlias,
        '@renderer': rendererDir
      }
    },
    plugins: [vue()]
  }
})
