<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-black/50 p-4">
      <div class="my-8 w-full max-w-6xl rounded-xl bg-white shadow-2xl dark:bg-dark-800">
        <div class="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">WorkBuddy 反代网关</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              这里就是 wb 网关里的真实账号：登录新增、签到、刷新积分、启停、删除都在这里完成，
              同步后每个账号会对应一个 sub2 账号，流量锁定到该上游账号。
            </p>
          </div>
          <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="close">✕</button>
        </div>

        <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-5 py-3 dark:border-dark-700">
          <select v-model="realm" class="input text-xs" @change="load">
            <option value="all">全部地区</option>
            <option value="cn">国内 (cn)</option>
            <option value="intl">国际 (intl)</option>
          </select>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="busy" @click="load">刷新</button>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="busy" @click="doCheckin('')">一键签到</button>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="busy" @click="doCredits('')">一键刷新积分</button>
          <button class="btn btn-primary px-3 py-1.5 text-xs" :disabled="busy || !loginEnabled" @click="startLogin">
            {{ loginState ? '重新登录…' : '新增登录' }}
          </button>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" :disabled="busy || !accounts.length" @click="syncAll">同步到 sub2 账号</button>
          <div class="flex flex-wrap items-center gap-2 rounded-lg border border-gray-200 px-2 py-1 dark:border-dark-600">
            <span class="text-xs text-gray-500 dark:text-gray-400">同步到分组</span>
            <label v-for="g in openaiGroups" :key="g.id" class="flex items-center gap-1 text-xs text-gray-700 dark:text-gray-200">
              <input v-model="selectedGroups" type="checkbox" :value="g.id" class="h-3 w-3" />
              {{ g.name }}
            </label>
            <span v-if="!openaiGroups.length" class="text-xs text-amber-600 dark:text-amber-400">没有 openai 分组</span>
          </div>
          <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">
            可用 {{ payload.usable ?? '-' }} / 共 {{ accounts.length }}
          </span>
        </div>

        <div v-if="loginState" class="border-b border-blue-100 bg-blue-50 px-5 py-3 text-sm dark:border-blue-900/40 dark:bg-blue-900/20">
          <div class="flex flex-wrap items-center gap-3">
            <span class="font-medium text-blue-800 dark:text-blue-200">在浏览器里完成授权：</span>
            <a :href="loginState.authUrl" target="_blank" rel="noopener" class="break-all text-blue-600 underline dark:text-blue-300">
              {{ loginState.authUrl }}
            </a>
            <span class="rounded-full bg-white px-2 py-0.5 text-xs text-blue-700 dark:bg-dark-800 dark:text-blue-200">
              {{ loginStatusText }}
            </span>
            <button class="ml-auto text-xs text-blue-700 underline dark:text-blue-300" @click="cancelLogin">取消</button>
          </div>
        </div>

        <div v-if="error" class="border-b border-red-100 bg-red-50 px-5 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
          {{ error }}
        </div>
        <div v-if="notice" class="border-b border-emerald-100 bg-emerald-50 px-5 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300">
          {{ notice }}
        </div>

        <div class="max-h-[60vh] overflow-auto">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-700 dark:text-gray-400">
              <tr>
                <th class="px-4 py-2">账号</th>
                <th class="px-4 py-2">身分</th>
                <th class="px-4 py-2">积分</th>
                <th class="px-4 py-2">签到</th>
                <th class="px-4 py-2">状态</th>
                <th class="px-4 py-2 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading"><td colspan="6" class="px-4 py-8 text-center text-gray-500">加载中…</td></tr>
              <tr v-else-if="!accounts.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">网关里还没有账号，点「新增登录」添加。</td></tr>
              <tr v-for="acc in accounts" :key="acc.uid" class="border-t border-gray-100 dark:border-dark-700">
                <td class="px-4 py-2">
                  <div class="font-medium text-gray-900 dark:text-white">{{ acc.nickname || acc.uid.slice(0, 8) }}</div>
                  <div class="text-xs text-gray-400">{{ acc.uid.slice(0, 8) }} · {{ acc.domain || acc.realm }}</div>
                </td>
                <td class="px-4 py-2 text-xs text-gray-600 dark:text-gray-300">{{ acc.product || acc.platform || '-' }}</td>
                <td class="px-4 py-2">
                  <div class="text-gray-900 dark:text-white">
                    {{ fmt(acc.credits?.remain) }} / {{ fmt(acc.credits?.size) }}
                  </div>
                  <div class="text-xs text-gray-400">已用 {{ acc.credits?.used_percent || '-' }}</div>
                </td>
                <td class="px-4 py-2 text-xs">
                  <span :class="acc.credits?.checkin?.today_checked_in ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'">
                    {{ acc.credits?.checkin?.today_checked_in ? '今日已签' : '未签到' }}
                  </span>
                  <span class="text-gray-400"> · 连签 {{ acc.credits?.checkin?.streak_days ?? 0 }} 天</span>
                </td>
                <td class="px-4 py-2 text-xs">
                  <span v-if="acc.enabled" class="text-emerald-600 dark:text-emerald-400">启用</span>
                  <span v-else class="text-gray-400">停用</span>
                  <div v-if="acc.lastError" class="max-w-[220px] truncate text-red-500" :title="acc.lastError">{{ acc.lastError }}</div>
                </td>
                <td class="px-4 py-2 text-right">
                  <div class="flex flex-wrap justify-end gap-1">
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="doCheckin(acc.uid)">签到</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="doCredits(acc.uid)">刷新积分</button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="toggleEnabled(acc)">
                      {{ acc.enabled ? '停用' : '启用' }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="toggleProduct(acc)">
                      切 {{ (acc.product || '').toLowerCase() === 'cli' ? 'workbuddy' : 'cli' }}
                    </button>
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="syncOne(acc)">同步</button>
                    <button class="btn btn-danger px-2 py-1 text-xs" :disabled="busy" @click="remove(acc)">删除</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex items-center justify-between gap-3 border-t border-gray-200 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
          <span>网关：{{ payload.base_url || '-' }} · 上游 Key：{{ payload.upstream_key ? '已配置' : '未配置' }}</span>
          <button class="btn btn-secondary px-3 py-1.5 text-xs" @click="close">关闭</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  checkinWorkBuddy,
  deleteWorkBuddyAccount,
  fetchWorkBuddyCredits,
  listWorkBuddyAccounts,
  pollWorkBuddyLogin,
  startWorkBuddyLogin,
  syncWorkBuddyAccounts,
  updateWorkBuddyAccount,
  type WBAcountsPayload,
  type WBAccount,
  type WBLoginStart,
} from '@/api/workbuddy'

interface GroupOption { id: number; name: string; platform?: string }

const props = defineProps<{ show: boolean; groups?: GroupOption[] }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'synced'): void }>()

const realm = ref('all')
const selectedGroups = ref<number[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const payload = ref<WBAcountsPayload>({ accounts: [] })
const loginState = ref<WBLoginStart | null>(null)
const loginStatus = ref('')
const loginEnabled = ref(true)

const accounts = computed<WBAccount[]>(() => payload.value.accounts || [])
const openaiGroups = computed<GroupOption[]>(() =>
  (props.groups || []).filter(g => !g.platform || g.platform === 'openai')
)
const loginStatusText = computed(() => loginStatus.value || '等待授权…')

let pollTimer: ReturnType<typeof setInterval> | null = null

function fmt(v: unknown): string {
  const n = Number(v ?? 0)
  return Number.isFinite(n) ? n.toFixed(0) : '-'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    payload.value = await listWorkBuddyAccounts(realm.value)
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function run(action: () => Promise<void>) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await action()
  } catch (e: any) {
    error.value = e?.message || '操作失败'
  } finally {
    busy.value = false
  }
}

const doCheckin = (uid: string) => run(async () => {
  const data = await checkinWorkBuddy(uid)
  if (data?.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  else await load()
  notice.value = uid ? '签到已触发' : '一键签到已触发'
})

const doCredits = (uid: string) => run(async () => {
  const data = await fetchWorkBuddyCredits(uid)
  if (data?.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  else await load()
  notice.value = '积分已刷新'
})

const toggleEnabled = (acc: WBAccount) => run(async () => {
  await updateWorkBuddyAccount({ uid: acc.uid, enabled: !acc.enabled })
  await load()
})

const toggleProduct = (acc: WBAccount) => run(async () => {
  const next = (acc.product || '').toLowerCase() === 'cli' ? 'workbuddy' : 'cli'
  await updateWorkBuddyAccount({ uid: acc.uid, product: next })
  await load()
  notice.value = `已切换到 ${next}`
})

const syncOne = (acc: WBAccount) => run(async () => {
  const data = await syncWorkBuddyAccounts([acc.uid], realm.value, selectedGroups.value)
  notice.value = `同步完成：新建 ${data.result.created.length}，更新 ${data.result.updated.length}` +
    (data.result.errors.length ? `，失败 ${data.result.errors.length}` : '')
  if (data.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  emit('synced')
})

const syncAll = () => run(async () => {
  const data = await syncWorkBuddyAccounts([], realm.value, selectedGroups.value)
  notice.value = `同步完成：新建 ${data.result.created.length}，更新 ${data.result.updated.length}` +
    (data.result.errors.length ? `，失败 ${data.result.errors.length}: ${data.result.errors[0]}` : '')
  if (data.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  emit('synced')
})

const remove = (acc: WBAccount) => run(async () => {
  if (!window.confirm(`确认从网关删除账号 ${acc.nickname || acc.uid.slice(0, 8)}？对应的 sub2 账号也会一并删除。`)) return
  const data = await deleteWorkBuddyAccount(acc.uid)
  if (data?.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  else await load()
  emit('synced')
})

async function startLogin() {
  error.value = ''
  loginEnabled.value = false
  try {
    loginState.value = await startWorkBuddyLogin(realm.value === 'all' ? 'cn' : realm.value, 'CLI')
    loginStatus.value = '等待授权…'
    startPolling()
  } catch (e: any) {
    error.value = e?.message || '发起登录失败'
  } finally {
    loginEnabled.value = true
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    if (!loginState.value) return
    try {
      const res = await pollWorkBuddyLogin(loginState.value.state)
      loginStatus.value = res.message || res.status
      if (res.status === 'success' || res.status === 'ok') {
        stopPolling()
        const state = loginState.value.state
        loginState.value = null
        await load()
        notice.value = '登录成功，账号已加入网关'
        await syncAll()
        void state
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
  emit('close')
}

watch(() => props.show, (v) => {
  if (v) {
    if (!selectedGroups.value.length) {
      const preset = openaiGroups.value.filter(g => /workbuddy/i.test(g.name)).map(g => g.id)
      selectedGroups.value = preset.length ? preset : openaiGroups.value.slice(0, 1).map(g => g.id)
    }
    load()
  } else {
    cancelLogin()
  }
})

onBeforeUnmount(stopPolling)
</script>