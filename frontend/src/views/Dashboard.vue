<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Play, X, CheckCircle2, Circle, FileSearch, HardDrive } from 'lucide-vue-next'
import { danmakuApi, type Series, type Episode, type Library } from '../services/danmaku'
import { useSearch } from '../composables/useSearch'
import { useScan } from '../composables/useScan'
import FolderTreeNode from '../components/FolderTreeNode.vue'

const route = useRoute()
const { searchQuery } = useSearch()
const { scanCounter } = useScan()

const seriesList = ref<Series[]>([])
const episodesList = ref<Episode[]>([])
const loading = ref(false)

// Folder View State & Actions
const libraryGroupedFiles = ref<{ library: Library; files: any[] }[]>([])
const loadingFolders = ref(false)

const loadSeries = async (search?: string) => {
  loading.value = true
  try {
    seriesList.value = await danmakuApi.getSeries(search)
  } catch (e) {
    console.error('Failed to load series:', e)
  } finally {
    loading.value = false
  }
}

const loadFolderViewData = async () => {
  loadingFolders.value = true
  try {
    const libs = await danmakuApi.getLibraries()
    const groups = []
    for (const lib of libs) {
      const files = await danmakuApi.getLibraryFiles(lib.ID)
      groups.push({
        library: lib,
        files: files
      })
    }
    libraryGroupedFiles.value = groups
  } catch (e) {
    console.error('Failed to load folder view data:', e)
  } finally {
    loadingFolders.value = false
  }
}

interface TreeNode {
  name: string
  path: string
  type: 'directory' | 'file'
  children?: TreeNode[]
  fileData?: any
}

const buildFileTree = (files: any[]) => {
  const root: TreeNode = { name: 'root', path: '', type: 'directory', children: [] }
  
  files.forEach(f => {
    const normalizedPath = f.relative_path.replace(/\\/g, '/')
    const parts = normalizedPath.split('/')
    let current = root
    let currentPath = ''
    
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      if (!part) continue
      
      currentPath = currentPath ? `${currentPath}/${part}` : part
      const isLast = i === parts.length - 1
      
      let next = current.children?.find(child => child.name === part)
      if (!next) {
        next = {
          name: part,
          path: currentPath,
          type: isLast ? 'file' : 'directory',
          children: isLast ? undefined : []
        }
        if (isLast) {
          next.fileData = f
        }
        current.children?.push(next)
      }
      current = next
    }
  })
  
  const sortTree = (node: TreeNode) => {
    if (node.children) {
      node.children.sort((a, b) => {
        if (a.type !== b.type) {
          return a.type === 'directory' ? -1 : 1
        }
        return a.name.localeCompare(b.name, 'zh-CN')
      })
      node.children.forEach(sortTree)
    }
  }
  sortTree(root)
  return root.children || []
}

const playLibraryFile = async (file: any) => {
  try {
    await danmakuApi.playEpisode(file.id)
  } catch (e) {
    alert('播放失败: ' + String(e))
  }
}

watch(searchQuery, (newQuery) => {
  if (currentView.value !== 'folder') {
    loadSeries(newQuery || undefined)
  }
})

watch(scanCounter, () => {
  if (currentView.value === 'folder') {
    loadFolderViewData()
  } else {
    loadSeries(searchQuery.value || undefined)
  }
})

const loadEpisodes = async (seriesId: string) => {
  try {
    const list = await danmakuApi.getEpisodes(seriesId)
    episodesList.value = list.sort((a, b) => (a.EpIndex || 0) - (b.EpIndex || 0))
  } catch (e) {
    console.error('Failed to load episodes:', e)
  }
}

const currentView = computed(() => route.query.view || 'season')

watch(currentView, (newView) => {
  if (newView === 'folder') {
    loadFolderViewData()
  } else if (newView !== 'unmatched') {
    loadSeries(searchQuery.value || undefined)
  }
}, { immediate: true })

onMounted(() => {
  if (currentView.value === 'folder') {
    loadFolderViewData()
  } else {
    loadSeries()
  }
})

const unmatchedItems = computed(() =>
  seriesList.value.filter(s => !s.BangumiID)
)

const matchedItems = computed(() =>
  seriesList.value.filter(s => s.BangumiID)
)

const formatAirDateToSeason = (dateStr?: string) => {
  if (!dateStr || dateStr === '未知') return '未知'
  const parts = dateStr.split('-')
  if (parts.length >= 2) {
    const year = parts[0]
    const month = parseInt(parts[1], 10)
    let season = ''
    if (month >= 1 && month <= 3) season = '冬季'
    else if (month >= 4 && month <= 6) season = '春季'
    else if (month >= 7 && month <= 9) season = '夏季'
    else if (month >= 10 && month <= 12) season = '秋季'
    return `${year}年${season}`
  }
  return dateStr
}

const groupedData = computed(() => {
  const view = currentView.value
  if (view === 'name') {
    const sorted = [...seriesList.value].sort((a, b) => {
      const nameA = a.NameCN || a.Title || ''
      const nameB = b.NameCN || b.Title || ''
      return nameA.localeCompare(nameB, 'zh-CN')
    })
    return [{ name: '所有番剧 (按中文名称排序)', items: sorted }]
  }
  if (view === 'recent') {
    const sorted = [...seriesList.value].sort((a, b) => {
      const timeA = a.LastPlayedAt && a.LastPlayedAt !== '0001-01-01T00:00:00Z' ? new Date(a.LastPlayedAt).getTime() : 0
      const timeB = b.LastPlayedAt && b.LastPlayedAt !== '0001-01-01T00:00:00Z' ? new Date(b.LastPlayedAt).getTime() : 0
      return timeB - timeA
    })
    return [{ name: '最近播放的番剧', items: sorted }]
  }
  if (view === 'rating') {
    const sorted = [...seriesList.value].sort((a, b) => {
      const ratingA = a.Rating || 0
      const ratingB = b.Rating || 0
      return ratingB - ratingA
    })
    return [{ name: '评分最高的番剧', items: sorted }]
  }
  const groups: Record<string, Series[]> = {}
  matchedItems.value.forEach(item => {
    const key = formatAirDateToSeason(item.AirDate) || '未知'
    if (!groups[key]) groups[key] = []
    groups[key].push(item)
  })
  const getSeasonValue = (key: string): number => {
    if (key === '未知') return 0
    const match = key.match(/^(\d+)年(冬季|春季|夏季|秋季)$/)
    if (match) {
      const year = parseInt(match[1], 10)
      const season = match[2]
      let seasonVal = 0
      if (season === '冬季') seasonVal = 1
      else if (season === '春季') seasonVal = 2
      else if (season === '夏季') seasonVal = 3
      else if (season === '秋季') seasonVal = 4
      return year * 10 + seasonVal
    }
    return 0
  }
  const sortedKeys = Object.keys(groups).sort((a, b) => getSeasonValue(b) - getSeasonValue(a))
  return sortedKeys.map(k => {
    const sortedItems = groups[k].sort((a, b) => {
      const numA = parseInt(a.ID, 10)
      const numB = parseInt(b.ID, 10)
      if (!isNaN(numA) && !isNaN(numB)) {
        return numA - numB
      }
      return a.ID.localeCompare(b.ID)
    })
    return { name: k, items: sortedItems }
  })
})

const viewTitle = computed(() => {
  const v = currentView.value
  if (v === 'name') return '媒体库 (按中文名称)'
  if (v === 'folder') return '媒体库 (本地文件夹)'
  if (v === 'unmatched') return '未识别内容'
  if (v === 'recent') return '最近播放'
  if (v === 'rating') return '评分最高'
  return '番剧季度'
})

const coverUrl = (s: Series) => {
  if (s.CoverPath) return `/covers/${s.CoverPath.replace('covers/', '')}`
  return 'https://placehold.co/300x400/e2e8f0/64748b?text=No+Cover'
}

const selectedSeries = ref<Series | null>(null)
const showComments = ref(false)
const showAllTags = ref(false)
const showAllSummary = ref(false)

const parsedTags = computed(() => {
  if (!selectedSeries.value?.Tags) return []
  
  let rawTags: string[] = []
  const tagsStr = selectedSeries.value.Tags.trim()
  if (tagsStr.startsWith('[') && tagsStr.endsWith(']')) {
    try {
      rawTags = JSON.parse(tagsStr)
    } catch (e) {
      rawTags = tagsStr.split(',')
    }
  } else {
    rawTags = tagsStr.split(',')
  }

  return rawTags
    .map(t => t.replace(/['"“”]+/g, '').trim())
    .filter(t => {
      if (!t) return false
      // 过滤年份 (如 2024, 2024年)
      if (/^\d{4}$/.test(t) || /^\d{4}年$/.test(t)) return false
      // 过滤年月 (如 2024-05, 2024/05, 2024年05月)
      if (/^\d{4}[-/\.]\d{1,2}$/.test(t) || /^\d{4}年\d{1,2}月$/.test(t)) return false
      return true
    })
})

const openDetails = async (item: Series) => {
  selectedSeries.value = item
  showComments.value = false
  showAllTags.value = false
  showAllSummary.value = false
  await loadEpisodes(item.ID)
}

const closeModal = () => {
  selectedSeries.value = null
  episodesList.value = []
}

const playEpisode = async (ep: Episode) => {
  try {
    await danmakuApi.playEpisode(ep.ID)
  } catch (e) {
    alert('播放失败: ' + String(e))
  }
  closeModal()
}

const selectedUnmatchedFile = ref<Series | null>(null)
const manualSearchQuery = ref('')
const manualSearchResults = ref<any[]>([])
const manualSearching = ref(false)
const manualBangumiId = ref('')

const openManualMatch = (file: Series) => {
  selectedUnmatchedFile.value = file
  manualSearchQuery.value = ''
  manualSearchResults.value = []
  manualBangumiId.value = ''
}
const closeManualMatch = () => {
  selectedUnmatchedFile.value = null
}

const searchBangumi = async () => {
  if (!manualSearchQuery.value.trim()) return
  manualSearching.value = true
  try {
    const results = await danmakuApi.searchBangumi(manualSearchQuery.value)
    manualSearchResults.value = results
  } catch (e) {
    console.error('Failed to search bangumi:', e)
    manualSearchResults.value = []
  } finally {
    manualSearching.value = false
  }
}

const bindToSeries = async (bangumiId: number) => {
  if (!selectedUnmatchedFile.value) return
  try {
    await danmakuApi.bindBangumi(Number(selectedUnmatchedFile.value.ID), bangumiId)
    closeManualMatch()
    await loadSeries()
  } catch (e) {
    console.error('Failed to bind:', e)
    alert('绑定失败: ' + String(e))
  }
}
</script>

<template>
  <div class="p-8 h-full flex flex-col overflow-y-auto relative">
    <!-- Header -->
    <header class="mb-8 flex-shrink-0 flex justify-between items-start">
      <div>
        <h2 class="text-3xl font-bold text-slate-800">{{ viewTitle }}</h2>
        <p class="text-slate-500 mt-1" v-if="currentView === 'unmatched'">扫描发现但无法自动刮削的具体视频文件</p>
      </div>
    </header>



    <!-- Content: Unmatched Files List -->
    <div v-if="currentView === 'unmatched'" class="flex-1 space-y-4 pb-8">
       <div v-if="unmatchedItems.length === 0" class="text-slate-400 py-4">无未识别的文件</div>

       <div v-for="file in unmatchedItems" :key="file.ID"
            class="flex items-center justify-between p-4 bg-white rounded-xl border border-slate-200 hover:shadow-md hover:border-blue-200 transition-all">
          <div class="flex items-center gap-5">
             <div class="w-16 h-20 rounded-md overflow-hidden bg-slate-100 border border-slate-200 shadow-sm flex-shrink-0">
                <img :src="coverUrl(file)" class="w-full h-full object-cover" />
             </div>
             <div class="flex flex-col">
                <h4 class="font-bold text-slate-800 text-[15px] mb-1 break-all">{{ file.Title }}</h4>
             </div>
          </div>

          <button @click="openManualMatch(file)" class="ml-4 flex-shrink-0 bg-blue-50 text-blue-600 hover:bg-blue-600 hover:text-white px-5 py-2.5 rounded-lg text-sm font-bold transition-colors border border-blue-200 hover:border-blue-600 flex items-center gap-2 shadow-sm">
             <FileSearch :size="16" />
             手动识别
          </button>
       </div>
    </div>

    <!-- Content: Folder View -->
    <div v-else-if="currentView === 'folder'" class="space-y-8 pb-8">
      <div v-if="loadingFolders" class="text-slate-400 py-8 text-center">加载中...</div>
      <div v-else-if="libraryGroupedFiles.length === 0" class="text-slate-400 py-8 text-center">无已添加的媒体库，请前往系统设置添加</div>
      
      <div v-for="group in libraryGroupedFiles" :key="group.library.ID" class="bg-white border border-slate-200 rounded-2xl overflow-hidden shadow-sm">
        <div class="px-6 py-4 bg-slate-50 border-b border-slate-200 flex items-center gap-3">
          <HardDrive class="text-indigo-500 flex-shrink-0" :size="20" />
          <div>
            <h3 class="font-bold text-slate-800 text-sm md:text-base">媒体库: {{ group.library.RootPath }}</h3>
            <p class="text-xs text-slate-500 mt-0.5">共 {{ group.files.length }} 个文件</p>
          </div>
        </div>
        
        <div class="p-6 space-y-4">
          <div v-if="group.files.length === 0" class="text-slate-400 text-sm py-4 text-center">该媒体库下暂无扫描到的视频文件</div>
          
          <div v-else class="space-y-1">
            <FolderTreeNode 
              v-for="node in buildFileTree(group.files)" 
              :key="node.path" 
              :node="node" 
              :depth="0"
              @play="playLibraryFile"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Content: Matched Series Grid -->
    <div v-else class="space-y-10 pb-8">
      <div v-if="loading" class="text-slate-400 py-8 text-center">加载中...</div>
      <div v-else-if="seriesList.length === 0" class="text-slate-400 py-8 text-center">无内容，请先扫描媒体库</div>

      <section v-for="group in groupedData" :key="group.name">
        <h3 class="text-xl font-bold text-slate-800 border-b border-slate-200 pb-2 mb-6">{{ group.name }}</h3>

        <div v-if="group.items.length === 0" class="text-slate-400 py-4">无内容</div>

        <div v-else class="grid grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-6">
          <div
            v-for="item in group.items"
            :key="item.ID"
            @click="openDetails(item)"
            class="group relative bg-white rounded-xl overflow-hidden border border-slate-200 hover:border-blue-300 transition-all duration-300 hover:shadow-xl hover:shadow-blue-500/10 hover:-translate-y-1 cursor-pointer"
          >
            <div class="aspect-[3/4] w-full relative overflow-hidden bg-slate-100">
              <img :src="coverUrl(item)" :alt="item.Title" class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105" />
              <div class="absolute inset-0 bg-gradient-to-t from-slate-900/80 via-transparent to-transparent opacity-80 transition-opacity duration-300"></div>
              <div class="absolute top-2 right-2 px-2 py-0.5 rounded bg-white/90 backdrop-blur text-[10px] font-bold border"
                :class="item.BangumiID ? 'border-green-500 text-green-600' : 'border-orange-500 text-orange-600'">
                {{ item.BangumiID ? '已匹配' : '未识别' }}
              </div>
              <div v-if="item.TotalEps" class="absolute bottom-2 left-2 text-white text-xs font-medium px-2 py-1 bg-black/60 rounded backdrop-blur-sm shadow">
                {{ item.TotalEps }} 集
              </div>
            </div>
            <div class="p-3">
              <div class="flex items-center justify-between gap-1">
                <h4 class="font-bold text-slate-800 text-sm truncate" :title="item.NameCN || item.Title">{{ item.NameCN || item.Title }}</h4>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <!-- Details Modal -->
    <transition name="modal">
      <div v-if="selectedSeries" class="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/50 backdrop-blur-sm p-4" @click="closeModal">
        <div class="bg-white rounded-2xl shadow-2xl w-full max-w-[1100px] h-[80vh] min-h-[600px] flex overflow-hidden relative" @click.stop>

          <button @click="closeModal" class="absolute top-4 right-4 p-2 bg-slate-100 hover:bg-slate-200 rounded-full text-slate-500 transition-colors z-20">
            <X :size="20" />
          </button>

          <!-- Left Side -->
          <div class="w-[45%] bg-slate-50 border-r border-slate-200 flex flex-col relative overflow-y-auto custom-scrollbar">
            <div class="p-8 flex-shrink-0 border-b border-slate-100">
              <div class="flex gap-6 mb-6">
                <div class="w-1/3 rounded-xl overflow-hidden shadow-md flex-shrink-0 relative self-start">
                  <img :src="coverUrl(selectedSeries)" class="w-full aspect-[3/4] object-cover" />
                </div>
                <div class="w-2/3 flex flex-col justify-center">
                  <h2 class="text-xl lg:text-2xl font-bold text-slate-900 mb-3 leading-tight truncate" :title="selectedSeries.NameCN || selectedSeries.Title">
                    {{ selectedSeries.NameCN || selectedSeries.Title }}
                  </h2>
                  <div class="flex flex-wrap items-center gap-2 text-xs text-slate-600 mb-3">
                    <span v-if="selectedSeries.TotalEps" class="bg-blue-100 text-blue-700 px-2 py-1 rounded font-bold flex-shrink-0">{{ selectedSeries.TotalEps }} 集</span>
                    <span v-if="selectedSeries.AirDate" class="bg-green-100 text-green-700 px-2 py-1 rounded font-bold flex-shrink-0">{{ selectedSeries.AirDate }}</span>
                    <span v-if="selectedSeries.Rating" class="bg-amber-100 text-amber-700 px-2 py-1 rounded font-bold flex-shrink-0 flex items-center gap-0.5">★ {{ selectedSeries.Rating.toFixed(1) }}</span>
                    
                    <div :class="showAllTags ? '' : 'max-h-[54px] overflow-hidden'" class="flex flex-wrap gap-2 transition-all duration-300">
                      <span v-for="tag in parsedTags" :key="tag" class="bg-slate-100 text-slate-600 px-2 py-1 rounded font-bold">
                        {{ tag }}
                      </span>
                    </div>
                    
                    <button 
                      v-if="parsedTags.length > 0"
                      @click="showAllTags = !showAllTags" 
                      class="text-xs text-blue-600 font-bold hover:underline focus:outline-none flex-shrink-0 ml-1"
                    >
                      {{ showAllTags ? '收起' : '展开' }}
                    </button>
                  </div>
                </div>
              </div>

              <!-- Summary Section -->
              <div v-if="selectedSeries.Summary" class="mb-6">
                <h3 class="text-sm font-bold text-slate-700 mb-2">番剧简介</h3>
                <p :class="showAllSummary ? '' : 'line-clamp-3'" class="text-sm text-slate-600 leading-relaxed transition-all duration-300">
                  {{ selectedSeries.Summary }}
                </p>
                <button 
                  @click="showAllSummary = !showAllSummary" 
                  class="text-xs text-blue-600 font-bold mt-2 hover:underline focus:outline-none"
                >
                  {{ showAllSummary ? '收起' : '展开全部' }}
                </button>
              </div>
            </div>
          </div>

          <!-- Right Side: Episodes -->
          <div class="w-[55%] flex flex-col h-full bg-white relative">
            <div class="px-8 py-5 border-b border-slate-100 bg-white sticky top-0 flex justify-between items-center z-10 shadow-sm">
              <h3 class="font-bold text-lg text-slate-800">剧集列表</h3>
              <span class="text-xs font-bold text-slate-500 bg-slate-100 px-3 py-1 rounded-full mr-12">共 {{ episodesList.length }} 话</span>
            </div>

            <div class="flex-1 overflow-y-auto p-6 grid grid-cols-1 md:grid-cols-2 gap-3 content-start bg-slate-50/30 custom-scrollbar">
              <div v-for="ep in episodesList" :key="ep.ID"
                   class="flex items-center justify-between p-3 rounded-xl border transition-all bg-white border-slate-200 hover:border-blue-300 hover:shadow-md">
                <div class="flex items-center gap-3">
                  <CheckCircle2 v-if="ep.DanmakuPath" class="text-green-500 flex-shrink-0" :size="18" />
                  <Circle v-else class="text-slate-300 flex-shrink-0" :size="18" />
                  <span class="font-medium text-sm text-slate-800">
                    {{ ep.EpIndex !== undefined ? `第 ${ep.EpIndex} 话` : ep.RelativePath.split('/').pop() }}
                  </span>
                </div>
                <button @click="playEpisode(ep)" class="p-2 rounded-full hover:bg-blue-50 text-blue-600 transition-colors flex-shrink-0" title="发送至播放器控制台">
                  <Play :size="16" class="fill-current" />
                </button>
              </div>
              <div v-if="episodesList.length === 0" class="col-span-2 text-slate-400 text-center py-8">暂无剧集数据</div>
            </div>
          </div>

        </div>
      </div>
    </transition>

    <!-- Manual Match Modal -->
    <transition name="modal">
      <div v-if="selectedUnmatchedFile" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-sm p-4" @click="closeManualMatch">
        <div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl max-h-[80vh] overflow-hidden flex flex-col" @click.stop>
          <div class="px-6 py-4 border-b border-slate-100 flex justify-between items-center bg-slate-50">
             <h3 class="font-bold text-slate-800">手动匹配番剧信息</h3>
             <button @click="closeManualMatch" class="text-slate-400 hover:text-slate-600 transition-colors"><X :size="20"/></button>
          </div>
          <div class="p-6 space-y-5 bg-white flex-1 overflow-y-auto">
             <div>
                <label class="block text-xs font-bold text-slate-400 mb-1.5 uppercase tracking-wide">当前未识别文件</label>
                <div class="text-sm text-slate-700 bg-slate-50 p-3 rounded-lg break-all border border-slate-200 font-medium">
                   {{ selectedUnmatchedFile.Title }}
                </div>
             </div>

             <div>
                <label class="block text-sm font-bold text-slate-700 mb-2">搜索番剧名称</label>
                <div class="flex gap-2">
                  <input 
                    type="text" 
                    v-model="manualSearchQuery" 
                    placeholder="例如: 葬送的芙莉莲" 
                    class="flex-1 bg-slate-50 border border-slate-300 rounded-lg px-4 py-2.5 text-sm focus:bg-white focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-all shadow-sm"
                    @keyup.enter="searchBangumi"
                  />
                  <button 
                    @click="searchBangumi" 
                    :disabled="manualSearching || !manualSearchQuery.trim()"
                    class="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-400 text-white rounded-lg text-sm font-medium transition-colors"
                  >
                    {{ manualSearching ? '搜索中...' : '搜索' }}
                  </button>
                </div>
             </div>

             <!-- Search Results -->
             <div v-if="manualSearchResults.length > 0" class="space-y-3">
                <label class="block text-sm font-bold text-slate-700">搜索结果 (点击选择)</label>
                <div v-for="result in manualSearchResults" :key="result.id" 
                     @click="bindToSeries(result.id)"
                     class="flex items-start gap-4 p-4 border border-slate-200 rounded-xl hover:border-blue-300 hover:bg-blue-50 cursor-pointer transition-all">
                  <img v-if="result.image_url" :src="result.image_url" class="w-16 h-22 object-cover rounded-lg flex-shrink-0" />
                  <div class="flex-1 min-w-0">
                    <h4 class="font-bold text-slate-800 text-sm">{{ result.name_cn || result.name }}</h4>
                    <p class="text-xs text-slate-500 mt-1">{{ result.name }}</p>
                    <div class="flex items-center gap-2 mt-2 text-xs text-slate-500">
                      <span v-if="result.date" class="bg-slate-100 px-2 py-0.5 rounded">{{ result.date }}</span>
                      <span v-if="result.total_episodes" class="bg-slate-100 px-2 py-0.5 rounded">{{ result.total_episodes }} 集</span>
                    </div>
                  </div>
                </div>
             </div>

             <div v-else-if="manualSearchQuery && !manualSearching" class="text-center text-slate-400 py-4">
                未找到结果，请尝试其他关键词
             </div>
          </div>
          <div class="px-6 py-4 border-t border-slate-100 bg-slate-50 flex justify-end gap-3">
             <button @click="closeManualMatch" class="px-5 py-2.5 rounded-lg text-sm font-bold text-slate-600 hover:bg-slate-200 transition-colors">取消</button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background-color: #cbd5e1;
  border-radius: 20px;
}
</style>
