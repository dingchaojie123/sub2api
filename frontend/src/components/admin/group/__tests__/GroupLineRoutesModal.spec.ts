import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const getGroupLineRoutes = vi.fn()
const setGroupLineRoute = vi.fn()
const getLineRoutesSwitch = vi.fn()
const setLineRoutesSwitch = vi.fn()
const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupLineRoutes: (...args: unknown[]) => getGroupLineRoutes(...args),
      setGroupLineRoute: (...args: unknown[]) => setGroupLineRoute(...args),
      getLineRoutesSwitch: (...args: unknown[]) => getLineRoutesSwitch(...args),
      setLineRoutesSwitch: (...args: unknown[]) => setLineRoutesSwitch(...args),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import GroupLineRoutesModal from '../GroupLineRoutesModal.vue'

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const W = 'Seedance2.5--720p--pro--按次--W'
const HH = 'Seedance2.5--720p--满参稳定--按次--HH'

const group = { id: 7, name: 'TT---Seedance--【GY专用】', platform: 'seedance-hub' } as const

const lines = (target = '', targetAccounts = 0) => [
  {
    line_name: W,
    platform: 'miaowu-seedance',
    version: '2.5',
    billing: 'per_request',
    hidden: false,
    accounts: 1,
    target_line_name: target,
    target_accounts: targetAccounts,
    candidates: [{ line_name: HH, platform: 'huahuo-seedance', accounts: 1 }],
  },
  {
    line_name: HH,
    platform: 'huahuo-seedance',
    version: '2.5',
    billing: 'per_request',
    hidden: false,
    accounts: 1,
    target_line_name: '',
    target_accounts: 0,
    candidates: [{ line_name: W, platform: 'miaowu-seedance', accounts: 1 }],
  },
]

const mountModal = async () => {
  const wrapper = mount(GroupLineRoutesModal, {
    props: { show: false, group: group as never },
    global: { stubs: { BaseDialog: BaseDialogStub } },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('GroupLineRoutesModal', () => {
  beforeEach(() => {
    getGroupLineRoutes.mockReset()
    setGroupLineRoute.mockReset()
    getLineRoutesSwitch.mockReset()
    getLineRoutesSwitch.mockResolvedValue(true)
    setLineRoutesSwitch.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('lists the lines of the group and loads them when opened', async () => {
    getGroupLineRoutes.mockResolvedValue(lines())
    const wrapper = await mountModal()

    expect(getGroupLineRoutes).toHaveBeenCalledWith(7)
    const rows = wrapper.findAll('[data-testid="line-route-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain(W)
    expect(rows[1].text()).toContain(HH)
  })

  it('saves nothing until a different target is chosen, then saves the choice', async () => {
    getGroupLineRoutes.mockResolvedValue(lines())
    setGroupLineRoute.mockResolvedValue(lines(HH, 1))
    const wrapper = await mountModal()

    const save = wrapper.findAll('[data-testid="line-route-save"]')[0]
    expect(save.attributes('disabled')).toBeDefined()

    await wrapper.findAll('[data-testid="line-route-select"]')[0].setValue(HH)
    expect(save.attributes('disabled')).toBeUndefined()

    await save.trigger('click')
    await flushPromises()

    expect(setGroupLineRoute).toHaveBeenCalledWith(7, W, HH)
    expect(showSuccess).toHaveBeenCalledWith('admin.groups.lineRoutesSaved')
  })

  it('flags a route whose target no longer has an account in the group', async () => {
    getGroupLineRoutes.mockResolvedValue(lines(HH, 0))
    const wrapper = await mountModal()

    expect(wrapper.text()).toContain('admin.groups.lineRoutesBroken')
  })

  it('shows the server message when a route is rejected', async () => {
    getGroupLineRoutes.mockResolvedValue(lines())
    setGroupLineRoute.mockRejectedValue(new Error('the group has no schedulable account'))
    const wrapper = await mountModal()

    await wrapper.findAll('[data-testid="line-route-select"]')[0].setValue(HH)
    await wrapper.findAll('[data-testid="line-route-save"]')[0].trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('the group has no schedulable account')
  })

  it('shows the master switch and turns it on and off', async () => {
    getGroupLineRoutes.mockResolvedValue(lines())
    getLineRoutesSwitch.mockResolvedValue(false)
    setLineRoutesSwitch.mockResolvedValue(true)
    const wrapper = await mountModal()

    // Off until an operator turns it on: the dialog says so and offers to turn it on.
    expect(wrapper.find('[data-testid="line-route-switch-state"]').text()).toBe('admin.groups.lineRoutesSwitchOff')
    expect(wrapper.find('[data-testid="line-route-switch"]').text()).toContain('admin.groups.lineRoutesSwitchOffHint')

    await wrapper.find('[data-testid="line-route-switch-toggle"]').trigger('click')
    await flushPromises()

    expect(setLineRoutesSwitch).toHaveBeenCalledWith(true)
    expect(wrapper.find('[data-testid="line-route-switch-state"]').text()).toBe('admin.groups.lineRoutesSwitchOn')
    expect(showSuccess).toHaveBeenCalledWith('admin.groups.lineRoutesSwitchSaved')
  })

  it('does not let the switch be toggled when its state could not be read', async () => {
    getGroupLineRoutes.mockResolvedValue(lines())
    getLineRoutesSwitch.mockRejectedValue(new Error('boom'))
    const wrapper = await mountModal()

    expect(wrapper.find('[data-testid="line-route-switch-toggle"]').attributes('disabled')).toBeDefined()
    expect(setLineRoutesSwitch).not.toHaveBeenCalled()
  })
})
