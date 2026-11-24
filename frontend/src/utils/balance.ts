import type { User } from '@/types'

type UserBalanceLike = Pick<User, 'balance' | 'display_balance'>

export function userDisplayBalance(user: UserBalanceLike | null | undefined): number {
  const displayBalance = user?.display_balance
  if (typeof displayBalance === 'number' && Number.isFinite(displayBalance)) {
    return displayBalance
  }
  const balance = user?.balance
  return typeof balance === 'number' && Number.isFinite(balance) ? balance : 0
}

/**
 * 冻结积分，与 userDisplayBalance 同一口径。frozen_balance 是真实余额口径，
 * 在充值有加送的账号上比积分小，不能和可用积分相加。
 * 后端没返回 frozen_display_balance（旧版本）时退回 frozen_balance。
 */
export function userFrozenCredits(
  user: Pick<User, 'frozen_balance' | 'frozen_display_balance'> | null | undefined,
): number {
  const frozenDisplay = user?.frozen_display_balance
  if (typeof frozenDisplay === 'number' && Number.isFinite(frozenDisplay)) {
    return frozenDisplay
  }
  const frozen = user?.frozen_balance
  return typeof frozen === 'number' && Number.isFinite(frozen) ? frozen : 0
}
