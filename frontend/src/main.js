import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import { init } from './store'

init()
createApp(App).mount('#app')
