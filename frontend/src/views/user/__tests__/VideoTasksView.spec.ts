import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import VideoTasksView from '../VideoTasksView.vue'
import { VideoContentError } from '@/api/videoTasks'

const listMock = vi.fn()
const fetchContentMock = vi.fn()

vi.mock('@/api/videoTasks', () => {
  class VideoContentError extends Error {
    status: number

    constructor(message: string, status: number) {
      super(message)
      this.status = status
    }
  }
  return {
    list: (...args: unknown[]) => listMock(...args),
    fetchContent: (...args: unknown[]) => fetchContentMock(...args),
    VideoContentError,
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => `fmt:${value}`,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function makeTask(overrides: Record<string, unknown> = {}) {
  return {
    id: 'ppvidtask_1',
    request_id: 'req_1',
    model: 'seedance2.0-933-C',
    status: 'succeeded',
    prompt: '场景：大将军府书房',
    output_count: 1,
    resolution: '720p',
    duration_seconds: 15,
    endpoint: '/v1/videos/generations',
    api_key_name: 'prod-key',
    credits_charged: 6.5,
    credits_held: 10,
    credits_refunded: 3.5,
    total_seconds: 576,
    error_code: '',
    error_message: '',
    can_play: true,
    created_at: '2026-11-06T14:34:08Z',
    finished_at: '2026-11-06T14:43:44Z',
    ...overrides,
  }
}

function page(items: unknown[], total = items.length) {
  return { items, total, page: 1, page_size: 20, pages: 1 }
}

async function renderView() {
  const wrapper = mount(VideoTasksView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        teleport: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('VideoTasksView', () => {
  beforeEach(() => {
    listMock.mockReset()
    fetchContentMock.mockReset()
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    URL.createObjectURL = vi.fn(() => 'blob:mock-video')
    URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders task rows with status and credit charges', async () => {
    listMock.mockResolvedValue(
      page([
        makeTask(),
        makeTask({
          id: 'ppvidtask_2',
          request_id: 'req_2',
          status: 'processing',
          credits_charged: null,
          credits_refunded: null,
          total_seconds: null,
          can_play: false,
        }),
      ])
    )
    const wrapper = await renderView()

    const rows = wrapper.findAll('[data-testid="video-task-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('req_1')
    expect(rows[0].text()).toContain('seedance2.0-933-C')
    expect(rows[0].get('[data-testid="video-task-status"]').text()).toBe('videoTasks.status.succeeded')
    expect(rows[0].get('[data-testid="video-task-charged"]').text()).toContain('6.50')
    expect(rows[0].text()).toContain('00:09:36')
    expect(rows[1].get('[data-testid="video-task-status"]').text()).toBe('videoTasks.status.processing')
    expect(rows[1].get('[data-testid="video-task-charged"]').text()).toContain('videoTasks.held')
  })

  it('shows the empty state when there are no tasks', async () => {
    listMock.mockResolvedValue(page([], 0))
    const wrapper = await renderView()

    expect(wrapper.find('[data-testid="video-task-row"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="video-tasks-empty"]').text()).toContain('videoTasks.empty')
  })

  it('sends the filters and resets to page 1 on search', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="filter-task-id"]').setValue('  abc ')
    await wrapper.get('[data-testid="filter-model"]').setValue('seedance')
    await wrapper.get('[data-testid="filter-status"]').setValue('failed')
    await wrapper.get('[data-testid="filter-start-date"]').setValue('2026-11-01')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    const params = listMock.mock.calls.at(-1)?.[0]
    expect(params).toMatchObject({
      page: 1,
      task_id: 'abc',
      model: 'seedance',
      status: 'failed',
      start_date: '2026-11-01',
    })
  })

  it('opens the detail drawer, shows prompt and refund, and loads the video', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    fetchContentMock.mockResolvedValue(new Blob(['video-bytes'], { type: 'video/mp4' }))
    const wrapper = await renderView()

    expect(wrapper.find('[data-testid="video-task-drawer"]').exists()).toBe(false)
    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="video-task-drawer"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="drawer-task-id"]').text()).toBe('req_1')
    expect(wrapper.get('[data-testid="drawer-prompt"]').text()).toBe('场景：大将军府书房')
    expect(wrapper.get('[data-testid="drawer-charged"]').text()).toContain('6.50')
    expect(wrapper.get('[data-testid="drawer-refunded"]').text()).toContain('3.50')
    expect(fetchContentMock).toHaveBeenCalledTimes(1)
    expect(fetchContentMock.mock.calls[0][0]).toBe('ppvidtask_1')
    expect(wrapper.get('[data-testid="drawer-video"]').attributes('src')).toBe('blob:mock-video')

    await wrapper.get('[data-testid="drawer-close"]').trigger('click')
    expect(wrapper.find('[data-testid="video-task-drawer"]').exists()).toBe(false)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-video')
  })

  it('plays octet-stream responses by forcing a video mime type on the blob', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    fetchContentMock.mockResolvedValue(new Blob(['video-bytes'], { type: 'application/octet-stream' }))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()

    const createdBlob = (URL.createObjectURL as unknown as ReturnType<typeof vi.fn>).mock.calls[0][0] as Blob
    expect(createdBlob.type).toBe('video/mp4')
    expect(wrapper.find('[data-testid="drawer-video"]').exists()).toBe(true)
  })

  it('shows an error with a retry button when the player cannot decode the video', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    fetchContentMock.mockResolvedValue(new Blob(['video-bytes'], { type: 'video/mp4' }))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="drawer-video"]').trigger('error')

    expect(wrapper.find('[data-testid="drawer-video"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="drawer-video-error"]').text()).toContain('videoTasks.detail.videoFailed')
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-video')

    await wrapper.get('[data-testid="drawer-video-retry"]').trigger('click')
    await flushPromises()

    expect(fetchContentMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="drawer-video"]').exists()).toBe(true)
  })

  it('shows the error state when the video request fails', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    fetchContentMock.mockRejectedValue(new Error('boom'))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="drawer-video-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="drawer-video"]').exists()).toBe(false)
  })

  it('explains that the video expired when the upstream no longer has it', async () => {
    listMock.mockResolvedValue(page([makeTask()]))
    fetchContentMock.mockRejectedValue(new VideoContentError('The video is no longer available', 404))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="drawer-video-error"]').text()).toContain('videoTasks.detail.videoExpired')
    expect(wrapper.find('[data-testid="drawer-video-retry"]').exists()).toBe(false)
  })

  it('shows the failure reason of a failed task, or a fallback when the upstream gave none', async () => {
    listMock.mockResolvedValue(
      page([
        makeTask({
          id: 'ppvidtask_f1', request_id: 'req_f1', status: 'failed', can_play: false,
          credits_charged: 0, credits_refunded: 2.24, error_message: 'generation failed',
        }),
        makeTask({
          id: 'ppvidtask_f2', request_id: 'req_f2', status: 'failed', can_play: false,
          credits_charged: 0, credits_refunded: 2.24, error_message: '',
        }),
      ])
    )
    const wrapper = await renderView()
    const buttons = wrapper.findAll('[data-testid="video-task-detail-button"]')

    await buttons[0].trigger('click')
    expect(wrapper.get('[data-testid="drawer-failure-reason"]').text()).toBe('generation failed')

    await wrapper.get('[data-testid="drawer-close"]').trigger('click')
    await buttons[1].trigger('click')
    expect(wrapper.get('[data-testid="drawer-failure-reason"]').text()).toBe('videoTasks.detail.errorUnknown')
    expect(fetchContentMock).not.toHaveBeenCalled()
  })

  it('does not request the video for tasks that cannot be played here', async () => {
    listMock.mockResolvedValue(page([makeTask({ can_play: false })]))
    const wrapper = await renderView()

    await wrapper.get('[data-testid="video-task-detail-button"]').trigger('click')
    await flushPromises()

    expect(fetchContentMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="drawer-result"]').text()).toContain('videoTasks.detail.videoUnavailable')
    expect(wrapper.find('[data-testid="drawer-video"]').exists()).toBe(false)
  })

  it('auto refreshes every 10 seconds only while a task is unfinished', async () => {
    vi.useFakeTimers()
    listMock.mockResolvedValue(page([makeTask({ status: 'processing', credits_charged: null })]))
    const wrapper = await renderView()
    expect(listMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(10_000)
    expect(listMock).toHaveBeenCalledTimes(2)

    listMock.mockResolvedValue(page([makeTask()]))
    await vi.advanceTimersByTimeAsync(10_000)
    expect(listMock).toHaveBeenCalledTimes(3)

    await vi.advanceTimersByTimeAsync(30_000)
    expect(listMock).toHaveBeenCalledTimes(3)

    wrapper.unmount()
  })
})
