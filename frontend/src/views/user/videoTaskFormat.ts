import type { VideoTask } from '@/api/videoTasks'

/** Statuses that mean the task is still running and worth polling. */
export function isUnfinishedStatus(status: string): boolean {
  return status === 'processing'
}

export function hasUnfinishedTasks(tasks: Pick<VideoTask, 'status'>[]): boolean {
  return tasks.some((task) => isUnfinishedStatus(task.status))
}

export type VideoTaskStatusTone = 'success' | 'danger' | 'warning' | 'info'

export interface VideoTaskStatusMeta {
  /** Key under `videoTasks.status.*` in the i18n messages. */
  labelKey: 'processing' | 'succeeded' | 'failed' | 'requiresAttention' | 'unknown'
  tone: VideoTaskStatusTone
}

export function videoTaskStatusMeta(status: string): VideoTaskStatusMeta {
  switch (status) {
    case 'succeeded':
      return { labelKey: 'succeeded', tone: 'success' }
    case 'failed':
      return { labelKey: 'failed', tone: 'danger' }
    case 'requires_attention':
      return { labelKey: 'requiresAttention', tone: 'warning' }
    case 'processing':
      return { labelKey: 'processing', tone: 'info' }
    default:
      return { labelKey: 'unknown', tone: 'info' }
  }
}

/**
 * Credits are stored with up to 8 decimals. Show at most 4 and drop trailing zeros,
 * keeping at least 2 decimals for amounts that are not whole numbers (6.5 -> "6.50").
 */
export function formatCredits(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-'
  const fixed = value.toFixed(4)
  const trimmed = fixed.replace(/0+$/, '').replace(/\.$/, '')
  const decimals = trimmed.includes('.') ? trimmed.split('.')[1].length : 0
  if (decimals === 0) return trimmed
  return decimals < 2 ? Number(trimmed).toFixed(2) : trimmed
}

/** 576 -> "00:09:36" (hh:mm:ss). */
export function formatTotalSeconds(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds) || seconds < 0) return '-'
  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(hours)}:${pad(minutes)}:${pad(secs)}`
}

/** 15 -> "15s", 5.5 -> "5.5s", 0 -> "-". */
export function formatVideoDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds) || seconds <= 0) return '-'
  const rounded = Math.round(seconds * 100) / 100
  return `${rounded}s`
}

/** Download name for a result video, safe for common file systems. */
export function videoDownloadName(task: Pick<VideoTask, 'request_id' | 'id'>): string {
  const base = (task.request_id || task.id || 'video').replace(/[^A-Za-z0-9._-]+/g, '_')
  return `${base}.mp4`
}
