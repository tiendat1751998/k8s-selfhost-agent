import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import './style.css'
import { registerSW } from './registerServiceWorker'

const app = createApp(App)

app.use(createPinia())
app.use(router)

;(window as any).__router = router

app.mount('#app')

registerSW()


