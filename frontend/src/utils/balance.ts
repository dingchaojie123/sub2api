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
