import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AffiliateView from '../AffiliateView.vue'
import type { UserAffiliateDetail } from '@/types'

const getAffiliateDetail = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}|${Object.values(params).join(',')}` : key,
    }),
  }
})

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: vi.fn().mockResolvedValue(undefined) }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

function makeDetail(overrides: Partial<UserAffiliateDetail> = {}): UserAffiliateDetail {
  return {
    user_id: 1,
    aff_code: 'ROOTCODE',
    aff_count: 1,
    aff_quota: 0,
    aff_frozen_quota: 0,
    aff_history_quota: 0,
    effective_rebate_rate_percent: 5,
    invitees: [
      { user_id: 2, email: 'b***@g***.com', username: 'bob', created_at: '2026-08-01T00:00:00Z', total_rebate: 12.5 },
    ],
    second_level_count: 1,
    second_level_invitees: [
      {
        user_id: 3,
        email: 'c***@q***.com',
        username: 'carol',
        created_at: '2026-08-02T00:00:00Z',
        via_user_id: 2,
        via_email: 'b***@g***.com',
      },
    ],
    ...overrides,
  }
}

async function mountView(detail: UserAffiliateDetail) {
  getAffiliateDetail.mockResolvedValue(detail)
  const wrapper = mount(AffiliateView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('AffiliateView second-level invitees', () => {
  beforeEach(() => {
    getAffiliateDetail.mockReset()
  })

  it('shows the first-level list by default and shows both counts on the tabs', async () => {
    const wrapper = await mountView(makeDetail({ aff_count: 4, second_level_count: 7 }))

    expect(wrapper.text()).toContain('bob')
    expect(wrapper.text()).not.toContain('carol')
    expect(wrapper.get('[data-testid="invitee-tab-first"]').text()).toContain('(4)')
    expect(wrapper.get('[data-testid="invitee-tab-second"]').text()).toContain('(7)')
  })

  it('switches to the second-level list and shows who invited each user, without a rebate column', async () => {
    const wrapper = await mountView(makeDetail())

    await wrapper.get('[data-testid="invitee-tab-second"]').trigger('click')

    expect(wrapper.text()).toContain('carol')
    expect(wrapper.text()).toContain('c***@q***.com')
    // 「邀请人（一级用户）」列展示脱敏后的一级用户邮箱
    expect(wrapper.text()).toContain('affiliate.secondLevel.columns.via')
    expect(wrapper.text()).toContain('b***@g***.com')
    // 一级用户的数据不应出现在二级视图里；二级没有返利列
    expect(wrapper.text()).not.toContain('bob')
    expect(wrapper.text()).not.toContain('affiliate.invitees.columns.rebate')
    expect(wrapper.text()).toContain('affiliate.secondLevel.hint')
  })

  it('shows an empty state when there are no second-level users', async () => {
    const wrapper = await mountView(makeDetail({ second_level_count: 0, second_level_invitees: [] }))

    await wrapper.get('[data-testid="invitee-tab-second"]').trigger('click')

    expect(wrapper.text()).toContain('affiliate.secondLevel.empty')
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('tells the user when the list is truncated and the real total is larger', async () => {
    const wrapper = await mountView(makeDetail({ second_level_count: 250 }))

    await wrapper.get('[data-testid="invitee-tab-second"]').trigger('click')

    const note = wrapper.get('[data-testid="second-level-truncated"]')
    expect(note.text()).toContain('250')
  })

  it('does not show the truncation note when the whole list is displayed', async () => {
    const wrapper = await mountView(makeDetail())

    await wrapper.get('[data-testid="invitee-tab-second"]').trigger('click')

    expect(wrapper.find('[data-testid="second-level-truncated"]').exists()).toBe(false)
  })
})
