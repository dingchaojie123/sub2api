/**
 * Video task records API (logged-in user, own tasks only)
 * Backend: GET /api/v1/video-tasks, GET /api/v1/video-tasks/:id/content
 */

import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export type VideoTaskStatus = 'processing' | 'succeeded' | 'failed' | 'requires_attention'

export interface VideoTask {
  /** Local task id; the key for the content endpoint. */
  id: string
  /** The id users see and can use against the public API (upstream id, else local id). */
  request_id: string
  model: string
  status: VideoTaskStatus | string
  prompt: string
  output_count: number
  resolution: string
  duration_seconds: number
  endpoint: string
  api_key_name: string
  /** Credits actually charged; null until the task is settled. */
  credits_charged: number | null
  /** Credits frozen when the task was submitted. */
  credits_held: number
  /** Credits returned to the account; null when nothing was refunded or not settled yet. */
  credits_refunded: number | null
  /** Seconds from creation to finish; null while unfinished. */
  total_seconds: number | null
  error_code: string
  error_message: string
  /** The result video can be streamed through this site. */
  can_play: boolean
  created_at: string
  finished_at: string | null
}

export interface VideoTaskListParams {
  page?: number
  page_size?: number
  task_id?: string
  model?: string
  status?: VideoTaskStatus | ''
  api_key?: string
  /** YYYY-MM-DD, inclusive. */
  start_date?: string
  /** YYYY-MM-DD, inclusive. */
  end_date?: string
}

export async function list(
  params: VideoTaskListParams = {},
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<VideoTask>> {
  const { data } = await apiClient.get<PaginatedResponse<VideoTask>>('/video-tasks', {
    params: Object.fromEntries(
      Object.entries(params).filter(([, value]) => value !== undefined && value !== '')
    ),
    signal: options?.signal
  })
  return data
}

/** The content endpoint failed. `status` is the HTTP status; 404 means the video is gone upstream. */
export class VideoContentError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'VideoContentError'
    this.status = status
  }
}

async function readErrorMessage(data: unknown): Promise<string> {
  try {
    const text = data instanceof Blob ? await data.text() : String(data ?? '')
    const parsed = JSON.parse(text) as { message?: unknown }
    return typeof parsed.message === 'string' ? parsed.message : ''
  } catch {
    return ''
  }
}

/** A real video is never smaller than this; anything below it is an error body or a truncated read. */
const MIN_VIDEO_BYTES = 1024

/**
 * Fetch the result video as a Blob so it can be played or saved with the user's session.
 * Non-2xx responses are turned into a VideoContentError carrying the status and server message.
 * Partial or truncated bodies are rejected too: handing a broken blob to the player only shows a black screen.
 */
export async function fetchContent(id: string, options?: { signal?: AbortSignal }): Promise<Blob> {
  const response = await apiClient.get<Blob>(`/video-tasks/${encodeURIComponent(id)}/content`, {
    responseType: 'blob',
    timeout: 5 * 60 * 1000,
    signal: options?.signal,
    // Keep error bodies (blobs) readable instead of letting the shared interceptor drop them.
    validateStatus: () => true
  })
  if (response.status < 200 || response.status >= 300) {
    throw new VideoContentError(await readErrorMessage(response.data), response.status)
  }
  const blob = response.data
  const expected = Number(response.headers?.['content-length'])
  if (
    response.status !== 200 ||
    !(blob instanceof Blob) ||
    blob.size < MIN_VIDEO_BYTES ||
    (Number.isFinite(expected) && expected > 0 && blob.size !== expected)
  ) {
    throw new VideoContentError('Incomplete video data', 502)
  }
  return blob
}

export const videoTasksAPI = { list, fetchContent }

export default videoTasksAPI
