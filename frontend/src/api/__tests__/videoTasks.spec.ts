import { beforeEach, describe, expect, it, vi } from 'vitest'

const getMock = vi.fn()

vi.mock('../client', () => ({
  apiClient: { get: (...args: unknown[]) => getMock(...args) },
}))

import { fetchContent, VideoContentError } from '../videoTasks'

function videoBlob(size: number) {
  return new Blob([new Uint8Array(size)], { type: 'video/mp4' })
}

async function failure(promise: Promise<unknown>): Promise<VideoContentError> {
  try {
    await promise
  } catch (error) {
    return error as VideoContentError
  }
  throw new Error('expected the request to fail')
}

describe('videoTasks.fetchContent', () => {
  beforeEach(() => getMock.mockReset())

  it('returns the blob for a complete 200 response', async () => {
    const blob = videoBlob(4096)
    getMock.mockResolvedValue({ status: 200, data: blob, headers: { 'content-length': '4096' } })

    await expect(fetchContent('ppvidtask_1')).resolves.toBe(blob)
    const [url, config] = getMock.mock.calls[0]
    expect(url).toBe('/video-tasks/ppvidtask_1/content')
    expect(config.responseType).toBe('blob')
  })

  it('accepts a complete response that has no content-length header', async () => {
    const blob = videoBlob(2048)
    getMock.mockResolvedValue({ status: 200, data: blob, headers: {} })

    await expect(fetchContent('ppvidtask_1')).resolves.toBe(blob)
  })

  it('rejects partial content instead of handing a broken blob to the player', async () => {
    getMock.mockResolvedValue({
      status: 206,
      data: videoBlob(1),
      headers: { 'content-length': '1', 'content-range': 'bytes 2617596-2617596/3023409' },
    })

    const error = await failure(fetchContent('ppvidtask_1'))
    expect(error).toBeInstanceOf(VideoContentError)
    expect(error.status).toBe(502)
  })

  it('rejects a body that is far too small to be a video', async () => {
    getMock.mockResolvedValue({ status: 200, data: videoBlob(40), headers: {} })

    await expect(fetchContent('ppvidtask_1')).rejects.toBeInstanceOf(VideoContentError)
  })

  it('rejects a truncated body whose size differs from content-length', async () => {
    getMock.mockResolvedValue({ status: 200, data: videoBlob(2048), headers: { 'content-length': '3023409' } })

    await expect(fetchContent('ppvidtask_1')).rejects.toBeInstanceOf(VideoContentError)
  })

  it('turns HTTP errors into a VideoContentError that keeps the status', async () => {
    getMock.mockResolvedValue({ status: 404, data: new Blob(['{"message":"gone"}']), headers: {} })

    const error = await failure(fetchContent('ppvidtask_1'))
    expect(error).toBeInstanceOf(VideoContentError)
    expect(error.status).toBe(404)
  })

  it('encodes the task id in the url', async () => {
    getMock.mockResolvedValue({ status: 200, data: videoBlob(2048), headers: {} })

    await fetchContent('a/b c')

    expect(getMock.mock.calls[0][0]).toBe('/video-tasks/a%2Fb%20c/content')
  })
})
