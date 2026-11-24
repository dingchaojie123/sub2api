import { describe, expect, it } from 'vitest'
import { userDisplayBalance, userFrozenCredits } from '../balance'

describe('userFrozenCredits', () => {
  it('uses frozen_display_balance (credits), not the real-money frozen_balance', () => {
    // 一笔 19 积分的任务，真实余额只冻结了 14.61
    const user = { balance: 78, display_balance: 85.56, frozen_balance: 14.61, frozen_display_balance: 19 }

    expect(userFrozenCredits(user)).toBe(19)
    expect(userDisplayBalance(user) + userFrozenCredits(user)).toBeCloseTo(104.56, 6)
  })

  it('falls back to frozen_balance when the backend does not send frozen_display_balance', () => {
    expect(userFrozenCredits({ frozen_balance: 3 })).toBe(3)
  })

  it('is 0 for a missing user or missing fields', () => {
    expect(userFrozenCredits(null)).toBe(0)
    expect(userFrozenCredits(undefined)).toBe(0)
    expect(userFrozenCredits({})).toBe(0)
  })
})
