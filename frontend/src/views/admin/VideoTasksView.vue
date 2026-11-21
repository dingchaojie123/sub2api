<template>
  <AppLayout>
    <TablePageLayout>
      <!-- Filters -->
      <template #filters>
        <div class="card p-4 sm:p-6">
          <div class="flex flex-wrap items-end justify-between gap-4">
            <div class="flex flex-1 flex-wrap items-end gap-4">
              <div class="w-full sm:w-auto sm:min-w-[220px]">
                <label class="input-label">{{ t('admin.videoTasks.filters.user') }}</label>
                <div class="relative">
                  <Icon
                    name="search"
                    size="md"
                    class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                  />
                  <input
                    v-model.trim="filters.user"
                    type="text"
                    class="input pl-10"
                    data-testid="filter-user"
                    @keyup.enter="search"
                  />
                </div>
              </div>

              <div class="w-full sm:w-auto sm:min-w-[120px]">
                <label class="input-label">{{ t('admin.videoTasks.filters.userId') }}</label>
                <input
                  v-model.trim="filters.userId"
                  type="text"
                  inputmode="numeric"
                  class="input"
                  data-testid="filter-user-id"
                  @keyup.enter="search"
                />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[200px]">
                <label class="input-label">{{ t('videoTasks.filters.taskId') }}</label>
                <input
                  v-model.trim="filters.taskId"
                  type="text"
                  class="input"
                  data-testid="filter-task-id"
                  @keyup.enter="search"
                />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[170px]">
                <label class="input-label">{{ t('videoTasks.filters.model') }}</label>
                <input
                  v-model.trim="filters.model"
                  type="text"
                  class="input"
                  data-testid="filter-model"
                  @keyup.enter="search"
                />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[170px]">
                <label class="input-label">{{ t('admin.videoTasks.filters.platform') }}</label>
                <input
                  v-model.trim="filters.platform"
                  type="text"
                  class="input"
                  data-testid="filter-platform"
                  @keyup.enter="search"
                />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[170px]">
                <label class="input-label">{{ t('admin.videoTasks.filters.apiKey') }}</label>
                <input
                  v-model.trim="filters.apiKey"
                  type="text"
                  class="input"
                  data-testid="filter-api-key"
                  @keyup.enter="search"
                />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[150px]">
                <label class="input-label">{{ t('videoTasks.columns.status') }}</label>
                <Select v-model="filters.status" :options="statusOptions" @change="search" />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[150px]">
                <label class="input-label">{{ t('videoTasks.filters.startDate') }}</label>
                <input v-model="filters.startDate" type="date" class="input" data-testid="filter-start-date" />
              </div>

              <div class="w-full sm:w-auto sm:min-w-[150px]">
                <label class="input-label">{{ t('videoTasks.filters.endDate') }}</label>
                <input v-model="filters.endDate" type="date" class="input" data-testid="filter-end-date" />
              </div>
            </div>

            <div class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
              <button type="button" class="btn btn-primary" :disabled="loading" data-testid="video-tasks-search" @click="search">
                {{ t('common.search') }}
              </button>
              <button type="button" class="btn btn-secondary" :disabled="loading" data-testid="video-tasks-reset" @click="resetFilters">
                {{ t('common.reset') }}
              </button>
              <button type="button" class="btn btn-secondary" :disabled="loading" data-testid="video-tasks-refresh" @click="load()">
                <Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin': loading }" />
                {{ t('videoTasks.refresh') }}
              </button>
            </div>
          </div>
          <p class="mt-3 text-xs text-gray-400">{{ t('videoTasks.autoRefreshHint') }}</p>
        </div>
      </template>

      <!-- Table -->
      <template #table>
        <DataTable :columns="columns" :data="tasks" :loading="loading" row-key="id">
          <template #cell-request_id="{ row }">
            <div class="min-w-0 max-w-[220px]">
              <div class="flex items-center gap-1.5">
                <span
                  class="block truncate font-mono text-xs text-gray-800 dark:text-gray-100"
                  :title="row.request_id"
                  data-testid="video-task-id"
                >{{ row.request_id }}</span>
                <button
                  type="button"
                  class="shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                  :aria-label="t('videoTasks.copyId')"
                  :title="t('videoTasks.copyId')"
                  @click="copyTaskId(row)"
                >
                  <Icon name="copy" size="xs" />
                </button>
              </div>
              <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400" :title="row.model">{{ row.model }}</div>
            </div>
          </template>

          <template #cell-user="{ row }">
            <div class="min-w-0 max-w-[220px]">
              <div class="truncate font-medium text-gray-900 dark:text-white" :title="row.user_email">
                {{ row.user_email || t('admin.videoTasks.userFallback', { id: row.user_id }) }}
              </div>
              <div class="mt-0.5 truncate text-xs text-gray-400">
                #{{ row.user_id }}<span v-if="row.api_key_name"> · {{ row.api_key_name }}</span>
              </div>
            </div>
          </template>

          <template #cell-platform="{ row }">
            <div class="min-w-0 max-w-[200px]">
              <div class="truncate font-mono text-xs text-gray-800 dark:text-gray-200" :title="row.platform">{{ row.platform }}</div>
              <div class="mt-0.5 truncate text-xs text-gray-400" :title="row.account_name">
                {{ row.account_name || `#${row.account_id}` }}
              </div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span
              class="inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold"
              :class="toneClass(row.status)"
              data-testid="video-task-status"
            >{{ statusLabel(row.status) }}</span>
          </template>

          <template #cell-prompt="{ row }">
            <span class="block max-w-[260px] truncate text-gray-600 dark:text-gray-300" :title="row.prompt">{{ row.prompt || '-' }}</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="whitespace-nowrap tabular-nums text-gray-500 dark:text-gray-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-total_seconds="{ value }">
            <span class="whitespace-nowrap tabular-nums text-gray-800 dark:text-gray-100">{{ formatTotalSeconds(value) }}</span>
          </template>

          <template #cell-charged="{ row }">
            <div class="whitespace-nowrap text-right tabular-nums" data-testid="video-task-charged">
              <template v-if="row.credits_charged !== null">
                <span class="font-semibold text-gray-900 dark:text-white">{{ formatCredits(row.credits_charged) }}</span>
                <span class="ml-1 text-xs text-gray-400">{{ t('videoTasks.credits') }}</span>
              </template>
              <span v-else-if="row.status === 'processing' && row.credits_held > 0" class="text-xs text-gray-400">
                {{ t('admin.videoTasks.held', { amount: formatCredits(row.credits_held) }) }}
              </span>
              <span v-else class="text-gray-300 dark:text-gray-600">-</span>
            </div>
          </template>

          <template #cell-actions="{ row }">
            <button
              type="button"
              class="inline-flex items-center gap-1 whitespace-nowrap font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              data-testid="video-task-detail-button"
              @click="openDetail(row)"
            >
              <Icon name="eye" size="sm" />
              {{ t('videoTasks.detailButton') }}
            </button>
          </template>

          <template #empty>
            <div class="flex flex-col items-center py-8" data-testid="video-tasks-empty">
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('videoTasks.empty') }}</p>
            </div>
          </template>
        </DataTable>
      </template>

      <!-- Pagination -->
      <template #pagination>
        <Pagination
          v-if="total > 0"
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Detail dialog -->
    <BaseDialog
      :show="selected !== null"
      :title="t('videoTasks.detail.title')"
      width="wide"
      :close-on-click-outside="true"
      @close="closeDetail"
    >
      <div v-if="selected" class="space-y-5 py-2" data-testid="video-task-detail">
        <div class="flex flex-wrap items-center gap-3">
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="toneClass(selected.status)"
          >{{ statusLabel(selected.status) }}</span>
          <span class="break-all font-mono text-xs text-gray-500 dark:text-gray-400" data-testid="drawer-task-id">
            {{ selected.request_id }}
          </span>
        </div>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.user') }}</dt>
            <dd class="detail-value">
              {{ selected.user_email || t('admin.videoTasks.userFallback', { id: selected.user_id }) }}
              <span v-if="selected.username" class="text-gray-400">({{ selected.username }})</span>
            </dd>
            <button
              type="button"
              class="mt-1 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
              data-testid="detail-view-user"
              @click="filterByUser(selected)"
            >{{ t('admin.videoTasks.detail.viewUser') }}</button>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.userId') }}</dt>
            <dd class="detail-value tabular-nums">{{ selected.user_id }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.apiKey') }}</dt>
            <dd class="detail-value">{{ selected.api_key_name || `#${selected.api_key_id}` }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.group') }}</dt>
            <dd class="detail-value">{{ selected.group_name || (selected.group_id ? `#${selected.group_id}` : '-') }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.platform') }}</dt>
            <dd class="detail-value font-mono text-xs">{{ selected.platform }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.account') }}</dt>
            <dd class="detail-value">{{ selected.account_name || '-' }} <span class="text-gray-400">#{{ selected.account_id }}</span></dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.model') }}</dt>
            <dd class="detail-value">{{ selected.model }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.endpoint') }}</dt>
            <dd class="detail-value font-mono text-xs">{{ selected.endpoint }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.resolution') }}</dt>
            <dd class="detail-value">{{ selected.resolution || '-' }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.duration') }}</dt>
            <dd class="detail-value">{{ formatVideoDuration(selected.duration_seconds) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.billing') }}</dt>
            <dd class="detail-value">{{ billingLabel(selected.billing_status) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.held') }}</dt>
            <dd class="detail-value tabular-nums">{{ creditsText(selected.credits_held) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.charged') }}</dt>
            <dd class="detail-value tabular-nums" data-testid="drawer-charged">{{ creditsText(selected.credits_charged) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.refunded') }}</dt>
            <dd class="detail-value tabular-nums" data-testid="drawer-refunded">{{ creditsText(selected.credits_refunded) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.createdAt') }}</dt>
            <dd class="detail-value tabular-nums">{{ formatDateTime(selected.created_at) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('videoTasks.detail.totalTime') }}</dt>
            <dd class="detail-value tabular-nums">{{ formatTotalSeconds(selected.total_seconds) }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.localTaskId') }}</dt>
            <dd class="detail-value break-all font-mono text-xs">{{ selected.id }}</dd>
          </div>
          <div>
            <dt class="detail-label">{{ t('admin.videoTasks.detail.upstreamTaskId') }}</dt>
            <dd class="detail-value break-all font-mono text-xs">
              {{ selected.upstream_task_id || t('admin.videoTasks.detail.noUpstreamId') }}
            </dd>
          </div>
        </dl>

        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <p class="detail-label">{{ t('videoTasks.detail.prompt') }}</p>
          <p
            class="mt-2 max-h-56 overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-100"
            data-testid="drawer-prompt"
          >{{ selected.prompt || t('videoTasks.detail.noPrompt') }}</p>
        </div>

        <div
          v-if="selected.status === 'failed'"
          class="rounded-xl border border-red-100 bg-red-50/70 p-4 dark:border-red-900/40 dark:bg-red-900/10"
          data-testid="drawer-failure"
        >
          <p class="text-xs text-red-500">{{ t('videoTasks.detail.error') }}</p>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-red-700 dark:text-red-300" data-testid="drawer-failure-reason">
            {{ selected.error_message || t('videoTasks.detail.errorUnknown') }}
          </p>
          <p v-if="selected.error_code" class="mt-1 font-mono text-xs text-red-400">{{ selected.error_code }}</p>
        </div>

        <div v-if="selected.status === 'succeeded'" class="rounded-xl border border-gray-100 p-3 dark:border-dark-700" data-testid="drawer-result">
          <template v-if="selected.can_play">
            <div class="overflow-hidden rounded-lg bg-black">
              <video
                v-if="player.videoUrl.value"
                :src="player.videoUrl.value"
                class="aspect-video w-full"
                controls
                playsinline
                preload="metadata"
                data-testid="drawer-video"
                @error="player.onVideoError"
              />
              <div v-else class="flex aspect-video flex-col items-center justify-center gap-3 px-4 text-center text-sm text-gray-300">
                <template v-if="player.state.value === 'error'">
                  <span data-testid="drawer-video-error">
                    {{ player.expired.value ? t('videoTasks.detail.videoExpired') : t('videoTasks.detail.videoFailed') }}
                  </span>
                  <button
                    v-if="!player.expired.value"
                    type="button"
                    class="rounded-lg border border-gray-500 px-3 py-1.5 text-xs text-gray-100 transition-colors hover:bg-white/10"
                    data-testid="drawer-video-retry"
                    @click="player.retry"
                  >
                    {{ t('videoTasks.detail.retry') }}
                  </button>
                </template>
                <span v-else class="flex items-center gap-2">
                  <Icon name="refresh" size="sm" class="animate-spin" />
                  {{ t('videoTasks.detail.loadingVideo') }}
                </span>
              </div>
            </div>
            <div class="mt-3 flex justify-end">
              <button type="button" class="btn btn-primary" :disabled="downloading" data-testid="drawer-download" @click="downloadVideo">
                <Icon name="download" size="xs" class="mr-1.5" />
                {{ t('videoTasks.detail.download') }}
              </button>
            </div>
          </template>
          <p v-else class="px-1 py-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('videoTasks.detail.videoUnavailable') }}
          </p>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { AdminVideoTask, AdminVideoTaskListParams } from '@/api/admin/videoTasks'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { useVideoTaskPlayer } from '@/composables/useVideoTaskPlayer'
import { formatDateTime } from '@/utils/format'
import {
  formatCredits,
  formatTotalSeconds,
  formatVideoDuration,
  hasUnfinishedTasks,
  videoDownloadName,
  videoTaskStatusMeta,
  type VideoTaskStatusTone
} from '@/views/user/videoTaskFormat'

const AUTO_REFRESH_INTERVAL_MS = 10_000

const { t, te } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const filters = reactive({
  user: '',
  userId: '',
  taskId: '',
  model: '',
  platform: '',
  apiKey: '',
  status: '' as AdminVideoTaskListParams['status'],
  startDate: '',
  endDate: ''
})

const tasks = ref<AdminVideoTask[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const columns = computed<Column[]>(() => [
  { key: 'request_id', label: t('videoTasks.columns.taskId') },
  { key: 'user', label: t('admin.videoTasks.columns.user') },
  { key: 'platform', label: t('admin.videoTasks.columns.platform') },
  { key: 'status', label: t('videoTasks.columns.status') },
  { key: 'prompt', label: t('videoTasks.columns.prompt') },
  { key: 'created_at', label: t('videoTasks.columns.createdAt') },
  { key: 'total_seconds', label: t('videoTasks.columns.totalTime') },
  { key: 'charged', label: t('videoTasks.columns.charged') },
  { key: 'actions', label: t('common.actions') }
])

const statusOptions = computed(() => [
  { value: '', label: t('videoTasks.filters.allStatus') },
  { value: 'processing', label: t('videoTasks.status.processing') },
  { value: 'succeeded', label: t('videoTasks.status.succeeded') },
  { value: 'failed', label: t('videoTasks.status.failed') },
  { value: 'requires_attention', label: t('videoTasks.status.requiresAttention') }
])

const toneClasses: Record<VideoTaskStatusTone, string> = {
  success: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  danger: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300',
  warning: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
  info: 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300'
}

function toneClass(status: string): string {
  return toneClasses[videoTaskStatusMeta(status).tone]
}

function statusLabel(status: string): string {
  return t(`videoTasks.status.${videoTaskStatusMeta(status).labelKey}`)
}

function billingLabel(status: string): string {
  const key = `admin.videoTasks.billing.${status}`
  return te(key) ? t(key) : status || '-'
}

function creditsText(value: number | null): string {
  return value === null ? '-' : `${formatCredits(value)} ${t('videoTasks.credits')}`
}

// ---------- 列表加载 ----------

let loadController: AbortController | null = null
let loadSeq = 0

function buildParams(): AdminVideoTaskListParams {
  const userId = Number(filters.userId)
  return {
    page: page.value,
    page_size: pageSize.value,
    user: filters.user,
    // 非正整数的用户 ID 直接忽略，避免后端因格式错误返回 400。
    user_id: Number.isInteger(userId) && userId > 0 ? userId : undefined,
    task_id: filters.taskId,
    model: filters.model,
    platform: filters.platform,
    api_key: filters.apiKey,
    status: filters.status,
    start_date: filters.startDate,
    end_date: filters.endDate
  }
}

function isCanceled(error: unknown): boolean {
  return axios.isCancel(error) || (error as { code?: string } | null)?.code === 'ERR_CANCELED'
}

/** silent=true 用于自动刷新：不出现加载态，失败也不打扰管理员。 */
async function load(silent = false) {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const seq = ++loadSeq
  if (!silent) loading.value = true
  try {
    const result = await adminAPI.videoTasks.list(buildParams(), { signal: controller.signal })
    if (seq !== loadSeq) return
    tasks.value = result.items ?? []
    total.value = result.total ?? 0
    // 筛选后当前页可能已超出范围，回到最后一页。
    if (!tasks.value.length && total.value > 0 && page.value > 1) {
      page.value = Math.max(1, Math.ceil(total.value / pageSize.value))
      void load(silent)
      return
    }
    syncSelected()
  } catch (error) {
    if (isCanceled(error) || seq !== loadSeq) return
    if (!silent) appStore.showError(t('videoTasks.loadFailed'))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function resetFilters() {
  filters.user = ''
  filters.userId = ''
  filters.taskId = ''
  filters.model = ''
  filters.platform = ''
  filters.apiKey = ''
  filters.status = ''
  filters.startDate = ''
  filters.endDate = ''
  search()
}

function onPageChange(next: number) {
  page.value = next
  void load()
}

function onPageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  void load()
}

async function copyTaskId(task: AdminVideoTask) {
  await copyToClipboard(task.request_id, t('videoTasks.copied'))
}

// ---------- 自动刷新 ----------

let refreshTimer: number | undefined

function autoRefreshTick() {
  if (document.visibilityState !== 'visible') return
  if (!hasUnfinishedTasks(tasks.value)) return
  void load(true)
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') autoRefreshTick()
}

// ---------- 详情 ----------

const selected = ref<AdminVideoTask | null>(null)
const downloading = ref(false)
const player = useVideoTaskPlayer(adminAPI.videoTasks.fetchContent)

function syncSelected() {
  if (!selected.value) return
  const fresh = tasks.value.find((task) => task.id === selected.value?.id)
  if (fresh) selected.value = fresh
}

function openDetail(task: AdminVideoTask) {
  player.reset()
  selected.value = task
  if (task.can_play) void player.load(task.id)
}

function closeDetail() {
  player.reset()
  selected.value = null
}

/** 从详情跳到「只看该用户」的列表。 */
function filterByUser(task: AdminVideoTask) {
  closeDetail()
  filters.userId = String(task.user_id)
  search()
}

// 详情打开期间任务从处理中变为已完成时，自动加载视频。
watch(
  () => selected.value?.can_play,
  (canPlay) => {
    if (canPlay && selected.value && player.state.value === 'idle') void player.load(selected.value.id)
  }
)

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

async function downloadVideo() {
  const task = selected.value
  if (!task || downloading.value) return
  downloading.value = true
  try {
    saveBlob(await player.getBlob(task.id), videoDownloadName(task))
  } catch {
    appStore.showError(t('videoTasks.detail.downloadFailed'))
  } finally {
    downloading.value = false
  }
}

onMounted(() => {
  void load()
  refreshTimer = window.setInterval(autoRefreshTick, AUTO_REFRESH_INTERVAL_MS)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) window.clearInterval(refreshTimer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  loadController?.abort()
})
</script>

<style scoped>
.detail-label {
  @apply text-xs text-gray-400;
}

.detail-value {
  @apply mt-1 break-words font-medium text-gray-900 dark:text-white;
}
</style>
