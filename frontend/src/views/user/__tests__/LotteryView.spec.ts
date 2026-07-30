import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import LotteryView from '../LotteryView.vue'

const getStatus = vi.hoisted(() => vi.fn())
const draw = vi.hoisted(() => vi.fn())
const getRecords = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'lottery.dailyUsage') return `${params?.used}/${params?.limit}`
        return key
      },
    }),
  }
})

vi.mock('@/api', () => ({
  lotteryAPI: {
    getStatus,
    draw,
    getRecords,
  },
}))

describe('LotteryView', () => {
  beforeEach(() => {
    getStatus.mockReset()
    draw.mockReset()
    getRecords.mockReset()
  })

  it('disables drawing when the user has no available chances left today', async () => {
    getStatus.mockResolvedValue({
      available_chances: 0,
      daily_used: 3,
      daily_limit: 3,
    })
    getRecords.mockResolvedValue([])

    const wrapper = shallowMount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('0')
    expect(wrapper.text()).toContain('3/3')
    expect(wrapper.get('[data-testid="lottery-draw-button"]').attributes('disabled')).toBeDefined()
    expect(draw).not.toHaveBeenCalled()
  })
})
