import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { syncUpstreamModelsMock, syncUpstreamModelsPreviewMock } = vi.hoisted(() => ({
  syncUpstreamModelsMock: vi.fn(),
  syncUpstreamModelsPreviewMock: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showInfo: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
  }),
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModelsPreview: syncUpstreamModelsPreviewMock,
    syncUpstreamModels: syncUpstreamModelsMock,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

const IconStub = defineComponent({
  name: 'Icon',
  props: { name: String },
  template: '<span />',
})

const ModelIconStub = defineComponent({
  name: 'ModelIcon',
  props: { model: String },
  template: '<span />',
})

function mountSelector(props: Record<string, unknown> = {}) {
  return mount(ModelWhitelistSelector, {
    props: {
      modelValue: [],
      platform: 'jimeng',
      syncCredentials: {
        platform: 'jimeng',
        type: 'apikey',
        base_url: 'https://jimeng-proxy.example.com/v1',
        api_key: 'jm-key',
      },
      ...props,
    },
    global: {
      stubs: {
        Icon: IconStub,
        ModelIcon: ModelIconStub,
      },
    },
  })
}

describe('ModelWhitelistSelector jimeng mode', () => {
  beforeEach(() => {
    syncUpstreamModelsMock.mockReset()
    syncUpstreamModelsPreviewMock.mockReset()
  })

  it('syncs jimeng models from upstream in create flow', async () => {
    syncUpstreamModelsPreviewMock.mockResolvedValue({
      models: ['by-seedance2.0-933', 'jimeng-video-live'],
    })
    const wrapper = mountSelector()

    const syncButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.syncUpstreamModels'))
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(syncUpstreamModelsMock).not.toHaveBeenCalled()
    expect(syncUpstreamModelsPreviewMock).toHaveBeenCalledWith({
      platform: 'jimeng',
      type: 'apikey',
      base_url: 'https://jimeng-proxy.example.com/v1',
      api_key: 'jm-key',
    })
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[
      'by-seedance2.0-933',
      'jimeng-video-live',
    ]])
  })

  it.each(['kling', 'happyhourse', 'seedance'] as const)(
    'shows upstream sync for %s video platform in create flow',
    async (platform) => {
      syncUpstreamModelsPreviewMock.mockResolvedValue({ models: [`${platform}-live-model`] })
      const wrapper = mountSelector({
        platform,
        syncCredentials: {
          platform,
          type: 'apikey',
          base_url: 'https://app.ppapi.ai/v1',
          api_key: 'pp-key',
        },
      })

      const syncButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.syncUpstreamModels'))
      expect(syncButton).toBeDefined()
      await syncButton!.trigger('click')
      await flushPromises()

      expect(syncUpstreamModelsPreviewMock).toHaveBeenCalledWith({
        platform,
        type: 'apikey',
        base_url: 'https://app.ppapi.ai/v1',
        api_key: 'pp-key',
      })
      expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[`${platform}-live-model`]])

      await wrapper.find('div.cursor-pointer').trigger('click')
      expect(
        wrapper.findAll('button').some(button => button.text().trim() === `${platform}-live-model`)
      ).toBe(true)
    }
  )
})
