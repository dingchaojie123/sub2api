<template>
  <div
    ref="trigger"
    class="inline-block cursor-help rounded px-1 py-0.5 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700"
    data-testid="group-accounts-trigger"
    @mouseenter="open"
    @mouseleave="scheduleClose"
    @focusin="open"
    @focusout="scheduleClose"
  >
    <slot />

    <!-- Teleported to the body: the table and its scroll container would clip the panel. -->
    <Teleport to="body">
      <div
        v-if="visible"
        ref="panel"
        role="tooltip"
        data-testid="group-accounts-panel"
        class="fixed z-[99999] w-80 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-xl dark:border-dark-600 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @mouseenter="cancelClose"
        @mouseleave="scheduleClose"
      >
        <div class="mb-2 flex items-center justify-between font-medium text-gray-900 dark:text-white">
          <span>{{ t('admin.groups.accountsHoverTitle') }}</span>
          <span class="text-gray-400">{{ total }}</span>
        </div>

        <div v-if="loading" class="py-3 text-center text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.accountsHoverLoading') }}
        </div>
        <div v-else-if="failed" class="py-3 text-center text-red-600 dark:text-red-400">
          {{ t('admin.groups.accountsHoverFailed') }}
        </div>
        <div v-else-if="accounts.length === 0" class="py-3 text-center text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.accountsHoverEmpty') }}
        </div>
        <ul v-else class="max-h-72 space-y-1 overflow-y-auto">
          <li
            v-for="account in accounts"
            :key="account.id"
            class="flex items-center gap-2 rounded px-1.5 py-1 hover:bg-gray-50 dark:hover:bg-dark-700"
            data-testid="group-accounts-item"
          >
            <span
              class="h-2 w-2 flex-shrink-0 rounded-full"
              :class="statusDot(account)"
              :title="statusText(account)"
            ></span>
            <PlatformIcon :platform="account.platform" size="sm" />
            <span class="min-w-0 flex-1 truncate text-gray-800 dark:text-gray-200" :title="account.name">
              {{ account.name }}
            </span>
            <span class="flex-shrink-0 text-gray-400">#{{ account.id }}</span>
            <span
              v-if="account.status === 'active' && !account.schedulable"
              class="flex-shrink-0 rounded bg-amber-50 px-1 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400"
            >
              {{ t('admin.groups.accountsHoverUnschedulable') }}
            </span>
          </li>
        </ul>

        <div v-if="!loading && !failed && total > accounts.length" class="mt-2 text-center text-gray-400">
          {{ t('admin.groups.accountsHoverMore', { shown: accounts.length, total }) }}
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

const props = defineProps<{
  groupId: number
  /** Number of accounts of the group, as the table shows it. */
  total: number
}>()

const { t } = useI18n()

// A group's accounts are listed at most this many; the table's total says how many there are.
const PAGE_SIZE = 50
// Moving the mouse from the cell to the panel crosses a gap; the panel closes only after this delay.
const CLOSE_DELAY_MS = 150
// The same group is hovered again and again while scanning the table; remember it briefly.
const CACHE_TTL_MS = 15_000

const cache = new Map<number, { at: number; accounts: Account[] }>()

const trigger = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const visible = ref(false)
const loading = ref(false)
const failed = ref(false)
const accounts = ref<Account[]>([])
const position = ref({ top: 0, left: 0 })

let closeTimer: ReturnType<typeof setTimeout> | undefined
// Only the latest request may write the result (the mouse can move to another group's row).
let requestSeq = 0

const cancelClose = () => {
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = undefined
  }
}

const scheduleClose = () => {
  cancelClose()
  closeTimer = setTimeout(() => {
    visible.value = false
  }, CLOSE_DELAY_MS)
}

// Below the cell, aligned to its left edge; above it when there is no room below; never outside
// the viewport.
const updatePosition = () => {
  const el = trigger.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const height = panel.value?.offsetHeight ?? 0
  const width = panel.value?.offsetWidth ?? 320
  let top = rect.bottom + 6
  if (height > 0 && top + height > window.innerHeight - 8) {
    top = Math.max(8, rect.top - height - 6)
  }
  const left = Math.max(8, Math.min(rect.left, window.innerWidth - width - 8))
  position.value = { top, left }
}

const load = async () => {
  const cached = cache.get(props.groupId)
  if (cached && Date.now() - cached.at < CACHE_TTL_MS) {
    accounts.value = cached.accounts
    failed.value = false
    loading.value = false
    return
  }
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  try {
    const result = await adminAPI.accounts.list(1, PAGE_SIZE, { group: String(props.groupId) })
    if (seq !== requestSeq) return
    accounts.value = result.items || []
    cache.set(props.groupId, { at: Date.now(), accounts: accounts.value })
  } catch (error) {
    if (seq !== requestSeq) return
    console.error('Failed to load group accounts:', error)
    accounts.value = []
    failed.value = true
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

const open = async () => {
  cancelClose()
  visible.value = true
  await nextTick()
  updatePosition()
  await load()
  // The panel changed size when the list arrived.
  await nextTick()
  updatePosition()
}

const statusDot = (account: Account) => {
  if (account.status === 'error') return 'bg-red-500'
  if (account.status !== 'active') return 'bg-gray-400'
  return account.schedulable ? 'bg-emerald-500' : 'bg-amber-500'
}

const statusText = (account: Account) => t('admin.accounts.status.' + account.status)

onBeforeUnmount(cancelClose)
</script>
