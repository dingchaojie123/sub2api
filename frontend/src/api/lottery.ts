/**
 * User lottery API endpoints.
 */

import { apiClient } from './client'
import type { LotteryDrawRecord, LotteryStatus } from '@/types'

export async function getStatus(): Promise<LotteryStatus> {
  const { data } = await apiClient.get<LotteryStatus>('/lottery/status')
  return data
}

export async function draw(): Promise<LotteryDrawRecord> {
  const { data } = await apiClient.post<LotteryDrawRecord>('/lottery/draw')
  return data
}

export async function getRecords(): Promise<LotteryDrawRecord[]> {
  const { data } = await apiClient.get<LotteryDrawRecord[]>('/lottery/records')
  return data
}

export const lotteryAPI = {
  getStatus,
  draw,
  getRecords,
}

export default lotteryAPI
