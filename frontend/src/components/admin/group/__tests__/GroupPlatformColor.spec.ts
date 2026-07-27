import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupRateMultipliers: vi.fn().mockResolvedValue([]),
      updateGroupRateMultipliers: vi.fn().mockResolvedValue([]),
      searchUsersForRateMultiplier: vi.fn().mockResolvedValue({ users: [] }),
      getGroupRPMOverrides: vi.fn().mockResolvedValue([]),
      updateGroupRPMOverrides: vi.fn().mockResolvedValue([]),
      searchUsersForRPMOverride: vi.fn().mockResolvedValue({ users: [] }),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

import GroupRateMultipliersModal from '../GroupRateMultipliersModal.vue'
import GroupRPMOverridesModal from '../GroupRPMOverridesModal.vue'

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const commonStubs = {
  BaseDialog: BaseDialogStub,
  Pagination: true,
  Icon: true,
  PlatformIcon: true,
}

const jimengGroup = {
  id: 7,
  name: 'Jimeng group',
  platform: 'jimeng',
  rate_multiplier: 1,
  rpm_limit: 60,
} as const

describe('group platform color accents', () => {
  it('renders Jimeng rate multiplier header with rose text', () => {
    const wrapper = mount(GroupRateMultipliersModal, {
      props: { show: true, group: jimengGroup },
      global: { stubs: commonStubs },
    })

    expect(wrapper.find('.text-rose-700').exists()).toBe(true)
  })

  it('renders Jimeng RPM override header with rose text', () => {
    const wrapper = mount(GroupRPMOverridesModal, {
      props: { show: true, group: jimengGroup },
      global: { stubs: commonStubs },
    })

    expect(wrapper.find('.text-rose-700').exists()).toBe(true)
  })
})
