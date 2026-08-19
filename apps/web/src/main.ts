import { createPinia } from 'pinia'
import { createApp } from 'vue'
import Root from './Root.vue'
import './styles/tokens.css'
import './styles/glass.css'
import './styles/base.css'
// 尽早初始化 dayjs 插件/locale（副作用 import），避免组件用到 fromNow 前未 extend
import './utils/time'

createApp(Root).use(createPinia()).mount('#app')
