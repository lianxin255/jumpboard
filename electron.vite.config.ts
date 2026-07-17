import { defineConfig, externalizeDepsPlugin } from 'electron-vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import Components from 'unplugin-vue-components/vite'
import { AntdvNextResolver } from '@antdv-next/auto-import-resolver'

const sharedDir = resolve(__dirname, 'src/shared')
const rendererDir = resolve(__dirname, 'src/renderer')

// alias 对象 key 不带尾斜杠时，Vite（@rollup/plugin-alias）会自动做前缀匹配，
// 因此 '@shared' 能匹配 '@shared/types'，'@renderer' 能匹配 '@renderer/...'。
const sharedAlias = {
  '@shared': sharedDir
}

export default defineConfig({
  main: {
    // externalizeDepsPlugin 把 package.json 的 dependencies（含 native 模块 better-sqlite3）
    // 标记为 external，运行时从 node_modules 加载，避免 native .node 文件被 rollup
    // 尝试 bundle 导致 "Could not dynamically require '*.node'" 错误。
    plugins: [externalizeDepsPlugin()],
    build: {
      rollupOptions: {
        input: { index: resolve(__dirname, 'src/main/index.ts') }
      }
    },
    resolve: { alias: sharedAlias }
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
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
    plugins: [
      vue(),
      Components({
        resolvers: [AntdvNextResolver()],
        // 只扫描 renderer，避免把 main/preload 的代码也扫描了
        dirs: []
      })
    ]
  }
})

