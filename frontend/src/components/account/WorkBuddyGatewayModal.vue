<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[100] overflow-y-auto bg-black/50 sm:p-4">
      <div class="mx-auto flex min-h-full w-full max-w-6xl flex-col bg-white shadow-2xl sm:min-h-0 sm:rounded-xl dark:bg-dark-800">
        <!-- 头部 -->
        <div class="flex items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 sm:px-5 sm:py-4 dark:border-dark-700">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-gray-900 sm:text-lg dark:text-white">WorkBuddy 反代网关</h3>
            <p class="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
              这里管理 wb 网关里的真实账号。每个账号对应一个独立的 sub2 账号，
              请求带 <code class="rounded bg-gray-100 px-1 dark:bg-dark-700">X-WB-Account</code>
              锁定到该上游账号，额度和并发都算它自己的。
            </p>
          </div>
          <button class="shrink-0 text-xl leading-none text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="close">✕</button>
        </div>

        <!-- 工具条 -->
        <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-4 py-2.5 sm:px-5 dark:border-dark-700">
          <select v-model="realm" class="input h-8 w-auto text-xs" @change="load">
            <option value="all">全部地区</option>
            <option value="cn">国内 (cn)</option>
            <option value="intl">国际 (intl)</option>
          </select>
          <button class="btn btn-secondary h-8 px-3 text-xs" :disabled="busy" @click="load">刷新</button>
          <button class="btn btn-secondary h-8 px-3 text-xs" :disabled="busy" @click="doCheckin('')">一键签到</button>
          <button class="btn btn-secondary h-8 px-3 text-xs" :disabled="busy" @click="doCredits('')">刷新积分</button>
          <button class="btn btn-primary h-8 px-3 text-xs" :disabled="busy" @click="toggleAdd">
            {{ addOpen ? '收起新增' : '+ 新增账号' }}
          </button>
          <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">
            可用 {{ payload.usable ?? '-' }} / 共 {{ accounts.length }}
          </span>
        </div>

        <!-- 新增账号：独立账号流程 -->
        <div v-if="addOpen" class="border-b border-blue-100 bg-blue-50/60 px-4 py-3 sm:px-5 dark:border-blue-900/40 dark:bg-blue-900/10">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">登录版本</span>
              <select v-model="addRealm" class="input h-9 w-full text-sm" :disabled="!!loginState">
                <option value="cn">国内版 (copilot.tencent.com)</option>
                <option value="intl">国际版 (www.workbuddy.ai)</option>
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">账号身份</span>
              <select v-model="addPlatform" class="input h-9 w-full text-sm" :disabled="!!loginState">
                <option value="CLI">CLI</option>
                <option value="workbuddy">WorkBuddy (Web)</option>
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">并发</span>
              <input v-model.number="addConcurrency" type="number" min="1" class="input h-9 w-full text-sm" :disabled="!!loginState" />
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

          <div class="mt-3 flex flex-wrap items-center gap-2">
            <button class="btn btn-primary h-9 px-4 text-sm" :disabled="busy || !!loginState" @click="startLogin">
              {{ loginState ? '等待授权…' : '开始登录并建号' }}
            </button>
            <button v-if="loginState" class="btn btn-secondary h-9 px-3 text-sm" @click="cancelLogin">取消</button>
            <span class="text-xs text-gray-500 dark:text-gray-400">
              登录成功后只给这个账号建独立 sub2 账号，不会把其它账号一起同步。
            </span>
          </div>

          <div v-if="loginState" class="mt-3 rounded-lg border border-blue-200 bg-white p-3 dark:border-blue-900/50 dark:bg-dark-800">
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
        </div>

        <div v-if="error" class="border-b border-red-100 bg-red-50 px-4 py-2 text-sm text-red-700 sm:px-5 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
          {{ error }}
        </div>
        <div v-if="notice" class="border-b border-emerald-100 bg-emerald-50 px-4 py-2 text-sm text-emerald-700 sm:px-5 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300">
          {{ notice }}
        </div>

        <!-- 桌面：表格 -->
        <div class="hidden min-h-0 flex-1 overflow-auto md:block">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 z-10 bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-700 dark:text-gray-400">
              <tr>
                <th class="px-4 py-2">账号</th>
                <th class="px-4 py-2">积分</th>
                <th class="px-4 py-2">签到</th>
                <th class="px-4 py-2">sub2 账号</th>
                <th class="px-4 py-2 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading"><td colspan="5" class="px-4 py-8 text-center text-gray-500">加载中…</td></tr>
              <tr v-else-if="!accounts.length"><td colspan="5" class="px-4 py-8 text-center text-gray-500">网关里还没有账号，点「+ 新增账号」添加。</td></tr>
              <tr v-for="acc in accounts" :key="acc.uid" class="border-t border-gray-100 align-top dark:border-dark-700">
                <td class="px-4 py-2">
                  <div class="font-medium text-gray-900 dark:text-white">{{ acc.nickname || acc.uid.slice(0, 8) }}</div>
                  <div class="text-xs text-gray-400">
                    {{ acc.uid.slice(0, 8) }} · {{ acc.domain || acc.realm }} · {{ acc.product || acc.platform || '-' }}
                  </div>
                </td>
                <td class="px-4 py-2">
                  <div class="text-gray-900 dark:text-white">{{ fmt(acc.credits?.remain) }} / {{ fmt(acc.credits?.size) }}</div>
                  <div class="text-xs text-gray-400">已用 {{ acc.credits?.used_percent || '-' }}</div>
                </td>
                <td class="px-4 py-2 text-xs">
                  <span :class="acc.credits?.checkin?.today_checked_in ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'">
                    {{ acc.credits?.checkin?.today_checked_in ? '今日已签' : '未签到' }}
                  </span>
                  <span class="text-gray-400"> · 连签 {{ acc.credits?.checkin?.streak_days ?? 0 }} 天</span>
                </td>
                <td class="px-4 py-2 text-xs">
                  <template v-if="acc.sub2_account">
                    <span class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                      独立账号 #{{ acc.sub2_account.id }}
                    </span>
                    <div class="mt-0.5 text-gray-400">{{ acc.sub2_account.name }}</div>
                  </template>
                  <span v-else class="text-amber-600 dark:text-amber-400">未建号</span>
                  <div class="mt-0.5">
                    <span :class="acc.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'">
                      {{ acc.enabled ? '网关启用' : '网关停用' }}
                    </span>
                  </div>
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
                    <button class="btn btn-secondary px-2 py-1 text-xs" :disabled="busy" @click="syncOne(acc)">
                      {{ acc.sub2_account ? '重新同步' : '建号' }}
                    </button>
                    <button class="btn btn-danger px-2 py-1 text-xs" :disabled="busy" @click="remove(acc)">删除</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 移动端：卡片 -->
        <div class="flex-1 space-y-2 overflow-auto p-3 md:hidden">
          <div v-if="loading" class="py-8 text-center text-sm text-gray-500">加载中…</div>
          <div v-else-if="!accounts.length" class="py-8 text-center text-sm text-gray-500">网关里还没有账号，点「+ 新增账号」添加。</div>
          <div v-for="acc in accounts" :key="'m-' + acc.uid" class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="truncate font-medium text-gray-900 dark:text-white">{{ acc.nickname || acc.uid.slice(0, 8) }}</div>
                <div class="text-xs text-gray-400">{{ acc.uid.slice(0, 8) }} · {{ acc.domain || acc.realm }}</div>
              </div>
              <span
                class="shrink-0 rounded-full px-2 py-0.5 text-[11px]"
                :class="acc.enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700'"
              >
                {{ acc.enabled ? '启用' : '停用' }}
              </span>
            </div>

            <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
              <div class="flex justify-between"><dt class="text-gray-500">积分</dt><dd class="text-gray-800 dark:text-gray-200">{{ fmt(acc.credits?.remain) }} / {{ fmt(acc.credits?.size) }}</dd></div>
              <div class="flex justify-between"><dt class="text-gray-500">签到</dt><dd class="text-gray-800 dark:text-gray-200">{{ acc.credits?.checkin?.today_checked_in ? '已签' : '未签' }} · {{ acc.credits?.checkin?.streak_days ?? 0 }}天</dd></div>
              <div class="col-span-2 flex justify-between">
                <dt class="text-gray-500">sub2 账号</dt>
                <dd class="text-gray-800 dark:text-gray-200">
                  <span v-if="acc.sub2_account">#{{ acc.sub2_account.id }} {{ acc.sub2_account.name }}</span>
                  <span v-else class="text-amber-600 dark:text-amber-400">未建号</span>
                </dd>
              </div>
            </dl>

            <div v-if="acc.lastError" class="mt-1 truncate text-xs text-red-500" :title="acc.lastError">{{ acc.lastError }}</div>

            <div class="mt-2 grid grid-cols-3 gap-1.5">
              <button class="btn btn-secondary py-1.5 text-xs" :disabled="busy" @click="doCheckin(acc.uid)">签到</button>
              <button class="btn btn-secondary py-1.5 text-xs" :disabled="busy" @click="doCredits(acc.uid)">刷新积分</button>
              <button class="btn btn-secondary py-1.5 text-xs" :disabled="busy" @click="toggleEnabled(acc)">{{ acc.enabled ? '停用' : '启用' }}</button>
              <button class="btn btn-secondary py-1.5 text-xs" :disabled="busy" @click="toggleProduct(acc)">切身份</button>
              <button class="btn btn-secondary py-1.5 text-xs" :disabled="busy" @click="syncOne(acc)">{{ acc.sub2_account ? '重新同步' : '建号' }}</button>
              <button class="btn btn-danger py-1.5 text-xs" :disabled="busy" @click="remove(acc)">删除</button>
            </div>
          </div>
        </div>

        <!-- 底部 -->
        <div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 px-4 py-3 text-xs text-gray-500 sm:px-5 dark:border-dark-700 dark:text-gray-400">
          <span class="min-w-0 truncate">网关：{{ payload.base_url || '-' }} · 上游 Key：{{ payload.upstream_key ? '已配置' : '未配置' }}</span>
          <button class="btn btn-secondary h-8 px-3 text-xs" @click="close">关闭</button>
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

const addOpen = ref(false)
const addRealm = ref('cn')
const addPlatform = ref('CLI')
const addConcurrency = ref(3)

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

function toggleAdd() {
  addOpen.value = !addOpen.value
  if (!addOpen.value) cancelLogin()
}

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

// 给单个账号建独立 sub2 账号（按账号维度同步，不牵连其它账号）
async function syncAccount(uid: string) {
  const data = await syncWorkBuddyAccounts([uid], realm.value, selectedGroups.value, addConcurrency.value)
  if (data?.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  else await load()
  emit('synced')
  return data
}

const syncOne = (acc: WBAccount) => run(async () => {
  const data = await syncAccount(acc.uid)
  notice.value = data.result.created.length
    ? `已为 ${acc.nickname || acc.uid.slice(0, 8)} 创建独立账号`
    : `已更新 ${acc.nickname || acc.uid.slice(0, 8)} 的独立账号`
})

async function startLogin() {
  error.value = ''
  notice.value = ''
  try {
    loginState.value = await startWorkBuddyLogin(addRealm.value, addPlatform.value)
    loginStatus.value = '等待授权…'
    startPolling()
  } catch (e: any) {
    error.value = e?.message || '发起登录失败'
    loginState.value = null
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
        const added = (res.account || null) as WBAccount | null
        loginState.value = null
        await load()
        if (added?.uid) {
          // 只为刚登录的这一个账号建独立 sub2 账号
          await run(async () => {
            const data = await syncAccount(added.uid)
            notice.value = data.result.created.length
              ? `已新增独立账号：${added.nickname || added.uid.slice(0, 8)}`
              : `账号已存在，已更新：${added.nickname || added.uid.slice(0, 8)}`
          })
        } else {
          notice.value = '登录成功，账号已加入网关，请点「建号」创建独立账号'
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

const remove = (acc: WBAccount) => run(async () => {
  if (!window.confirm(`确认从网关删除账号 ${acc.nickname || acc.uid.slice(0, 8)}？对应的 sub2 账号也会一并删除。`)) return
  const data = await deleteWorkBuddyAccount(acc.uid)
  if (data?.accounts) payload.value = { ...payload.value, accounts: data.accounts }
  else await load()
  emit('synced')
})

function close() {
  cancelLogin()
  addOpen.value = false
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
