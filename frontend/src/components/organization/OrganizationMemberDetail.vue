<template>
  <div class="space-y-5" data-testid="member-detail">
    <!-- 筛选条：成员 + 时间范围 -->
    <section class="md-card flex flex-wrap items-end gap-x-5 gap-y-4 p-4">
      <label class="block">
        <span class="md-label">成员</span>
        <select
          :value="userId"
          class="md-input mt-1.5 w-64"
          aria-label="选择成员"
          @change="emit('update:userId', Number(($event.target as HTMLSelectElement).value))"
        >
          <option v-for="m in members" :key="m.user_id" :value="m.user_id">{{ m.username || m.email }}（{{ m.email }}）</option>
        </select>
      </label>

      <div>
        <span class="md-label">时间范围</span>
        <div class="mt-1.5 flex flex-wrap items-center gap-2">
          <input v-model="startDate" type="date" class="md-input w-40" :max="endDate || undefined" aria-label="开始日期" />
          <span class="text-gray-400">至</span>
          <input v-model="endDate" type="date" class="md-input w-40" :min="startDate || undefined" :max="today" aria-label="结束日期" />
          <div class="ml-1 flex gap-1.5">
            <button
              v-for="preset in presets"
              :key="preset.days"
              type="button"
              class="md-chip"
              :class="{ 'md-chip-active': activePreset === preset.days }"
              @click="applyPreset(preset.days)"
            >{{ preset.label }}</button>
          </div>
        </div>
      </div>

      <button type="button" class="md-refresh ml-auto" :disabled="loading || !userId" @click="reload">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        刷新
      </button>
    </section>

    <!-- 汇总 -->
    <section class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div v-for="card in summaryCards" :key="card.label" class="md-card flex items-start gap-3 p-4">
        <span class="md-kpi-icon" :class="card.tone"><Icon :name="card.icon" size="sm" /></span>
        <div class="min-w-0">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-1 truncate text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ card.value }}</p>
          <p v-if="card.hint" class="mt-0.5 truncate text-xs text-gray-400">{{ card.hint }}</p>
        </div>
      </div>
    </section>

    <!-- 分栏：模型用量 / 生成内容 -->
    <section class="md-card overflow-hidden">
      <div class="flex items-center gap-1 border-b border-gray-100 px-3 pt-2 dark:border-dark-700" role="tablist">
        <button
          v-for="item in views"
          :key="item.key"
          type="button"
          role="tab"
          :aria-selected="view === item.key"
          class="md-seg"
          :class="{ 'md-seg-active': view === item.key }"
          @click="view = item.key"
        >
          <Icon :name="item.icon" size="sm" />
          {{ item.label }}
          <span class="md-seg-count">{{ item.count }}</span>
        </button>
      </div>

      <!-- 模型用量 -->
      <div v-if="view === 'usage'" class="overflow-x-auto">
        <table class="md-table">
          <thead>
            <tr>
              <th>模型</th>
              <th class="text-right">请求</th>
              <th class="text-right">输入 Token</th>
              <th class="text-right">输出 Token</th>
              <th class="text-right">图片</th>
              <th class="text-right">视频</th>
              <th class="w-56">费用</th>
              <th>最近使用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !usage"><td colspan="8" class="md-empty">加载中...</td></tr>
            <tr v-else-if="!usage || usage.models.length === 0"><td colspan="8" class="md-empty">所选时间范围内该成员没有使用记录</td></tr>
            <tr v-for="row in usage?.models || []" :key="row.model">
              <td class="font-medium text-gray-900 dark:text-white">{{ row.model }}</td>
              <td class="text-right tabular-nums">{{ nf(row.total_requests) }}</td>
              <td class="text-right tabular-nums">{{ nf(row.input_tokens) }}</td>
              <td class="text-right tabular-nums">{{ nf(row.output_tokens) }}</td>
              <td class="text-right tabular-nums">{{ row.image_count || '-' }}</td>
              <td class="text-right tabular-nums">{{ row.video_count || '-' }}</td>
              <td>
                <div class="flex items-center gap-3">
                  <span class="w-20 shrink-0 text-right font-mono tabular-nums">{{ money(row.total_cost) }}</span>
                  <span class="md-bar"><span :style="{ width: costShare(row.total_cost) }" /></span>
                </div>
              </td>
              <td class="whitespace-nowrap text-gray-500 dark:text-gray-400">{{ row.last_used_at ? dateTime(row.last_used_at) : '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 生成内容 -->
      <div v-else>
        <p class="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5 text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-900/30 dark:text-gray-400">
          展示视频任务和批量图片任务；点「查看」可预览视频成果并看完整提示词。对话、文本请求和单张同步出图不保存内容，无法在此查看；更早之前的视频任务可能没有提示词。
        </p>
        <div class="overflow-x-auto">
          <table class="md-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>类型</th>
                <th>模型</th>
                <th class="min-w-[18rem]">提示词</th>
                <th>规格</th>
                <th>状态</th>
                <th class="text-right">费用</th>
                <th class="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="tasksLoading && tasks.length === 0"><td colspan="8" class="md-empty">加载中...</td></tr>
              <tr v-else-if="tasks.length === 0"><td colspan="8" class="md-empty">所选时间范围内没有视频或批量图片任务</td></tr>
              <tr v-for="task in tasks" :key="`${task.kind}:${task.task_id}`" class="md-row" @click="openTask(task)">
                <td class="whitespace-nowrap tabular-nums">{{ dateTime(task.created_at) }}</td>
                <td class="whitespace-nowrap">
                  <span class="md-kind" :class="task.kind === 'video' ? 'md-kind-video' : 'md-kind-image'">
                    <Icon :name="task.kind === 'video' ? 'play' : 'grid'" size="xs" />
                    {{ task.kind === 'video' ? '视频' : '批量图片' }}
                  </span>
                </td>
                <td class="whitespace-nowrap">{{ task.model || '-' }}</td>
                <td class="max-w-md">
                  <p v-if="task.prompt" class="line-clamp-2 whitespace-pre-line break-words text-gray-800 dark:text-gray-200" :title="task.prompt">{{ task.prompt }}</p>
                  <span v-else class="text-gray-400">未保存提示词</span>
                </td>
                <td class="whitespace-nowrap text-gray-500 dark:text-gray-400">{{ taskSpec(task) }}</td>
                <td class="whitespace-nowrap"><span class="md-badge" :class="toneClass(task.status)">{{ statusLabel(task.status) }}</span></td>
                <td class="text-right font-mono tabular-nums">{{ task.cost === null ? '-' : money(task.cost) }}</td>
                <td class="whitespace-nowrap text-right">
                  <button
                    type="button"
                    class="md-view"
                    :class="{ 'md-view-play': task.kind === 'video' && task.can_play }"
                    @click.stop="openTask(task)"
                  >
                    <Icon :name="task.kind === 'video' && task.can_play ? 'play' : 'eye'" size="xs" />
                    {{ task.kind === 'video' && task.can_play ? '看视频' : '查看' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="taskTotal > 0" class="border-t border-gray-100 px-3 py-2 dark:border-dark-700">
          <Pagination :page="taskPage" :page-size="taskPageSize" :total="taskTotal" @update:page="loadTasks" @update:pageSize="changeTaskPageSize" />
        </div>
      </div>
    </section>

    <!-- 任务详情抽屉：视频成果 + 提示词 -->
    <Teleport to="body">
      <Transition name="md-drawer">
        <div v-if="selected" class="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-label="任务详情" data-testid="member-task-drawer">
          <div class="md-drawer-overlay absolute inset-0 bg-black/30" @click="closeTask" />
          <aside class="md-drawer-panel absolute right-0 top-0 flex h-full w-full max-w-[560px] flex-col bg-white shadow-2xl dark:bg-gray-800">
            <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-gray-700">
              <div class="min-w-0">
                <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ selected.kind === 'video' ? '视频任务详情' : '批量图片任务详情' }}</h2>
                <p class="mt-1 break-all font-mono text-xs text-gray-400">{{ selected.task_id }}</p>
              </div>
              <button
                type="button"
                class="shrink-0 rounded-xl border border-gray-200 p-2 text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700 dark:border-gray-600 dark:hover:bg-gray-700"
                aria-label="关闭"
                @click="closeTask"
              >
                <Icon name="x" size="sm" />
              </button>
            </header>

            <div class="flex-1 space-y-5 overflow-y-auto px-6 py-5">
              <!-- 视频成果放在最前面：管理员最关心的就是生成了什么 -->
              <div v-if="selected.kind === 'video' && selected.status === 'succeeded'" class="rounded-xl border border-gray-100 p-3 dark:border-gray-700" data-testid="member-task-result">
                <template v-if="selected.can_play">
                  <div class="overflow-hidden rounded-lg bg-black">
                    <video
                      v-if="videoUrl"
                      :src="videoUrl"
                      class="aspect-video w-full"
                      controls
                      playsinline
                      preload="metadata"
                      data-testid="member-task-video"
                      @error="onVideoError"
                    />
                    <div v-else class="flex aspect-video flex-col items-center justify-center gap-3 px-4 text-center text-sm text-gray-300">
                      <template v-if="videoState === 'error'">
                        <span>{{ videoExpired ? t('videoTasks.detail.videoExpired') : t('videoTasks.detail.videoFailed') }}</span>
                        <button
                          v-if="!videoExpired"
                          type="button"
                          class="rounded-lg border border-gray-500 px-3 py-1.5 text-xs text-gray-100 transition-colors hover:bg-white/10"
                          @click="retryVideo"
                        >{{ t('videoTasks.detail.retry') }}</button>
                      </template>
                      <span v-else class="flex items-center gap-2">
                        <Icon name="refresh" size="sm" class="animate-spin" />
                        {{ t('videoTasks.detail.loadingVideo') }}
                      </span>
                    </div>
                  </div>
                  <div class="mt-3 flex items-center justify-between">
                    <span class="text-xs text-gray-400">生成结果</span>
                    <button
                      type="button"
                      class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-emerald-600 px-4 text-xs font-semibold text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="downloading"
                      @click="downloadVideo"
                    >
                      <Icon name="download" size="xs" />
                      {{ t('videoTasks.detail.download') }}
                    </button>
                  </div>
                </template>
                <p v-else class="px-1 py-2 text-sm text-gray-500 dark:text-gray-400">{{ t('videoTasks.detail.videoUnavailable') }}</p>
              </div>

              <dl class="grid grid-cols-2 gap-x-6 gap-y-5 text-sm">
                <div>
                  <dt class="md-dt">状态</dt>
                  <dd class="mt-1"><span class="md-badge" :class="toneClass(selected.status)">{{ statusLabel(selected.status) }}</span></dd>
                </div>
                <div>
                  <dt class="md-dt">费用</dt>
                  <dd class="md-dd tabular-nums">{{ selected.cost === null ? '-' : money(selected.cost) }}</dd>
                </div>
                <div>
                  <dt class="md-dt">模型</dt>
                  <dd class="md-dd">{{ selected.model || '-' }}</dd>
                </div>
                <div>
                  <dt class="md-dt">规格</dt>
                  <dd class="md-dd">{{ taskSpec(selected) }}</dd>
                </div>
                <div>
                  <dt class="md-dt">提交时间</dt>
                  <dd class="md-dd tabular-nums">{{ dateTime(selected.created_at) }}</dd>
                </div>
                <div>
                  <dt class="md-dt">完成时间</dt>
                  <dd class="md-dd tabular-nums">{{ selected.finished_at ? dateTime(selected.finished_at) : '-' }}</dd>
                </div>
                <div>
                  <dt class="md-dt">提交成员</dt>
                  <dd class="md-dd">{{ memberLabel }}</dd>
                </div>
              </dl>

              <div class="rounded-xl border border-gray-100 bg-gray-50/70 p-4 dark:border-gray-700 dark:bg-gray-900/30">
                <p class="md-dt">{{ selected.kind === 'video' ? '提示词' : '提示词（前 3 条预览）' }}</p>
                <p class="mt-2 max-h-64 overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-100" data-testid="member-task-prompt">{{ selected.prompt || t('videoTasks.detail.noPrompt') }}</p>
              </div>

              <div v-if="selected.status === 'failed'" class="rounded-xl border border-red-100 bg-red-50/70 p-4 dark:border-red-900/40 dark:bg-red-900/10">
                <p class="text-xs text-red-500">失败原因</p>
                <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-red-700 dark:text-red-300">{{ selected.error_message || t('videoTasks.detail.errorUnknown') }}</p>
              </div>
            </div>
          </aside>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { useVideoTaskPlayer } from '@/composables/useVideoTaskPlayer'
import { organizationsAPI, type OrganizationMember, type OrganizationMemberTask, type OrganizationMemberUsageDetail } from '@/api/organizations'
import { videoDownloadName, videoTaskStatusMeta, type VideoTaskStatusTone } from '@/views/user/videoTaskFormat'

/**
 * 团队空间「成员明细」：超级管理员 / 管理员按成员查看用了哪些模型、提交了哪些视频 / 批量图片任务，
 * 并能直接预览视频成果。数据范围见后端 service/organization_member_detail.go。
 */
const props = defineProps<{
  organizationId: number
  members: OrganizationMember[]
  /** 当前选中的成员；由父级持有，「成员」页的「查看明细」会直接设置它 */
  userId: number
  /** 该标签页是否正在显示：只有显示时才请求数据 */
  active: boolean
}>()
const emit = defineEmits<{ (e: 'update:userId', value: number): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const money = (value: number) => Number(value || 0).toFixed(4)
const nf = (value: number) => Number(value || 0).toLocaleString()
const dateTime = (value: string) => new Date(value).toLocaleString()
const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const daysAgo = (days: number) => ymd(new Date(Date.now() - days * 86400000))

// ---------- 筛选 ----------

const today = ymd(new Date())
const startDate = ref(daysAgo(30))
const endDate = ref(today)
const presets = [
  { days: 7, label: '近 7 天' },
  { days: 30, label: '近 30 天' },
  { days: 90, label: '近 90 天' },
]
const activePreset = computed(() => presets.find((p) => endDate.value === today && startDate.value === daysAgo(p.days))?.days ?? 0)
function applyPreset(days: number) {
  startDate.value = daysAgo(days)
  endDate.value = today
}
const range = () => ({ start: startDate.value || undefined, end: endDate.value || undefined })

const memberLabel = computed(() => {
  const m = props.members.find((item) => item.user_id === props.userId)
  return m ? `${m.username || m.email}（${m.email}）` : '-'
})

// ---------- 数据 ----------

const usage = ref<OrganizationMemberUsageDetail | null>(null)
const loading = ref(false)
const tasks = ref<OrganizationMemberTask[]>([])
const taskTotal = ref(0)
const taskPage = ref(1)
const taskPageSize = ref(20)
const tasksLoading = ref(false)
let usageSeq = 0
let tasksSeq = 0

function reset() {
  usageSeq++
  tasksSeq++
  loading.value = false
  tasksLoading.value = false
  usage.value = null
  tasks.value = []
  taskTotal.value = 0
  taskPage.value = 1
  closeTask()
}

async function loadTasks(page = taskPage.value) {
  if (!props.organizationId || !props.userId) return
  const seq = ++tasksSeq
  const orgId = props.organizationId
  const uid = props.userId
  tasksLoading.value = true
  try {
    const data = await organizationsAPI.memberTasks(orgId, uid, range(), page, taskPageSize.value)
    if (seq !== tasksSeq) return
    tasks.value = data.items
    taskTotal.value = data.total
    taskPage.value = data.page
  } catch (error) {
    if (seq === tasksSeq) appStore.showError((error as { message?: string })?.message || '加载生成内容失败')
  } finally {
    if (seq === tasksSeq) tasksLoading.value = false
  }
}

async function reload() {
  if (!props.organizationId || !props.userId) return
  reset()
  const seq = usageSeq
  loading.value = true
  try {
    const [detail] = await Promise.all([organizationsAPI.memberUsage(props.organizationId, props.userId, range()), loadTasks(1)])
    // 期间切换了成员 / 日期 / 团队时丢弃这次结果
    if (seq === usageSeq) usage.value = detail
  } catch (error) {
    if (seq === usageSeq) appStore.showError((error as { message?: string })?.message || '加载成员明细失败')
  } finally {
    if (seq === usageSeq) loading.value = false
  }
}

function changeTaskPageSize(size: number) {
  taskPageSize.value = size
  return loadTasks(1)
}

// ---------- 展示 ----------

type ViewKey = 'usage' | 'content'
const view = ref<ViewKey>('usage')
const views = computed<Array<{ key: ViewKey; label: string; icon: 'chartBar' | 'play'; count: number }>>(() => [
  { key: 'usage', label: '模型用量', icon: 'chartBar', count: usage.value?.models.length ?? 0 },
  { key: 'content', label: '生成内容', icon: 'play', count: taskTotal.value },
])

const summaryCards = computed(() => {
  const s = usage.value?.summary
  return [
    { label: '请求次数', value: s ? nf(s.total_requests) : '-', hint: '', icon: 'bolt' as const, tone: 'md-tone-blue' },
    { label: '消耗额度', value: s ? money(s.total_cost) : '-', hint: '', icon: 'creditCard' as const, tone: 'md-tone-amber' },
    { label: '图片 / 视频', value: s ? `${nf(s.image_count)} / ${nf(s.video_count)}` : '-', hint: '生成数量', icon: 'sparkles' as const, tone: 'md-tone-violet' },
    { label: 'Token', value: s ? nf(s.input_tokens + s.output_tokens) : '-', hint: s ? `输入 ${nf(s.input_tokens)} · 输出 ${nf(s.output_tokens)}` : '', icon: 'database' as const, tone: 'md-tone-emerald' },
  ]
})

function costShare(cost: number): string {
  const total = usage.value?.summary.total_cost || 0
  if (total <= 0 || cost <= 0) return '0%'
  return `${Math.max(2, Math.min(100, (cost / total) * 100)).toFixed(1)}%`
}

const toneClasses: Record<VideoTaskStatusTone, string> = {
  success: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  danger: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300',
  warning: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
  info: 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300',
}
const toneClass = (status: string) => toneClasses[videoTaskStatusMeta(status).tone]
const statusLabel = (status: string) => t(`videoTasks.status.${videoTaskStatusMeta(status).labelKey}`)

function taskSpec(task: OrganizationMemberTask): string {
  if (task.kind === 'image_batch') return `${task.item_count} 张${task.success_count > 0 ? `（成功 ${task.success_count}）` : ''}`
  return [task.resolution, task.duration_seconds > 0 ? `${task.duration_seconds} 秒` : '', task.item_count > 1 ? `${task.item_count} 条` : ''].filter(Boolean).join(' · ') || '-'
}

// ---------- 详情抽屉 + 视频播放 ----------

const selected = ref<OrganizationMemberTask | null>(null)
const downloading = ref(false)
const player = useVideoTaskPlayer((id, options) => organizationsAPI.memberTaskContent(props.organizationId, props.userId, id, options))
const { videoUrl, state: videoState, expired: videoExpired } = player

function openTask(task: OrganizationMemberTask) {
  player.reset()
  selected.value = task
  if (task.kind === 'video' && task.status === 'succeeded' && task.can_play) void player.load(task.task_id)
}

function closeTask() {
  player.reset()
  selected.value = null
}

function retryVideo() {
  player.retry()
}

function onVideoError() {
  player.onVideoError()
}

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
    saveBlob(await player.getBlob(task.task_id), videoDownloadName({ request_id: task.task_id, id: task.task_id }))
  } catch {
    appStore.showError(t('videoTasks.detail.downloadFailed'))
  } finally {
    downloading.value = false
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && selected.value) closeTask()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

// 显示时 / 换成员 / 改日期 / 成员列表刷新（含切换团队）都重新加载；选中的成员不在当前团队时自动选第一个。
// 放在最后：immediate 会在 setup 阶段同步执行，reload 里会用到上面声明的播放器等状态。
watch(
  () => [props.active, props.organizationId, props.userId, startDate.value, endDate.value, props.members] as const,
  () => {
    if (!props.active) return
    if (!props.members.some((m) => m.user_id === props.userId)) {
      const first = props.members[0]?.user_id || 0
      if (first !== props.userId) emit('update:userId', first)
      return
    }
    void reload()
  },
  { immediate: true }
)
</script>

<style scoped>
.md-card { @apply rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800; }
.md-label { @apply block text-xs font-medium text-gray-500 dark:text-gray-400; }
.md-input { @apply h-10 rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-500/15 dark:border-dark-600 dark:bg-dark-800 dark:text-white; }
.md-chip { @apply h-8 rounded-full border border-gray-200 bg-white px-3 text-xs font-medium text-gray-600 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300; }
.md-chip-active { @apply border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-950/30 dark:text-primary-300; }
.md-refresh { @apply inline-flex h-10 items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-700 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:bg-dark-700; }

.md-kpi-icon { @apply inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg; }
.md-tone-blue { @apply bg-sky-50 text-sky-600 dark:bg-sky-900/20 dark:text-sky-300; }
.md-tone-amber { @apply bg-amber-50 text-amber-600 dark:bg-amber-900/20 dark:text-amber-300; }
.md-tone-violet { @apply bg-violet-50 text-violet-600 dark:bg-violet-900/20 dark:text-violet-300; }
.md-tone-emerald { @apply bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300; }

.md-seg { @apply -mb-px inline-flex items-center gap-2 border-b-2 border-transparent px-4 py-3 text-sm font-medium text-gray-500 transition hover:text-gray-900 dark:text-gray-400 dark:hover:text-white; }
.md-seg-active { @apply border-primary-500 text-primary-600 dark:text-primary-400; }
.md-seg-count { @apply rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold tabular-nums text-gray-600 dark:bg-dark-700 dark:text-gray-300; }
.md-seg-active .md-seg-count { @apply bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-300; }

.md-table { @apply w-full text-left text-sm; }
.md-table th { @apply whitespace-nowrap border-b border-gray-100 bg-gray-50/60 px-4 py-3 text-xs font-medium uppercase tracking-wide text-gray-500 dark:border-dark-700 dark:bg-dark-900/30 dark:text-gray-400; }
.md-table td { @apply border-b border-gray-50 px-4 py-3.5 align-middle text-gray-700 dark:border-dark-700/60 dark:text-gray-300; }
.md-table tbody tr:last-child td { @apply border-b-0; }
.md-row { @apply cursor-pointer transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-700/40; }
.md-empty { @apply py-14 text-center text-gray-400; }

.md-bar { @apply block h-1.5 w-full min-w-[4rem] overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700; }
.md-bar > span { @apply block h-full rounded-full bg-primary-500; }

.md-kind { @apply inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium; }
.md-kind-video { @apply bg-indigo-50 text-indigo-700 dark:bg-indigo-900/20 dark:text-indigo-300; }
.md-kind-image { @apply bg-pink-50 text-pink-700 dark:bg-pink-900/20 dark:text-pink-300; }
.md-badge { @apply inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold; }
.md-view { @apply inline-flex h-8 items-center gap-1.5 rounded-lg border border-gray-200 px-3 text-xs font-medium text-gray-600 transition hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700; }
.md-view-play { @apply border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300; }

.md-dt { @apply text-xs text-gray-400; }
.md-dd { @apply mt-1 break-words font-medium text-gray-900 dark:text-white; }

.md-drawer-enter-active,
.md-drawer-leave-active { transition: opacity 0.2s ease; }
.md-drawer-enter-active .md-drawer-panel,
.md-drawer-leave-active .md-drawer-panel { transition: transform 0.25s ease; }
.md-drawer-enter-from,
.md-drawer-leave-to { opacity: 0; }
.md-drawer-enter-from .md-drawer-panel,
.md-drawer-leave-to .md-drawer-panel { transform: translateX(100%); }
</style>
