import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelListView from '../ModelListView.vue'
import { modelSquareCatalog } from '../modelPricingCatalog'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const translations: Record<string, string> = {
    'modelList.eyebrow': '公开报价',
    'modelList.title': '模型与价格',
    'modelList.description': '查看当前支持的文本、图片与视频模型价格。',
    'modelList.search': '搜索模型',
    'modelList.searchPlaceholder': '搜索模型或规格',
    'modelList.emptyTitle': '未找到匹配模型',
    'modelList.emptyDescription': '请调整搜索关键词。',
    'modelList.availabilityNote': '展示价格仅供估算。',
    'modelList.filters.title': '筛选模型',
    'modelList.filters.showing': '显示 {shown} / {total}',
    'modelList.filters.clear': '清除筛选',
    'modelList.filters.all': '全部',
    'modelList.filters.category': '类型',
    'modelList.filters.channel': '渠道',
    'modelList.filters.billing': '计费方式',
    'modelList.card.channel': '渠道',
    'modelList.card.billing': '计费方式',
    'modelList.card.morePrices': '另有 {count} 档价格',
    'modelList.card.detail': '查看详情',
    'modelList.detail.prices': '用户价格',
    'modelList.detail.pricesHint': '展示价格。',
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
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<div v-if="show" data-testid="dialog"><h3>{{ title }}</h3><slot /></div>',
        },
        Icon: true,
        ModelIcon: true,
      },
    },
  })
}

describe('ModelListView', () => {
  it('renders every model as a card with channel, billing and price', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型与价格')
    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(modelSquareCatalog.length)
    expect(wrapper.text()).toContain('Seedance2.0-933')
    expect(wrapper.text()).toContain('限时限量-Seedance 2.5 满参')
    expect(wrapper.text()).toContain('19')
    expect(wrapper.text()).toContain('积分 / 次')
    expect(wrapper.text()).toContain(`显示 ${modelSquareCatalog.length} / ${modelSquareCatalog.length}`)
  })

  it('keeps legacy CNY-priced models alongside the credit-priced ones', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('Pixverse-V6')
    expect(wrapper.text()).toContain('¥0.72')
    expect(wrapper.text()).toContain('7 折')
  })

  it('prices image models in credits (¥1 = 1.62 credits)', () => {
    const wrapper = renderView()
    const imageCards = modelSquareCatalog.filter((item) => item.category === 'image')

    expect(imageCards).toHaveLength(3)
    imageCards.forEach((item) => {
      item.prices.forEach((tier) => expect(tier.suffix).toBe('积分 / 张'))
    })
    expect(wrapper.text()).toContain('Midjourney')
    expect(wrapper.text()).toContain('1.94')
    expect(wrapper.text()).not.toContain('¥1.20')
  })

  it('filters by channel and billing, and clears the filters', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="filter-channel-特价"]').setValue(true)
    const specialCount = modelSquareCatalog.filter((item) => item.channel === '特价').length
    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(specialCount)
    expect(wrapper.text()).toContain('特价-Seedance 2.0')
    expect(wrapper.text()).not.toContain('Veo 3.1')

    await wrapper.get('[data-testid="filter-billing-按次"]').setValue(true)
    expect(wrapper.findAll('[data-testid="model-card"]').length).toBeLessThanOrEqual(specialCount)

    await wrapper.get('[data-testid="model-filter-clear"]').trigger('click')
    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(modelSquareCatalog.length)
  })

  it('opens the detail dialog with all price tiers and specs', async () => {
    const wrapper = renderView()
    await wrapper.get('[data-testid="model-pricing-search"]').setValue('Seedance 2.0-无参考视频')

    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="dialog"]').exists()).toBe(false)

    await wrapper.get('[data-testid="model-detail-button"]').trigger('click')

    const dialog = wrapper.get('[data-testid="dialog"]')
    expect(dialog.text()).toContain('Seedance 2.0-无参考视频')
    expect(wrapper.findAll('[data-testid="model-detail-price"]')).toHaveLength(3)
    expect(dialog.text()).toContain('480P')
    expect(dialog.text()).toContain('1080P')
    expect(dialog.text()).toContain('1.8')
    expect(dialog.text()).toContain('5–15 秒')
    expect(dialog.text()).toContain('提示词上限')
    expect(dialog.text()).toContain('10000 字')
  })

  it('shows an empty state when nothing matches', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-pricing-search"]').setValue('不存在的模型')

    expect(wrapper.find('[data-testid="model-card"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('未找到匹配模型')
  })
})
