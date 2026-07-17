import { createApp } from 'vue'
import { createPinia } from 'pinia'
import Antd from 'antdv-next'
import App from './App.vue'
import './styles/tokens.css'
import './styles/glass.css'
import './styles/base.css'

createApp(App).use(createPinia()).use(Antd).mount('#app')
