import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
const backendPort = process.env.PORT || '8085'
const backendTarget = `http://127.0.0.1:${backendPort}`
const wsTarget = `ws://127.0.0.1:${backendPort}`

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 1420,
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: wsTarget,
        ws: true,
      },
      '/covers': {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
})
