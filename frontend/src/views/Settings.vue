<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { MonitorPlay, HardDrive, Save, Type, Check, FolderOpen, Trash2, Database, Key, FileText } from 'lucide-vue-next'
import { api } from '../services/api'
import { danmakuApi, type Library } from '../services/danmaku'

const currentTab = ref<'basic' | 'player' | 'api' | 'danmaku'>('basic')

const playerPath = ref('')
const discordRpc = ref(true)
const isSavingDb = ref(false)
const saveDbSuccess = ref(false)
const isSavingPlayer = ref(false)
const savePlayerSuccess = ref(false)
const savingRpc = ref(false)
const saveRpcSuccess = ref(false)
const isSavingApi = ref(false)
const saveApiSuccess = ref(false)
const isSavingDanmaku = ref(false)
const saveDanmakuSuccess = ref(false)
const healthStatus = ref<'ready' | 'warning' | 'error'>('error')

// DB & Libraries
const dbPath = ref('')
const libraries = ref<Library[]>([])
const newLibraryPath = ref('')

// Bangumi APIs
const bangumiAppId = ref('')
const bangumiAppSecret = ref('')
const bangumiAccessToken = ref('')

const danmakuSettings = reactive({
  fontSize: 24,
  opacity: 1.0,
  speed: 144,
  density: 0,
  showTop: true,
  showBottom: true,
  showScroll: true,
})

const loadLibraries = async () => {
  try {
    libraries.value = await danmakuApi.getLibraries()
  } catch (e) {
    console.error('Failed to load libraries:', e)
  }
}

const addLibrary = async () => {
  if (!newLibraryPath.value.trim()) return
  try {
    await danmakuApi.createLibrary(newLibraryPath.value.trim())
    newLibraryPath.value = ''
    await loadLibraries()
  } catch (e) {
    alert('添加失败: ' + String(e))
  }
}

const deleteLibrary = async (id: number | string) => {
  if (!confirm('确定删除该媒体库吗？这不会删除物理文件，但会从数据库清除关联的内容和弹幕记录！')) return
  try {
    await danmakuApi.deleteLibrary(id)
    await loadLibraries()
  } catch (e) {
    alert('删除失败: ' + String(e))
  }
}

const browseNewLibraryDir = () => {
  const result = prompt('请输入目录路径', newLibraryPath.value)
  if (result !== null) {
    newLibraryPath.value = result
  }
}

const browsePlayerPath = async () => {
  const result = prompt('请输入播放器路径', playerPath.value)
  if (result !== null) {
    playerPath.value = result
  }
}

const browseDbPath = async () => {
  const result = prompt('请输入数据库文件路径', dbPath.value)
  if (result !== null) {
    dbPath.value = result
  }
}

const loadSettings = async () => {
  try {
    const data = await api.get<Record<string, any>>('/settings')
    if (data.playerPath) playerPath.value = JSON.parse(data.playerPath)
    if (data.discordRpc) discordRpc.value = JSON.parse(data.discordRpc)
    if (data.db_path) {
      dbPath.value = JSON.parse(data.db_path)
    } else {
      const status = await api.get<{ db_path: string }>('/library/init/status')
      dbPath.value = status.db_path || ''
    }
    
    if (data.danmakuSettings) {
      Object.assign(danmakuSettings, JSON.parse(data.danmakuSettings))
    } else {
      const saved = localStorage.getItem('danmaku_settings')
      if (saved) Object.assign(danmakuSettings, JSON.parse(saved))
    }

    if (data.api_keys) {
      try {
        const keys = JSON.parse(data.api_keys)
        bangumiAppId.value = keys.bangumi_app_id || ''
        bangumiAppSecret.value = keys.bangumi_app_secret || ''
        bangumiAccessToken.value = keys.bangumi_access_token || ''
      } catch (e) {
        console.error('Failed to parse api_keys:', e)
      }
    }
  } catch (error) {
    console.error('Failed to load settings:', error)
  }
}

const checkHealth = async () => {
  try {
    const res = await danmakuApi.checkHealth()
    if (res.status === 'ok') {
      if (res.database === 'ready') {
        healthStatus.value = 'ready'
      } else {
        healthStatus.value = 'warning'
      }
    } else {
      healthStatus.value = 'error'
    }
  } catch (e) {
    healthStatus.value = 'error'
  }
}

const savePlayerConfig = async () => {
  isSavingPlayer.value = true
  try {
    await api.put('/settings', {
      playerPath: JSON.stringify(playerPath.value)
    })
    savePlayerSuccess.value = true
    setTimeout(() => { savePlayerSuccess.value = false }, 2000)
  } catch (error) {
    console.error('Failed to save player path:', error)
    alert('保存播放器路径失败')
  } finally {
    isSavingPlayer.value = false
  }
}

const saveDiscordRpc = async () => {
  savingRpc.value = true
  try {
    await api.put('/settings', {
      discordRpc: JSON.stringify(discordRpc.value)
    })
    saveRpcSuccess.value = true
    setTimeout(() => { saveRpcSuccess.value = false }, 2000)
  } catch (error) {
    console.error('Failed to save Discord RPC setting:', error)
    alert('保存 Discord RPC 状态失败')
  } finally {
    savingRpc.value = false
  }
}

const saveApiKeys = async () => {
  isSavingApi.value = true
  try {
    await api.put('/settings', {
      api_keys: JSON.stringify({
        bangumi_app_id: bangumiAppId.value,
        bangumi_app_secret: bangumiAppSecret.value,
        bangumi_access_token: bangumiAccessToken.value
      })
    })
    saveApiSuccess.value = true
    setTimeout(() => { saveApiSuccess.value = false }, 2000)
  } catch (error) {
    console.error('Failed to save API credentials:', error)
    alert('保存 API 凭证失败')
  } finally {
    isSavingApi.value = false
  }
}

const saveDanmakuSettings = async () => {
  isSavingDanmaku.value = true
  try {
    await api.put('/settings', {
      danmakuSettings: JSON.stringify(danmakuSettings)
    })
    localStorage.setItem('danmaku_settings', JSON.stringify(danmakuSettings))
    saveDanmakuSuccess.value = true
    setTimeout(() => { saveDanmakuSuccess.value = false }, 2000)
  } catch (error) {
    console.error('Failed to save danmaku settings:', error)
    alert('保存弹幕设置失败')
  } finally {
    isSavingDanmaku.value = false
  }
}

const saveDbPath = async () => {
  let path = dbPath.value.trim()
  if (!path) {
    alert('请输入数据库文件路径')
    return
  }

  // 如果有必要就自动添加danmaku.db
  if (!path.toLowerCase().endsWith('.db')) {
    if (path.endsWith('/') || path.endsWith('\\')) {
      path += 'danmaku.db'
    } else {
      const separator = path.includes('\\') && !path.includes('/') ? '\\' : '/'
      path += separator + 'danmaku.db'
    }
  }
  dbPath.value = path

  isSavingDb.value = true
  try {
    await api.post('/library/init', { db_path: path })
    await api.put('/settings', { db_path: JSON.stringify(path) })
    saveDbSuccess.value = true
    setTimeout(() => { saveDbSuccess.value = false }, 2000)
    await loadSettings()
    await loadLibraries()
    await checkHealth()
    await fetchStats()
  } catch (error) {
    console.error('Failed to save database path:', error)
    alert('保存数据库位置失败: ' + String(error))
  } finally {
    isSavingDb.value = false
  }
}

const totalFiles = ref(0)
const scrapedFiles = ref(0)
const isScanningOnly = ref(false)
const isScrapingOnly = ref(false)
const libraryScanProgress = ref('')
const libraryScrapeProgress = ref('')

const fetchStats = async () => {
  try {
    const episodes = await api.get<any[]>('/episodes')
    totalFiles.value = episodes.length
    
    const status = await danmakuApi.getTaskStatus()
    scrapedFiles.value = status?.scrape?.current || 0
  } catch (e) {
    console.error('Failed to fetch stats:', e)
  }
}

const runScan = async () => {
  if (isScanningOnly.value) return
  isScanningOnly.value = true
  try {
    await danmakuApi.triggerScan()
    libraryScanProgress.value = '扫描中...'
    while (true) {
      const status = await danmakuApi.getTaskStatus()
      if (status?.scan?.status === 'completed' || status?.scan?.status === 'idle') {
        break
      }
      libraryScanProgress.value = `扫描中 ${status?.scan?.percentage || 0}%`
      await new Promise(r => setTimeout(r, 1000))
    }
    libraryScanProgress.value = '扫描完成'
    setTimeout(() => { libraryScanProgress.value = '' }, 3000)
    await fetchStats()
  } catch (e) {
    libraryScanProgress.value = '扫描失败'
    setTimeout(() => { libraryScanProgress.value = '' }, 3000)
  } finally {
    isScanningOnly.value = false
  }
}

const runScrape = async () => {
  if (isScrapingOnly.value) return
  isScrapingOnly.value = true
  try {
    await danmakuApi.triggerScrape()
    libraryScrapeProgress.value = '刮削中...'
    while (true) {
      const status = await danmakuApi.getTaskStatus()
      if (status?.scrape?.status === 'completed' || status?.scrape?.status === 'idle') {
        break
      }
      const current = status?.scrape?.current || 0
      const total = status?.scrape?.total || 0
      libraryScrapeProgress.value = `刮削中 ${current}/${total}`
      await new Promise(r => setTimeout(r, 1000))
    }
    libraryScrapeProgress.value = '刮削完成'
    setTimeout(() => { libraryScrapeProgress.value = '' }, 3000)
    await fetchStats()
  } catch (e) {
    libraryScrapeProgress.value = '刮削失败'
    setTimeout(() => { libraryScrapeProgress.value = '' }, 3000)
  } finally {
    isScrapingOnly.value = false
  }
}

const showLogs = ref(false)
const systemLogs = ref<any[]>([])
const loadingLogs = ref(false)
const logFilterLevel = ref('')

const toggleLogs = async () => {
  showLogs.value = !showLogs.value
  if (showLogs.value) {
    await fetchLogs()
  }
}

const fetchLogs = async () => {
  loadingLogs.value = true
  try {
    const params: Record<string, string> = { limit: '100' }
    if (logFilterLevel.value) {
      params.level = logFilterLevel.value
    }
    const res = await api.get<{ logs: any[]; total: number }>('/logs', params)
    systemLogs.value = res.logs || []
  } catch (error) {
    console.error('Failed to fetch logs:', error)
  } finally {
    loadingLogs.value = false
  }
}

const formatLogTime = (timeStr: string) => {
  try {
    const date = new Date(timeStr)
    return date.toLocaleTimeString()
  } catch (e) {
    return timeStr
  }
}

const formatLogAttrs = (attrs?: Record<string, string>) => {
  if (!attrs || Object.keys(attrs).length === 0) return ''
  return Object.entries(attrs)
    .map(([k, v]) => `${k}=${v}`)
    .join(' ')
}

onMounted(() => {
  loadSettings()
  loadLibraries()
  checkHealth()
  fetchStats()
})
</script>

<template>
  <div class="p-8 max-w-4xl h-full flex flex-col">
    <header class="mb-6 flex justify-between items-end">
      <div>
        <h2 class="text-3xl font-bold text-slate-800">系统设置</h2>
        <p class="text-slate-500 mt-1">配置播放器、目录与刮削偏好</p>
      </div>
    </header>


    <!-- Tabs Navigation -->
    <div class="flex border-b border-slate-200 mb-6 bg-slate-100/50 p-1.5 rounded-xl">
      <button 
        @click="currentTab = 'basic'"
        class="flex-1 py-2 text-sm font-semibold rounded-lg flex items-center justify-center gap-2 transition-all"
        :class="currentTab === 'basic' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
      >
        <Database :size="16" />
        基本设置
      </button>
      <button 
        @click="currentTab = 'player'"
        class="flex-1 py-2 text-sm font-semibold rounded-lg flex items-center justify-center gap-2 transition-all"
        :class="currentTab === 'player' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
      >
        <MonitorPlay :size="16" />
        播放器配置
      </button>
      <button 
        @click="currentTab = 'api'"
        class="flex-1 py-2 text-sm font-semibold rounded-lg flex items-center justify-center gap-2 transition-all"
        :class="currentTab === 'api' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
      >
        <Key :size="16" />
        账户和 API
      </button>
      <button 
        @click="currentTab = 'danmaku'"
        class="flex-1 py-2 text-sm font-semibold rounded-lg flex items-center justify-center gap-2 transition-all"
        :class="currentTab === 'danmaku' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-800'"
      >
        <Type :size="16" />
        弹幕外观
      </button>
    </div>

    <div class="flex-1 overflow-y-auto">
      <div class="bg-white border border-slate-200 rounded-xl p-6 shadow-sm min-h-[300px]">
        
        <!-- Tab 1: Basic Settings -->
        <div v-if="currentTab === 'basic'" class="space-y-6">
          <!-- DB Path -->
          <div>
            <h3 class="text-base font-bold text-slate-800 mb-3 flex items-center gap-2">
              <Database class="text-blue-500" :size="18" />
              数据库位置
            </h3>
            <div class="flex gap-2">
              <input 
                type="text" 
                v-model="dbPath"
                placeholder="例如: data/danmaku.db"
                class="flex-1 bg-slate-50 border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 shadow-sm transition-all"
              />
              <button @click="browseDbPath" class="px-3 py-2 bg-white hover:bg-slate-50 text-slate-600 rounded-lg text-sm transition-colors border border-slate-200 shadow-sm" title="浏览文件">
                <FolderOpen :size="16" />
              </button>
              <button 
                @click="saveDbPath"
                :disabled="isSavingDb"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-70 text-white rounded-lg text-sm font-medium transition-colors shadow-sm flex items-center gap-1.5"
              >
                <Check v-if="saveDbSuccess" :size="16" />
                <Save v-else :size="16" />
                {{ saveDbSuccess ? '已保存' : (isSavingDb ? '保存中...' : '保存') }}
              </button>
            </div>
            <p class="text-xs text-slate-500 mt-1.5">修改数据库位置会自动迁移数据到新位置并重启服务。</p>
          </div>

          <hr class="border-slate-100" />

          <!-- Media Libraries -->
          <div>
            <h3 class="text-base font-bold text-slate-800 mb-3 flex items-center gap-2">
              <HardDrive class="text-indigo-500" :size="18" />
              媒体库管理
            </h3>

            <!-- View Media Libraries -->
            <div v-if="libraries.length === 0" class="text-sm text-slate-500 bg-slate-50 rounded-lg p-4 text-center border border-dashed border-slate-200 mb-4">
              暂无媒体库，请在下方添加。
            </div>
            <div v-else class="space-y-2.5 mb-4">
              <div v-for="lib in libraries" :key="lib.ID" class="flex items-center justify-between p-3.5 bg-slate-50 rounded-lg border border-slate-200 shadow-sm">
                <div class="flex items-center gap-2.5 truncate">
                  <FolderOpen class="text-slate-400 flex-shrink-0" :size="16" />
                  <span class="text-sm font-medium text-slate-700 truncate">{{ lib.RootPath }}</span>
                </div>
                <button 
                  @click="deleteLibrary(lib.ID)" 
                  class="p-2 text-red-500 hover:bg-red-50 rounded-lg transition-colors border border-transparent hover:border-red-100"
                  title="删除媒体库"
                >
                  <Trash2 :size="16" />
                </button>
              </div>
            </div>

            <!-- Add Library -->
            <div class="flex gap-2">
              <input 
                type="text" 
                v-model="newLibraryPath"
                placeholder="添加媒体库根路径，例如: D:\Anime"
                class="flex-1 bg-white border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 shadow-sm transition-all"
              />
              <button @click="browseNewLibraryDir" class="px-3 py-2 bg-white hover:bg-slate-50 text-slate-600 rounded-lg text-sm transition-colors border border-slate-200 shadow-sm" title="浏览目录">
                <FolderOpen :size="16" />
              </button>
              <button @click="addLibrary" class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm font-medium transition-colors shadow-sm">
                添加媒体库
              </button>
            </div>
          </div>

          <hr class="border-slate-100" />

          <!-- DB Scan & Scrape Stats -->
          <div>
            <h3 class="text-base font-bold text-slate-800 mb-3 flex items-center gap-2">
              <FolderOpen class="text-amber-500" :size="18" />
              数据统计与任务
            </h3>
            
            <div class="grid grid-cols-2 gap-4 mb-4">
              <div class="p-4 bg-slate-50 border border-slate-200 rounded-lg shadow-sm">
                <div class="text-xs text-slate-500 font-medium">所有本地文件</div>
                <div class="text-2xl font-bold text-slate-800 mt-1">{{ totalFiles }} <span class="text-xs font-normal text-slate-500">个文件</span></div>
              </div>
              <div class="p-4 bg-slate-50 border border-slate-200 rounded-lg shadow-sm">
                <div class="text-xs text-slate-500 font-medium">所有已刮削文件</div>
                <div class="text-2xl font-bold text-slate-800 mt-1">{{ scrapedFiles }} <span class="text-xs font-normal text-slate-500">个文件</span></div>
              </div>
            </div>

            <div class="flex items-center justify-between p-4 bg-slate-50 border border-slate-200 rounded-lg shadow-sm">
              <div class="flex gap-3">
                <button 
                  @click="runScan" 
                  :disabled="isScanningOnly"
                  class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
                >
                  {{ isScanningOnly ? '正在扫描...' : '立即扫描' }}
                </button>
                
                <button 
                  @click="runScrape" 
                  :disabled="isScrapingOnly"
                  class="px-4 py-2 bg-emerald-600 hover:bg-emerald-700 disabled:bg-emerald-400 text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
                >
                  {{ isScrapingOnly ? '正在刮削...' : '立即刮削' }}
                </button>
              </div>

              <div class="flex flex-col items-end text-xs font-semibold space-y-1">
                <span v-if="libraryScanProgress" class="text-blue-600 animate-pulse">{{ libraryScanProgress }}</span>
                <span v-if="libraryScrapeProgress" class="text-emerald-600 animate-pulse">{{ libraryScrapeProgress }}</span>
              </div>
            </div>
          </div>

          <hr class="border-slate-100" />

          <!-- Logs Viewer -->
          <div>
            <h3 class="text-base font-bold text-slate-800 mb-3 flex items-center justify-between">
              <span class="flex items-center gap-2">
                <FileText class="text-slate-500" :size="18" />
                系统运行日志
              </span>
              <button 
                @click="toggleLogs"
                class="text-xs text-blue-600 hover:underline font-semibold focus:outline-none"
              >
                {{ showLogs ? '收起日志' : '展开查看日志' }}
              </button>
            </h3>
            
            <div v-if="showLogs" class="space-y-3">
              <div class="flex justify-between items-center gap-2">
                <div class="flex gap-2">
                  <select 
                    v-model="logFilterLevel" 
                    class="bg-white border border-slate-200 rounded-lg px-2.5 py-1.5 text-xs text-slate-700 focus:outline-none shadow-sm"
                    @change="fetchLogs"
                  >
                    <option value="">全部级别</option>
                    <option value="INFO">INFO</option>
                    <option value="WARN">WARN</option>
                    <option value="ERROR">ERROR</option>
                  </select>
                  <button 
                    @click="fetchLogs" 
                    class="px-2.5 py-1.5 bg-white hover:bg-slate-50 border border-slate-200 text-slate-600 rounded-lg text-xs font-medium shadow-sm transition-colors"
                  >
                    刷新
                  </button>
                </div>
                <span class="text-[10px] text-slate-400">仅显示最近 100 条日志</span>
              </div>
              
              <div class="bg-slate-900 text-slate-300 font-mono text-xs rounded-xl p-4 overflow-y-auto max-h-[300px] border border-slate-800 shadow-inner custom-scrollbar">
                <div v-if="loadingLogs" class="text-slate-500 text-center py-4">加载中...</div>
                <div v-else-if="systemLogs.length === 0" class="text-slate-500 text-center py-4">暂无日志记录</div>
                <div v-else class="space-y-1.5">
                  <div v-for="(log, idx) in systemLogs" :key="idx" class="whitespace-pre-wrap break-all border-b border-slate-800/40 pb-1 leading-relaxed">
                    <span class="text-slate-500">[{{ formatLogTime(log.time) }}]</span>
                    <span 
                      class="font-bold px-1 rounded ml-1"
                      :class="{
                        'text-green-400 bg-green-950/30': log.level === 'INFO',
                        'text-yellow-400 bg-yellow-950/30': log.level === 'WARN',
                        'text-red-400 bg-red-950/30': log.level === 'ERROR'
                      }"
                    >
                      {{ log.level }}
                    </span>
                    <span class="text-slate-300 ml-1.5">{{ log.msg }}</span>
                    <span v-if="log.attrs && Object.keys(log.attrs).length > 0" class="text-slate-400 ml-2 text-[10px] italic">
                      {{ formatLogAttrs(log.attrs) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 2: Player Config -->
        <div v-if="currentTab === 'player'" class="space-y-5">
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-2">默认播放器路径 (mpv / MPC-BE)</label>
            <div class="flex gap-2">
              <input 
                type="text" 
                v-model="playerPath"
                class="flex-1 bg-white border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-purple-500 focus:ring-1 focus:ring-purple-500 shadow-sm transition-all"
              />
              <button @click="browsePlayerPath" class="px-3 py-2 bg-white hover:bg-slate-50 text-slate-600 rounded-lg text-sm transition-colors border border-slate-200 shadow-sm" title="浏览文件">
                <FolderOpen :size="16" />
              </button>
              <button 
                @click="savePlayerConfig"
                :disabled="isSavingPlayer"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-70 text-white rounded-lg text-sm font-medium transition-colors shadow-sm flex items-center gap-1.5"
              >
                <Check v-if="savePlayerSuccess" :size="16" />
                <Save v-else :size="16" />
                {{ savePlayerSuccess ? '已保存' : (isSavingPlayer ? '保存中...' : '保存') }}
              </button>
            </div>
          </div>
          
          <div class="flex items-center justify-between gap-3 bg-slate-50 p-4 rounded-lg border border-slate-200 shadow-sm">
            <div class="flex items-center gap-3">
              <input type="checkbox" id="rpc" v-model="discordRpc" @change="saveDiscordRpc" class="w-4 h-4 rounded border-slate-300 text-purple-600 focus:ring-purple-500" />
              <label for="rpc" class="text-sm text-slate-700 select-none">自动开启 Discord Rich Presence 状态同步</label>
            </div>
            <span v-if="savingRpc" class="text-xs text-slate-400 animate-pulse">正在保存...</span>
            <span v-else-if="saveRpcSuccess" class="text-xs text-green-600 flex items-center gap-1"><Check :size="12" />已保存</span>
          </div>
        </div>

        <!-- Tab 3: Account and API -->
        <div v-if="currentTab === 'api'" class="space-y-5">
          <p class="text-sm text-slate-500">在此手动输入 Bangumi App 凭证及 Access Token。配置完成后，刮削和状态同步才能正常运行。</p>
          
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-slate-600 mb-1.5">Bangumi App ID</label>
              <input 
                type="text" 
                v-model="bangumiAppId"
                placeholder="例如: bgm1234567890abcdef"
                class="w-full bg-white border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-yellow-500 focus:ring-1 focus:ring-yellow-500 shadow-sm transition-all"
              />
            </div>
            
            <div>
              <label class="block text-sm font-medium text-slate-600 mb-1.5">Bangumi App Secret</label>
              <input 
                type="password" 
                v-model="bangumiAppSecret"
                placeholder="输入 App Secret"
                class="w-full bg-white border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-yellow-500 focus:ring-1 focus:ring-yellow-500 shadow-sm transition-all"
              />
            </div>
            
            <div>
              <label class="block text-sm font-medium text-slate-600 mb-1.5">Bangumi Access Token (Token)</label>
              <input 
                type="password" 
                v-model="bangumiAccessToken"
                placeholder="输入用户 Access Token"
                class="w-full bg-white border border-slate-200 rounded-lg px-4 py-2 text-sm text-slate-800 focus:outline-none focus:border-yellow-500 focus:ring-1 focus:ring-yellow-500 shadow-sm transition-all"
              />
              <p class="text-xs text-slate-400 mt-1">可以直接填入你在 Bangumi 开发者设置中生成的“个人授权口令”。</p>
            </div>

            <div class="flex justify-end pt-2">
              <button 
                @click="saveApiKeys"
                :disabled="isSavingApi"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-70 text-white rounded-lg text-sm font-medium transition-colors shadow-sm flex items-center gap-1.5"
              >
                <Check v-if="saveApiSuccess" :size="16" />
                <Save v-else :size="16" />
                {{ saveApiSuccess ? '已保存凭证' : (isSavingApi ? '保存中...' : '保存 API 凭证') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Tab 4: Danmaku Settings -->
        <div v-if="currentTab === 'danmaku'" class="space-y-5">
          <!-- Font Size -->
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-2">字体大小 ({{ danmakuSettings.fontSize }}px)</label>
            <input
              type="range"
              v-model.number="danmakuSettings.fontSize"
              min="12"
              max="48"
              step="1"
              class="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-cyan-500"
            />
          </div>

          <!-- Opacity -->
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-2">透明度 ({{ Math.round(danmakuSettings.opacity * 100) }}%)</label>
            <input
              type="range"
              v-model.number="danmakuSettings.opacity"
              min="0.1"
              max="1.0"
              step="0.05"
              class="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-cyan-500"
            />
          </div>

          <!-- Speed -->
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-2">滚动速度 ({{ danmakuSettings.speed }})</label>
            <input
              type="range"
              v-model.number="danmakuSettings.speed"
              min="50"
              max="300"
              step="10"
              class="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-cyan-500"
            />
          </div>

          <!-- Density -->
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-2">同屏密度限制 ({{ danmakuSettings.density === 0 ? '无限制' : danmakuSettings.density }})</label>
            <input
              type="range"
              v-model.number="danmakuSettings.density"
              min="0"
              max="100"
              step="5"
              class="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-cyan-500"
            />
          </div>

          <!-- Toggles -->
          <div class="flex flex-wrap gap-4 pt-2">
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="danmakuSettings.showScroll" class="w-4 h-4 rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
              <span class="text-sm text-slate-700 select-none">滚动弹幕</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="danmakuSettings.showTop" class="w-4 h-4 rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
              <span class="text-sm text-slate-700 select-none">顶部弹幕</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" v-model="danmakuSettings.showBottom" class="w-4 h-4 rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
              <span class="text-sm text-slate-700 select-none">底部弹幕</span>
            </label>
          </div>

          <div class="flex justify-end pt-4 border-t border-slate-100">
            <button 
              @click="saveDanmakuSettings"
              :disabled="isSavingDanmaku"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-70 text-white rounded-lg text-sm font-medium transition-colors shadow-sm flex items-center gap-1.5"
            >
              <Check v-if="saveDanmakuSuccess" :size="16" />
              <Save v-else :size="16" />
              {{ saveDanmakuSuccess ? '外观已保存' : (isSavingDanmaku ? '保存中...' : '保存外观设置') }}
            </button>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>
