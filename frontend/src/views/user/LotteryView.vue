<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="grid gap-4 md:grid-cols-3">
        <div class="card p-5">
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-emerald-100 dark:bg-emerald-900/30">
              <Icon name="gift" size="md" class="text-emerald-600 dark:text-emerald-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lottery.availableChances') }}</p>
              <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ status?.available_chances ?? 0 }}</p>
            </div>
          </div>
        </div>

        <div class="card p-5">
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-100 dark:bg-blue-900/30">
              <Icon name="clock" size="md" class="text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lottery.todayUsed') }}</p>
              <p class="text-2xl font-bold text-gray-900 dark:text-white">
                {{ t('lottery.dailyUsage', { used: status?.daily_used ?? 0, limit: status?.daily_limit ?? 3 }) }}
              </p>
            </div>
          </div>
        </div>

        <div class="card p-5">
          <div class="flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center rounded-xl bg-amber-100 dark:bg-amber-900/30">
              <Icon name="sparkles" size="md" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lottery.dailyLimit') }}</p>
              <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ status?.daily_limit ?? 3 }}</p>
            </div>
          </div>
        </div>
      </div>

      <div v-if="errorMessage" class="card border-red-200 bg-red-50 p-5 dark:border-red-800/50 dark:bg-red-900/20">
        <div class="flex items-start gap-3">
          <Icon name="exclamationCircle" size="md" class="mt-0.5 text-red-600 dark:text-red-400" />
          <div>
            <p class="text-sm font-semibold text-red-800 dark:text-red-300">{{ t('lottery.loadFailed') }}</p>
            <p class="mt-1 text-sm text-red-700 dark:text-red-400">{{ errorMessage }}</p>
          </div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_380px]">
        <section class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('lottery.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('lottery.description') }}</p>
          </div>

          <div class="flex flex-col items-center gap-6 p-6">
            <div class="wheel-wrap">
              <div class="wheel-pointer" aria-hidden="true"></div>
              <div class="wheel" :style="wheelStyle">
                <div
                  v-for="segment in prizeSegments"
                  :key="segment.key"
                  class="wheel-label"
                  :style="{ transform: `rotate(${segment.labelAngle}deg) translateY(calc(-1 * var(--label-radius))) rotate(${-segment.labelAngle}deg)` }"
                >
                  <span>{{ t(segment.labelKey) }}</span>
                  <strong>${{ segment.amount }}</strong>
                </div>
              </div>
              <div class="wheel-center">
                <Icon name="gift" size="lg" class="text-primary-600 dark:text-primary-300" />
              </div>
            </div>

            <button
              type="button"
              data-testid="lottery-draw-button"
              :disabled="drawDisabled"
              class="btn btn-primary min-h-[44px] min-w-[180px] px-6"
              @click="handleDraw"
            >
              <svg v-if="drawing" class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <Icon v-else name="play" size="md" class="mr-2" />
              {{ drawing ? t('lottery.drawing') : drawButtonText }}
            </button>
          </div>
        </section>

        <aside class="space-y-6">
          <section class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lottery.rulesTitle') }}</h2>
            </div>
            <div class="overflow-hidden">
              <table class="w-full text-sm">
                <thead class="bg-gray-50 text-left text-xs font-semibold uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                  <tr>
                    <th class="px-5 py-3">{{ t('lottery.prize') }}</th>
                    <th class="px-5 py-3 text-right">{{ t('lottery.probability') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="segment in prizeSegments" :key="segment.key">
                    <td class="px-5 py-3 text-gray-900 dark:text-white">
                      {{ t(segment.labelKey) }} ${{ segment.amount }}
                    </td>
                    <td class="px-5 py-3 text-right text-gray-500 dark:text-dark-400">{{ segment.odds }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lottery.recordsTitle') }}</h2>
            </div>

            <div v-if="loadingRecords" class="flex justify-center py-8">
              <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
            </div>

            <div v-else-if="records.length" class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="record in records" :key="record.id" class="px-5 py-4">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                      {{ record.prize_name }} · ${{ record.value }}
                    </p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(record.created_at) }}</p>
                    <p class="mt-2 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ record.code }}</p>
                  </div>
                  <button
                    type="button"
                    class="btn btn-secondary min-h-[36px] px-3 text-xs"
                    :title="t('lottery.copyCode')"
                    @click="copyCode(record)"
                  >
                    <Icon :name="copiedRecordId === record.id ? 'check' : 'copy'" size="sm" class="mr-1.5" />
                    {{ copiedRecordId === record.id ? t('lottery.copied') : t('lottery.copy') }}
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="empty-state py-8">
              <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800">
                <Icon name="clock" size="lg" class="text-gray-400 dark:text-dark-500" />
              </div>
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('lottery.noRecords') }}</p>
            </div>
          </section>
        </aside>
      </div>
    </div>

    <transition name="fade">
      <div v-if="drawResult" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeResult">
        <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-900">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-2xl bg-amber-100 dark:bg-amber-900/30">
              <Icon name="sparkles" size="lg" class="text-amber-600 dark:text-amber-400" />
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('lottery.winTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ drawResult.prize_name }} · ${{ drawResult.value }}
              </p>
            </div>
          </div>

          <div class="mt-5 rounded-lg bg-gray-50 p-4 dark:bg-dark-800">
            <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ t('lottery.redeemCode') }}</p>
            <p class="mt-2 break-all font-mono text-lg font-semibold text-gray-900 dark:text-white">{{ drawResult.code }}</p>
          </div>

          <div class="mt-6 flex flex-col gap-3 sm:flex-row">
            <button type="button" class="btn btn-primary flex-1" @click="copyCode(drawResult)">
              <Icon :name="copiedRecordId === drawResult.id ? 'check' : 'copy'" size="md" class="mr-2" />
              {{ copiedRecordId === drawResult.id ? t('lottery.copied') : t('lottery.copyCode') }}
            </button>
            <button type="button" class="btn btn-secondary flex-1" @click="closeResult">
              {{ t('common.close') }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { lotteryAPI } from '@/api'
import type { LotteryDrawRecord, LotteryStatus } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()

const prizeSegments = [
  { key: 'first', labelKey: 'lottery.firstPrize', amount: 300, odds: '1%', labelAngle: 45 },
  { key: 'second', labelKey: 'lottery.secondPrize', amount: 100, odds: '5%', labelAngle: 135 },
  { key: 'third', labelKey: 'lottery.thirdPrize', amount: 50, odds: '20%', labelAngle: 225 },
  { key: 'fourth', labelKey: 'lottery.fourthPrize', amount: 10, odds: '74%', labelAngle: 315 },
] as const

const status = ref<LotteryStatus | null>(null)
const records = ref<LotteryDrawRecord[]>([])
const drawResult = ref<LotteryDrawRecord | null>(null)
const copiedRecordId = ref<number | null>(null)
const loadingStatus = ref(false)
const loadingRecords = ref(false)
const drawing = ref(false)
const errorMessage = ref('')
const wheelRotation = ref(0)

const hasReachedDailyLimit = computed(() => {
  if (!status.value) return false
  return status.value.daily_limit > 0 && status.value.daily_used >= status.value.daily_limit
})

const drawDisabled = computed(() => {
  return loadingStatus.value || drawing.value || !status.value || status.value.available_chances <= 0 || hasReachedDailyLimit.value
})

const drawButtonText = computed(() => {
  if (!status.value || loadingStatus.value) return t('lottery.loading')
  if (status.value.available_chances <= 0) return t('lottery.noChancesButton')
  if (hasReachedDailyLimit.value) return t('lottery.dailyLimitReached')
  return t('lottery.drawButton')
})

const wheelStyle = computed(() => ({
  transform: `rotate(${wheelRotation.value}deg)`,
}))

function getErrorMessage(error: unknown, fallback: string) {
  const candidate = error as {
    message?: string
    response?: { data?: { detail?: string; message?: string } }
  }
  return candidate.response?.data?.detail || candidate.response?.data?.message || candidate.message || fallback
}

function getPrizeIndex(record: LotteryDrawRecord) {
  const byValue = prizeSegments.findIndex((segment) => segment.amount === record.value)
  if (byValue >= 0) return byValue

  const byName = prizeSegments.findIndex((segment) => record.prize_name.includes(String(segment.amount)))
  return byName >= 0 ? byName : prizeSegments.length - 1
}

async function loadStatus() {
  loadingStatus.value = true
  try {
    status.value = await lotteryAPI.getStatus()
  } catch (error) {
    errorMessage.value = getErrorMessage(error, t('lottery.statusFailed'))
  } finally {
    loadingStatus.value = false
  }
}

async function loadRecords() {
  loadingRecords.value = true
  try {
    records.value = await lotteryAPI.getRecords()
  } catch (error) {
    errorMessage.value = getErrorMessage(error, t('lottery.recordsFailed'))
  } finally {
    loadingRecords.value = false
  }
}

async function handleDraw() {
  if (drawDisabled.value) return

  drawing.value = true
  errorMessage.value = ''
  drawResult.value = null

  try {
    const result = await lotteryAPI.draw()
    const prizeIndex = getPrizeIndex(result)
    const segmentCenter = prizeIndex * 90 + 45
    const currentTurns = Math.ceil(wheelRotation.value / 360) * 360
    wheelRotation.value = currentTurns + 1440 + (360 - segmentCenter)

    await new Promise((resolve) => window.setTimeout(resolve, 900))
    drawResult.value = result
    await Promise.all([loadStatus(), loadRecords()])
  } catch (error) {
    errorMessage.value = getErrorMessage(error, t('lottery.drawFailed'))
  } finally {
    drawing.value = false
  }
}

async function copyCode(record: LotteryDrawRecord) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(record.code)
    } else if (!fallbackCopy(record.code)) {
      throw new Error(t('lottery.copyFailed'))
    }
    copiedRecordId.value = record.id
    window.setTimeout(() => {
      if (copiedRecordId.value === record.id) {
        copiedRecordId.value = null
      }
    }, 1600)
  } catch (error) {
    errorMessage.value = getErrorMessage(error, t('lottery.copyFailed'))
  }
}

function fallbackCopy(text: string) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;pointer-events:none'
  document.body.appendChild(textarea)
  textarea.focus({ preventScroll: true })
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

function closeResult() {
  drawResult.value = null
}

onMounted(() => {
  loadStatus()
  loadRecords()
})
</script>

<style scoped>
.wheel-wrap {
  position: relative;
  width: min(76vw, 340px);
  aspect-ratio: 1;
  --label-radius: min(28vw, 126px);
}

.wheel {
  position: absolute;
  inset: 0;
  border-radius: 9999px;
  border: 10px solid rgb(255 255 255 / 0.9);
  background:
    conic-gradient(
      from -45deg,
      #f59e0b 0deg 90deg,
      #10b981 90deg 180deg,
      #3b82f6 180deg 270deg,
      #ef4444 270deg 360deg
    );
  box-shadow: 0 24px 60px rgb(15 23 42 / 0.18);
  transition: transform 0.9s cubic-bezier(0.18, 0.84, 0.28, 1);
}

.wheel::after {
  content: '';
  position: absolute;
  inset: 14px;
  border: 1px solid rgb(255 255 255 / 0.35);
  border-radius: inherit;
}

.wheel-label {
  position: absolute;
  left: calc(50% - 54px);
  top: calc(50% - 30px);
  display: flex;
  width: 108px;
  min-height: 60px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: white;
  text-align: center;
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.22);
}

.wheel-label span {
  font-size: 0.75rem;
  font-weight: 700;
}

.wheel-label strong {
  font-size: 1.25rem;
  line-height: 1.5rem;
}

.wheel-center {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 2;
  display: flex;
  height: 76px;
  width: 76px;
  transform: translate(-50%, -50%);
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: white;
  box-shadow: 0 10px 30px rgb(15 23 42 / 0.22);
}

.wheel-pointer {
  position: absolute;
  left: 50%;
  top: -8px;
  z-index: 3;
  height: 0;
  width: 0;
  transform: translateX(-50%);
  border-left: 18px solid transparent;
  border-right: 18px solid transparent;
  border-top: 34px solid #111827;
  filter: drop-shadow(0 5px 8px rgb(15 23 42 / 0.25));
}

:global(.dark) .wheel {
  border-color: rgb(30 41 59 / 0.95);
}

:global(.dark) .wheel-center {
  background: #1e293b;
}

:global(.dark) .wheel-pointer {
  border-top-color: #f8fafc;
}

.fade-enter-active,
.fade-leave-active {
  transition: all 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
