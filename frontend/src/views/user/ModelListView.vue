<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="border-b border-gray-200 pb-5 dark:border-gray-700">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
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
              class="h-10 w-full rounded-md border border-gray-300 bg-white pl-10 pr-3 text-sm text-gray-900 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-white"
              :placeholder="t('modelList.searchPlaceholder')"
              data-testid="model-pricing-search"
            >
          </label>
        </div>
      </header>

      <nav
        class="flex gap-1 overflow-x-auto border-b border-gray-200 pb-px dark:border-gray-700"
        :aria-label="t('modelList.categories')"
      >
        <button
          v-for="category in modelPricingCatalog"
          :key="category.id"
          type="button"
          class="relative inline-flex h-11 shrink-0 items-center gap-2 px-4 text-sm font-semibold transition-colors"
          :class="category.id === activeCategoryId
            ? activeTabClass(category.id)
            : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
          :data-testid="`pricing-category-${category.id}`"
          @click="selectCategory(category.id)"
        >
          <Icon :name="categoryIcon(category.id)" size="sm" />
          <span>{{ category.title }}</span>
          <span class="text-xs font-medium opacity-70">{{ category.rows.length }}</span>
        </button>
      </nav>

      <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
        <div class="flex flex-col gap-3 border-b border-gray-200 px-5 py-4 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700">
          <div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">
              {{ activeCategory.title }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ activeCategory.description }}
            </p>
          </div>
          <div class="flex items-center gap-3 text-sm">
            <span class="font-semibold text-gray-900 dark:text-white">{{ activeCategory.unit }}</span>
            <span class="h-4 w-px bg-gray-200 dark:bg-gray-600" />
            <span class="text-gray-500 dark:text-gray-400">
              {{ t('modelList.matchCount', { count: filteredRows.length }) }}
            </span>
          </div>
        </div>

        <div v-if="filteredRows.length" class="overflow-x-auto">
          <table
            class="w-full min-w-[760px] table-fixed border-collapse text-sm"
            :class="{ 'min-w-[980px]': activeCategory.id === 'subsidized-video' }"
          >
            <thead>
              <tr class="bg-gray-50 text-left text-xs font-semibold text-gray-500 dark:bg-gray-900/50 dark:text-gray-400">
                <th class="sticky left-0 z-10 w-[210px] min-w-[210px] border-b border-gray-200 bg-gray-50 px-5 py-3 sm:w-[320px] sm:min-w-[320px] dark:border-gray-700 dark:bg-gray-900">
                  {{ t('modelList.columns.model') }}
                </th>
                <th
                  v-for="column in activeCategory.columns"
                  :key="column.key"
                  class="whitespace-nowrap border-b border-gray-200 px-4 py-3 dark:border-gray-700"
                >
                  {{ column.label }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in filteredRows"
                :key="`${row.model}-${row.values.resolution ?? ''}`"
                class="group border-b border-gray-100 last:border-b-0 hover:bg-gray-50/80 dark:border-gray-700 dark:hover:bg-gray-700/30"
              >
                <td class="sticky left-0 z-[5] break-words bg-white px-5 py-4 font-medium leading-5 text-gray-900 group-hover:bg-gray-50 dark:bg-gray-800 dark:text-gray-100 dark:group-hover:bg-gray-700">
                  {{ row.model }}
                </td>
                <td
                  v-for="column in activeCategory.columns"
                  :key="column.key"
                  class="whitespace-nowrap px-4 py-4 tabular-nums"
                  :class="cellClass(row.values[column.key], column.price)"
                >
                  {{ row.values[column.key] }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="flex min-h-56 flex-col items-center justify-center px-6 py-12 text-center">
          <Icon name="search" size="lg" class="text-gray-300 dark:text-gray-600" />
          <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-200">
            {{ t('modelList.emptyTitle') }}
          </p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('modelList.emptyDescription') }}
          </p>
        </div>

        <footer class="border-t border-gray-200 bg-gray-50 px-5 py-3 text-xs leading-5 text-gray-500 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-400">
          {{ t('modelList.availabilityNote') }}
        </footer>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { modelPricingCatalog, type ModelPricingCategoryId } from './modelPricingCatalog'

const { t } = useI18n()
const activeCategoryId = ref<ModelPricingCategoryId>('video')
const searchQuery = ref('')

const activeCategory = computed(() => {
  return modelPricingCatalog.find((category) => category.id === activeCategoryId.value) ?? modelPricingCatalog[0]
})

const filteredRows = computed(() => {
  const keyword = searchQuery.value.trim().toLocaleLowerCase()
  if (!keyword) return activeCategory.value.rows

  return activeCategory.value.rows.filter((row) => {
    const values = Object.values(row.values).join(' ')
    return `${row.model} ${row.searchTerms ?? ''} ${values}`.toLocaleLowerCase().includes(keyword)
  })
})

function selectCategory(categoryId: ModelPricingCategoryId) {
  activeCategoryId.value = categoryId
}

function categoryIcon(categoryId: ModelPricingCategoryId): 'play' | 'bolt' | 'sparkles' | 'chat' {
  if (categoryId === 'subsidized-video') return 'bolt'
  if (categoryId === 'image') return 'sparkles'
  if (categoryId === 'text') return 'chat'
  return 'play'
}

function activeTabClass(categoryId: ModelPricingCategoryId) {
  const classes: Record<ModelPricingCategoryId, string> = {
    video: 'text-emerald-700 after:absolute after:inset-x-2 after:bottom-[-1px] after:h-0.5 after:bg-emerald-500 dark:text-emerald-300',
    'subsidized-video': 'text-amber-700 after:absolute after:inset-x-2 after:bottom-[-1px] after:h-0.5 after:bg-amber-500 dark:text-amber-300',
    image: 'text-rose-700 after:absolute after:inset-x-2 after:bottom-[-1px] after:h-0.5 after:bg-rose-500 dark:text-rose-300',
    text: 'text-sky-700 after:absolute after:inset-x-2 after:bottom-[-1px] after:h-0.5 after:bg-sky-500 dark:text-sky-300',
  }
  return classes[categoryId]
}

function cellClass(value: string, isPrice = false) {
  if (value === '-') return 'text-gray-300 dark:text-gray-600'
  if (isPrice) return 'font-semibold text-gray-900 dark:text-white'
  return 'text-gray-600 dark:text-gray-300'
}
</script>
