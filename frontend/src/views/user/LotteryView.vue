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
            <p class="text-sm font-semibold text-red-800 dark:text-red-300">{{ t('lottery.noticeTitle') }}</p>
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

          <div class="blind-box-panel p-5 sm:p-7">
            <div
              data-testid="lottery-blind-box"
              class="blind-box-stage"
              :class="{ 'is-opening': drawing }"
            >
              <div class="blind-box-atmosphere" aria-hidden="true"></div>
              <div class="blind-box-display">
                <div class="blind-box-shadow" aria-hidden="true"></div>
                <div class="blind-box-light" aria-hidden="true"></div>
                <div v-if="showConfetti" data-testid="lottery-confetti" class="blind-box-confetti" aria-hidden="true">
                  <span
                    v-for="piece in confettiPieces"
                    :key="piece.key"
                    :class="piece.className"
                    :style="piece.style"
                  ></span>
                </div>
                <div class="blind-box">
                  <div class="blind-box-lid">
                    <div class="blind-box-lid-top"></div>
                    <div class="blind-box-lid-front">
                      <span class="blind-box-seal">?</span>
                    </div>
                  </div>
                  <div class="blind-box-body">
                    <div class="blind-box-face">
                      <Icon name="gift" size="lg" class="blind-box-gift" />
                      <span class="blind-box-wordmark">LUCKY DROP</span>
                      <strong>?</strong>
                      <span class="blind-box-face-caption">{{ t('lottery.blindBoxFace') }}</span>
                    </div>
                    <div class="blind-box-ribbon blind-box-ribbon-vertical"></div>
                    <div class="blind-box-ribbon blind-box-ribbon-horizontal"></div>
                  </div>
                </div>
              </div>
              <p class="blind-box-caption">{{ t('lottery.blindBoxSubtitle') }}</p>

              <div class="blind-box-meta">
                <div>
                  <span>{{ t('lottery.availableChances') }}</span>
                  <strong>{{ status?.available_chances ?? 0 }}</strong>
                </div>
                <div>
                  <span>{{ t('lottery.todayUsed') }}</span>
                  <strong>{{ t('lottery.dailyUsage', { used: status?.daily_used ?? 0, limit: status?.daily_limit ?? 3 }) }}</strong>
                </div>
              </div>

              <button
                type="button"
                data-testid="lottery-draw-button"
                :disabled="drawDisabled"
                class="btn btn-primary blind-box-button min-h-[46px] w-full px-6"
                @click="handleDraw"
              >
                <svg v-if="drawing" class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <Icon v-else name="gift" size="md" class="mr-2" />
                {{ drawing ? t('lottery.drawing') : drawButtonText }}
              </button>

              <div class="blind-box-rewards" aria-hidden="true">
                <span v-for="segment in prizeSegments" :key="segment.key" :class="segment.chipClass">
                  {{ t(segment.labelKey) }} · ${{ segment.amount }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <aside class="space-y-6">
          <section class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('lottery.rulesTitle') }}</h2>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div
                v-for="segment in prizeSegments"
                :key="segment.key"
                class="flex items-center justify-between gap-4 px-5 py-4"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <span class="prize-marker" :class="segment.markerClass" aria-hidden="true">
                    <Icon name="sparkles" size="sm" />
                  </span>
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ t(segment.labelKey) }}</p>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('lottery.prizeCodeReward') }}</p>
                  </div>
                </div>
                <div class="shrink-0 rounded-md bg-gray-50 px-3 py-1.5 text-sm font-bold text-gray-900 dark:bg-dark-800 dark:text-white">
                  ${{ segment.amount }}
                </div>
              </div>
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
  {
    key: 'first',
    labelKey: 'lottery.firstPrize',
    amount: 30,
    chipClass: 'reward-gold',
    markerClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
  },
  {
    key: 'second',
    labelKey: 'lottery.secondPrize',
    amount: 10,
    chipClass: 'reward-teal',
    markerClass: 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
  },
  {
    key: 'third',
    labelKey: 'lottery.thirdPrize',
    amount: 5,
    chipClass: 'reward-sky',
    markerClass: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
  },
  {
    key: 'fourth',
    labelKey: 'lottery.fourthPrize',
    amount: 2,
    chipClass: 'reward-rose',
    markerClass: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
  },
] as const

const confettiPieces = [
  { key: 'gold-left', className: 'confetti-gold confetti-wide', style: '--x: -112px; --y: -168px; --r: -42deg; --d: 0ms' },
  { key: 'teal-left', className: 'confetti-teal', style: '--x: -72px; --y: -206px; --r: 36deg; --d: 45ms' },
  { key: 'sky-left', className: 'confetti-sky confetti-slim', style: '--x: -138px; --y: -112px; --r: 72deg; --d: 80ms' },
  { key: 'rose-left', className: 'confetti-rose', style: '--x: -36px; --y: -176px; --r: -86deg; --d: 120ms' },
  { key: 'gold-center', className: 'confetti-gold', style: '--x: -14px; --y: -226px; --r: 14deg; --d: 20ms' },
  { key: 'teal-center', className: 'confetti-teal confetti-wide', style: '--x: 22px; --y: -196px; --r: -22deg; --d: 95ms' },
  { key: 'sky-center', className: 'confetti-sky', style: '--x: 58px; --y: -226px; --r: 64deg; --d: 55ms' },
  { key: 'rose-right', className: 'confetti-rose confetti-slim', style: '--x: 116px; --y: -152px; --r: -52deg; --d: 125ms' },
  { key: 'gold-right', className: 'confetti-gold confetti-wide', style: '--x: 142px; --y: -104px; --r: 88deg; --d: 70ms' },
  { key: 'teal-right', className: 'confetti-teal', style: '--x: 86px; --y: -188px; --r: -118deg; --d: 150ms' },
  { key: 'sky-far', className: 'confetti-sky confetti-wide', style: '--x: -168px; --y: -72px; --r: -18deg; --d: 165ms' },
  { key: 'rose-far', className: 'confetti-rose', style: '--x: 170px; --y: -78px; --r: 32deg; --d: 180ms' },
] as const

const status = ref<LotteryStatus | null>(null)
const records = ref<LotteryDrawRecord[]>([])
const drawResult = ref<LotteryDrawRecord | null>(null)
const copiedRecordId = ref<number | null>(null)
const loadingStatus = ref(false)
const loadingRecords = ref(false)
const drawing = ref(false)
const showConfetti = ref(false)
const errorMessage = ref('')

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

function getErrorMessage(error: unknown, fallback: string) {
  const candidate = error as {
    message?: string
    response?: { data?: { detail?: string; message?: string } }
  }
  return candidate.response?.data?.detail || candidate.response?.data?.message || candidate.message || fallback
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
  showConfetti.value = false
  errorMessage.value = ''
  drawResult.value = null

  try {
    const result = await lotteryAPI.draw()
    showConfetti.value = true

    await new Promise((resolve) => window.setTimeout(resolve, 950))
    drawResult.value = result
    showConfetti.value = false
    await Promise.all([loadStatus(), loadRecords()])
  } catch (error) {
    showConfetti.value = false
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
.blind-box-panel {
  background:
    radial-gradient(circle at 50% 26%, rgb(45 212 191 / 0.2), transparent 32%),
    linear-gradient(135deg, rgb(248 250 252 / 0.97), rgb(236 253 245 / 0.68) 50%, rgb(255 251 235 / 0.58)),
    repeating-linear-gradient(135deg, rgb(15 23 42 / 0.03) 0 1px, transparent 1px 18px);
}

.blind-box-stage {
  position: relative;
  margin: 0 auto;
  width: min(100%, 560px);
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.74);
  padding: 26px;
  box-shadow:
    0 22px 58px rgb(15 23 42 / 0.12),
    inset 0 1px 0 rgb(255 255 255 / 0.88);
}

.blind-box-stage::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    linear-gradient(90deg, transparent 0 50%, rgb(15 23 42 / 0.035) 50% 50.4%, transparent 50.4%),
    linear-gradient(0deg, transparent 0 58%, rgb(15 23 42 / 0.03) 58% 58.4%, transparent 58.4%);
  pointer-events: none;
}

.blind-box-atmosphere {
  position: absolute;
  inset: 12px;
  border-radius: 8px;
  background:
    radial-gradient(circle at 50% 30%, rgb(250 204 21 / 0.18), transparent 34%),
    radial-gradient(circle at 32% 70%, rgb(14 165 233 / 0.12), transparent 30%),
    radial-gradient(circle at 72% 76%, rgb(244 63 94 / 0.1), transparent 28%);
  filter: blur(12px);
  opacity: 0.9;
  pointer-events: none;
}

.blind-box-display {
  position: relative;
  height: 300px;
  margin: 0 auto;
  display: grid;
  place-items: end center;
  perspective: 900px;
}

.blind-box-shadow {
  position: absolute;
  bottom: 22px;
  width: 260px;
  height: 44px;
  border-radius: 9999px;
  background: rgb(15 23 42 / 0.14);
  filter: blur(13px);
  transform: scaleX(1);
  transition:
    transform 0.65s cubic-bezier(0.2, 0.8, 0.2, 1),
    opacity 0.65s ease;
}

.blind-box-light {
  position: absolute;
  bottom: 90px;
  width: 230px;
  height: 160px;
  transform: scaleY(0.55);
  transform-origin: bottom;
  background: radial-gradient(ellipse at center, rgb(250 204 21 / 0.42), rgb(45 212 191 / 0.18) 42%, transparent 70%);
  filter: blur(6px);
  opacity: 0;
  transition:
    opacity 0.5s ease,
    transform 0.72s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.blind-box-confetti {
  position: absolute;
  left: 50%;
  bottom: 116px;
  z-index: 5;
  height: 1px;
  width: 1px;
  pointer-events: none;
}

.blind-box-confetti span {
  position: absolute;
  left: 0;
  top: 0;
  height: 12px;
  width: 7px;
  border-radius: 2px;
  opacity: 0;
  transform-origin: center;
  animation: confetti-pop 1.05s cubic-bezier(0.16, 0.82, 0.28, 1) var(--d) both;
  box-shadow: 0 6px 10px rgb(15 23 42 / 0.12);
}

.blind-box-confetti .confetti-wide {
  height: 9px;
  width: 15px;
}

.blind-box-confetti .confetti-slim {
  height: 16px;
  width: 5px;
}

.confetti-gold {
  background: linear-gradient(135deg, #fde68a, #f59e0b);
}

.confetti-teal {
  background: linear-gradient(135deg, #99f6e4, #14b8a6);
}

.confetti-sky {
  background: linear-gradient(135deg, #bae6fd, #38bdf8);
}

.confetti-rose {
  background: linear-gradient(135deg, #fecdd3, #f43f5e);
}

.blind-box {
  position: relative;
  width: min(68vw, 236px);
  height: 238px;
  transform-style: preserve-3d;
  animation: blind-box-idle 3.8s ease-in-out infinite;
}

.blind-box-body {
  position: absolute;
  left: 18px;
  right: 18px;
  bottom: 0;
  height: 170px;
  border-radius: 8px;
  background:
    linear-gradient(135deg, #14b8a6, #0f766e 56%, #115e59),
    linear-gradient(90deg, rgb(255 255 255 / 0.28), transparent 38%);
  box-shadow:
    0 24px 44px rgb(15 23 42 / 0.22),
    inset 0 1px 0 rgb(255 255 255 / 0.36),
    inset -26px 0 36px rgb(15 23 42 / 0.13);
}

.blind-box-body::before,
.blind-box-body::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  width: 34px;
  background: rgb(15 23 42 / 0.08);
}

.blind-box-body::before {
  left: 0;
  border-radius: 8px 0 0 8px;
}

.blind-box-body::after {
  right: 0;
  border-radius: 0 8px 8px 0;
  background: rgb(255 255 255 / 0.13);
}

.blind-box-face {
  position: absolute;
  inset: 22px 24px 20px;
  z-index: 2;
  display: grid;
  place-items: center;
  border: 1px solid rgb(255 255 255 / 0.38);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.16);
  color: white;
  text-align: center;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.28);
  backdrop-filter: blur(4px);
}

.blind-box-gift {
  color: rgb(255 255 255 / 0.92);
}

.blind-box-wordmark {
  color: rgb(255 255 255 / 0.72);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.12em;
}

.blind-box-face strong {
  font-size: 3rem;
  line-height: 1;
  text-shadow: 0 8px 20px rgb(15 23 42 / 0.22);
}

.blind-box-face-caption {
  max-width: 120px;
  overflow: hidden;
  color: rgb(255 255 255 / 0.82);
  font-size: 0.78rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.blind-box-ribbon {
  position: absolute;
  z-index: 1;
  background:
    linear-gradient(135deg, #fbbf24, #f59e0b),
    linear-gradient(90deg, rgb(255 255 255 / 0.36), transparent);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.4);
}

.blind-box-ribbon-vertical {
  top: 0;
  bottom: 0;
  left: calc(50% - 12px);
  width: 24px;
}

.blind-box-ribbon-horizontal {
  left: 0;
  right: 0;
  top: 54px;
  height: 24px;
}

.blind-box-lid {
  position: absolute;
  left: 6px;
  right: 6px;
  bottom: 146px;
  z-index: 4;
  height: 78px;
  transform-origin: 50% 78%;
  transition:
    transform 0.78s cubic-bezier(0.2, 0.82, 0.24, 1),
    filter 0.78s ease;
}

.blind-box-lid-top {
  position: absolute;
  left: 10px;
  right: 10px;
  top: 0;
  height: 28px;
  transform: skewX(-14deg);
  border-radius: 8px 8px 4px 4px;
  background: linear-gradient(135deg, #2dd4bf, #0d9488);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.32);
}

.blind-box-lid-front {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 58px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background:
    linear-gradient(135deg, #0f766e, #134e4a),
    linear-gradient(90deg, rgb(255 255 255 / 0.18), transparent);
  box-shadow:
    0 16px 28px rgb(15 23 42 / 0.18),
    inset 0 1px 0 rgb(255 255 255 / 0.28);
}

.blind-box-seal {
  display: inline-grid;
  height: 38px;
  width: 38px;
  place-items: center;
  border: 2px solid rgb(255 255 255 / 0.62);
  border-radius: 9999px;
  color: white;
  font-size: 1.4rem;
  font-weight: 900;
  background: rgb(255 255 255 / 0.14);
}

.blind-box-caption {
  position: relative;
  margin-top: 10px;
  color: rgb(71 85 105);
  text-align: center;
  font-size: 0.95rem;
  font-weight: 600;
}

.blind-box-meta {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 18px;
}

.blind-box-meta div {
  display: flex;
  min-width: 156px;
  min-height: 56px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.76);
  padding: 12px 14px;
}

.blind-box-meta span {
  min-width: 0;
  color: rgb(100 116 139);
  font-size: 0.8rem;
  font-weight: 600;
}

.blind-box-meta strong {
  color: rgb(15 23 42);
  font-size: 1.2rem;
  font-weight: 800;
  white-space: nowrap;
}

.blind-box-button {
  position: relative;
  margin-top: 18px;
  box-shadow: 0 14px 28px rgb(13 148 136 / 0.22);
}

.blind-box-rewards {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  margin-top: 16px;
}

.blind-box-rewards span {
  border: 1px solid rgb(226 232 240 / 0.82);
  border-radius: 9999px;
  padding: 6px 10px;
  color: rgb(71 85 105);
  font-size: 0.72rem;
  font-weight: 700;
  background: rgb(255 255 255 / 0.72);
}

.reward-gold {
  box-shadow: inset 0 0 0 999px rgb(251 191 36 / 0.1);
}

.reward-teal {
  box-shadow: inset 0 0 0 999px rgb(20 184 166 / 0.1);
}

.reward-sky {
  box-shadow: inset 0 0 0 999px rgb(56 189 248 / 0.1);
}

.reward-rose {
  box-shadow: inset 0 0 0 999px rgb(244 63 94 / 0.1);
}

.prize-marker {
  display: inline-flex;
  height: 36px;
  width: 36px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
}

.blind-box-stage.is-opening .blind-box {
  animation: blind-box-open-float 0.95s cubic-bezier(0.2, 0.82, 0.24, 1);
}

.blind-box-stage.is-opening .blind-box-lid {
  transform: translateY(-34px) rotateX(-20deg) rotateZ(-4deg);
  filter: drop-shadow(0 18px 20px rgb(15 23 42 / 0.14));
}

.blind-box-stage.is-opening .blind-box-light {
  opacity: 1;
  transform: scaleY(1.08);
}

.blind-box-stage.is-opening .blind-box-shadow {
  opacity: 0.72;
  transform: scaleX(0.86);
}

.blind-box-stage.is-opening .blind-box-seal {
  animation: blind-box-seal-glow 0.95s ease;
}

.blind-box-stage.is-opening .blind-box-atmosphere {
  animation: blind-box-pulse 0.95s ease;
}

:global(.dark) .blind-box-panel {
  background:
    radial-gradient(circle at 50% 26%, rgb(45 212 191 / 0.16), transparent 32%),
    linear-gradient(135deg, rgb(15 23 42 / 0.98), rgb(6 78 59 / 0.56) 50%, rgb(69 26 3 / 0.42)),
    repeating-linear-gradient(135deg, rgb(226 232 240 / 0.045) 0 1px, transparent 1px 18px);
}

:global(.dark) .blind-box-stage {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.66);
  box-shadow:
    0 22px 58px rgb(0 0 0 / 0.3),
    inset 0 1px 0 rgb(148 163 184 / 0.16);
}

:global(.dark) .blind-box-caption,
:global(.dark) .blind-box-meta span,
:global(.dark) .blind-box-rewards span {
  color: rgb(203 213 225);
}

:global(.dark) .blind-box-meta div,
:global(.dark) .blind-box-rewards span {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42 / 0.64);
}

:global(.dark) .blind-box-meta strong {
  color: rgb(248 250 252);
}

:global(.dark) .blind-box-body {
  background:
    linear-gradient(135deg, #0f766e, #115e59 56%, #134e4a),
    linear-gradient(90deg, rgb(255 255 255 / 0.18), transparent 38%);
}

@keyframes blind-box-idle {
  0%,
  100% {
    transform: translateY(0) rotateZ(0deg);
  }

  50% {
    transform: translateY(-5px) rotateZ(-0.6deg);
  }
}

@keyframes blind-box-open-float {
  0% {
    transform: translateY(0) rotateZ(0deg);
  }

  44% {
    transform: translateY(-10px) rotateZ(1.4deg);
  }

  100% {
    transform: translateY(0) rotateZ(0deg);
  }
}

@keyframes blind-box-seal-glow {
  0%,
  100% {
    box-shadow: none;
  }

  45% {
    box-shadow:
      0 0 0 8px rgb(255 255 255 / 0.12),
      0 0 24px rgb(250 204 21 / 0.8);
  }
}

@keyframes blind-box-pulse {
  0%,
  100% {
    opacity: 0.9;
    transform: scale(1);
  }

  45% {
    opacity: 1;
    transform: scale(1.04);
  }
}

@keyframes confetti-pop {
  0% {
    opacity: 0;
    transform: translate(-50%, 0) scale(0.68) rotate(0deg);
  }

  16% {
    opacity: 1;
  }

  52% {
    opacity: 1;
    transform: translate(calc(-50% + var(--x) * 0.78), calc(var(--y) * 0.86)) scale(1) rotate(calc(var(--r) * 0.55));
  }

  100% {
    opacity: 0;
    transform: translate(calc(-50% + var(--x)), calc(var(--y) + 46px)) scale(0.92) rotate(var(--r));
  }
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
