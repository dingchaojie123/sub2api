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

function mountSelector() {
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

  it('keeps jimeng models fixed to Seedance 2.0 without calling upstream sync', async () => {
    const wrapper = mountSelector()

    const syncButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.syncUpstreamModels'))
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(syncUpstreamModelsMock).not.toHaveBeenCalled()
    expect(syncUpstreamModelsPreviewMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['seedance 2.0']])
  })
})
