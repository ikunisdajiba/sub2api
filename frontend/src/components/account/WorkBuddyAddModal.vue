<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[100] overflow-y-auto bg-black/50 sm:p-4">
      <div class="mx-auto flex min-h-full w-full max-w-2xl flex-col bg-white shadow-2xl sm:min-h-0 sm:rounded-xl dark:bg-dark-800">
        <!-- 头部 -->
        <div class="flex items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5 sm:py-4 dark:border-dark-700">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-gray-900 sm:text-lg dark:text-white">添加 WorkBuddy 账号</h3>
            <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
              登录一次只添加这一个账号，并自动建好对应的独立 sub2 账号。
            </p>
          </div>
          <button class="shrink-0 text-xl leading-none text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="close">✕</button>
        </div>

        <!-- 表单 -->
        <div class="px-4 py-4 sm:px-5">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">登录版本</span>
              <select v-model="realm" class="input h-9 w-full text-sm" :disabled="!!loginState">
                <option value="cn">国内版 (copilot.tencent.com)</option>
                <option value="intl">国际版 (www.workbuddy.ai)</option>
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">账号身份</span>
              <select v-model="platform" class="input h-9 w-full text-sm" :disabled="!!loginState">
                <option value="CLI">CLI</option>
                <option value="workbuddy">WorkBuddy (Web)</option>
              </select>
              <span v-if="realm === 'intl'" class="mt-1 block text-xs text-amber-600 dark:text-amber-400">
                国际版固定用 CLI 身份，桌面端会 403
              </span>
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">并发</span>
              <input v-model.number="concurrency" type="number" min="1" class="input h-9 w-full text-sm" :disabled="!!loginState" />
            </label>
            <div class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">放进分组</span>
              <div class="flex max-h-16 flex-wrap items-center gap-x-3 gap-y-1 overflow-auto rounded-lg border border-gray-200 px-2 py-1.5 dark:border-dark-600">
                <label v-for="g in openaiGroups" :key="g.id" class="flex items-center gap-1 text-xs text-gray-700 dark:text-gray-200">
                  <input v-model="selectedGroups" type="checkbox" :value="g.id" class="h-3 w-3" :disabled="!!loginState" />
                  {{ g.name }}
                </label>
                <span v-if="!openaiGroups.length" class="text-xs text-amber-600 dark:text-amber-400">没有 openai 分组</span>
              </div>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap items-center gap-2">
            <button class="btn btn-primary h-9 px-4 text-sm" :disabled="busy || !!loginState" @click="startLogin">
              {{ loginState ? '等待授权…' : '开始登录并建号' }}
            </button>
            <button v-if="loginState" class="btn btn-secondary h-9 px-3 text-sm" @click="cancelLogin">取消</button>
            <span class="text-xs text-gray-500 dark:text-gray-400">
              登录成功后只建这一个账号，不会把其它账号一起同步。
            </span>
          </div>

          <div v-if="loginState" class="mt-4 rounded-lg border border-blue-200 bg-blue-50/60 p-3 dark:border-blue-900/50 dark:bg-blue-900/10">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-xs font-medium text-blue-800 dark:text-blue-200">在浏览器（手机也行）打开授权：</span>
              <a :href="loginState.authUrl" target="_blank" rel="noopener" class="min-w-0 flex-1 truncate text-xs text-blue-600 underline dark:text-blue-300">
                {{ loginState.authUrl }}
              </a>
              <button class="btn btn-secondary h-7 px-2 text-xs" @click="copyAuthUrl">复制链接</button>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <span class="inline-flex h-2 w-2 animate-pulse rounded-full bg-blue-500" />
              <span class="text-xs text-gray-600 dark:text-gray-300">{{ loginStatusText }}</span>
            </div>
          </div>

          <div v-if="error" class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
            {{ error }}
          </div>
          <div v-if="notice" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300">
            {{ notice }}
          </div>
        </div>

        <!-- 底部 -->
        <div class="flex items-center justify-end gap-2 border-t border-gray-200 px-4 py-3 sm:px-5 dark:border-dark-700">
          <button class="btn btn-secondary h-8 px-3 text-xs" @click="close">关闭</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  pollWorkBuddyLogin,
  startWorkBuddyLogin,
  syncWorkBuddyAccounts,
  type WBAccount,
  type WBLoginStart,
} from '@/api/workbuddy'

interface GroupOption { id: number; name: string; platform?: string }

const props = defineProps<{ show: boolean; groups?: GroupOption[] }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'created'): void }>()

const realm = ref('cn')
const platform = ref('CLI')
const concurrency = ref(3)
const selectedGroups = ref<number[]>([])
const busy = ref(false)
const error = ref('')
const notice = ref('')
const loginState = ref<WBLoginStart | null>(null)
const loginStatus = ref('')

const openaiGroups = computed<GroupOption[]>(() =>
  (props.groups || []).filter(g => !g.platform || g.platform === 'openai')
)
const loginStatusText = computed(() => loginStatus.value || '等待授权…')

let pollTimer: ReturnType<typeof setInterval> | null = null

async function copyAuthUrl() {
  const url = loginState.value?.authUrl
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    notice.value = '授权链接已复制，可以发到手机上打开'
  } catch {
    notice.value = '复制失败，请长按链接手动复制'
  }
}

async function startLogin() {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    loginState.value = await startWorkBuddyLogin(realm.value, realm.value === 'intl' ? 'cli' : platform.value)
    loginStatus.value = '等待授权…'
    startPolling()
  } catch (e: any) {
    error.value = e?.message || '发起登录失败'
    loginState.value = null
  } finally {
    busy.value = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    const st = loginState.value
    if (!st) return
    try {
      const res = await pollWorkBuddyLogin(st.state)
      loginStatus.value = res.message || res.status
      if (res.status === 'success' || res.status === 'ok') {
        stopPolling()
        const added = (res.account || null) as WBAccount | null
        loginState.value = null
        if (added?.uid) {
          await createAccount(added)
        } else {
          notice.value = '登录成功，账号已加入网关'
          emit('created')
        }
      } else if (res.status === 'error' || res.status === 'expired' || res.status === 'unknown') {
        stopPolling()
        error.value = res.message || '登录失败'
        loginState.value = null
      }
    } catch {
      /* 轮询失败静默重试 */
    }
  }, 3000)
}

async function createAccount(added: WBAccount) {
  busy.value = true
  error.value = ''
  try {
    const data = await syncWorkBuddyAccounts([added.uid], realm.value, selectedGroups.value, concurrency.value)
    notice.value = data.result.created.length
      ? `已添加账号：${added.nickname || added.uid.slice(0, 8)}`
      : `账号已存在，已更新：${added.nickname || added.uid.slice(0, 8)}`
    emit('created')
  } catch (e: any) {
    error.value = e?.message || '建号失败，账号已加进网关，可在网关面板点「建号」重试'
    emit('created')
  } finally {
    busy.value = false
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function cancelLogin() {
  stopPolling()
  loginState.value = null
  loginStatus.value = ''
}

function close() {
  cancelLogin()
  error.value = ''
  notice.value = ''
  emit('close')
}

watch(() => props.show, (v) => {
  if (!v) {
    cancelLogin()
    return
  }
  error.value = ''
  notice.value = ''
  if (realm.value === 'intl') platform.value = 'cli'
  if (!selectedGroups.value.length) {
    const preset = openaiGroups.value.filter(g => /workbuddy/i.test(g.name)).map(g => g.id)
    selectedGroups.value = preset.length ? preset : openaiGroups.value.slice(0, 1).map(g => g.id)
  }
})

watch(realm, (v) => {
  if (v === 'intl') platform.value = 'cli'
})

onBeforeUnmount(stopPolling)
</script>
