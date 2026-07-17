import { createApp } from 'vue'
import { createPinia } from 'pinia'
import Antd from 'antdv-next'
import Root from './Root.vue'
import './styles/tokens.css'
import './styles/glass.css'
import './styles/base.css'

createApp(Root).use(createPinia()).use(Antd).mount('#app')
