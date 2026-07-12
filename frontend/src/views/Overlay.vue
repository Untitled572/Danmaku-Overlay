<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { DanmakuEngine, type DanmakuLine } from '../services/danmaku-engine'

const container = ref<HTMLDivElement>()
let engine: DanmakuEngine | null = null
let ws: WebSocket | null = null

const episodeId = ref<number | null>(null)
const wsToken = ref('')
const isConnected = ref(false)
const currentTime = ref(0)
const isPaused = ref(false)

function getApiBase(): string {
  return window.location.origin
}

function getWsUrl(episodeId: number, token: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws?client=overlay&ep=${episodeId}&token=${token}`
}

function loadDanmakuConfig() {
  const saved = localStorage.getItem('danmaku_settings')
  if (saved && engine) {
    const settings = JSON.parse(saved)
    engine.updateConfig({
      fontSize: settings.fontSize,
      opacity: settings.opacity,
      speed: settings.speed,
    })
  }
}

async function loadDanmaku(epId: number): Promise<DanmakuLine[]> {
  try {
    const resp = await fetch(`${getApiBase()}/api/v1/episodes/${epId}/danmaku`, {
      headers: {
        Authorization: `Bearer ${wsToken.value}`,
      },
    })
    if (!resp.ok) return []
    return await resp.json()
  } catch {
    return []
  }
}

function connectWebSocket(epId: number, token: string) {
  if (ws) {
    ws.close()
    ws = null
  }

  const url = getWsUrl(epId, token)
  ws = new WebSocket(url)

  ws.onopen = () => {
    isConnected.value = true
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)

      if (msg.type === 'time_sync') {
        const payload = msg.payload
        currentTime.value = payload.current_time
        isPaused.value = payload.is_paused

        if (engine) {
          engine.syncTime(payload.current_time, payload.is_seeking)
        }
      }

      if (msg.type === 'config_sync') {
        const payload = msg.payload
        if (engine) {
          engine.updateConfig(payload)
        }
      }
    } catch {}
  }

  ws.onclose = () => {
    isConnected.value = false
    setTimeout(() => {
      if (episodeId.value) {
        connectWebSocket(episodeId.value, token)
      }
    }, 3000)
  }

  ws.onerror = () => {
    ws?.close()
  }
}

onMounted(async () => {
  const params = new URLSearchParams(window.location.hash.split('?')[1] || '')
  const epParam = params.get('ep')
  const tokenParam = params.get('token')

  if (epParam) episodeId.value = parseInt(epParam, 10)
  if (tokenParam) wsToken.value = tokenParam

  if (container.value) {
    engine = new DanmakuEngine(container.value)
    engine.init()
    loadDanmakuConfig()

    window.addEventListener('resize', () => engine?.resize())
  }

  if (episodeId.value && wsToken.value) {
    const lines = await loadDanmaku(episodeId.value)
    if (engine && lines.length > 0) {
      engine.loadComments(lines)
    }
    connectWebSocket(episodeId.value, wsToken.value)
  }
})

onUnmounted(() => {
  engine?.destroy()
  ws?.close()
  ws = null
})
</script>

<template>
  <div ref="container" class="overlay-container" />
</template>

<style>
html, body {
  background: transparent !important;
  margin: 0;
  padding: 0;
  overflow: hidden;
}

.overlay-container {
  width: 100vw;
  height: 100vh;
  position: relative;
  pointer-events: none;
}
</style>
