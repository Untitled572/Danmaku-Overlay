<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Calendar, AArrowDown, Folder, ShieldQuestion, Settings as SettingsIcon, Search, Clock, Star } from 'lucide-vue-next'
import { danmakuApi } from './services/danmaku'
import { useSearch } from './composables/useSearch'

const route = useRoute()
const { searchQuery } = useSearch()

const backendStatus = ref<'ready' | 'warning' | 'error'>('error')

const checkHealth = async () => {
  try {
    const res = await danmakuApi.checkHealth()
    if (res.status === 'ok') {
      if (res.database === 'ready') {
        backendStatus.value = 'ready'
      } else {
        // 后端返回 status ok 但 database 未就绪 (如 uninitialized)
        backendStatus.value = 'warning'
      }
    } else {
      backendStatus.value = 'error'
    }
  } catch (e) {
    // 报错为红色
    backendStatus.value = 'error'
  }
}

let healthInterval: number

onMounted(() => {
  checkHealth()
  healthInterval = window.setInterval(checkHealth, 5000)
})

onUnmounted(() => {
  if (healthInterval) clearInterval(healthInterval)
})

const isView = (view: string) => {
  if (route.name !== 'dashboard') return false;
  if (!route.query.view && view === 'season') return true; // default
  return route.query.view === view;
}
</script>

<template>
  <div class="flex h-screen w-full bg-slate-50 text-slate-800 overflow-hidden font-sans">
    <!-- Sidebar -->
    <aside class="w-56 bg-white border-r border-slate-200 flex flex-col transition-all duration-300 shadow-sm z-10 flex-shrink-0">
      <div class="h-16 flex items-center px-5 border-b border-slate-100">
        <h1 class="text-lg font-bold bg-gradient-to-r from-blue-600 to-indigo-600 bg-clip-text text-transparent truncate">
          Danmaku Overlay
        </h1>
      </div>

      <div class="flex-1 py-6 px-3 space-y-1 overflow-y-auto">
        <div class="text-[11px] font-bold text-slate-400 mb-3 px-2 uppercase tracking-wider">展示样式</div>

        <router-link
          to="/?view=recent"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200"
          :class="isView('recent') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <Clock :size="18" />
          <span class="font-medium text-sm">最近播放</span>
        </router-link>

        <router-link
          to="/?view=season"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200"
          :class="isView('season') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <Calendar :size="18" />
          <span class="font-medium text-sm">番剧季度</span>
        </router-link>

        <router-link
          to="/?view=name"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200"
          :class="isView('name') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <AArrowDown :size="18" />
          <span class="font-medium text-sm">名称</span>
        </router-link>

        <router-link
          to="/?view=rating"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200"
          :class="isView('rating') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <Star :size="18" />
          <span class="font-medium text-sm">评分</span>
        </router-link>

        <div class="text-[11px] font-bold text-slate-400 mt-8 mb-3 px-2 uppercase tracking-wider">快捷操作</div>

        <router-link
          to="/?view=folder"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200 mb-1"
          :class="isView('folder') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <Folder :size="18" />
          <span class="font-medium text-sm">本地文件夹</span>
        </router-link>

        <router-link
          to="/?view=unmatched"
          class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors duration-200 mb-1"
          :class="isView('unmatched') ? 'bg-blue-50 text-blue-600' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-50'"
        >
          <ShieldQuestion :size="18" />
          <span class="font-medium text-sm">未识别内容</span>
        </router-link>
      </div>
    </aside>

    <!-- Main Content wrapper -->
    <div class="flex-1 flex flex-col h-full overflow-hidden bg-slate-50 relative">
      <!-- Top Header -->
      <header class="h-16 bg-white/90 backdrop-blur-md border-b border-slate-200 flex items-center justify-end px-6 z-10 flex-shrink-0">

        <!-- Right Side: Search, Settings -->
        <div class="flex items-center gap-4">
          <!-- Search -->
          <div class="relative group" v-if="route.name === 'dashboard'">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-blue-500 transition-colors" :size="16" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索番剧..."
              class="bg-slate-50 border border-slate-200 rounded-full pl-9 pr-4 py-1.5 text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all w-48 lg:w-64 shadow-sm"
            />
          </div>

          <!-- Settings -->
          <router-link
            to="/settings"
            class="p-2 rounded-full transition-colors duration-200 flex items-center gap-2"
            :class="route.name === 'settings' ? 'bg-blue-50 text-blue-600' : 'text-slate-500 hover:text-slate-800 hover:bg-slate-100'"
            title="系统设置 (Go Core Online)"
          >
            <div class="relative flex items-center justify-center">
              <SettingsIcon :size="22" />
              <!-- Backend Status Dot inside Settings gear -->
              <span class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 flex h-1.5 w-1.5 mt-[0.5px]">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75"
                      :class="{ 'bg-green-400': backendStatus === 'ready', 'bg-yellow-400': backendStatus === 'warning', 'bg-red-400': backendStatus === 'error' }"></span>
                <span class="relative inline-flex rounded-full h-1.5 w-1.5"
                      :class="{ 'bg-green-500': backendStatus === 'ready', 'bg-yellow-500': backendStatus === 'warning', 'bg-red-500': backendStatus === 'error' }"></span>
              </span>
            </div>
            <span class="text-sm font-medium pr-1" v-if="route.name === 'settings'">系统设置</span>
          </router-link>
        </div>
      </header>

      <!-- Main view -->
      <main class="flex-1 overflow-hidden relative">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
