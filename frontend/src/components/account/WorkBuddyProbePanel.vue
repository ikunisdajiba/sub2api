<template>
  <BaseDialog
    :show="show"
    title="WorkBuddy 账号探活"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <p class="text-sm leading-relaxed text-gray-500 dark:text-gray-400">
        主动探活会把 wb 网关里所有账号的真实健康状态刷一遍：上游返回 429/403 的账号立即停用并记录原因，
        恢复正常的账号自动启用。探活结果直接反映到账号列表的状态与停用原因上。
      </p>

      <!-- 手动探活 -->
      <div class="flex flex-wrap items-end gap-3">
        <label class="block">
          <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">地区</span>
          <select v-model="realm" class="input h-9 w-40 text-sm" :disabled="busy">
            <option value="all">全部地区</option>
            <option value="cn">国内 (cn)</option>
            <option value="intl">国际 (intl)</option>
          </select>
        </label>
        <label class="block">
          <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">探活模型（可留空）</span>
          <input v-model.trim="model" class="input h-9 w-56 text-sm" placeholder="默认由网关决定" :disabled="busy" />
        </label>
        <button class="btn btn-primary h-9 px-4 text-sm" :disabled="busy" @click="doProbeAll">
          {{ busy ? '探活中…' : '立即探活全部' }}
        </button>
      </div>

      <!-- 结果 -->
      <div
        v-if="notice"
        class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-300"
      >
        {{ notice }}
      </div>
      <div
        v-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ error }}
      </div>

      <!-- 明细 -->
      <div v-if="results.length" class="max-h-64 overflow-auto rounded-lg border border-gray-200 dark:border-dark-700">
        <table class="w-full text-left text-xs">
          <thead class="bg-gray-50 text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2 font-medium">账号</th>
              <th class="px-3 py-2 font-medium">结果</th>
              <th class="px-3 py-2 font-medium">动作</th>
              <th class="px-3 py-2 font-medium">说明</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in results"
              :key="row.uid"
              class="border-t border-gray-100 dark:border-dark-700"
            >
              <td class="px-3 py-2 text-gray-900 dark:text-white">
                {{ row.nickname || row.uid.slice(0, 8) }}
              </td>
              <td class="px-3 py-2" :class="row.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">
                {{ row.ok ? '正常' : '异常' }}
              </td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ row.action || '-' }}</td>
              <td class="px-3 py-2 text-gray-500 dark:text-gray-400">
                {{ row.error || row.code || '-' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
        <div class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">定时探活</div>
        <div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-200">
            <input v-model="schedule.enabled" type="checkbox" class="h-4 w-4" :disabled="busy" />
            <span>启用</span>
          </label>
          <span class="text-sm text-gray-500 dark:text-gray-400">每</span>
          <input
            v-model.number="schedule.interval_minutes"
            type="number"
            min="1"
            class="input h-9 w-20 text-sm"
            :disabled="busy || !schedule.enabled"
          />
          <span class="text-sm text-gray-500 dark:text-gray-400">分钟探活一次</span>
          <button class="btn btn-secondary h-9 px-3 text-sm" :disabled="busy" @click="saveSchedule">
            保存
          </button>
          <span v-if="schedule.last_run_at" class="text-xs text-gray-400">
            上次运行：{{ schedule.last_run_at }}
          </span>
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-secondary" @click="emit('close')">关闭</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  getWorkBuddyProbeSchedule,
  probeWorkBuddyAll,
  saveWorkBuddyProbeSchedule,
  type WBProbeResult
} from '@/api/workbuddy'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'probed'): void }>()

const realm = ref('all')
const model = ref('')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const results = ref<NonNullable<WBProbeResult['results']>>([])
const schedule = ref<{ enabled: boolean; interval_minutes: number; last_run_at?: string }>({
  enabled: false,
  interval_minutes: 30
})

async function loadSchedule() {
  try {
    const s = await getWorkBuddyProbeSchedule()
    schedule.value = {
      enabled: !!s.enabled,
      interval_minutes: Number(s.interval_minutes) || 30,
      last_run_at: s.last_run_at
    }
  } catch {
    // 读不到就保留默认值，不阻塞探活
  }
}

async function doProbeAll() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const data = await probeWorkBuddyAll(model.value, realm.value === 'all' ? '' : realm.value)
    const s = data.summary || { total: 0, stopped: 0, resumed: 0 }
    notice.value = `探活完成：共 ${s.total} 个，停用 ${s.stopped}，恢复 ${s.resumed}`
    results.value = data.results || []
    emit('probed')
  } catch (e: any) {
    error.value = e?.message || '探活失败'
  } finally {
    busy.value = false
  }
}

async function saveSchedule() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const s = await saveWorkBuddyProbeSchedule({
      enabled: schedule.value.enabled,
      interval_minutes: schedule.value.interval_minutes,
      model: model.value
    })
    schedule.value = {
      enabled: !!s.enabled,
      interval_minutes: Number(s.interval_minutes) || 30,
      last_run_at: s.last_run_at
    }
    notice.value = s.enabled
      ? `定时探活已开启，每 ${s.interval_minutes} 分钟一次`
      : '定时探活已关闭'
  } catch (e: any) {
    error.value = e?.message || '保存失败'
  } finally {
    busy.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      notice.value = ''
      error.value = ''
      loadSchedule()
    }
  }
)

onMounted(() => {
  if (props.show) loadSchedule()
})
</script>
