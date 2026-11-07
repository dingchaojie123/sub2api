<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p class="text-sm font-medium text-emerald-600 dark:text-emerald-400">
            {{ t('modelList.eyebrow') }}
          </p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('modelList.title') }}
          </h1>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('modelList.description') }}
          </p>
        </div>

        <label class="relative block w-full lg:w-80">
          <span class="sr-only">{{ t('modelList.search') }}</span>
          <Icon
            name="search"
            size="sm"
            class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
          />
          <input
            v-model="searchQuery"
            type="search"
            class="h-10 w-full rounded-xl border border-gray-200 bg-white pl-10 pr-3 text-sm text-gray-900 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
            :placeholder="t('modelList.searchPlaceholder')"
            data-testid="model-pricing-search"
          >
        </label>
      </header>

      <div class="grid gap-6 lg:grid-cols-[260px_minmax(0,1fr)]">
        <!-- 筛选栏 -->
        <aside
          class="self-start rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800 lg:sticky lg:top-4"
          data-testid="model-filters"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-bold text-gray-900 dark:text-white">
                {{ t('modelList.filters.title') }}
              </h2>
              <p class="mt-1 text-xs text-gray-400" data-testid="model-filter-summary">
                {{ t('modelList.filters.showing', { shown: filteredItems.length, total: modelSquareCatalog.length }) }}
              </p>
            </div>
            <button
              v-if="hasActiveFilters"
              type="button"
              class="shrink-0 text-xs font-medium text-emerald-600 hover:text-emerald-700 dark:text-emerald-400"
              data-testid="model-filter-clear"
              @click="clearFilters"
            >
              {{ t('modelList.filters.clear') }}
            </button>
          </div>

          <div
            v-for="group in filterGroups"
            :key="group.key"
            class="mt-5 border-t border-gray-100 pt-4 dark:border-gray-700"
          >
            <h3 class="text-xs font-semibold tracking-wide text-gray-500 dark:text-gray-400">
              {{ group.title }}
            </h3>
            <ul class="mt-3 space-y-1">
              <li>
                <button
                  type="button"
                  class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors"
                  :class="selected[group.key].length === 0
                    ? 'bg-emerald-50 font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                    : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50'"
                  :data-testid="`filter-${group.key}-all`"
                  @click="selected[group.key] = []"
                >
                  <span>{{ t('modelList.filters.all') }}</span>
                  <span class="text-xs opacity-70">{{ modelSquareCatalog.length }}</span>
                </button>
              </li>
              <li v-for="option in group.options" :key="option.value">
                <label
                  class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50"
                >
                  <input
                    v-model="selected[group.key]"
                    type="checkbox"
                    :value="option.value"
                    class="h-4 w-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500"
                    :data-testid="`filter-${group.key}-${option.value}`"
                  >
                  <span class="min-w-0 flex-1 truncate">{{ option.label }}</span>
                  <span class="text-xs text-gray-400">{{ option.count }}</span>
                </label>
              </li>
            </ul>
          </div>
        </aside>

        <!-- 卡片列表 -->
        <section class="min-w-0">
          <div v-if="filteredItems.length" class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
            <article
              v-for="item in filteredItems"
              :key="item.id"
              class="flex flex-col rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md dark:border-gray-700 dark:bg-gray-800"
              data-testid="model-card"
            >
              <div class="flex items-start gap-3">
                <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
                  <ModelIcon :model="item.iconModel" size="24px" />
                </div>
                <h3 class="min-w-0 flex-1 break-words text-base font-bold leading-6 text-gray-900 dark:text-white">
                  {{ item.name }}
                </h3>
                <span
                  class="shrink-0 rounded-md border px-2 py-0.5 text-xs font-semibold"
                  :class="channelBadgeClass(item.channel)"
                >
                  {{ item.channel }}
                </span>
              </div>

              <p class="mt-4 line-clamp-3 min-h-[4.5rem] text-sm leading-6 text-gray-500 dark:text-gray-400">
                {{ item.description }}
              </p>

              <div class="mt-3">
                <span class="rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                  {{ categoryLabels[item.category] }}
                </span>
              </div>

              <dl class="mt-4 grid grid-cols-2 gap-4 border-t border-gray-100 pt-4 text-sm dark:border-gray-700">
                <div>
                  <dt class="text-xs text-gray-400">{{ t('modelList.card.channel') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ item.channel }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-400">{{ t('modelList.card.billing') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ item.billing }}</dd>
                </div>
              </dl>

              <ul class="mt-4 space-y-2 border-t border-gray-100 pt-4 dark:border-gray-700">
                <li
                  v-for="tier in item.prices.slice(0, CARD_PRICE_LIMIT)"
                  :key="tier.label"
                  class="flex items-baseline justify-between gap-3 text-sm"
                >
                  <span class="text-gray-400">{{ tier.label }}</span>
                  <span class="text-right">
                    <span class="font-bold tabular-nums text-gray-900 dark:text-white">{{ tier.amount }}</span>
                    <span v-if="tier.suffix" class="ml-1 text-xs text-gray-400">{{ tier.suffix }}</span>
                  </span>
                </li>
                <li
                  v-if="item.prices.length > CARD_PRICE_LIMIT"
                  class="text-right text-xs text-gray-400"
                >
                  {{ t('modelList.card.morePrices', { count: item.prices.length - CARD_PRICE_LIMIT }) }}
                </li>
              </ul>

              <div class="mt-auto flex justify-end pt-5">
                <button
                  type="button"
                  class="inline-flex h-10 items-center justify-center gap-1.5 rounded-lg bg-emerald-600 px-5 text-sm font-semibold text-white transition-colors hover:bg-emerald-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/40"
                  data-testid="model-detail-button"
                  @click="activeItem = item"
                >
                  {{ t('modelList.card.detail') }}
                  <Icon name="chevronRight" size="sm" />
                </button>
              </div>

            </article>
          </div>

          <div
            v-else
            class="flex min-h-56 flex-col items-center justify-center rounded-2xl border border-gray-200 bg-white px-6 py-12 text-center dark:border-gray-700 dark:bg-gray-800"
          >
            <Icon name="search" size="lg" class="text-gray-300 dark:text-gray-600" />
            <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('modelList.emptyTitle') }}
            </p>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('modelList.emptyDescription') }}
            </p>
          </div>

          <p class="mt-5 text-xs leading-5 text-gray-400">
            {{ t('modelList.availabilityNote') }}
          </p>
        </section>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <BaseDialog
      :show="activeItem !== null"
      :title="activeItem?.name ?? ''"
      width="wide"
      @close="activeItem = null"
    >
      <div v-if="activeItem" class="space-y-6" data-testid="model-detail">
        <div class="flex items-center gap-3">
          <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
            <ModelIcon :model="activeItem.iconModel" size="28px" />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="rounded-md border px-2 py-0.5 text-xs font-semibold"
              :class="channelBadgeClass(activeItem.channel)"
            >
              {{ activeItem.channel }}
            </span>
            <span class="rounded-md bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-700 dark:text-gray-300">
              {{ categoryLabels[activeItem.category] }}
            </span>
          </div>
        </div>

        <p class="rounded-xl border border-gray-100 bg-gray-50/60 p-4 text-sm leading-7 text-gray-700 dark:border-gray-700 dark:bg-gray-900/30 dark:text-gray-200">
          {{ activeItem.description }}
        </p>

        <section>
          <h4 class="text-base font-bold text-gray-900 dark:text-white">
            {{ t('modelList.detail.prices') }}
          </h4>
          <p class="mt-1 text-xs text-gray-400">{{ t('modelList.detail.pricesHint') }}</p>
          <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
            <div
              v-for="tier in activeItem.prices"
              :key="tier.label"
              class="rounded-xl border border-gray-100 p-4 dark:border-gray-700"
              data-testid="model-detail-price"
            >
              <p class="text-xs text-gray-400">{{ tier.label }}</p>
              <p class="mt-1.5">
                <span class="text-lg font-bold tabular-nums text-gray-900 dark:text-white">{{ tier.amount }}</span>
                <span v-if="tier.suffix" class="ml-1 text-xs text-gray-400">{{ tier.suffix }}</span>
              </p>
            </div>
          </div>
        </section>

        <dl class="divide-y divide-gray-100 border-y border-gray-100 text-sm dark:divide-gray-700 dark:border-gray-700">
          <div
            v-for="row in detailRows"
            :key="row.label"
            class="flex items-start justify-between gap-6 py-3.5"
          >
            <dt class="shrink-0 text-gray-500 dark:text-gray-400">{{ row.label }}</dt>
            <dd class="text-right font-semibold text-gray-900 dark:text-white">{{ row.value }}</dd>
          </div>
        </dl>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import {
  billingOrder,
  categoryLabels,
  channelOrder,
  modelSquareCatalog,
  type ModelCategoryId,
  type ModelSquareItem,
} from './modelPricingCatalog'

type FilterKey = 'category' | 'channel' | 'billing'

const CARD_PRICE_LIMIT = 3

const { t } = useI18n()
const searchQuery = ref('')
const activeItem = ref<ModelSquareItem | null>(null)
const selected = reactive<Record<FilterKey, string[]>>({
  category: [],
  channel: [],
  billing: [],
})

function itemValue(item: ModelSquareItem, key: FilterKey): string {
  return item[key]
}

function buildOptions(key: FilterKey, order: string[], labelOf: (value: string) => string) {
  return order
    .map((value) => ({
      value,
      label: labelOf(value),
      count: modelSquareCatalog.filter((item) => itemValue(item, key) === value).length,
    }))
    .filter((option) => option.count > 0)
}

const filterGroups = computed(() => [
  {
    key: 'category' as const,
    title: t('modelList.filters.category'),
    options: buildOptions('category', Object.keys(categoryLabels), (value) => categoryLabels[value as ModelCategoryId]),
  },
  {
    key: 'channel' as const,
    title: t('modelList.filters.channel'),
    options: buildOptions('channel', channelOrder, (value) => value),
  },
  {
    key: 'billing' as const,
    title: t('modelList.filters.billing'),
    options: buildOptions('billing', billingOrder, (value) => value),
  },
])

const hasActiveFilters = computed(() => Object.values(selected).some((values) => values.length > 0))

const filteredItems = computed(() => {
  const keyword = searchQuery.value.trim().toLocaleLowerCase()

  return modelSquareCatalog.filter((item) => {
    const matchesFilters = (Object.keys(selected) as FilterKey[]).every(
      (key) => selected[key].length === 0 || selected[key].includes(itemValue(item, key)),
    )
    if (!matchesFilters) return false
    if (!keyword) return true

    const haystack = [
      item.name,
      item.description,
      item.channel,
      item.billing,
      categoryLabels[item.category],
      ...item.prices.map((tier) => `${tier.label} ${tier.amount} ${tier.suffix}`),
      ...item.specs.map((row) => row.value),
    ]
    return haystack.join(' ').toLocaleLowerCase().includes(keyword)
  })
})

const detailRows = computed(() => {
  const item = activeItem.value
  if (!item) return []
  return [
    ...item.specs.map((row) => ({ label: row.label, value: row.value })),
    { label: t('modelList.card.channel'), value: item.channel },
    { label: t('modelList.card.billing'), value: item.billing },
  ]
})

function clearFilters() {
  selected.category = []
  selected.channel = []
  selected.billing = []
}

function channelBadgeClass(channel: string) {
  const classes: Record<string, string> = {
    官方: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300',
    官key: 'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900/20 dark:text-teal-300',
    官转: 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-900/20 dark:text-violet-300',
    特价: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300',
    常规: 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300',
  }
  return classes[channel] ?? 'border-gray-200 bg-gray-50 text-gray-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300'
}
</script>
