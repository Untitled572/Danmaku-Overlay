<script setup lang="ts">
import { ref, computed } from 'vue'
import { Folder, FolderOpen, Play } from 'lucide-vue-next'

const props = defineProps<{
  node: {
    name: string
    path: string
    type: 'directory' | 'file'
    children?: any[]
    fileData?: any
  }
  depth: number
}>()

const emit = defineEmits(['play'])

const isExpanded = ref(false)
const toggle = () => {
  isExpanded.value = !isExpanded.value
}

const fileStatus = computed(() => {
  const file = props.node.fileData
  if (!file) return { text: '未知', class: 'bg-slate-50 border-slate-200 text-slate-700', dotClass: 'bg-slate-300' }

  const match = file.match_status || ''
  const scrape = file.scrape_status || ''

  // 已刮削/匹配成功
  if (scrape === 'completed' || match === 'completed' || match === 'matched') {
    return {
      text: '已匹配',
      class: 'bg-green-50 border-green-200 text-green-700',
      dotClass: 'bg-green-500'
    }
  }

  // 刮削/匹配失败
  if (scrape === 'no_match' || match === 'no_match') {
    return {
      text: '未匹配',
      class: 'bg-red-50 border-red-200 text-red-700',
      dotClass: 'bg-red-500'
    }
  }

  // 默认待刮削/匹配状态
  return {
    text: '未匹配',
    class: 'bg-orange-50 border-orange-200 text-orange-700',
    dotClass: 'bg-orange-500'
  }
})
</script>

<template>
  <div class="select-none">
    <!-- Directory Node -->
    <div 
      v-if="node.type === 'directory'" 
      @click="toggle"
      class="flex items-center gap-2 py-1.5 px-2 hover:bg-slate-100/80 rounded-lg cursor-pointer transition-colors"
      :style="{ paddingLeft: `${depth * 16 + 8}px` }"
    >
      <FolderOpen v-if="isExpanded" class="text-amber-500 flex-shrink-0" :size="16" />
      <Folder v-else class="text-amber-500 flex-shrink-0" :size="16" />
      <span class="text-sm font-semibold text-slate-700 truncate">{{ node.name }}</span>
      <span class="text-[10px] text-slate-400 font-bold bg-slate-100 px-1.5 py-0.5 rounded-full ml-1 flex-shrink-0">
        {{ node.children?.length || 0 }} 项
      </span>
    </div>

    <!-- File Node -->
    <div 
      v-else-if="node.type === 'file'"
      class="flex items-center justify-between py-1.5 px-2 hover:bg-slate-50 border-b border-slate-100/50 transition-colors"
      :style="{ paddingLeft: `${depth * 16 + 8}px` }"
    >
      <div class="flex items-center gap-3 truncate">
        <span class="w-1.5 h-1.5 rounded-full flex-shrink-0" 
              :class="fileStatus.dotClass"></span>
        <span class="text-sm text-slate-600 truncate">{{ node.name }}</span>
        <span v-if="node.fileData?.series_title" class="text-xs text-slate-400 truncate">({{ node.fileData.series_title }})</span>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-[9px] font-bold px-1.5 py-0.5 rounded border flex-shrink-0"
              :class="fileStatus.class">
          {{ fileStatus.text }}
        </span>
        <button @click="$emit('play', node.fileData)" class="p-1 rounded-full hover:bg-blue-50 text-blue-600 transition-colors flex-shrink-0" title="播放">
          <Play :size="14" class="fill-current" />
        </button>
      </div>
    </div>

    <!-- Recursive Self Render -->
    <div v-if="node.type === 'directory' && isExpanded && node.children?.length" class="mt-0.5">
      <FolderTreeNode 
        v-for="child in node.children" 
        :key="child.path" 
        :node="child" 
        :depth="depth + 1"
        @play="$emit('play', $event)"
      />
    </div>
  </div>
</template>
