<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { apiFetch } from '../utils/api'
import { ChevronForwardOutline, SyncOutline, FlashOutline } from '@vicons/ionicons5'
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
const cardRef = ref<HTMLElement | null>(null)
const gridRef = ref<HTMLElement | null>(null)

// 折叠时滚动到卡片顶部
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

// 展开时滚动到选中节点
watch(
  () => expandedState.value[props.group.name],
  (isExpanded) => {
    if (isExpanded && props.group.all.length > 10) {
      nextTick(() => {
        if (!gridRef.value) return
        const selectedEl = gridRef.value.querySelector('.proxy-card--active')
        if (selectedEl) {
          const rect = selectedEl.getBoundingClientRect()
          const isVisible = rect.top >= 0 && rect.bottom <= window.innerHeight
          if (!isVisible) {
            selectedEl.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'smooth' })
          }
        }
      })
    }
  },
  { immediate: true }
)

// ===== 健康度预览 =====
const shouldUseBar = computed(() => props.group.all.length > 10)

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
    { pct: (green / total) * 100, class: 'bg-emerald-400' },
    { pct: (yellow / total) * 100, class: 'bg-amber-400' },
    { pct: (red / total) * 100, class: 'bg-red-400' },
    { pct: (loading / total) * 100, class: 'bg-slate-400 animate-pulse' },
    { pct: (none / total) * 100, class: 'bg-slate-600' }
  ].filter(s => s.pct > 0)
})

const getGroupDotSegments = computed(() => {
  const nodes = props.group.all || []
  const { low, mid } = delayThresholds.value
  return nodes.map(name => {
    const delay = delays.value[name]
    const isSelected = props.group.now === name
    let colorClass = 'bg-slate-600'
    if (delay === 0) colorClass = 'bg-slate-400 animate-pulse'
    else if (delay === -1) colorClass = 'bg-red-400'
    else if (delay && delay > 0 && delay <= low) colorClass = 'bg-emerald-400'
    else if (delay && delay > low && delay <= mid) colorClass = 'bg-amber-400'
    else if (delay && delay > mid) colorClass = 'bg-red-400'
    return { name, isSelected, colorClass }
  })
})

// ===== 排序 =====
const sortedNodes = computed(() => {
  let nodes = props.group.all

  const regexStr = filterRegex.value
  if (regexStr) {
    try {
      const regex = new RegExp(regexStr)
      nodes = nodes.filter(name => !regex.test(name))
    } catch (e) {
      console.warn('Invalid filter regex:', regexStr)
    }
  }

  const order = sortOrder.value
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
      return getSortKey(a).localeCompare(getSortKey(b))
    })
  }

  if (order === 'quality') {
    return [...nodes].sort((a, b) => {
      const sa = qualityScores.value[a] ?? 0
      const sb = qualityScores.value[b] ?? 0
      if (sa !== sb) return sb - sa
      return a.localeCompare(b)
    })
  }

  return nodes
})

// ===== 操作 =====
const handleSelectProxy = async (proxyName: string) => {
  if (delays.value[proxyName] === 0) {
    globalStore.showToast(t('proxies.testing'), 'warning')
    return
  }
  const originalNow = props.group.now

  // 乐观更新
  props.group.now = proxyName

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

// ===== 延迟样式 =====
const getDelayPillClass = (delay?: number) => {
  if (delay === undefined || delay === null) return 'bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-500'
  if (delay === 0) return 'bg-slate-100 dark:bg-slate-800 text-slate-400 animate-pulse'
  if (delay === -1) return 'bg-red-50 dark:bg-red-500/10 text-red-500 border-red-200 dark:border-red-500/20'
  if (delay <= 200) return 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-500/20'
  if (delay <= 500) return 'bg-amber-50 dark:bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-200 dark:border-amber-500/20'
  return 'bg-red-50 dark:bg-red-500/10 text-red-500 border-red-200 dark:border-red-500/20'
}

const getDelayText = (delay?: number) => {
  if (delay === undefined || delay === null) return '—'
  if (delay === 0) return '...'
  if (delay === -1) return t('proxies.timeout')
  return `${delay}ms`
}

const getNowDelay = computed(() => {
  if (!props.group.now) return undefined
  return delays.value[props.group.now]
})

const getNowDelayClass = computed(() => {
  const d = getNowDelay.value
  if (d === undefined || d === null) return 'text-slate-400'
  if (d === -1) return 'text-red-400'
  if (d <= 200) return 'text-emerald-400'
  if (d <= 500) return 'text-amber-400'
  return 'text-red-400'
})

// 节点类型缩写
const getTypeLabel = (proxyName: string) => {
  const raw = allProxiesRaw.value[proxyName]
  if (!raw || !raw.type) return ''
  return raw.type
}
</script>

<template>
  <div
    ref="cardRef"
    class="bg-white/70 dark:bg-slate-900/60 rounded-2xl border border-slate-200/50 dark:border-slate-800/60 transition-all duration-300 hover:border-slate-300/80 dark:hover:border-slate-700/80 hover:shadow-md overflow-hidden"
  >
    <!-- === 头部 === -->
    <div
      class="px-4 py-3.5 cursor-pointer select-none transition-colors duration-200"
      :class="expandedState[group.name] ? 'bg-slate-50/80 dark:bg-slate-800/50 border-b border-slate-100 dark:border-slate-800' : 'hover:bg-slate-50/50 dark:hover:bg-slate-800/30'"
      @click="expandedState[group.name] = !expandedState[group.name]"
    >
      <!-- 第一行：组名 + 类型标签 + 延迟 + 操作 -->
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2.5 min-w-0 flex-1">
          <ChevronForwardOutline
            class="w-3.5 h-3.5 text-slate-400 shrink-0 transition-transform duration-300"
            :class="{ 'rotate-90': expandedState[group.name] }"
          />
          <span class="text-sm font-bold text-slate-800 dark:text-slate-100 truncate">{{ group.name }}</span>
          <span class="px-1.5 py-0.5 text-[10px] font-bold rounded-md bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-500 uppercase shrink-0 tracking-wider tabular-nums">
            {{ group.type }} · {{ group.all.length }}
          </span>
          <!-- 当前节点延迟标签 -->
          <span
            v-if="getNowDelay !== undefined"
            class="hidden sm:inline-flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] font-mono font-bold rounded-md shrink-0"
            :class="[
              getNowDelayClass,
              getNowDelay === -1 ? 'bg-red-50 dark:bg-red-500/10' :
              getNowDelay <= 200 ? 'bg-emerald-50 dark:bg-emerald-500/10' :
              getNowDelay <= 500 ? 'bg-amber-50 dark:bg-amber-500/10' :
              'bg-red-50 dark:bg-red-500/10'
            ]"
          >
            <FlashOutline v-if="getNowDelay === -1" class="w-3 h-3" />
            <template v-else>{{ getNowDelay }}ms</template>
          </span>
        </div>

        <!-- 操作按钮组 -->
        <div class="flex items-center gap-1 shrink-0">
          <button
            @click.stop="handleTestGroup"
            :disabled="isTesting"
            class="p-1.5 rounded-lg text-slate-400 hover:text-accent hover:bg-accent/10 transition-all"
            :title="t('proxies.test_group')"
          >
            <SyncOutline class="w-4 h-4" :class="{ 'animate-spin': isTesting }" />
          </button>
        </div>
      </div>

      <!-- 第二行：当前选中节点 -->
      <div class="flex items-center gap-2 mt-2">
        <div class="flex items-center gap-1.5 min-w-0 flex-1">
          <span class="text-[11px] text-slate-400 dark:text-slate-500 shrink-0">{{ t('proxies.now') }}</span>
          <span class="text-[11px] font-medium text-slate-600 dark:text-slate-300 truncate">{{ group.now }}</span>
        </div>
      </div>

      <!-- 健康度条 / 圆点 -->
      <div class="flex items-center w-full mt-2.5" :class="shouldUseBar ? 'h-1 gap-0' : 'h-1.5 gap-0.5'">
        <template v-if="shouldUseBar">
          <span
            v-for="(seg, sIdx) in getGroupBarSegments"
            :key="sIdx"
            :style="{ flex: seg.pct }"
            :class="[seg.class, 'h-full first:rounded-l last:rounded-r transition-all duration-500']"
          ></span>
        </template>
        <template v-else>
          <span
            v-for="(dot, dIdx) in getGroupDotSegments"
            :key="dIdx"
            :class="[dot.colorClass, 'w-1.5 h-1.5 rounded-full shrink-0 relative transition-colors duration-300']"
            :title="dot.name"
          >
            <span v-if="dot.isSelected" class="absolute inset-0.5 rounded-full bg-white/80"></span>
          </span>
        </template>
      </div>
    </div>

    <!-- === 节点网格 === -->
    <div
      v-if="expandedState[group.name]"
      ref="gridRef"
      class="p-3 bg-slate-50/40 dark:bg-slate-950/30 animate-[fadeIn_0.2s_ease-out]"
    >
      <div class="grid grid-cols-1 sm:grid-cols-[repeat(auto-fill,minmax(145px,1fr))] gap-2">
        <div
          v-for="name in sortedNodes"
          :key="name"
          @click="handleSelectProxy(name)"
          class="proxy-card group flex items-center justify-between gap-2 p-2.5 rounded-xl border cursor-pointer transition-all duration-200 hover:-translate-y-0.5 active:scale-[0.98]"
          :class="group.now === name
            ? 'proxy-card--active bg-accent border-accent shadow-md shadow-accent/10'
            : 'bg-white/80 dark:bg-slate-900/50 border-slate-200/40 dark:border-slate-800/60 hover:border-slate-300 dark:hover:border-slate-700 hover:shadow-sm'"
        >
          <!-- 节点信息 -->
          <div class="min-w-0 flex-1">
            <span
              class="block truncate text-xs font-semibold leading-tight"
              :class="group.now === name ? 'text-white' : 'text-slate-700 dark:text-slate-200'"
              :title="name"
            >{{ name }}</span>
            <span
              v-if="getTypeLabel(name)"
              class="text-[9px] font-mono mt-0.5 block truncate tracking-tight"
              :class="group.now === name ? 'text-white/60' : 'text-slate-400 dark:text-slate-500'"
            >{{ getTypeLabel(name) }}</span>
          </div>

          <!-- 延迟标签 -->
          <span
            class="shrink-0 text-[10px] font-mono font-bold px-1.5 py-1 rounded-lg leading-none text-center min-w-[38px] transition-all cursor-pointer border"
            :class="getDelayPillClass(delays[name])"
            @click.stop="handleTestSingle(name)"
            :title="t('proxies.test')"
          >{{ getDelayText(delays[name]) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
