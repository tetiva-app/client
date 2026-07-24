import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './assets/index.css'
import { useSettingsStore } from './stores/settings'

const app = createApp(App)
app.use(createPinia())

// Instantiate before mount so theme/font DOM effects and cross-window sync are
// live in every window mode, including child windows that skip App.vue's main-mode setup.
useSettingsStore()

app.mount('#app')
