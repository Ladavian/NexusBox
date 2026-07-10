<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTrafficPolicyStore, type ClientItem, type DiscoveredDevice } from '../store/trafficPolicy'
import { useGlobalStore } from '../store/global'
import { ShieldCheckmarkOutline, AddOutline, CloseOutline, SearchOutline, DesktopOutline, CloudDownloadOutline } from '@vicons/ionicons5'

const { t } = useI18n()
const store = useTrafficPolicyStore()
const global = useGlobalStore()

// 新增/编辑弹窗
const showAddDialog = ref(false)
const editingList = ref<'whitelist' | 'blacklist'>('whitelist')
const editIP = ref('')
const editRemark = ref('')
const clientSearch = ref('')

// 设备发现弹窗
const showDeviceDialog = ref(false)
const deviceTargetList = ref<'whitelist' | 'blacklist'>('whitelist')

onMounted(async () => {
  await store.loadConfig()
})

const openAddDialog = (list: 'whitelist' | 'blacklist') => {
  editingList.value = list
  editIP.value = ''
  editRemark.value = ''
  showAddDialog.value = true
}

const saveClient = () => {
  if (!editIP.value.trim()) return
  store.addClient(editingList.value, {
    ip: editIP.value.trim(),
    remark: editRemark.value.trim(),
  })
  showAddDialog.value = false
}

const removeClient = (list: 'whitelist' | 'blacklist', ip: string) => {
  store.removeClient(list, ip)
}

const openDeviceDialog = async (list: 'whitelist' | 'blacklist') => {
  deviceTargetList.value = list
  await store.loadDevices()
  showDeviceDialog.value = true
}

const addDeviceToList = (dev: DiscoveredDevice) => {
  store.addClient(deviceTargetList.value, {
    ip: dev.ip,
    remark: dev.hostname || '',
  })
  store.devices = store.devices.filter(d => d.ip !== dev.ip)
}

const handleSave = async () => {
  const ok = await store.saveConfig()
  if (ok) {
    global.showToast(t('common.success'), 'success')
  } else {
    global.showToast(t('common.operation_failed'), 'error')
  }
}

const filteredClients = (list: 'whitelist' | 'blacklist') => {
  const q = clientSearch.value.trim().toLowerCase()
  if (!q) return store.config[list]
  return store.config[list].filter(c =>
    c.ip.toLowerCase().includes(q) || c.remark.toLowerCase().includes(q)
  )
}
</script>

<template>
  <div class="flex flex-col flex-1 min-h-0 gap-4 h-full overflow-y-auto">
    <!-- 顶部 -->
    <div class="glass-medium shadow-none px-6 py-3 rounded-xl border border-slate-200/50 dark:border-slate-800/50 flex items-center justify-between shrink-0">
      <h3 class="text-base font-semibold flex items-center gap-2">
        <ShieldCheckmarkOutline class="w-5 h-5 text-accent" />
        {{ t('trafficPolicy.title') }}
      </h3>
      <button @click="handleSave" :disabled="store.saving"
        class="px-4 py-1.5 bg-accent hover:bg-accent-hover text-white text-xs font-semibold rounded-lg shadow-sm transition-all disabled:opacity-50">
        {{ store.saving ? '...' : t('common.save') }}
      </button>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- 模式选择 -->
      <div class="live-card bg-white/70 dark:bg-slate-900/60 rounded-xl border border-slate-200/50 dark:border-slate-800/60 p-5 space-y-4">
        <h4 class="font-bold text-sm">{{ t('trafficPolicy.mode') }}</h4>
        <div class="flex flex-col gap-2">
          <label v-for="m in (['all','whitelist','blacklist'] as const)" :key="m"
            class="flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all"
            :class="store.config.mode === m ? 'border-accent bg-accent/5' : 'border-slate-200 dark:border-slate-700 hover:border-slate-300'">
            <input type="radio" :value="m" v-model="store.config.mode" class="accent-accent" />
            <div>
              <span class="text-sm font-semibold">{{ t(`trafficPolicy.mode_${m}`) }}</span>
              <p class="text-[11px] text-slate-400 mt-0.5">{{ t(`trafficPolicy.mode_${m}_desc`) }}</p>
            </div>
          </label>
        </div>

        <!-- Fast Path -->
        <div class="flex items-center justify-between pt-3 border-t border-slate-100 dark:border-slate-800">
          <div>
            <span class="text-sm font-semibold">{{ t('trafficPolicy.fast_path') }}</span>
            <p class="text-[11px] text-slate-400 mt-0.5">{{ t('trafficPolicy.fast_path_desc') }}</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="store.config.enable_fast_path" class="sr-only peer" />
            <div class="w-9 h-5 bg-slate-300 dark:bg-slate-700 rounded-full peer peer-checked:bg-accent peer-focus:ring-2 peer-focus:ring-accent/20 transition-all after:content-[''] after:absolute after:top-0.5 after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full"></div>
          </label>
        </div>
      </div>

      <!-- 状态卡片 -->
      <div class="live-card bg-white/70 dark:bg-slate-900/60 rounded-xl border border-slate-200/50 dark:border-slate-800/60 p-5 space-y-3">
        <h4 class="font-bold text-sm">{{ t('trafficPolicy.status') }}</h4>
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50">
            <span class="text-[11px] text-slate-400">{{ t('trafficPolicy.current_mode') }}</span>
            <p class="font-bold text-accent">{{ t(`trafficPolicy.mode_${store.config.mode}`) }}</p>
          </div>
          <div class="p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50">
            <span class="text-[11px] text-slate-400">{{ t('trafficPolicy.fast_path') }}</span>
            <p class="font-bold" :class="store.config.enable_fast_path ? 'text-emerald-500' : 'text-slate-400'">
              {{ store.config.enable_fast_path ? 'ON' : 'OFF' }}
            </p>
          </div>
          <div class="p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50">
            <span class="text-[11px] text-slate-400">{{ t('trafficPolicy.whitelist_count') }}</span>
            <p class="font-bold">{{ store.config.whitelist.length }}</p>
          </div>
          <div class="p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50">
            <span class="text-[11px] text-slate-400">{{ t('trafficPolicy.blacklist_count') }}</span>
            <p class="font-bold">{{ store.config.blacklist.length }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- 客户端列表 -->
    <div v-if="store.config.mode !== 'all'" class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div v-for="list in (store.config.mode === 'whitelist' ? ['whitelist'] : ['blacklist'] as const)" :key="list"
        class="live-card bg-white/70 dark:bg-slate-900/60 rounded-xl border border-slate-200/50 dark:border-slate-800/60 p-5 space-y-3">
        <div class="flex items-center justify-between">
          <h4 class="font-bold text-sm">{{ t(`trafficPolicy.${list}`) }} ({{ store.config[list].length }})</h4>
          <div class="flex items-center gap-2">
            <button @click="openDeviceDialog(list)"
              class="px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-500 hover:text-accent transition-all flex items-center gap-1">
              <CloudDownloadOutline class="w-3 h-3" />{{ t('trafficPolicy.discover') }}
            </button>
            <button @click="openAddDialog(list)"
              class="px-2.5 py-1 text-[11px] font-semibold rounded-lg bg-accent/10 text-accent hover:bg-accent hover:text-white transition-all flex items-center gap-1">
              <AddOutline class="w-3 h-3" />{{ t('trafficPolicy.add') }}
            </button>
          </div>
        </div>

        <input type="text" v-model="clientSearch" :placeholder="t('trafficPolicy.search_placeholder')"
          class="w-full px-3 py-1.5 text-xs rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 focus:ring-2 focus:ring-accent outline-none" />

        <div class="space-y-1 max-h-[300px] overflow-y-auto">
          <div v-for="c in filteredClients(list)" :key="c.ip"
            class="flex items-center justify-between p-2 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
            <div class="min-w-0 flex-1">
              <span class="text-xs font-mono font-semibold text-slate-700 dark:text-slate-200">{{ c.ip }}</span>
              <span v-if="c.remark" class="text-[10px] text-slate-400 ml-2">{{ c.remark }}</span>
            </div>
            <button @click="removeClient(list, c.ip)" class="p-1 text-slate-300 hover:text-red-500 transition-colors">
              <CloseOutline class="w-4 h-4" />
            </button>
          </div>
          <div v-if="filteredClients(list).length === 0" class="py-8 text-center text-xs text-slate-400">
            {{ clientSearch ? t('trafficPolicy.no_match') : t('trafficPolicy.empty_list') }}
          </div>
        </div>
      </div>
    </div>

    <!-- 新增客户端弹窗 -->
    <Teleport to="body">
      <div v-if="showAddDialog" class="fixed inset-0 glass-mask z-[9999] flex items-center justify-center p-4" @click.self="showAddDialog = false">
        <div class="glass-heavy w-full max-w-sm rounded-[20px] shadow-2xl border p-6 flex flex-col gap-4 animate-[zoomIn_0.15s_ease-out]">
          <h4 class="text-lg font-bold">{{ t('trafficPolicy.add_client') }}</h4>
          <input type="text" v-model="editIP" placeholder="192.168.1.100 或 192.168.1.0/24"
            class="w-full px-3 py-2 text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 focus:ring-2 focus:ring-accent outline-none font-mono" />
          <input type="text" v-model="editRemark" :placeholder="t('trafficPolicy.remark_placeholder')"
            class="w-full px-3 py-2 text-sm rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 focus:ring-2 focus:ring-accent outline-none" />
          <div class="flex justify-end gap-2.5 pt-2">
            <button @click="showAddDialog = false" class="px-4 py-2 text-sm font-semibold rounded-xl bg-white border border-slate-200 hover:bg-slate-50 dark:bg-slate-800 dark:border-slate-700 text-slate-500 transition-all">{{ t('common.cancel') }}</button>
            <button @click="saveClient" class="px-4 py-2 text-sm font-semibold rounded-xl bg-accent hover:bg-accent-hover text-white transition-all">{{ t('common.save') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 设备发现弹窗 -->
    <Teleport to="body">
      <div v-if="showDeviceDialog" class="fixed inset-0 glass-mask z-[9999] flex items-center justify-center p-4" @click.self="showDeviceDialog = false">
        <div class="glass-heavy w-full max-w-md rounded-[20px] shadow-2xl border p-6 flex flex-col gap-4 animate-[zoomIn_0.15s_ease-out] max-h-[80vh] overflow-y-auto">
          <h4 class="text-lg font-bold flex items-center gap-2">
            <DesktopOutline class="w-5 h-5 text-accent" />{{ t('trafficPolicy.discovered_devices') }}
          </h4>
          <div v-if="store.devices.length === 0" class="py-8 text-center text-sm text-slate-400">{{ t('trafficPolicy.no_devices') }}</div>
          <div v-for="dev in store.devices" :key="dev.ip"
            class="flex items-center justify-between p-3 rounded-lg border border-slate-200 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
            <div>
              <span class="text-sm font-mono font-semibold">{{ dev.ip }}</span>
              <span v-if="dev.hostname" class="text-xs text-slate-400 ml-2">{{ dev.hostname }}</span>
            </div>
            <button @click="addDeviceToList(dev)"
              class="px-3 py-1 text-[11px] font-semibold rounded-lg bg-accent/10 text-accent hover:bg-accent hover:text-white transition-all flex items-center gap-1">
              <AddOutline class="w-3 h-3" />{{ t('trafficPolicy.add_to_list') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
