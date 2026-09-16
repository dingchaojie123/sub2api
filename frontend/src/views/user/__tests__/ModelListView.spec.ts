import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelListView from '../ModelListView.vue'

const mockPublicSettings = vi.hoisted(() => ({ value: null as Record<string, unknown> | null }))
const mockFetchPublicSettings = vi.hoisted(() => vi.fn())

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: mockPublicSettings.value,
    fetchPublicSettings: mockFetchPublicSettings,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const translations: Record<string, string> = {
    'modelList.title': '模型与价格',
    'modelList.description': '按你可用的分组查看模型价格。',
    'modelList.refresh': '重置分组',
    'modelList.groupCount': '{count} 个模型',
    'modelList.columns.model': '模型',
    'modelList.columns.platformInput': '本平台输入',
    'modelList.columns.platformOutput': '本平台输出',
    'modelList.columns.officialInput': '官方输入',
    'modelList.columns.officialOutput': '官方输出',
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
  beforeEach(() => {
    mockPublicSettings.value = null
    mockFetchPublicSettings.mockReset()
  })

  it('renders the special ChatGPT pricing by default', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型与价格')
    expect(wrapper.text()).toContain('ChatGPT【特惠分组】')
    expect(wrapper.text()).toContain('openai 分组模型价格')
    expect(wrapper.text()).toContain('codex-auto-review')
    expect(wrapper.text()).toContain('¥0.04 / 1M')
    expect(wrapper.text()).toContain('$30 / 1M')
    expect(wrapper.findAll('[data-testid^="model-group-"]')).toHaveLength(7)
    expect(wrapper.text()).toContain('glm - 官key openai协议')
    expect(wrapper.text()).toContain('glm - 官key claude协议')
  })

  it('switches groups and renders Gemini pricing', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-group-gemini"]').trigger('click')

    expect(wrapper.text()).toContain('gemini')
    expect(wrapper.text()).toContain('gemini 分组模型价格')
    expect(wrapper.text()).toContain('1.0x')
    expect(wrapper.text()).toContain('gemini-2.5-pro')
    expect(wrapper.text()).toContain('¥1.25 / 1M')
    expect(wrapper.text()).toContain('$10 / 1M')
  })

  it('renders ChatGPT enterprise pricing in the corrected column order', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-group-ChatGPT【企业分组】"]').trigger('click')

    const row = wrapper.findAll('tbody tr').find((item) => item.text().includes('gpt-5.4'))
    expect(row).toBeTruthy()
    expect(row?.findAll('td').map((cell) => cell.text())).toEqual([
      'gpt-5.4',
      '¥2.5 / 1M',
      '¥15 / 1M',
      '$3.5 / 1M',
      '$21 / 1M',
    ])
  })

  it('renders all expected pricing columns', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型')
    expect(wrapper.text()).toContain('本平台输入')
    expect(wrapper.text()).toContain('本平台输出')
    expect(wrapper.text()).toContain('官方输入')
    expect(wrapper.text()).toContain('官方输出')
  })

  it('resets to the default group when refresh is clicked', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-group-ChatGPT【企业分组】"]').trigger('click')
    expect(wrapper.text()).toContain('ChatGPT【企业分组】')

    await wrapper.get('[data-testid="model-list-refresh"]').trigger('click')
    expect(wrapper.text()).toContain('ChatGPT【特惠分组】')
    expect(wrapper.text()).toContain('¥0.04 / 1M')
  })

  it('uses model pricing page data supplied by public settings', () => {
    mockPublicSettings.value = {
      model_pricing_page_data: [
        {
          id: 'server-managed',
          name: 'Server Managed',
          provider: 'openai',
          multiplier: 'custom',
          rows: [
            {
              model: 'server-model-1',
              platform_input: '¥0.11 / 1M',
              platform_output: '¥0.22 / 1M',
              official_input: '$0.33 / 1M',
              official_output: '$0.44 / 1M',
            },
          ],
        },
      ],
    }

    const wrapper = renderView()

    expect(wrapper.text()).toContain('Server Managed')
    expect(wrapper.text()).toContain('server-model-1')
    expect(wrapper.text()).toContain('¥0.11 / 1M')
    expect(wrapper.text()).not.toContain('gpt pro')
  })
})
