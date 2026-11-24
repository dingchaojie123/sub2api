import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  memberUsage: vi.fn(),
  memberTasks: vi.fn(),
  memberTaskContent: vi.fn(),
}))

vi.mock('@/api/organizations', () => ({ organizationsAPI: api }))
vi.mock('@/api/videoTasks', () => ({
  VideoContentError: class VideoContentError extends Error {
    status: number
    constructor(message: string, status: number) {
      super(message)
      this.status = status
    }
  },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import OrganizationMemberDetail from '../OrganizationMemberDetail.vue'

const members = [
  { user_id: 3, email: 'a@example.com', username: 'Alice', role: 'member', status: 'active' },
  { user_id: 4, email: 'b@example.com', username: 'Bob', role: 'member', status: 'active' },
] as any

const usage = {
  user_id: 3,
  email: 'a@example.com',
  username: 'Alice',
  summary: { total_requests: 12, total_cost: 30, input_tokens: 100, output_tokens: 200, image_count: 1, video_count: 2 },
  models: [
    { model: 'gpt-5.6-sol', total_requests: 10, total_cost: 20, input_tokens: 100, output_tokens: 200, image_count: 0, video_count: 0, last_used_at: '2026-11-20T10:00:00Z' },
    { model: 'MiniMax-H3', total_requests: 2, total_cost: 10, input_tokens: 0, output_tokens: 0, image_count: 0, video_count: 2, last_used_at: null },
  ],
}

const task = (over: Record<string, unknown>) => ({
  kind: 'video',
  task_id: 'task-1',
  model: 'MiniMax-H3',
  status: 'succeeded',
  prompt: '一只在跳舞的狗',
  item_count: 1,
  success_count: 0,
  resolution: '480p',
  duration_seconds: 5,
  cost: 1,
  error_message: '',
  created_at: '2026-11-20T10:00:00Z',
  finished_at: '2026-11-20T10:05:00Z',
  can_play: true,
  ...over,
})

function mountDetail(props: Record<string, unknown> = {}) {
  return mount(OrganizationMemberDetail, {
    props: { organizationId: 7, members, userId: 3, active: true, ...props },
    global: { stubs: { teleport: true, Pagination: true } },
  })
}

async function openContentTab(wrapper: ReturnType<typeof mountDetail>) {
  const tab = wrapper.findAll('[role="tab"]').find((el) => el.text().includes('生成内容'))
  expect(tab).toBeTruthy()
  await tab!.trigger('click')
}

beforeEach(() => {
  vi.clearAllMocks()
  api.memberUsage.mockResolvedValue(usage)
  api.memberTasks.mockResolvedValue({ items: [task({})], total: 1, page: 1, page_size: 20, pages: 1 })
  api.memberTaskContent.mockResolvedValue(new Blob([new Uint8Array(2048)], { type: 'video/mp4' }))
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:member-video'), revokeObjectURL: vi.fn() }))
})

describe('OrganizationMemberDetail', () => {
  it('loads the selected member and keeps model usage and generated content on separate tabs', async () => {
    const wrapper = mountDetail()
    await flushPromises()

    expect(api.memberUsage).toHaveBeenCalledWith(7, 3, expect.objectContaining({ start: expect.any(String), end: expect.any(String) }))
    expect(api.memberTasks).toHaveBeenCalledWith(7, 3, expect.any(Object), 1, 20)

    // 默认只显示模型用量，不把任务内容堆在同一屏
    expect(wrapper.text()).toContain('gpt-5.6-sol')
    expect(wrapper.text()).toContain('MiniMax-H3')
    expect(wrapper.text()).not.toContain('一只在跳舞的狗')

    await openContentTab(wrapper)
    expect(wrapper.text()).toContain('一只在跳舞的狗')
    expect(wrapper.text()).not.toContain('gpt-5.6-sol')
  })

  it('does not request anything while the tab is not active', async () => {
    mountDetail({ active: false })
    await flushPromises()
    expect(api.memberUsage).not.toHaveBeenCalled()
    expect(api.memberTasks).not.toHaveBeenCalled()
  })

  it('opens a playable video in the drawer and streams it through the member content endpoint', async () => {
    const wrapper = mountDetail()
    await flushPromises()
    await openContentTab(wrapper)

    await wrapper.get('.md-view-play').trigger('click')
    await flushPromises()

    expect(api.memberTaskContent).toHaveBeenCalledWith(7, 3, 'task-1', expect.objectContaining({ signal: expect.anything() }))
    const video = wrapper.get('[data-testid="member-task-video"]')
    expect(video.attributes('src')).toBe('blob:member-video')
    expect(wrapper.get('[data-testid="member-task-prompt"]').text()).toBe('一只在跳舞的狗')
  })

  it('shows the prompt but does not fetch a video that cannot be played here', async () => {
    api.memberTasks.mockResolvedValue({
      items: [task({ task_id: 'task-2', can_play: false }), task({ kind: 'image_batch', task_id: 'batch-1', model: 'gemini-image', prompt: 'a\nb', item_count: 3 })],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = mountDetail()
    await flushPromises()
    await openContentTab(wrapper)

    await wrapper.findAll('.md-row')[0].trigger('click')
    await flushPromises()
    expect(api.memberTaskContent).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="member-task-video"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="member-task-result"]').text()).toContain('videoTasks.detail.videoUnavailable')

    await wrapper.findAll('.md-row')[1].trigger('click')
    await flushPromises()
    // 批量图片任务没有视频成果区，只有提示词预览
    expect(wrapper.find('[data-testid="member-task-result"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="member-task-prompt"]').text()).toBe('a\nb')
  })

  it('asks the parent to pick another member when the selected one is not in the team', async () => {
    const wrapper = mountDetail({ userId: 99 })
    await flushPromises()
    expect(wrapper.emitted('update:userId')?.[0]).toEqual([3])
    expect(api.memberUsage).not.toHaveBeenCalled()
  })
})
