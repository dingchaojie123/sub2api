<template>
  <AppLayout>
    <div class="space-y-6">
      <section
        class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
      >
        <div
          class="flex flex-col gap-4 border-b border-gray-200 px-5 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700"
        >
          <div>
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">模型列表</p>
            <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
              {{ t('modelList.title') }}
            </h1>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
              {{ t('modelList.description') }}
            </p>
          </div>
          <button
            type="button"
            class="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 transition-colors hover:border-gray-300 hover:bg-gray-50 hover:text-gray-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
            :title="t('modelList.refresh')"
            data-testid="model-list-refresh"
            @click="reloadPricingGroups"
          >
            <Icon name="refresh" size="md" />
          </button>
        </div>

        <div class="grid min-h-[520px] grid-cols-1 lg:grid-cols-[260px_minmax(0,1fr)]">
          <aside
            class="border-b border-gray-200 bg-gray-50/70 p-4 dark:border-gray-700 dark:bg-gray-900/30 lg:border-b-0 lg:border-r"
          >
            <div class="flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible lg:pb-0">
              <button
                v-for="group in pricingGroups"
                :key="group.id"
                type="button"
                class="min-w-[180px] rounded-lg border px-4 py-3 text-left transition-colors lg:min-w-0"
                :class="group.id === selectedGroup.id
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-900 shadow-sm dark:border-emerald-700/70 dark:bg-emerald-900/30 dark:text-emerald-100'
                  : 'border-transparent bg-white text-gray-700 hover:border-gray-200 hover:bg-gray-50 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-gray-700 dark:hover:bg-gray-700/70'"
                :data-testid="`model-group-${group.name}`"
                @click="selectGroup(group.id)"
              >
                <span class="block truncate text-sm font-semibold">{{ group.name }}</span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                  {{ group.provider }} · {{ t('modelList.groupCount', { count: group.rows.length }) }}
                </span>
              </button>
            </div>
          </aside>

          <section class="min-w-0 bg-white p-5 dark:bg-gray-800">
            <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ selectedGroup.name }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ selectedGroup.provider }} 分组模型价格
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span
                  v-if="selectedGroup.multiplier"
                  class="inline-flex items-center rounded-full bg-indigo-50 px-3 py-1 text-sm font-semibold text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200"
                >
                  {{ selectedGroup.multiplier }}
                </span>
                <span
                  class="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-300"
                >
                  {{ t('modelList.groupCount', { count: selectedGroup.rows.length }) }}
                </span>
              </div>
            </div>

            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
              <table class="min-w-[820px] w-full border-separate border-spacing-0 text-sm">
                <thead>
                  <tr class="bg-gray-50 text-left text-xs font-semibold uppercase text-gray-500 dark:bg-gray-900/50 dark:text-gray-400">
                    <th class="px-4 py-3">{{ t('modelList.columns.model') }}</th>
                    <th class="px-4 py-3">{{ t('modelList.columns.platformInput') }}</th>
                    <th class="px-4 py-3">{{ t('modelList.columns.platformOutput') }}</th>
                    <th class="px-4 py-3">{{ t('modelList.columns.officialInput') }}</th>
                    <th class="px-4 py-3">{{ t('modelList.columns.officialOutput') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in selectedGroup.rows"
                    :key="row.model"
                    class="border-t border-gray-100 odd:bg-white even:bg-gray-50/60 dark:border-gray-700 dark:odd:bg-gray-800 dark:even:bg-gray-900/30"
                  >
                    <td class="whitespace-nowrap px-4 py-3 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {{ row.model }}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 font-semibold text-emerald-600 dark:text-emerald-300">
                      {{ row.platformInput }}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 font-semibold text-rose-600 dark:text-rose-300">
                      {{ row.platformOutput }}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 font-medium text-amber-600 dark:text-amber-300">
                      {{ row.officialInput }}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 font-medium text-amber-600 dark:text-amber-300">
                      {{ row.officialOutput }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ModelPricingPageGroup as ServerModelPricingPageGroup } from '@/types'

interface ModelPriceRow {
  model: string
  platformInput: string
  platformOutput: string
  officialInput: string
  officialOutput: string
}

interface ModelPriceGroup {
  id: string
  name: string
  provider: 'openai' | 'anthropic'
  multiplier?: string
  rows: ModelPriceRow[]
}

const { t } = useI18n()
const appStore = useAppStore()

const defaultPricingGroups: ModelPriceGroup[] = [
  {
    id: 'gpt-pro',
    name: 'gpt pro',
    provider: 'openai',
    rows: [
      { model: 'codex-auto-review', platformInput: '¥1.30 / 1M', platformOutput: '¥7.80 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.4', platformInput: '¥0.65 / 1M', platformOutput: '¥3.90 / 1M', officialInput: '$2.50 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'gpt-5.4-mini', platformInput: '¥0.195 / 1M', platformOutput: '¥1.17 / 1M', officialInput: '$0.75 / 1M', officialOutput: '$4.50 / 1M' },
      { model: 'gpt-5.5', platformInput: '¥1.30 / 1M', platformOutput: '¥7.80 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.6-luna', platformInput: '¥0.26 / 1M', platformOutput: '¥1.56 / 1M', officialInput: '$1.00 / 1M', officialOutput: '$6.00 / 1M' },
      { model: 'gpt-5.6-sol', platformInput: '¥1.30 / 1M', platformOutput: '¥7.80 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.6-terra', platformInput: '¥0.65 / 1M', platformOutput: '¥3.90 / 1M', officialInput: '$2.50 / 1M', officialOutput: '$15.00 / 1M' },
    ],
  },
  {
    id: 'gpt-plus',
    name: 'gpt plus',
    provider: 'openai',
    rows: [
      { model: 'codex-auto-review', platformInput: '¥0.6000 / 1M', platformOutput: '¥3.60 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.4', platformInput: '¥0.3000 / 1M', platformOutput: '¥1.80 / 1M', officialInput: '$2.50 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'gpt-5.4-mini', platformInput: '¥0.0900 / 1M', platformOutput: '¥0.5400 / 1M', officialInput: '$0.7500 / 1M', officialOutput: '$4.50 / 1M' },
      { model: 'gpt-5.5', platformInput: '¥0.6000 / 1M', platformOutput: '¥3.60 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.6-luna', platformInput: '¥0.1200 / 1M', platformOutput: '¥0.7200 / 1M', officialInput: '$1.00 / 1M', officialOutput: '$6.00 / 1M' },
      { model: 'gpt-5.6-sol', platformInput: '¥0.6000 / 1M', platformOutput: '¥3.60 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$30.00 / 1M' },
      { model: 'gpt-5.6-terra', platformInput: '¥0.3000 / 1M', platformOutput: '¥1.80 / 1M', officialInput: '$2.50 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'gpt-image-2', platformInput: '¥0.6000 / 1M', platformOutput: '¥1.20 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$10.00 / 1M' },
    ],
  },
  {
    id: 'gpt-image-2',
    name: 'gpt-image-2生图',
    provider: 'openai',
    multiplier: '2x',
    rows: [
      { model: 'gpt-image-2', platformInput: '¥10.00 / 1M', platformOutput: '¥20.00 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$10.00 / 1M' },
    ],
  },
  {
    id: 'claude-kiro',
    name: 'Claude-kiro',
    provider: 'anthropic',
    multiplier: '0.3x',
    rows: [
      { model: 'claude-fable-5', platformInput: '¥3.00 / 1M', platformOutput: '¥15.00 / 1M', officialInput: '$10.00 / 1M', officialOutput: '$50.00 / 1M' },
      { model: 'claude-haiku-4-5-20251001', platformInput: '¥0.3000 / 1M', platformOutput: '¥1.50 / 1M', officialInput: '$1.00 / 1M', officialOutput: '$5.00 / 1M' },
      { model: 'claude-haiku-4.5', platformInput: '¥0.0750 / 1M', platformOutput: '¥0.3750 / 1M', officialInput: '$0.2500 / 1M', officialOutput: '$1.25 / 1M' },
      { model: 'claude-opus-4-5-20251101', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-6', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-7', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-8', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4.5', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4.6', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4.7', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4.8', platformInput: '¥1.50 / 1M', platformOutput: '¥7.50 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-sonnet-4-5-20250929', platformInput: '¥0.9000 / 1M', platformOutput: '¥4.50 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-4-6', platformInput: '¥0.9000 / 1M', platformOutput: '¥4.50 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-4-8', platformInput: '¥0.9000 / 1M', platformOutput: '¥4.50 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-4.5', platformInput: '¥0.9000 / 1M', platformOutput: '¥4.50 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-4.6', platformInput: '¥0.9000 / 1M', platformOutput: '¥4.50 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-5', platformInput: '¥0.6000 / 1M', platformOutput: '¥3.00 / 1M', officialInput: '$2.00 / 1M', officialOutput: '$10.00 / 1M' },
    ],
  },
  {
    id: 'claude-max-1-1',
    name: 'Claude-max-1.1',
    provider: 'anthropic',
    multiplier: '2.2x',
    rows: [
      { model: 'claude-fable-5', platformInput: '¥22.00 / 1M', platformOutput: '¥110.00 / 1M', officialInput: '$10.00 / 1M', officialOutput: '$50.00 / 1M' },
      { model: 'claude-haiku-4-5-20251001', platformInput: '¥2.20 / 1M', platformOutput: '¥11.00 / 1M', officialInput: '$1.00 / 1M', officialOutput: '$5.00 / 1M' },
      { model: 'claude-opus-4-5-20251101', platformInput: '¥11.00 / 1M', platformOutput: '¥55.00 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-6', platformInput: '¥11.00 / 1M', platformOutput: '¥55.00 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-7', platformInput: '¥11.00 / 1M', platformOutput: '¥55.00 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-opus-4-8', platformInput: '¥11.00 / 1M', platformOutput: '¥55.00 / 1M', officialInput: '$5.00 / 1M', officialOutput: '$25.00 / 1M' },
      { model: 'claude-sonnet-4-5-20250929', platformInput: '¥6.60 / 1M', platformOutput: '¥33.00 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-4-6', platformInput: '¥6.60 / 1M', platformOutput: '¥33.00 / 1M', officialInput: '$3.00 / 1M', officialOutput: '$15.00 / 1M' },
      { model: 'claude-sonnet-5', platformInput: '¥4.40 / 1M', platformOutput: '¥22.00 / 1M', officialInput: '$2.00 / 1M', officialOutput: '$10.00 / 1M' },
    ],
  },
]

const pricingGroups = computed(() => {
  return normalizePricingGroups(appStore.cachedPublicSettings?.model_pricing_page_data) ?? defaultPricingGroups
})

const selectedGroupId = ref(defaultPricingGroups[0].id)

const selectedGroup = computed(() => {
  return pricingGroups.value.find((group) => group.id === selectedGroupId.value) ?? pricingGroups.value[0]
})

function selectGroup(groupId: string) {
  selectedGroupId.value = groupId
}

function resetSelection() {
  selectedGroupId.value = pricingGroups.value[0].id
}

async function reloadPricingGroups() {
  await appStore.fetchPublicSettings(true)
  resetSelection()
}

function normalizePricingGroups(input: unknown): ModelPriceGroup[] | null {
  if (!Array.isArray(input) || input.length === 0) return null

  const groups = input
    .map((group) => normalizePricingGroup(group))
    .filter((group): group is ModelPriceGroup => group !== null)

  return groups.length > 0 ? groups : null
}

function normalizePricingGroup(input: unknown): ModelPriceGroup | null {
  const group = input as Partial<ServerModelPricingPageGroup> | null
  if (!group || typeof group !== 'object') return null
  if (!group.id?.trim() || !group.name?.trim() || !Array.isArray(group.rows) || group.rows.length === 0) return null

  const rows = group.rows
    .map((row) => {
      if (!row || typeof row !== 'object') return null
      if (!row.model?.trim()) return null
      return {
        model: row.model,
        platformInput: row.platform_input || '-',
        platformOutput: row.platform_output || '-',
        officialInput: row.official_input || '-',
        officialOutput: row.official_output || '-',
      }
    })
    .filter((row): row is ModelPriceRow => row !== null)

  if (rows.length === 0) return null

  return {
    id: group.id,
    name: group.name,
    provider: group.provider === 'anthropic' ? 'anthropic' : 'openai',
    multiplier: group.multiplier?.trim() || undefined,
    rows,
  }
}
</script>
