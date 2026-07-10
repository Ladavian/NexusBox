import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiFetch } from '../utils/api'

export interface ClientItem {
  ip: string
  remark: string
}

export interface TrafficPolicyData {
  mode: 'all' | 'whitelist' | 'blacklist'
  enable_fast_path: boolean
  whitelist: ClientItem[]
  blacklist: ClientItem[]
}

export interface DiscoveredDevice {
  ip: string
  hostname: string
  vendor: string
}

export const useTrafficPolicyStore = defineStore('trafficPolicy', () => {
  const config = ref<TrafficPolicyData>({
    mode: 'all',
    enable_fast_path: false,
    whitelist: [],
    blacklist: [],
  })
  const devices = ref<DiscoveredDevice[]>([])
  const loading = ref(false)
  const saving = ref(false)

  const loadConfig = async () => {
    loading.value = true
    try {
      const resp = await apiFetch('/config/traffic-policy')
      if (resp.ok) {
        config.value = await resp.json()
      }
    } catch (e) {
      console.error('加载流量策略失败', e)
    } finally {
      loading.value = false
    }
  }

  const saveConfig = async (): Promise<boolean> => {
    saving.value = true
    try {
      const resp = await apiFetch('/config/traffic-policy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config.value),
      })
      return resp.ok
    } catch (e) {
      console.error('保存流量策略失败', e)
      return false
    } finally {
      saving.value = false
    }
  }

  const loadDevices = async () => {
    try {
      const resp = await apiFetch('/config/traffic-policy/devices')
      if (resp.ok) {
        devices.value = await resp.json()
      }
    } catch (e) {
      console.error('加载设备列表失败', e)
    }
  }

  const addClient = (list: 'whitelist' | 'blacklist', client: ClientItem) => {
    if (!config.value[list].find(c => c.ip === client.ip)) {
      config.value[list].push(client)
    }
  }

  const removeClient = (list: 'whitelist' | 'blacklist', ip: string) => {
    config.value[list] = config.value[list].filter(c => c.ip !== ip)
  }

  return { config, devices, loading, saving, loadConfig, saveConfig, loadDevices, addClient, removeClient }
})
