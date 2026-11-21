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
    expect(wrapper.text()).toContain('Seedance2.0--720p--933--按次--Z')
    expect(wrapper.text()).toContain('Seedance2.5--720p--pro--按次--W')
    expect(wrapper.text()).toContain('19')
    expect(wrapper.text()).toContain('积分 / 次')
    expect(wrapper.text()).toContain(`显示 ${modelSquareCatalog.length} / ${modelSquareCatalog.length}`)
  })

  it('lists Seedance2.5 slow as a per-second model with a price per resolution', () => {
    const model = modelSquareCatalog.find((item) => item.name === 'Seedance2.5--480p/720p--特价满血--按秒--W')

    expect(model).toMatchObject({ category: 'video', channel: '常规', billing: '按秒' })
    expect(model?.prices).toEqual([
      expect.objectContaining({ label: '480P', amount: '0.7', suffix: '积分 / 秒' }),
      expect.objectContaining({ label: '720P', amount: '1', suffix: '积分 / 秒' }),
    ])
    expect(model?.specs).toEqual(expect.arrayContaining([
      { label: '时长', value: '4–30 秒' },
      { label: '参考图片', value: '最多 30 张' },
    ]))
  })

  it('shows the lines of the 2026-11-17 operations sheet and hides the withdrawn ones', () => {
    const names = modelSquareCatalog.map((item) => item.name)

    // 2026-11-19 final table: 喵呜 fast (and fast-deal / tj / wx) and 浙创 930 / 903 are withdrawn.
    for (const withdrawn of [
      'Seedance2.0--480p/720p--fast--按次--W',
      'Seedance2.0--720p--fast-deal--按次--W',
      'Seedance2.5--720p--flash-tj--按次--W',
      'Seedance2.5--480p/720p--tj--按次--W',
      'Seedance2.5--480p/720p--wx--按次--W',
      'Seedance2.0--720p--930--按次--Z',
      'Seedance2.0--720p--903--按次--Z',
      'Seedance2.0--480p/720p--kaopu-mini--按次--Z',
    ]) {
      expect(names, withdrawn).not.toContain(withdrawn)
    }
    expect(modelSquareCatalog.find((item) => item.name === 'Seedance2.0--720p--933--按次--Z')?.prices).toEqual([
      expect.objectContaining({ label: '720P', amount: '8', suffix: '积分 / 次' }),
    ])

    // Hidden lines (光影, 星光, paisio, Apophis and the unlisted 极九/喵呜 lines) are not shown.
    for (const hidden of [
      'Seedance2.5--720p--全参--按秒--G',
      'Seedance2.0--720p--933--按次--X',
      'Seedance2.5--720p--V--按次--X',
      'Seedance2.0--720p--903z--按次--J',
      'Seedance2.0--720p--916--按次--W',
      'Seedance2.5--480p--Z--按秒--A',
      'Seedance2.5--720p--原生过人脸--按秒--P',
    ]) {
      expect(names, hidden).not.toContain(hidden)
    }
    expect(names.some((name) => /--(G|X|P|A)$/.test(name))).toBe(false)
  })

  it('shows exactly the 13 models of the 2026-11-19 final table, with the table prices', () => {
    // [聚合名称, 计费, [[分辨率, 积分], ...], 参考素材 图/视频/音频]
    const table: Array<[string, string, Array<[string, string]>, [string, string, string]]> = [
      ['Seedance2.0--480p/720p/1080p--sd2.0--按秒--XM', '按秒', [['480P', '0.5'], ['720P', '0.8'], ['1080P', '1.2']], ['9', '0', '3']],
      ['Seedance2.5--720p--稳定满参真人脸--按次--XM', '按次', [['720P', '19']], ['30', '0', '10']],
      ['Seedance2.5--480p/720p--flash--按秒--XM', '按秒', [['480P', '0.6'], ['720P', '1']], ['10', '0', '5']],
      ['Seedance2.0--720p--doubao-fast--按次--W', '按次', [['720P', '5']], ['9', '0', '0']],
      ['Seedance2.0--720p--doubao-mini--按次--W', '按次', [['720P', '3.5']], ['9', '0', '0']],
      ['Seedance2.5--480p/720p--特价满血--按秒--W', '按秒', [['480P', '0.7'], ['720P', '1']], ['30', '0', '10']],
      ['Seedance2.5--720p--pro--按次--W', '按次', [['720P', '19']], ['30', '0', '10']],
      ['Seedance2.5--480p/720p--flash--按次--W', '按次', [['480P', '11'], ['720P', '15']], ['10', '0', '5']],
      ['Seedance2.5--480p/720p--flash-s--按秒--W', '按秒', [['480P', '0.6'], ['720P', '1']], ['10', '0', '5']],
      ['Seedance2.0--720p--933--按次--Z', '按次', [['720P', '8']], ['9', '3', '3']],
      ['Seedance2.5--720p--满参稳定--按秒--HH', '按秒', [['720P', '1']], ['30', '0', '10']],
      ['Seedance2.5--720p--满参稳定--按次--HH', '按次', [['720P', '19']], ['30', '0', '10']],
      ['Seedance2.5--720p--shuliang1--按次--HH', '按次', [['720P', '19']], ['30', '10', '10']],
    ]
    const refValue = (item: (typeof modelSquareCatalog)[number], label: string) =>
      item.specs.find((row) => row.label === label)?.value ?? ''

    for (const [name, billing, prices, [images, videos, audios]] of table) {
      const item = modelSquareCatalog.find((card) => card.name === name)
      expect(item, name).toBeDefined()
      expect(item?.billing, name).toBe(billing)
      expect(item?.prices.map((tier) => [tier.label, tier.amount]), name).toEqual(prices)
      expect(refValue(item!, '参考图片'), name).toContain(images)
      expect(refValue(item!, '参考视频'), name).toContain(videos === '0' ? '不支持' : videos)
      expect(refValue(item!, '参考音频'), name).toContain(audios === '0' ? '不支持' : audios)
    }
  })

  it('lists the 喵呜 tj family and the V6K models added on 2026-11-21 at a flat price per request', () => {
    // [聚合名称, 积分, 参考图/视频/音频, 时长]
    const table: Array<[string, string, [string, string, string], string]> = [
      ['Seedance2.0--480p/720p/1080p/4k--tj--按次--W', '11', ['9', '3', '3'], '5–15 秒'],
      ['Seedance2.0--480p/720p/1080p/4k--fast-tj--按次--W', '11', ['9', '3', '3'], '5–15 秒'],
      ['Seedance2.0--480p/720p/1080p/4k--mini-tj--按次--W', '11', ['9', '3', '3'], '5–15 秒'],
      ['Seedance2.0--480p/720p/1080p/4k--官方按条--按次--V6K', '11', ['9', '0', '3'], '4–15 秒'],
      ['Seedance2.0--480p/720p--官方fast--按次--V6K', '5', ['9', '0', '3'], '4–15 秒'],
      ['Seedance2.0--480p/720p--mini--按次--V6K', '4', ['9', '0', '3'], '4–15 秒'],
    ]
    const refValue = (item: (typeof modelSquareCatalog)[number], label: string) =>
      item.specs.find((row) => row.label === label)?.value ?? ''

    for (const [name, amount, [images, videos, audios], duration] of table) {
      const item = modelSquareCatalog.find((card) => card.name === name)
      expect(item, name).toBeDefined()
      expect(item?.billing, name).toBe('按次')
      expect(item?.prices.map((tier) => tier.amount), name).toEqual([amount])
      expect(refValue(item!, '时长'), name).toBe(duration)
      expect(refValue(item!, '参考图片'), name).toContain(images)
      expect(refValue(item!, '参考视频'), name).toContain(videos === '0' ? '不支持' : videos)
      expect(refValue(item!, '参考音频'), name).toContain(audios)
      expect(refValue(item!, '聚合分组中的名称'), name).toBe(name)
    }
  })

  it('lists the 红佩 s / nh / X lines added on 2026-11-21 with the operator prices', () => {
    const find = (name: string) => modelSquareCatalog.find((card) => card.name === name)
    const prices = (name: string) => find(name)?.prices.map((tier) => [tier.label, tier.amount, tier.suffix])

    expect(prices('Seedance2.5--720p--s--按次--H')).toEqual([['720P', '19', '积分 / 次']])
    expect(prices('Seedance2.5--720p--X--按次--H')).toEqual([['720P', '19', '积分 / 次']])
    expect(prices('Seedance2.5--480p/720p/1080p--nh--按秒--H')).toEqual([
      ['480P', '0.7', '积分 / 秒'],
      ['720P', '1', '积分 / 秒'],
      ['1080P', '4', '积分 / 秒'],
    ])
    expect(find('Seedance2.5--720p--s--按次--H')?.billing).toBe('按次')
    expect(find('Seedance2.5--480p/720p/1080p--nh--按秒--H')?.billing).toBe('按秒')
    expect(find('Seedance2.5--720p--X--按次--H')?.billing).toBe('按次')
  })

  it('no longer lists the withdrawn models', () => {
    const ids = modelSquareCatalog.map((item) => item.id)

    for (const id of ['mai-token-seedance-sdtj-2.5-480p', 'tiktok-seedance-2.5-face', 'tiktok-seedance-2.5-full', 'tiktok-seedance-2.0-full']) {
      expect(ids).not.toContain(id)
    }
    expect(modelSquareCatalog.some((item) => item.name === 'Seedance2.5-480全参')).toBe(false)
  })

  it('no longer lists the 综合视频模型 / Wan3.0 / Pixverse-V6 models taken down on 2026-11-21', () => {
    const ids = modelSquareCatalog.map((item) => item.id)

    for (const id of [
      'official-seedance-2.5',
      'official-seedance-2.0',
      'official-seedance-2.0-mini',
      'official-seedance-2.0-fast',
      'official-gemini-omni-flash',
      'official-gemini-omni-flash-1.1',
      'official-grok-imagine-video',
      'official-grok-imagine-video-1.5',
      'official-kling-3.0',
      'official-veo-3.1',
      'official-veo-3.1-fast',
      'official-wan-3.0-video',
      'legacy-grok-imagine-video-key',
      'legacy-pixverse-v6',
    ]) {
      expect(ids, id).not.toContain(id)
    }
  })

  it('shows the previously CNY-quoted models in credits with the numbers unchanged', () => {
    const wrapper = renderView()
    const price = (id: string, label: string) =>
      modelSquareCatalog.find((item) => item.id === id)?.prices.find((tier) => tier.label === label)?.amount

    expect(wrapper.text()).not.toContain('Pixverse-V6')
    expect(wrapper.text()).toContain('7 折')
    expect(price('special-seedance-2.0-933-z', '720P')).toBe('8')
    expect(price('legacy-kling-3.0-turbo', '720P')).toBe('1.30')
  })

  it('prices MiniMax per second at 480P 0.30, 768P 0.40, 1080P 0.60, 2K 1.00, 4K 2.00 credits', () => {
    const minimax = modelSquareCatalog.find((item) => item.id === 'legacy-minimax')

    expect(minimax?.prices.map((tier) => [tier.label, tier.amount, tier.suffix])).toEqual([
      ['480P', '0.30', '积分 / 秒'],
      ['768P', '0.40', '积分 / 秒'],
      ['1080P', '0.60', '积分 / 秒'],
      ['2K', '1.00', '积分 / 秒'],
      ['4K', '2.00', '积分 / 秒'],
    ])
    expect(minimax?.specs.find((row) => row.label === '分辨率')?.value).toBe('480P / 768P / 1080P / 2K / 4K')
  })

  it('uses a single unit: no card or spec mentions yuan any more', () => {
    const wrapper = renderView()
    expect(wrapper.text()).not.toContain('¥')

    modelSquareCatalog
      .filter((item) => item.category !== 'text')
      .forEach((item) => {
        item.prices.forEach((tier) => {
          expect(tier.suffix, `${item.name} ${tier.label}`).toMatch(/^积分 \/ (次|秒|张)$/)
          expect(tier.amount, `${item.name} ${tier.label}`).toMatch(/^\d+(\.\d{1,2})?$/)
        })
        item.specs.forEach((row) => expect(row.value, `${item.name} ${row.label}`).not.toContain('¥'))
      })
  })

  it('prices image models in credits', () => {
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
    expect(wrapper.text()).toContain('Seedance2.0--720p--933--按次--Z')
    expect(wrapper.text()).not.toContain('Veo 3.1')

    await wrapper.get('[data-testid="filter-billing-按次"]').setValue(true)
    expect(wrapper.findAll('[data-testid="model-card"]').length).toBeLessThanOrEqual(specialCount)

    await wrapper.get('[data-testid="model-filter-clear"]').trigger('click')
    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(modelSquareCatalog.length)
  })

  it('merges the former TikTok subsidy channel into the 特价 channel', async () => {
    const wrapper = renderView()
    // The three former TikTok cards (浙创 930 / 903) are withdrawn since the 2026-11-19 final table.
    const tiktokItems = modelSquareCatalog.filter((item) => item.id.startsWith('tiktok-'))

    expect(tiktokItems).toHaveLength(0)
    tiktokItems.forEach((item) => expect(item.channel, item.name).toBe('特价'))
    expect(modelSquareCatalog.some((item) => item.channel.includes('补贴'))).toBe(false)
    expect(wrapper.find('[data-testid="filter-channel-TikTok 补贴"]').exists()).toBe(false)

    await wrapper.get('[data-testid="filter-channel-特价"]').setValue(true)
    const special = modelSquareCatalog.filter((item) => item.channel === '特价')
    expect(special.length).toBeGreaterThanOrEqual(6)
    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(special.length)
  })

  it('opens the detail dialog with all price tiers and specs', async () => {
    const wrapper = renderView()
    await wrapper.get('[data-testid="model-pricing-search"]').setValue('全参D--按秒--J')

    expect(wrapper.findAll('[data-testid="model-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="dialog"]').exists()).toBe(false)

    await wrapper.get('[data-testid="model-detail-button"]').trigger('click')

    const dialog = wrapper.get('[data-testid="dialog"]')
    expect(dialog.text()).toContain('Seedance2.5--480p/720p/1080p--全参D--按秒--J')
    expect(wrapper.findAll('[data-testid="model-detail-price"]')).toHaveLength(3)
    expect(dialog.text()).toContain('480P')
    expect(dialog.text()).toContain('1080P')
    expect(dialog.text()).toContain('0.7')
    expect(dialog.text()).toContain('4–30 秒')
    expect(dialog.text()).toContain('参考图片')
  })

  it('shows an empty state when nothing matches', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-pricing-search"]').setValue('不存在的模型')

    expect(wrapper.find('[data-testid="model-card"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('未找到匹配模型')
  })
})
