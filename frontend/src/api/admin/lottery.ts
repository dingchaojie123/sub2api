/**
 * Admin Lottery API endpoints
 * Handles prize pool management for administrators.
 */

import { apiClient } from '../client'
import type { LotteryPoolSummary } from '@/types'

export async function getPool(): Promise<LotteryPoolSummary[]> {
  const { data } = await apiClient.get<LotteryPoolSummary[]>('/admin/lottery/pool')
  return data
}

export async function bindPool(ids: number[]): Promise<{ bound: number }> {
  const { data } = await apiClient.post<{ bound: number }>('/admin/lottery/pool/bind', { ids })
  return data
}

export async function unbindPool(ids: number[]): Promise<{ unbound: number }> {
  const { data } = await apiClient.post<{ unbound: number }>('/admin/lottery/pool/unbind', { ids })
  return data
}

export const lotteryAPI = {
  getPool,
  bindPool,
  unbindPool
}

export default lotteryAPI
