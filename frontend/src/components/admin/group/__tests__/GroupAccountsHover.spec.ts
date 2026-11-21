import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const list = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: (...args: unknown[]) => list(...args),
    },
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key} ${JSON.stringify(params)}` : key,
    }),
  }
})

import GroupAccountsHover from '../GroupAccountsHover.vue'

const account = (id: number, name: string, overrides: Record<string, unknown> = {}) => ({
  id,
  name,
  platform: 'miaowu-seedance',
  status: 'active',
  schedulable: true,
  ...overrides,
})

// Every test uses its own group id: the component remembers a group's accounts for a few seconds.
let nextGroupId = 100

const mountHover = (total: number) => {
  const groupId = nextGroupId++
  const wrapper = mount(GroupAccountsHover, {
    props: { groupId, total },
    slots: { default: '<span data-testid="cell">1 个账号</span>' },
    global: { stubs: { teleport: true, PlatformIcon: true } },
  })
  return { wrapper, groupId }
}

describe('GroupAccountsHover', () => {
  beforeEach(() => {
    list.mockReset()
  })

  it('shows nothing, and loads nothing, until the cell is hovered', () => {
    const { wrapper } = mountHover(2)

    expect(wrapper.find('[data-testid="cell"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="group-accounts-panel"]').exists()).toBe(false)
    expect(list).not.toHaveBeenCalled()
  })

  it('lists the accounts of the group on hover', async () => {
    list.mockResolvedValue({
      items: [account(1, 'miaowu-a'), account(2, 'huahuo-b', { schedulable: false })],
      total: 2,
    })
    const { wrapper, groupId } = mountHover(2)

    await wrapper.find('[data-testid="group-accounts-trigger"]').trigger('mouseenter')
    await flushPromises()

    expect(list).toHaveBeenCalledWith(1, 50, { group: String(groupId) })
    const items = wrapper.findAll('[data-testid="group-accounts-item"]')
    expect(items).toHaveLength(2)
    expect(items[0].text()).toContain('miaowu-a')
    expect(items[1].text()).toContain('huahuo-b')
    // An account that is active but switched off for scheduling is flagged.
    expect(items[1].text()).toContain('admin.groups.accountsHoverUnschedulable')
    expect(items[0].text()).not.toContain('admin.groups.accountsHoverUnschedulable')
  })

  it('says so when the group has more accounts than are listed', async () => {
    list.mockResolvedValue({ items: [account(1, 'a')], total: 80 })
    const { wrapper } = mountHover(80)

    await wrapper.find('[data-testid="group-accounts-trigger"]').trigger('mouseenter')
    await flushPromises()

    expect(wrapper.find('[data-testid="group-accounts-panel"]').text()).toContain('admin.groups.accountsHoverMore')
  })

  it('shows an error instead of an empty list when loading fails', async () => {
    list.mockRejectedValue(new Error('boom'))
    const { wrapper } = mountHover(1)

    await wrapper.find('[data-testid="group-accounts-trigger"]').trigger('mouseenter')
    await flushPromises()

    const panel = wrapper.find('[data-testid="group-accounts-panel"]')
    expect(panel.text()).toContain('admin.groups.accountsHoverFailed')
    expect(wrapper.findAll('[data-testid="group-accounts-item"]')).toHaveLength(0)
  })

  it('does not load the same group again within a few seconds', async () => {
    list.mockResolvedValue({ items: [account(1, 'a')], total: 1 })
    const { wrapper } = mountHover(1)
    const trigger = wrapper.find('[data-testid="group-accounts-trigger"]')

    await trigger.trigger('mouseenter')
    await flushPromises()
    await trigger.trigger('mouseenter')
    await flushPromises()

    expect(list).toHaveBeenCalledTimes(1)
  })
})
