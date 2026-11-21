import { beforeEach, describe, expect, it, vi } from 'vitest'

const getMock = vi.fn()

vi.mock('../../client', () => ({
  apiClient: { get: (...args: unknown[]) => getMock(...args) },
}))

import { VideoContentError } from '../../videoTasks'
import { fetchContent, list } from '../videoTasks'

describe('admin videoTasks.list', () => {
  beforeEach(() => getMock.mockReset())

  it('calls the admin endpoint and drops empty filters', async () => {
    const page = { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
    getMock.mockResolvedValue({ data: page })

    const result = await list({ page: 1, page_size: 20, user: '', user_id: 42, status: '', platform: 'seedance' })

    expect(result).toBe(page)
    const [url, config] = getMock.mock.calls[0]
    expect(url).toBe('/admin/video-tasks')
    expect(config.params).toEqual({ page: 1, page_size: 20, user_id: 42, platform: 'seedance' })
  })

  it('passes the abort signal through', async () => {
    getMock.mockResolvedValue({ data: { items: [], total: 0 } })
    const controller = new AbortController()

    await list({}, { signal: controller.signal })

    expect(getMock.mock.calls[0][1].signal).toBe(controller.signal)
  })
})

describe('admin videoTasks.fetchContent', () => {
  beforeEach(() => getMock.mockReset())

  it('uses the admin content endpoint and encodes the task id', async () => {
    const blob = new Blob([new Uint8Array(2048)], { type: 'video/mp4' })
    getMock.mockResolvedValue({ status: 200, data: blob, headers: {} })

    await expect(fetchContent('a/b c')).resolves.toBe(blob)

    expect(getMock.mock.calls[0][0]).toBe('/admin/video-tasks/a%2Fb%20c/content')
  })

  it('keeps the HTTP status of a failed download', async () => {
    getMock.mockResolvedValue({ status: 404, data: new Blob(['{"message":"gone"}']), headers: {} })

    await expect(fetchContent('ppvidtask_1')).rejects.toMatchObject({
      status: 404,
      name: VideoContentError.name,
    })
  })
})
