<template>
  <AppLayout>
    <div class="space-y-5">
      <header>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('apiDocs.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('apiDocs.description') }}</p>
        <p v-if="locale !== 'zh'" class="mt-1 text-xs text-gray-400 dark:text-gray-500" data-testid="api-docs-language-note">
          {{ t('apiDocs.languageNote') }}
        </p>
      </header>

      <div class="grid gap-6 lg:grid-cols-[250px_minmax(0,1fr)]">
        <aside class="space-y-4 lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:self-start lg:overflow-y-auto">
          <!-- 小屏：下拉切换 -->
          <select
            class="h-11 w-full rounded-xl border border-gray-200 bg-white px-3 text-sm text-gray-900 outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-white lg:hidden"
            :value="activeSlug"
            :aria-label="t('apiDocs.docs')"
            data-testid="api-docs-select"
            @change="goTo(($event.target as HTMLSelectElement).value)"
          >
            <optgroup v-for="group in groups" :key="group.id" :label="group.label">
              <option v-for="item in group.pages" :key="item.slug" :value="item.slug">{{ item.title }}</option>
            </optgroup>
          </select>

          <!-- 大屏：文档列表 -->
          <nav
            class="hidden rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-gray-700 dark:bg-gray-800 lg:block"
            :aria-label="t('apiDocs.docs')"
            data-testid="api-docs-nav"
          >
            <div v-for="group in groups" :key="group.id" class="mb-3 last:mb-0">
              <p class="px-2 pb-1 text-xs font-semibold text-gray-400">{{ group.label }}</p>
              <RouterLink
                v-for="item in group.pages"
                :key="item.slug"
                :to="apiDocRoute(item.slug)"
                class="block rounded-lg px-3 py-2 text-sm transition-colors"
                :class="item.slug === activeSlug
                  ? 'bg-emerald-50 font-semibold text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                  : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-700/50'"
                :data-testid="`api-docs-nav-${item.slug}`"
              >
                {{ item.title }}
              </RouterLink>
            </div>
          </nav>

          <!-- 本页目录 -->
          <nav
            v-if="rendered.headings.length"
            class="hidden rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-gray-700 dark:bg-gray-800 lg:block"
            :aria-label="t('apiDocs.toc')"
            data-testid="api-docs-toc"
          >
            <p class="px-2 pb-1 text-xs font-semibold text-gray-400">{{ t('apiDocs.toc') }}</p>
            <button
              v-for="heading in rendered.headings"
              :key="heading.id"
              type="button"
              class="block w-full truncate rounded-md px-2 py-1.5 text-left text-sm text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700/50 dark:hover:text-white"
              :class="heading.level === 3 ? 'pl-5 text-xs' : ''"
              :title="heading.text"
              @click="scrollToHeading(heading.id)"
            >
              {{ heading.text }}
            </button>
          </nav>
        </aside>

        <main ref="articleRef" class="min-w-0 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800 sm:p-8">
          <article
            v-if="page"
            class="api-doc-content"
            data-testid="api-docs-content"
            @click="onContentClick"
            v-html="rendered.html"
          />
          <div v-else class="py-10 text-center" data-testid="api-docs-not-found">
            <p class="text-base font-semibold text-gray-900 dark:text-white">{{ t('apiDocs.notFoundTitle') }}</p>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('apiDocs.notFoundDescription') }}</p>
            <RouterLink
              :to="apiDocRoute(API_DOC_DEFAULT_SLUG)"
              class="mt-4 inline-flex h-10 items-center rounded-xl bg-emerald-600 px-5 text-sm font-semibold text-white transition-colors hover:bg-emerald-700"
            >
              {{ t('apiDocs.backToStart') }}
            </RouterLink>
          </div>
        </main>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useClipboard } from '@/composables/useClipboard'
import {
  API_DOC_DEFAULT_SLUG,
  apiDocGroups,
  apiDocPages,
  apiDocRoute,
  findApiDocPage,
  resolveApiDocSlug,
} from './apiDocsCatalog'
import { renderApiDoc, type RenderedApiDoc } from './apiDocsRender'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const { copyToClipboard } = useClipboard()

const articleRef = ref<HTMLElement | null>(null)

const activeSlug = computed(() => String(route.params.slug || API_DOC_DEFAULT_SLUG))
const page = computed(() => findApiDocPage(activeSlug.value))

const groups = computed(() =>
  apiDocGroups.map((group) => ({
    ...group,
    pages: apiDocPages.filter((item) => item.group === group.id),
  }))
)

const rendered = computed<RenderedApiDoc>(() => {
  if (!page.value) return { html: '', headings: [] }
  return renderApiDoc(page.value.markdown, {
    currentPath: page.value.path,
    resolveSlug: resolveApiDocSlug,
    routeFor: apiDocRoute,
    copyLabel: t('apiDocs.copy'),
  })
})

function goTo(slug: string) {
  void router.push(apiDocRoute(slug))
}

function scrollToHeading(id: string) {
  document.getElementById(id)?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
}

/** 正文是 v-html 注入的，站内链接和复制按钮都在这里统一处理（事件委托）。 */
function onContentClick(event: MouseEvent) {
  const target = event.target as HTMLElement | null
  if (!target) return

  const copyButton = target.closest('[data-api-doc-copy]')
  if (copyButton) {
    const code = (copyButton.parentElement?.querySelector('pre')?.textContent ?? '').replace(/\n+$/, '')
    void copyToClipboard(code, t('apiDocs.copied'))
    return
  }

  const link = target.closest('a[data-api-doc-slug]')
  if (link) {
    // 带修饰键或非左键时交给浏览器（新标签页打开）。
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return
    event.preventDefault()
    void router.push(link.getAttribute('href') ?? '')
  }
}

// 切换文档后回到正文顶部。
watch(activeSlug, async () => {
  await nextTick()
  articleRef.value?.scrollIntoView?.({ block: 'start' })
})
</script>

<style scoped>
.api-doc-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
}

.api-doc-content :deep(h1) {
  @apply mb-4 text-2xl font-bold text-gray-900 dark:text-white;
}

.api-doc-content :deep(h2) {
  @apply mb-3 mt-10 border-b border-gray-200 pb-2 text-xl font-bold text-gray-900 dark:border-gray-700 dark:text-white;
  scroll-margin-top: 1rem;
}

.api-doc-content :deep(h3) {
  @apply mb-2 mt-7 text-lg font-semibold text-gray-900 dark:text-white;
  scroll-margin-top: 1rem;
}

.api-doc-content :deep(p) {
  @apply mb-4 text-gray-700 dark:text-gray-300;
}

.api-doc-content :deep(ul) {
  @apply mb-4 list-disc pl-6;
}

.api-doc-content :deep(ol) {
  @apply mb-4 list-decimal pl-6;
}

.api-doc-content :deep(li) {
  @apply mb-1 text-gray-700 dark:text-gray-300;
}

.api-doc-content :deep(a) {
  @apply text-emerald-600 underline underline-offset-4 hover:text-emerald-700 dark:text-emerald-300;
}

.api-doc-content :deep(blockquote) {
  @apply my-5 border-l-4 border-gray-300 pl-4 text-gray-600 dark:border-gray-600 dark:text-gray-400;
}

.api-doc-content :deep(:not(pre) > code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[0.85em] text-rose-700 dark:bg-gray-700 dark:text-rose-300;
}

.api-doc-content :deep(.api-doc-code) {
  @apply relative my-5;
}

.api-doc-content :deep(.api-doc-code pre) {
  @apply m-0 overflow-x-auto rounded-xl bg-gray-950 p-4 pr-16 text-sm leading-6 text-gray-100;
}

.api-doc-content :deep(.api-doc-code pre code) {
  @apply bg-transparent p-0 font-mono text-inherit;
}

.api-doc-content :deep(.api-doc-copy) {
  @apply absolute right-2 top-2 rounded-md bg-white/10 px-2 py-1 text-xs text-gray-200 opacity-0 transition hover:bg-white/20;
}

.api-doc-content :deep(.api-doc-code:hover .api-doc-copy),
.api-doc-content :deep(.api-doc-copy:focus-visible) {
  opacity: 1;
}

@media (hover: none) {
  .api-doc-content :deep(.api-doc-copy) {
    opacity: 1;
  }
}

/* 表格：宽表横向滚动，而不是把各列挤成一两个字宽。 */
.api-doc-content :deep(.api-doc-table) {
  @apply my-5 overflow-x-auto rounded-xl border border-gray-200 dark:border-gray-700;
}

.api-doc-content :deep(.api-doc-table table) {
  @apply w-full border-collapse text-sm;
  min-width: 640px;
}

.api-doc-content :deep(th) {
  @apply whitespace-nowrap border-b border-gray-200 bg-gray-50 px-3 py-2 text-left font-semibold dark:border-gray-700 dark:bg-gray-900/40;
}

.api-doc-content :deep(td) {
  @apply border-b border-gray-100 px-3 py-2 align-top dark:border-gray-700;
  min-width: 6rem;
}

.api-doc-content :deep(tr:last-child td) {
  @apply border-b-0;
}

.api-doc-content :deep(td code) {
  white-space: normal;
  overflow-wrap: anywhere;
}

.api-doc-content :deep(hr) {
  @apply my-7 border-gray-200 dark:border-gray-700;
}
</style>
