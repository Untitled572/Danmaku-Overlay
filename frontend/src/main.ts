import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import OverlayApp from './OverlayApp.vue'
import router from './router'

const isOverlay = window.location.hash.startsWith('#/overlay')

const app = createApp(isOverlay ? OverlayApp : App)
app.use(router)
app.mount('#app')
