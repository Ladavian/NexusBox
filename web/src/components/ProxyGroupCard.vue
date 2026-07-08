<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { apiFetch } from '../utils/api'
import { ChevronForwardOutline, SyncOutline } from '@vicons/ionicons5'
import { useProxyStore, type ProxyGroup } from '../store/proxies'
import { useGlobalStore } from '../store/global'

const props = defineProps<{
  group: ProxyGroup
}>()

const { t } = useI18n()
const proxyStore = useProxyStore()
const globalStore = useGlobalStore()
const { delays, allProxiesRaw, expandedState, sortOrder, delayThresholds, qualityScores, filterRegex } = storeToRefs(proxyStore)

const isTesting = ref(false)

// 卡片容器引用（用于折叠后滚动）
const cardRef = ref<HTMLElement | null>(null)

// 折叠时滚动到顶部
watch(
  () => expandedState.value[props.group.name],
  (newVal, oldVal) => {
    if (oldVal === true && newVal === false) {
      nextTick(() => {
        cardRef.value?.scrollIntoView({ block: 'start', inline: 'nearest', behavior: 'smooth' })
      })
    }
  }
)

const shouldUseBar = computed(() => {
  return props.group.all.length > 10
})

const getGroupBarSegments = computed(() => {
  const nodes = props.group.all || []
  if (nodes.length === 0) return []
  const { low, mid } = delayThresholds.value
  let green = 0, yellow = 0, red = 0, loading = 0, none = 0
  nodes.forEach(name => {
    const delay = delays.value[name]
    if (delay === undefined || delay === null) none++
    else if (delay === 0) loading++
    else if (delay === -1) red++
    else if (delay > 0 && delay <= low) green++
    else if (delay > low && delay <= mid) yellow++
    else red++
  })
  const total = nodes.length
  return [
    { pct: (green / total) * 100, class: 'bg-success' },
    { pct: (yellow / total) * 100, class: 'bg-amber-500' },
    { pct: (red / total) * 100, class: 'bg-red-500' },
    { pct: (loading / total) * 100, class: 'bg-slate-300 dark:bg-slate-700 animate-pulse' },
    { pct: (none / total) * 100, class: 'bg-slate-200 dark:bg-slate-800' }
  ].filter(s => s.pct > 0)
})

const getGroupDotSegments = computed(() => {
  const nodes = props.group.all || []
  const { low, mid } = delayThresholds.value
  return nodes.map(name => {
    const delay = delays.value[name]
    const isSelected = props.group.now === name
    let colorClass = 'bg-slate-200 dark:bg-slate-800'
    if (delay === 0) colorClass = 'bg-slate-300 dark:bg-slate-700 animate-pulse'
    else if (delay === -1) colorClass = 'bg-red-500'
    else if (delay && delay > 0 && delay <= low) colorClass = 'bg-success'
    else if (delay && delay > low && delay <= mid) colorClass = 'bg-amber-500'
    else if (delay && delay > mid) colorClass = 'bg-red-400'
    return { name, isSelected, colorClass }
  })
})

// ===== 排序计算属性（增强版） =====
const sortedNodes = computed(() => {
    let nodes = props.group.all
 
    // 应用正则过滤（如果存在）
    const regexStr = filterRegex.value
    if (regexStr) {
      try {
       const regex = new RegExp(regexStr)
        nodes = nodes.filter(name => !regex.test(name))
      } catch (e) {
        // 无效正则，忽略过滤
        console.warn('Invalid filter regex:', regexStr)
      }
    }
    
  const order = sortOrder.value

  // 提取纯文本排序键（去除 Emoji、特殊符号，保留字母数字汉字空格连字符点）
  const getSortKey = (name: string) =>
    name.replace(/[^\p{L}\p{N}\s\-.]/gu, '').trim()

  if (order === 'default') return nodes

  if (order === 'name') {
    return [...nodes].sort((a, b) =>
      getSortKey(a).localeCompare(getSortKey(b))
    )
  }

  if (order === 'delay') {
    return [...nodes].sort((a, b) => {
      const da = delays.value[a]
      const db = delays.value[b]
      const getVal = (d: number | undefined) => {
        if (d === undefined || d === null || d <= 0) return Infinity
        return d
      }
      const va = getVal(da)
      const vb = getVal(db)
      if (va !== vb) return va - vb
      // 延迟相同，按纯净名称排序
      return getSortKey(a).localeCompare(getSortKey(b))
    })
  }

  if (order === 'quality') {
    return [...nodes].sort((a, b) => {
      const sa = qualityScores.value[a] ?? 0
      const sb = qualityScores.value[b] ?? 0
      if (sa !== sb) return sb - sa // 降序
      return a.localeCompare(b)
    })
  }

  return nodes
})

const gridRef = ref<HTMLElement | null>(null)
// 监听展开状态，当展开且节点数 > 10 时，滚动到选中节点
watch(
  () => expandedState.value[props.group.name],
  (isExpanded) => {
    if (isExpanded && props.group.all.length > 10) {
      nextTick(() => {
        if (!gridRef.value) return
        // 查找当前选中的节点（拥有 border-accent 类的元素）
        const selectedEl = gridRef.value.querySelector('.border-accent')
        if (selectedEl) {
          // 检查是否在视口内，若不在则滚动到视口中央
          const rect = selectedEl.getBoundingClientRect()
          const isVisible = rect.top >= 0 && rect.bottom <= window.innerHeight
          if (!isVisible) {
            selectedEl.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'smooth' })
          }
        }
      })
    }
  },
  { immediate: true }  // 组件挂载时若已展开也立即执行
)

const handleSelectProxy = async (proxyName: string) => {
  if (delays.value[proxyName] === 0) {
    globalStore.showToast(t('proxies.testing'), 'warning')
    return
  }
  const originalNow = props.group.now
  props.group.now = proxyName // 乐观更新

  try {
    const encodedGroup = encodeURIComponent(props.group.name)
    const resp = await apiFetch(`/proxies/${encodedGroup}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: proxyName })
    })
    if (resp.ok) {
      globalStore.showToast(`${t('proxies.switched')}: ${props.group.name} → ${proxyName}`, 'success')
    } else {
      props.group.now = originalNow
      globalStore.showToast(t('proxies.switch_failed'), 'error')
    }
  } catch (e: any) {
    props.group.now = originalNow
    globalStore.showToast(t('proxies.switch_failed') + ': ' + e.message, 'error')
  }
}

const handleTestSingle = async (proxyName: string) => {
  if (delays.value[proxyName] === 0) return
  await proxyStore.testDelay(proxyName)
  proxyStore.fetchProxies(true)
}

const handleTestGroup = async () => {
  if (isTesting.value) return
  isTesting.value = true
  try {
    await proxyStore.testProxiesWithConcurrency(props.group.all)
    proxyStore.fetchProxies(true)
    globalStore.showToast(t('proxies.test_complete'), 'success')
  } catch (e) {
    globalStore.showToast(t('common.operation_failed'), 'error')
  } finally {
    isTesting.value = false
  }
}

const getDelayClass = (delay?: number) => {
  if (delay === undefined) return 'bg-slate-100/80 dark:bg-slate-800/80 border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:bg-accent hover:text-white hover:border-accent'
  if (delay === 0) return 'bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-400 animate-pulse'
  if (delay === -1) return 'bg-red-500/10 border-red-500/20 text-red-500 dark:text-red-400 hover:bg-red-500 hover:text-white hover:border-red-500'
  if (delay <= 200) return 'bg-success/10 border-success/20 text-success dark:text-success hover:bg-success hover:text-white hover:border-success'
  if (delay <= 500) return 'bg-amber-500/10 border-amber-500/20 text-amber-500 dark:text-amber-400 hover:bg-amber-500 hover:text-white hover:border-amber-500'
  return 'bg-red-500/10 border-red-500/20 text-red-400 dark:text-red-400 hover:bg-red-500 hover:text-white hover:border-red-500'
}

const getDelayText = (delay?: number) => {
  if (delay === undefined) return t('proxies.test')
  if (delay === 0) return '...'
  if (delay === -1) return t('proxies.timeout')
  return `${delay}ms`
}
</script>

<template>
  <div ref="cardRef" class="bg-white/60 dark:bg-slate-900/50 rounded-2xl border border-slate-200/60 dark:border-slate-800/60 transition-all duration-300 hover:shadow-sm overflow-hidden">
    <!-- 头部 -->
    <div
      class="px-4 pt-3.5 pb-3 cursor-pointer select-none transition-all duration-200"
      :class="expandedState[group.name] ? 'bg-slate-50/80 dark:bg-slate-800/40 border-b border-slate-100 dark:border-slate-800' : ''"
      @click="expandedState[group.name] = !expandedState[group.name]"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2.5 min-w-0 flex-1">
          <ChevronForwardOutline class="w-3.5 h-3.5 text-slate-400 shrink-0 transition-transform duration-300" :class="{ 'rotate-90': expandedState[group.name] }" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-bold text-slate-800 dark:text-slate-100 truncate">{{ group.name }}</span>
              <span class="px-1.5 py-0.5 text-[10px] font-bold rounded-md bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-500 uppercase shrink-0 tracking-wider">{{ group.type }}</span>
            </div>
            <div class="text-[11px] text-slate-400 dark:text-slate-500 mt-0.5 truncate">
              {{ group.now }}
            </div>
          </div>
        </div>
        <button @click.stop="handleTestGroup" :disabled="isTesting" class="p-1.5 text-slate-300 hover:text-accent rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-all shrink-0" :title="t('proxies.test')">
          <SyncOutline class="w-4 h-4" :class="{ 'animate-spin': isTesting }" />
        </button>
      </div>
      <!-- 健康度条 -->
      <div class="group-health flex items-center w-full mt-2" :class="shouldUseBar ? 'h-1 gap-0' : 'h-1.5 gap-0.5'">
        <template v-if="shouldUseBar">
          <span v-for="(seg, sIdx) in getGroupBarSegments" :key="sIdx" :style="{ flex: seg.pct }" :class="[seg.class, 'h-full first:rounded-l last:rounded-r transition-all duration-500']"></span>
        </template>
        <template v-else>
          <span v-for="(dot, dIdx) in getGroupDotSegments" :key="dIdx" :class="[dot.colorClass, 'w-1.5 h-1.5 rounded-full shrink-0 relative transition-colors duration-300']" :title="dot.name">
            <span v-if="dot.isSelected" class="absolute inset-0.5 rounded-full bg-white/80"></span>
          </span>
        </template>
      </div>
    </div>

    <!-- 节点网格 -->
    <div v-if="expandedState[group.name]" ref="gridRef" class="grid grid-cols-2 gap-2 p-3 bg-slate-50/40 dark:bg-slate-950/30 animate-[fadeIn_0.2s_ease-out]">
      <div
        v-for="name in sortedNodes"
        :key="name"
        @click="handleSelectProxy(name)"
        class="group flex items-center justify-between p-2.5 rounded-xl border cursor-pointer transition-all duration-200 hover:-translate-y-0.5 active:scale-[0.98]"
        :class="group.now === name
          ? 'bg-accent/10 dark:bg-accent/20 border-accent/40 shadow-sm'
          : 'bg-white/80 dark:bg-slate-900/50 border-slate-200/40 dark:border-slate-800/60 hover:border-slate-300 dark:hover:border-slate-700 hover:shadow-sm'"
      >
        <div class="min-w-0 flex-1 mr-2">
          <span class="block truncate text-xs font-semibold text-slate-700 dark:text-slate-200" :class="{ 'text-accent': group.now === name }" :title="name">{{ name }}</span>
          <div v-if="allProxiesRaw[name]" class="flex items-center gap-1 mt-1">
            <span class="text-[9px] font-mono text-slate-400 dark:text-slate-500 bg-slate-100 dark:bg-slate-800/50 px-1 rounded">{{ allProxiesRaw[name].type }}</span>
          </div>
        </div>
        <span
          class="text-[10px] font-mono font-bold shrink-0 px-2 py-1 rounded-lg leading-none text-center min-w-[40px] transition-all cursor-pointer border"
          :class="getDelayClass(delays[name])"
          @click.stop="handleTestSingle(name)"
          :title="t('proxies.test')"
        >{{ getDelayText(delays[name]) }}</span>
      </div>
    </div>
  </div>
</template>