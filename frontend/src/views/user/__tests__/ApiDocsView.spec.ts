import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import ApiDocsView from '../ApiDocsView.vue'
import { apiDocPages } from '../apiDocsCatalog'

const routeState = reactive({ params: {} as Record<string, string> })
const pushMock = vi.fn()
const copyMock = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: pushMock }),
  RouterLink: {
    props: ['to'],
    template: '<a :href="to"><slot /></a>',
  },
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: copyMock }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }),
  }
})

function render() {
  return mount(ApiDocsView, {
    global: {
      stubs: { AppLayout: { template: '<div><slot /></div>' } },
    },
  })
}

describe('ApiDocsView', () => {
  beforeEach(() => {
    routeState.params = {}
    pushMock.mockReset()
    copyMock.mockReset()
  })

  it('shows the quick start by default, with every document in the list', () => {
    const wrapper = render()

    expect(wrapper.get('[data-testid="api-docs-content"]').text()).toContain('站点视频生成接口')
    apiDocPages.forEach((page) => {
      expect(wrapper.find(`[data-testid="api-docs-nav-${page.slug}"]`).exists(), page.slug).toBe(true)
    })
  })

  it('highlights the active document and lists its headings as a table of contents', () => {
    const wrapper = render()

    expect(wrapper.get('[data-testid="api-docs-nav-video"]').classes()).toContain('font-semibold')
    expect(wrapper.get('[data-testid="api-docs-nav-wan"]').classes()).not.toContain('font-semibold')
    expect(wrapper.get('[data-testid="api-docs-toc"]').text()).toContain('选择模型')
  })

  it('renders another document from the route and hides the table of contents when it has no headings', () => {
    routeState.params = { slug: 'wan' }
    const wrapper = render()

    expect(wrapper.get('[data-testid="api-docs-content"]').text()).toContain('Wan3.0')
    expect(wrapper.get('[data-testid="api-docs-nav-wan"]').classes()).toContain('font-semibold')
    expect(wrapper.find('[data-testid="api-docs-toc"]').exists()).toBe(false)
  })

  it('shows a not-found state for an unknown document', () => {
    routeState.params = { slug: 'does-not-exist' }
    const wrapper = render()

    expect(wrapper.find('[data-testid="api-docs-content"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="api-docs-not-found"]').text()).toContain('apiDocs.notFoundTitle')
  })

  it('follows links between documents inside the app instead of reloading the page', async () => {
    const wrapper = render()
    const link = wrapper.get('[data-testid="api-docs-content"] a[data-api-doc-slug="advanced"]')

    await link.trigger('click')

    expect(pushMock).toHaveBeenCalledWith('/api-docs/advanced')
  })

  it('leaves ctrl/cmd-clicks to the browser so links can open in a new tab', async () => {
    const wrapper = render()
    const link = wrapper.get('[data-testid="api-docs-content"] a[data-api-doc-slug="advanced"]')

    await link.trigger('click', { ctrlKey: true })

    expect(pushMock).not.toHaveBeenCalled()
  })

  it('copies the code of a block when its copy button is clicked', async () => {
    const wrapper = render()
    const firstBlock = wrapper.get('[data-testid="api-docs-content"] .api-doc-code')
    const expected = firstBlock.get('pre').text()

    await firstBlock.get('button[data-api-doc-copy]').trigger('click')

    expect(copyMock).toHaveBeenCalledTimes(1)
    expect(copyMock.mock.calls[0][0]).toBe(expected)
  })

  it('switches documents from the small-screen select', async () => {
    const wrapper = render()

    await wrapper.get('[data-testid="api-docs-select"]').setValue('pixverse')

    expect(pushMock).toHaveBeenCalledWith('/api-docs/pixverse')
  })
})
