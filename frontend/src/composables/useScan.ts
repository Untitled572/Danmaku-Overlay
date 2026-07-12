import { ref } from 'vue'
import { danmakuApi } from '../services/danmaku'

const isScanning = ref(false)
const scanCounter = ref(0)
const scanMessage = ref('')

const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

export function useScan() {
  const triggerScan = async () => {
    if (isScanning.value) return
    isScanning.value = true
    try {
      scanMessage.value = '启动扫描...'
      await danmakuApi.triggerScan()
      
      while (true) {
        const status = await danmakuApi.getTaskStatus()
        if (status?.scan?.status === 'completed' || status?.scan?.status === 'idle') {
          break
        }
        const percent = status?.scan?.percentage || 0
        scanMessage.value = `扫描中 ${percent}%`
        await delay(1000)
      }

      scanMessage.value = '启动刮削...'
      await danmakuApi.triggerScrape()
      
      while (true) {
        const status = await danmakuApi.getTaskStatus()
        if (status?.scrape?.status === 'completed' || status?.scrape?.status === 'idle') {
          break
        }
        const current = status?.scrape?.current || 0
        const total = status?.scrape?.total || 0
        scanMessage.value = `刮削中 ${current}/${total}`
        await delay(1000)
      }

      scanMessage.value = '全部完成'
      setTimeout(() => { scanMessage.value = '' }, 3000)
    } catch (e) {
      console.error('Failed to trigger scan/scrape:', e)
      scanMessage.value = '任务失败'
      setTimeout(() => { scanMessage.value = '' }, 3000)
    } finally {
      isScanning.value = false
      scanCounter.value++
    }
  }

  return {
    isScanning,
    scanCounter,
    scanMessage,
    triggerScan
  }
}
