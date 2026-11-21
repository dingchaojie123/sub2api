/**
 * Admin video task API (all customers' tasks).
 * Backend: GET /api/v1/admin/video-tasks, GET /api/v1/admin/video-tasks/:id/content
 */

import { apiClient } from '../client'
import { fetchVideoBlob, type VideoTask, type VideoTaskStatus } from '../videoTasks'
import type { PaginatedResponse } from '@/types'

/** A customer's video task: the user-facing view plus ownership and upstream routing info. */
export interface AdminVideoTask extends VideoTask {
  /** Upstream task id; empty until the upstream accepted the task. */
  upstream_task_id: string
  platform: string
  billing_status: string
  user_id: number
  user_email: string
  username: string
  api_key_id: number
  account_id: number
  account_name: string
  group_id: number | null
  group_name: string
}

export interface AdminVideoTaskListParams {
  page?: number
  page_size?: number
  /** Exact user id. */
  user_id?: number
  /** Substring of the user's email or username. */
  user?: string
  /** Substring of the platform identifier. */
  platform?: string
  task_id?: string
  model?: string
  status?: VideoTaskStatus | ''
  /** Substring of the API key name. */
  api_key?: string
  /** YYYY-MM-DD, inclusive. */
  start_date?: string
  /** YYYY-MM-DD, inclusive. */
  end_date?: string
  timezone?: string
}

export async function list(
  params: AdminVideoTaskListParams = {},
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminVideoTask>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminVideoTask>>('/admin/video-tasks', {
    params: Object.fromEntries(
      Object.entries(params).filter(([, value]) => value !== undefined && value !== '')
    ),
    signal: options?.signal
  })
  return data
}

export async function fetchContent(id: string, options?: { signal?: AbortSignal }): Promise<Blob> {
  return fetchVideoBlob(`/admin/video-tasks/${encodeURIComponent(id)}/content`, options)
}

export const videoTasksAPI = { list, fetchContent }

export default videoTasksAPI
