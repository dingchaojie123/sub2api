import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.platforms.kling': 'K-Ling',
    'admin.accounts.platforms.happyhourse': 'Happy-Hourse',
    'admin.accounts.platforms.seedance': 'Seedance'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] || key
    })
  }
})

describe('AccountTableFilters', () => {
  it('offers the three video account platforms in the platform filter', () => {
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
  })
})
