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

  it('does not reveal prize probabilities in the rules panel', async () => {
    getStatus.mockResolvedValue({
      available_chances: 1,
      daily_used: 0,
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

    expect(wrapper.text()).not.toContain('lottery.probability')
    expect(wrapper.text()).not.toContain('1%')
    expect(wrapper.text()).not.toContain('5%')
    expect(wrapper.text()).not.toContain('20%')
    expect(wrapper.text()).not.toContain('74%')
  })

  it('renders the updated blind box prize amounts', async () => {
    getStatus.mockResolvedValue({
      available_chances: 1,
      daily_used: 0,
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

    expect(wrapper.text()).toMatch(/lottery\.firstPrize\s*·\s*\$30(?!0)/)
    expect(wrapper.text()).toMatch(/lottery\.secondPrize\s*·\s*\$10(?!0)/)
    expect(wrapper.text()).toMatch(/lottery\.thirdPrize\s*·\s*\$5(?!0)/)
    expect(wrapper.text()).toMatch(/lottery\.fourthPrize\s*·\s*\$2(?!0)/)
    expect(wrapper.text()).not.toContain('lottery.firstPrize · $300')
    expect(wrapper.text()).not.toContain('lottery.secondPrize · $100')
    expect(wrapper.text()).not.toContain('lottery.thirdPrize · $50')
  })

  it('renders the blind box and shows a soft opening state during a draw', async () => {
    getStatus.mockResolvedValue({
      available_chances: 2,
      daily_used: 1,
      daily_limit: 3,
    })
    getRecords.mockResolvedValue([])
    draw.mockReturnValue(new Promise(() => {}))

    const wrapper = shallowMount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-testid="lottery-blind-box"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="lottery-prize-machine"]').exists()).toBe(false)
    expect(wrapper.find('.machine-card').exists()).toBe(false)

    await wrapper.get('[data-testid="lottery-draw-button"]').trigger('click')

    expect(wrapper.get('[data-testid="lottery-blind-box"]').classes()).toContain('is-opening')
  })

  it('shows confetti after a successful draw while the blind box opens', async () => {
    getStatus.mockResolvedValue({
      available_chances: 2,
      daily_used: 1,
      daily_limit: 3,
    })
    getRecords.mockResolvedValue([])
    draw.mockResolvedValue({
      id: 1,
      prize_name: '一等奖',
      value: 30,
      code: 'CODE-30',
      created_at: '2026-07-30T00:00:00Z',
    })

    const wrapper = shallowMount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.find('[data-testid="lottery-confetti"]').exists()).toBe(false)

    await wrapper.get('[data-testid="lottery-draw-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="lottery-confetti"]').exists()).toBe(true)
  })
})
