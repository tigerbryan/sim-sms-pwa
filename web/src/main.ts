import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'
import { initPWAInstall } from './composables/pwaInstall'
import { initPWAUpdates } from './composables/pwaUpdate'

initPWAInstall()
createApp(App).use(createPinia()).use(router).mount('#app')

window.addEventListener('load', initPWAUpdates)
