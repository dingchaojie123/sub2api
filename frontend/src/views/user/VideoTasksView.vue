<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('videoTasks.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('videoTasks.description') }}
          </p>
          <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
            {{ t('videoTasks.autoRefreshHint') }}
          </p>
        </div>
        <button
          type="button"
          class="inline-flex h-10 items-center gap-2 self-start rounded-xl border border-gray-200 bg-white px-4 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
          :disabled="loading"
          data-testid="video-tasks-refresh"
          @click="load()"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('videoTasks.refresh') }}
        </button>
      </header>

      <!-- 筛选 -->
      <section class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <form class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" @submit.prevent="search">
          <input
            v-model="filters.taskId"
            type="text"
            class="filter-input"
            :placeholder="t('videoTasks.filters.taskId')"
            data-testid="filter-task-id"
          >
          <input
            v-model="filters.model"
            type="text"
            class="filter-input"
            :placeholder="t('videoTasks.filters.model')"
            data-testid="filter-model"
          >
          <select v-model="filters.status" class="filter-input" data-testid="filter-status">
            <option value="">{{ t('videoTasks.filters.allStatus') }}</option>
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
          <input
            v-model="filters.apiKey"
            type="text"
            class="filter-input"
            :placeholder="t('videoTasks.filters.apiKey')"
            data-testid="filter-api-key"
          >
          <input
            v-model="filters.startDate"
            type="date"
            class="filter-input"
            :aria-label="t('videoTasks.filters.startDate')"
            data-testid="filter-start-date"
          >
          <input
            v-model="filters.endDate"
            type="date"
            class="filter-input"
            :aria-label="t('videoTasks.filters.endDate')"
            data-testid="filter-end-date"
          >
          <div class="flex gap-3 sm:col-span-2">
            <button
              type="submit"
              class="inline-flex h-11 flex-1 items-center justify-center rounded-xl bg-emerald-600 px-5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-emerald-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/40"
              data-testid="video-tasks-search"
            >
              {{ t('videoTasks.filters.search') }}
            </button>
            <button
              type="button"
              class="inline-flex h-11 items-center justify-center rounded-xl border border-gray-200 bg-white px-5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
              data-testid="video-tasks-reset"
              @click="reset"
            >
              {{ t('videoTasks.filters.reset') }}
            </button>
          </div>
        </form>
      </section>

      <!-- 列表 -->
      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="overflow-x-auto">
          <table class="w-full min-w-[960px] border-collapse text-sm">
            <thead>
              <tr class="bg-gray-50 text-left text-xs font-semibold text-gray-500 dark:bg-gray-900/40 dark:text-gray-400">
                <th class="px-5 py-3.5">{{ t('videoTasks.columns.taskId') }}</th>
                <th class="px-4 py-3.5">{{ t('videoTasks.columns.model') }}</th>
                <th class="px-4 py-3.5">{{ t('videoTasks.columns.status') }}</th>
                <th class="px-4 py-3.5">{{ t('videoTasks.columns.prompt') }}</th>
                <th class="px-4 py-3.5 text-center">{{ t('videoTasks.columns.outputCount') }}</th>
                <th class="px-4 py-3.5">{{ t('videoTasks.columns.createdAt') }}</th>
                <th class="px-4 py-3.5">{{ t('videoTasks.columns.totalTime') }}</th>
                <th class="px-4 py-3.5 text-right">{{ t('videoTasks.columns.charged') }}</th>
                <th class="px-5 py-3.5 text-right">{{ t('videoTasks.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody v-if="tasks.length">
              <tr
                v-for="task in tasks"
                :key="task.id"
                class="border-t border-gray-100 transition-colors hover:bg-gray-50/70 dark:border-gray-700 dark:hover:bg-gray-700/30"
                data-testid="video-task-row"
              >
                <td class="px-5 py-4">
                  <div class="flex items-center gap-1.5">
                    <span
                      class="block max-w-[200px] truncate font-mono text-xs text-gray-800 dark:text-gray-100"
                      :title="task.request_id"
                      data-testid="video-task-id"
                    >{{ task.request_id }}</span>
                    <button
                      type="button"
                      class="shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700"
                      :aria-label="t('videoTasks.copyId')"
                      :title="t('videoTasks.copyId')"
                      @click="copyTaskId(task)"
                    >
                      <Icon name="copy" size="xs" />
                    </button>
                  </div>
                </td>
                <td class="px-4 py-4 font-medium text-gray-900 dark:text-gray-100">{{ task.model }}</td>
                <td class="px-4 py-4">
                  <span
                    class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
                    :class="toneClass(task.status)"
                    data-testid="video-task-status"
                  >{{ statusLabel(task.status) }}</span>
                </td>
                <td class="px-4 py-4 text-gray-600 dark:text-gray-300">
                  <span class="block max-w-[320px] truncate" :title="task.prompt">{{ task.prompt || '-' }}</span>
                </td>
                <td class="px-4 py-4 text-center tabular-nums text-gray-800 dark:text-gray-100">{{ task.output_count }}</td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums text-gray-500 dark:text-gray-400">
                  {{ formatDateTime(task.created_at) }}
                </td>
                <td class="whitespace-nowrap px-4 py-4 tabular-nums text-gray-800 dark:text-gray-100">
                  {{ formatTotalSeconds(task.total_seconds) }}
                </td>
                <td class="whitespace-nowrap px-4 py-4 text-right tabular-nums" data-testid="video-task-charged">
                  <template v-if="task.credits_charged !== null">
                    <span class="font-semibold text-gray-900 dark:text-white">{{ formatCredits(task.credits_charged) }}</span>
                    <span class="ml-1 text-xs text-gray-400">{{ t('videoTasks.credits') }}</span>
                  </template>
                  <span v-else-if="task.status === 'processing' && task.credits_held > 0" class="text-xs text-gray-400">
                    {{ t('videoTasks.held', { amount: formatCredits(task.credits_held) }) }}
                  </span>
                  <span v-else class="text-gray-300 dark:text-gray-600">-</span>
                </td>
                <td class="px-5 py-4 text-right">
                  <button
                    type="button"
                    class="inline-flex h-9 items-center rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                    data-testid="video-task-detail-button"
                    @click="openDetail(task)"
                  >
                    {{ t('videoTasks.detailButton') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="!tasks.length"
          class="flex min-h-48 items-center justify-center px-6 py-12 text-sm text-gray-400"
          data-testid="video-tasks-empty"
        >
          {{ loading ? '' : t('videoTasks.empty') }}
          <Icon v-if="loading" name="refresh" size="md" class="animate-spin" />
        </div>

        <Pagination
          v-if="total > 0"
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="onPageChange"
          @update:page-size="onPageSizeChange"
        />
      </section>
    </div>

    <!-- 任务详情抽屉 -->
    <Teleport to="body">
      <Transition name="drawer">
        <div
          v-if="selected"
          class="fixed inset-0 z-50"
          role="dialog"
          aria-modal="true"
          :aria-label="t('videoTasks.detail.title')"
          data-testid="video-task-drawer"
        >
          <div class="drawer-overlay absolute inset-0 bg-black/30" @click="closeDetail" />
          <aside class="drawer-panel absolute right-0 top-0 flex h-full w-full max-w-[560px] flex-col bg-white shadow-2xl dark:bg-gray-800">
            <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-gray-700">
              <div class="min-w-0">
                <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('videoTasks.detail.title') }}</h2>
                <p class="mt-1 break-all font-mono text-xs text-gray-400" data-testid="drawer-task-id">{{ selected.request_id }}</p>
              </div>
              <button
                type="button"
                class="shrink-0 rounded-xl border border-gray-200 p-2 text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 dark:border-gray-600 dark:hover:bg-gray-700"
                :aria-label="t('videoTasks.detail.close')"
                data-testid="drawer-close"
                @click="closeDetail"
              >
                <Icon name="x" size="sm" />
              </button>
            </header>

            <div class="flex-1 space-y-5 overflow-y-auto px-6 py-5">
              <dl class="grid grid-cols-2 gap-x-6 gap-y-5 text-sm">
                <div>
                  <dt class="detail-label">{{ t('videoTasks.detail.status') }}</dt>
                  <dd class="mt-1">
                    <span
                      class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
                      :class="toneClass(selected.status)"
                    >{{ statusLabel(selected.status) }}</span>
                  </dd>
                </div>
                <div>
                  <dt class="detail-label">{{ t('videoTasks.detail.totalTime') }}</dt>
                  <dd class="detail-value tabular-nums">{{ formatTotalSeconds(selected.total_seconds) }}</dd>
                </div>
                <div>
                  <dt class="detail-label">{{ t('videoTasks.detail.model') }}</dt>
                  <dd class="detail-value">{{ selected.model }}</dd>
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
              </dl>

              <div class="rounded-xl border border-gray-100 bg-gray-50/70 p-4 dark:border-gray-700 dark:bg-gray-900/30">
                <p class="detail-label">{{ t('videoTasks.detail.prompt') }}</p>
                <p
                  class="mt-2 max-h-56 overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-100"
                  data-testid="drawer-prompt"
                >{{ selected.prompt || t('videoTasks.detail.noPrompt') }}</p>
              </div>

              <div
                v-if="selected.status === 'failed' && selected.error_message"
                class="rounded-xl border border-red-100 bg-red-50/70 p-4 dark:border-red-900/40 dark:bg-red-900/10"
              >
                <p class="text-xs text-red-500">{{ t('videoTasks.detail.error') }}</p>
                <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-red-700 dark:text-red-300">{{ selected.error_message }}</p>
              </div>

              <div
                v-if="selected.status === 'succeeded'"
                class="rounded-xl border border-gray-100 p-3 dark:border-gray-700"
                data-testid="drawer-result"
              >
                <template v-if="selected.can_play">
                  <div class="overflow-hidden rounded-lg bg-black">
                    <video
                      v-if="videoUrl"
                      :src="videoUrl"
                      class="aspect-video w-full"
                      controls
                      playsinline
                      preload="metadata"
                      data-testid="drawer-video"
                      @error="onVideoError"
                    />
                    <div v-else class="flex aspect-video flex-col items-center justify-center gap-3 px-4 text-center text-sm text-gray-300">
                      <template v-if="videoState === 'error'">
                        <span data-testid="drawer-video-error">
                          {{ videoExpired ? t('videoTasks.detail.videoExpired') : t('videoTasks.detail.videoFailed') }}
                        </span>
                        <button
                          v-if="!videoExpired"
                          type="button"
                          class="rounded-lg border border-gray-500 px-3 py-1.5 text-xs text-gray-100 transition-colors hover:bg-white/10"
                          data-testid="drawer-video-retry"
                          @click="retryVideo"
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
                  <div class="mt-3 flex items-center justify-between">
                    <span class="text-xs text-gray-400">{{ t('videoTasks.detail.resultIndex', { index: 1 }) }}</span>
                    <button
                      type="button"
                      class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-emerald-600 px-4 text-xs font-semibold text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="downloading"
                      data-testid="drawer-download"
                      @click="downloadVideo"
                    >
                      <Icon name="download" size="xs" />
                      {{ t('videoTasks.detail.download') }}
                    </button>
                  </div>
                </template>
                <p v-else class="px-1 py-2 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('videoTasks.detail.videoUnavailable') }}
                </p>
              </div>
            </div>
          </aside>
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'
import {
  fetchContent,
  list as listVideoTasks,
  VideoContentError,
  type VideoTask,
  type VideoTaskListParams,
} from '@/api/videoTasks'
import {
  formatCredits,
  formatTotalSeconds,
  formatVideoDuration,
  hasUnfinishedTasks,
  videoDownloadName,
  videoTaskStatusMeta,
  type VideoTaskStatusTone,
} from './videoTaskFormat'

const AUTO_REFRESH_INTERVAL_MS = 10_000

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const filters = reactive({
  taskId: '',
  model: '',
  status: '' as VideoTaskListParams['status'],
  apiKey: '',
  startDate: '',
  endDate: '',
})

const tasks = ref<VideoTask[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const statusOptions = computed(() => [
  { value: 'processing', label: t('videoTasks.status.processing') },
  { value: 'succeeded', label: t('videoTasks.status.succeeded') },
  { value: 'failed', label: t('videoTasks.status.failed') },
  { value: 'requires_attention', label: t('videoTasks.status.requiresAttention') },
])

const toneClasses: Record<VideoTaskStatusTone, string> = {
  success: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  danger: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300',
  warning: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
  info: 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300',
}

function toneClass(status: string): string {
  return toneClasses[videoTaskStatusMeta(status).tone]
}

function statusLabel(status: string): string {
  return t(`videoTasks.status.${videoTaskStatusMeta(status).labelKey}`)
}

function creditsText(value: number | null): string {
  return value === null ? '-' : `${formatCredits(value)} ${t('videoTasks.credits')}`
}

// ---------- 列表加载 ----------

let loadController: AbortController | null = null
let loadSeq = 0

function buildParams(): VideoTaskListParams {
  return {
    page: page.value,
    page_size: pageSize.value,
    task_id: filters.taskId.trim(),
    model: filters.model.trim(),
    status: filters.status,
    api_key: filters.apiKey.trim(),
    start_date: filters.startDate,
    end_date: filters.endDate,
  }
}

function isCanceled(error: unknown): boolean {
  return axios.isCancel(error) || (error as { code?: string } | null)?.code === 'ERR_CANCELED'
}

/** silent=true 用于自动刷新：不出现加载态，失败也不打扰用户。 */
async function load(silent = false) {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const seq = ++loadSeq
  if (!silent) loading.value = true
  try {
    const result = await listVideoTasks(buildParams(), { signal: controller.signal })
    if (seq !== loadSeq) return
    tasks.value = result.items ?? []
    total.value = result.total ?? 0
    // 删除或筛选后当前页可能已超出范围，回到最后一页。
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

function reset() {
  filters.taskId = ''
  filters.model = ''
  filters.status = ''
  filters.apiKey = ''
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

async function copyTaskId(task: VideoTask) {
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

// ---------- 详情抽屉 ----------

type VideoState = 'idle' | 'loading' | 'ready' | 'error'

const selected = ref<VideoTask | null>(null)
const videoUrl = ref<string | null>(null)
const videoState = ref<VideoState>('idle')
/** 404 表示上游已删除视频（通常是任务太久远），重试没有意义。 */
const videoExpired = ref(false)
const downloading = ref(false)
let videoBlob: Blob | null = null
let videoController: AbortController | null = null

function syncSelected() {
  if (!selected.value) return
  const fresh = tasks.value.find((task) => task.id === selected.value?.id)
  if (fresh) selected.value = fresh
}

function resetVideo() {
  videoController?.abort()
  videoController = null
  if (videoUrl.value) URL.revokeObjectURL(videoUrl.value)
  videoUrl.value = null
  videoBlob = null
  videoState.value = 'idle'
  videoExpired.value = false
}

async function loadVideo(task: VideoTask) {
  if (videoState.value === 'loading' || videoState.value === 'ready') return
  videoController?.abort()
  const controller = new AbortController()
  videoController = controller
  videoState.value = 'loading'
  try {
    const blob = await fetchContent(task.id, { signal: controller.signal })
    if (controller.signal.aborted || selected.value?.id !== task.id) return
    // 上游常返回 application/octet-stream，播放器对这种 blob 类型的解码并不可靠，统一按 mp4 处理。
    const type = blob.type.toLowerCase().startsWith('video/') ? blob.type : 'video/mp4'
    videoBlob = new Blob([blob], { type })
    videoUrl.value = URL.createObjectURL(videoBlob)
    videoState.value = 'ready'
  } catch (error) {
    if (isCanceled(error) || controller.signal.aborted || selected.value?.id !== task.id) return
    videoExpired.value = error instanceof VideoContentError && error.status === 404
    videoState.value = 'error'
  }
}

/** 视频字节已取到但播放器无法解码：保留 blob，用户仍然可以下载。 */
function onVideoError() {
  if (videoUrl.value) URL.revokeObjectURL(videoUrl.value)
  videoUrl.value = null
  videoState.value = 'error'
}

function retryVideo() {
  const task = selected.value
  if (!task) return
  resetVideo()
  void loadVideo(task)
}

function openDetail(task: VideoTask) {
  resetVideo()
  selected.value = task
  if (task.can_play) void loadVideo(task)
}

function closeDetail() {
  resetVideo()
  selected.value = null
}

// 抽屉打开期间任务从处理中变为已完成时，自动加载视频。
watch(
  () => selected.value?.can_play,
  (canPlay) => {
    if (canPlay && selected.value && videoState.value === 'idle') void loadVideo(selected.value)
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
    const blob = videoBlob ?? (await fetchContent(task.id))
    saveBlob(blob, videoDownloadName(task))
  } catch {
    appStore.showError(t('videoTasks.detail.downloadFailed'))
  } finally {
    downloading.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && selected.value) closeDetail()
}

onMounted(() => {
  void load()
  refreshTimer = window.setInterval(autoRefreshTick, AUTO_REFRESH_INTERVAL_MS)
  document.addEventListener('visibilitychange', onVisibilityChange)
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) window.clearInterval(refreshTimer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('keydown', onKeydown)
  loadController?.abort()
  resetVideo()
})
</script>

<style scoped>
.filter-input {
  @apply h-11 w-full rounded-xl border border-gray-200 bg-white px-4 text-sm text-gray-900 outline-none transition placeholder:text-gray-400 focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-white;
}

.detail-label {
  @apply text-xs text-gray-400;
}

.detail-value {
  @apply mt-1 break-words font-medium text-gray-900 dark:text-white;
}

.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 0.2s ease;
}

.drawer-enter-active .drawer-panel,
.drawer-leave-active .drawer-panel {
  transition: transform 0.25s ease;
}

.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}

.drawer-enter-from .drawer-panel,
.drawer-leave-to .drawer-panel {
  transform: translateX(100%);
}
</style>
