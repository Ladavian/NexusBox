<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiFetch } from '../utils/api'
import { useGlobalStore } from '../store/global'
import { LockClosedOutline, PersonOutline, EyeOutline, EyeOffOutline } from '@vicons/ionicons5'

const { t } = useI18n()
const globalStore = useGlobalStore()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const loading = ref(false)
const errorMsg = ref('')

const handleLogin = async () => {
  if (!username.value || !password.value) {
    errorMsg.value = t('login.empty_fields')
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    const resp = await apiFetch('/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value })
    })
    const data = await resp.json()
    if (resp.ok && data.status === 'ok') {
      globalStore.isAuthenticated = true
    } else {
      errorMsg.value = data.message || t('login.failed')
    }
  } catch (e) {
    errorMsg.value = t('common.network_error')
  } finally {
    loading.value = false
  }
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter') handleLogin()
}
</script>

<template>
  <div class="flex h-screen w-screen items-center justify-center bg-[#f1f5f9] dark:bg-[#0f172a] transition-colors duration-200 p-4">
    <div class="w-full max-w-sm glass-heavy rounded-[24px] shadow-2xl border border-slate-200/50 dark:border-slate-800/50 p-8 animate-[zoomIn_0.2s_ease-out]">
      <!-- Logo -->
      <div class="flex flex-col items-center gap-3 mb-8">
        <div class="w-14 h-14 rounded-2xl bg-accent/10 flex items-center justify-center">
          <LockClosedOutline class="w-7 h-7 text-accent" />
        </div>
        <h2 class="text-xl font-extrabold text-slate-800 dark:text-slate-100 tracking-wide">Fluxor</h2>
        <p class="text-xs text-slate-400 dark:text-slate-500">{{ t('login.title') }}</p>
      </div>

      <!-- Error -->
      <div v-if="errorMsg" class="mb-4 p-3 rounded-xl bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20 text-xs font-semibold text-red-600 dark:text-red-400 text-center">
        {{ errorMsg }}
      </div>

      <!-- Form -->
      <div class="flex flex-col gap-4">
        <div class="relative">
          <div class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400">
            <PersonOutline class="w-4 h-4" />
          </div>
          <input
            v-model="username"
            type="text"
            :placeholder="t('login.username')"
            @keydown="handleKeydown"
            autocomplete="username"
            class="w-full pl-10 pr-4 py-3 text-sm rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 focus:ring-2 focus:ring-accent outline-none text-slate-800 dark:text-slate-200 placeholder-slate-400 transition-all"
          />
        </div>

        <div class="relative">
          <div class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400">
            <LockClosedOutline class="w-4 h-4" />
          </div>
          <input
            v-model="password"
            :type="showPassword ? 'text' : 'password'"
            :placeholder="t('login.password')"
            @keydown="handleKeydown"
            autocomplete="current-password"
            class="w-full pl-10 pr-12 py-3 text-sm rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50 focus:ring-2 focus:ring-accent outline-none text-slate-800 dark:text-slate-200 placeholder-slate-400 transition-all"
          />
          <button
            @click="showPassword = !showPassword"
            type="button"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
          >
            <EyeOutline v-if="!showPassword" class="w-4 h-4" />
            <EyeOffOutline v-else class="w-4 h-4" />
          </button>
        </div>

        <button
          @click="handleLogin"
          :disabled="loading"
          class="w-full py-3 bg-accent hover:bg-accent-hover text-white text-sm font-bold rounded-xl shadow-lg shadow-accent/20 hover:shadow-accent/30 transition-all flex items-center justify-center gap-2 disabled:opacity-60 disabled:cursor-not-allowed active:scale-[0.98]"
        >
          <div v-if="loading" class="w-4 h-4 border-2 border-white/30 !border-t-white rounded-full animate-spin"></div>
          {{ loading ? t('login.logging_in') : t('login.login_btn') }}
        </button>
      </div>
    </div>
  </div>
</template>
