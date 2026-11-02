import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelListView from '../ModelListView.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const translations: Record<string, string> = {
    'modelList.eyebrow': '公开报价',
    'modelList.title': '模型与价格',
    'modelList.description': '查看当前支持的文本、图片与视频模型价格。',
    'modelList.search': '搜索模型',
    'modelList.searchPlaceholder': '搜索模型或规格',
    'modelList.categories': '模型分类',
    'modelList.matchCount': '{count} 个报价',
    'modelList.columns.model': '模型',
    'modelList.emptyTitle': '未找到匹配模型',
    'modelList.emptyDescription': '请调整搜索关键词。',
    'modelList.availabilityNote': '“-”表示该档位暂不提供。',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const value = translations[key] ?? key
        if (!params) return value
        return Object.entries(params).reduce(
          (text, [paramKey, paramValue]) => text.replace(`{${paramKey}}`, String(paramValue)),
          value,
        )
      },
    }),
  }
})

function renderView() {
  return mount(ModelListView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
}

describe('ModelListView', () => {
  it('renders the spreadsheet video price matrix by default', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型与价格')
    expect(wrapper.text()).toContain('视频生成')
    expect(wrapper.text()).toContain('grok-imagine-video【官key】')
    expect(wrapper.text()).toContain('¥0.18')
    expect(wrapper.text()).toContain('veo-3.1-fast【官】')
    expect(wrapper.text()).toContain('¥5.80')
    expect(wrapper.findAll('tbody tr')).toHaveLength(14)
  })

  it('shows subsidized per-request and per-second Seedance prices', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="pricing-category-subsidized-video"]').trigger('click')

    expect(wrapper.text()).toContain('TikTok 补贴视频')
    expect(wrapper.text()).toContain('按次')
    expect(wrapper.text()).toContain('¥3.36 / 次')
    expect(wrapper.text()).toContain('¥17.00 / 次')
    expect(wrapper.text()).toContain('按秒')
    expect(wrapper.text()).toContain('¥0.80 / 秒')
    expect(wrapper.findAll('tbody tr')).toHaveLength(7)
  })

  it('switches to image and text pricing', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="pricing-category-image"]').trigger('click')
    expect(wrapper.text()).toContain('gpt-image【官】')
    expect(wrapper.text()).toContain('Midjourney【官】')
    expect(wrapper.text()).toContain('¥1.20')

    await wrapper.get('[data-testid="pricing-category-text"]').trigger('click')
    expect(wrapper.text()).toContain('Claude Code【官】')
    expect(wrapper.text()).toContain('7 折')
    expect(wrapper.text()).toContain('Claude Code【官转】')
    expect(wrapper.text()).toContain('3 折')
  })

  it('filters the active category and shows an empty state', async () => {
    const wrapper = renderView()
    const search = wrapper.get('[data-testid="model-pricing-search"]')

    await search.setValue('Pixverse')
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).toContain('Pixverse-V6【官】')
    expect(wrapper.text()).not.toContain('grok-imagine-video【官key】')

    await search.setValue('不存在的模型')
    expect(wrapper.find('tbody').exists()).toBe(false)
    expect(wrapper.text()).toContain('未找到匹配模型')
  })
})
