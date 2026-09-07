import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.platforms.kling': 'K-Ling',
    'admin.accounts.platforms.happyhourse': 'Happy-Hourse',
    'admin.accounts.platforms.seedance': 'Seedance',
    'admin.accounts.platforms.bytedance': 'ByteDance',
    'admin.accounts.platforms.wan3': 'Wan3.0',
    'admin.accounts.platforms.minimax-h3': 'MiniMax-H3',
    'admin.accounts.platforms.pixverse-v6': 'Pixverse-V6'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] || key
    })
  }
})

describe('AccountTableFilters', () => {
  it('offers all video account platforms in the platform filter', () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {}
      },
      global: {
        stubs: {
          SearchInput: true,
          Select: {
            props: ['options'],
            template: '<div>{{ options.map((option) => option.label).join("|") }}</div>'
          }
        }
      }
    })

    expect(wrapper.text()).toContain('K-Ling')
    expect(wrapper.text()).toContain('Happy-Hourse')
    expect(wrapper.text()).toContain('Seedance')
    expect(wrapper.text()).toContain('ByteDance')
    expect(wrapper.text()).toContain('Wan3.0')
    expect(wrapper.text()).toContain('MiniMax-H3')
    expect(wrapper.text()).toContain('Pixverse-V6')
  })
})
