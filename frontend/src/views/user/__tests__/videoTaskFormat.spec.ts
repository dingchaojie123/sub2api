import { describe, expect, it } from 'vitest'
import {
  formatCredits,
  formatTotalSeconds,
  formatVideoDuration,
  hasUnfinishedTasks,
  isUnfinishedStatus,
  videoDownloadName,
  videoTaskStatusMeta,
} from '../videoTaskFormat'

describe('videoTaskFormat', () => {
  it('formats credits with at most 4 decimals and no trailing zeros', () => {
    expect(formatCredits(19)).toBe('19')
    expect(formatCredits(6.5)).toBe('6.50')
    expect(formatCredits(1.62)).toBe('1.62')
    expect(formatCredits(0.19440001)).toBe('0.1944')
    expect(formatCredits(0)).toBe('0')
    expect(formatCredits(2088.12345678)).toBe('2088.1235')
  })

  it('shows a dash for missing credits', () => {
    expect(formatCredits(null)).toBe('-')
    expect(formatCredits(undefined)).toBe('-')
    expect(formatCredits(Number.NaN)).toBe('-')
  })

  it('formats the total time as hh:mm:ss', () => {
    expect(formatTotalSeconds(576)).toBe('00:09:36')
    expect(formatTotalSeconds(3725)).toBe('01:02:05')
    expect(formatTotalSeconds(0)).toBe('00:00:00')
    expect(formatTotalSeconds(null)).toBe('-')
    expect(formatTotalSeconds(-1)).toBe('-')
  })

  it('formats the video duration', () => {
    expect(formatVideoDuration(15)).toBe('15s')
    expect(formatVideoDuration(5.5)).toBe('5.5s')
    expect(formatVideoDuration(0)).toBe('-')
    expect(formatVideoDuration(null)).toBe('-')
  })

  it('maps statuses to a label and tone', () => {
    expect(videoTaskStatusMeta('succeeded')).toEqual({ labelKey: 'succeeded', tone: 'success' })
    expect(videoTaskStatusMeta('failed')).toEqual({ labelKey: 'failed', tone: 'danger' })
    expect(videoTaskStatusMeta('requires_attention')).toEqual({ labelKey: 'requiresAttention', tone: 'warning' })
    expect(videoTaskStatusMeta('processing')).toEqual({ labelKey: 'processing', tone: 'info' })
    expect(videoTaskStatusMeta('whatever')).toEqual({ labelKey: 'unknown', tone: 'info' })
  })

  it('treats only processing tasks as unfinished', () => {
    expect(isUnfinishedStatus('processing')).toBe(true)
    expect(isUnfinishedStatus('succeeded')).toBe(false)
    expect(hasUnfinishedTasks([{ status: 'succeeded' }, { status: 'failed' }])).toBe(false)
    expect(hasUnfinishedTasks([{ status: 'succeeded' }, { status: 'processing' }])).toBe(true)
    expect(hasUnfinishedTasks([])).toBe(false)
  })

  it('builds a safe download file name', () => {
    expect(videoDownloadName({ request_id: 'task/../abc 1', id: 'ppvidtask_1' })).toBe('task_.._abc_1.mp4')
    expect(videoDownloadName({ request_id: '', id: 'ppvidtask_1' })).toBe('ppvidtask_1.mp4')
    expect(videoDownloadName({ request_id: '', id: '' })).toBe('video.mp4')
  })
})
