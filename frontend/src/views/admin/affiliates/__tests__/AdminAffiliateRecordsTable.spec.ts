import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import AdminAffiliateRecordsTable from '../AdminAffiliateRecordsTable.vue'

const listInviteRecords = vi.hoisted(() => vi.fn())
const listSecondLevelRecords = vi.hoisted(() => vi.fn())
const listRebateRecords = vi.hoisted(() => vi.fn())
const listTransferRecords = vi.hoisted(() => vi.fn())
const getUserOverview = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI: {
    listInviteRecords,
    listSecondLevelRecords,
    listRebateRecords,
    listTransferRecords,
    getUserOverview,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

// 极简 DataTable：按 columns 渲染表头，并把每个单元格交给 cell-<key> 插槽。
const DataTableStub = defineComponent({
  props: { columns: { type: Array, default: () => [] }, data: { type: Array, default: () => [] } },
  template: `
    <table>
      <thead><tr><th v-for="c in columns" :key="c.key" :data-col="c.key">{{ c.label }}</th></tr></thead>
      <tbody>
        <tr v-for="(row, i) in data" :key="i">
          <td v-for="c in columns" :key="c.key"><slot :name="'cell-' + c.key" :row="row" :value="row[c.key]" /></td>
        </tr>
      </tbody>
    </table>`,
})

const secondLevelRow = {
  root_id: 11,
  root_email: 'root@example.com',
  root_username: 'rootname',
  via_id: 22,
  via_email: 'via@example.com',
  via_username: 'vianame',
  invitee_id: 33,
  invitee_email: 'second@example.com',
  invitee_username: 'secondname',
  created_at: '2026-08-01T00:00:00Z',
}

function mountTable(type: 'invites' | 'second-level') {
  return mount(AdminAffiliateRecordsTable, {
    props: { type },
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
        },
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: true,
        Icon: true,
      },
    },
  })
}

describe('AdminAffiliateRecordsTable second-level mode', () => {
  beforeEach(() => {
    for (const fn of [listInviteRecords, listSecondLevelRecords, listRebateRecords, listTransferRecords, getUserOverview]) {
      fn.mockReset()
    }
    localStorage.clear()
    listSecondLevelRecords.mockResolvedValue({ items: [secondLevelRow], total: 1, page: 1, page_size: 20 })
    listInviteRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  })

  it('loads from the second-level endpoint only', async () => {
    mountTable('second-level')
    await flushPromises()

    expect(listSecondLevelRecords).toHaveBeenCalledTimes(1)
    expect(listSecondLevelRecords.mock.calls[0][0]).toMatchObject({ page: 1, page_size: 20 })
    expect(listInviteRecords).not.toHaveBeenCalled()
    expect(listRebateRecords).not.toHaveBeenCalled()
    expect(listTransferRecords).not.toHaveBeenCalled()
  })

  it('renders root → level-1 → level-2 columns without a rebate column', async () => {
    const wrapper = mountTable('second-level')
    await flushPromises()

    const cols = wrapper.findAll('th').map((th) => th.attributes('data-col'))
    expect(cols).toEqual(['root', 'via', 'invitee', 'created_at'])

    const text = wrapper.text()
    expect(text).toContain('root@example.com')
    expect(text).toContain('via@example.com')
    expect(text).toContain('second@example.com')
    expect(text).toContain('#11')
    expect(text).toContain('#22')
    expect(text).toContain('#33')
    expect(text).not.toContain('admin.affiliates.records.totalRebate')
  })

  it('opens the affiliate overview for the root, level-1 and level-2 users', async () => {
    getUserOverview.mockResolvedValue({
      user_id: 11,
      email: 'root@example.com',
      username: 'rootname',
      aff_code: 'ROOTCODE',
      rebate_rate_percent: 5,
      invited_count: 2,
      second_level_count: 3,
      rebated_invitee_count: 1,
      available_quota: 0,
      history_quota: 0,
    })
    const wrapper = mountTable('second-level')
    await flushPromises()

    const clickEmail = async (email: string) => {
      const btn = wrapper.findAll('button').find((b) => b.text() === email)
      expect(btn, `button for ${email}`).toBeDefined()
      await btn!.trigger('click')
      await flushPromises()
    }

    await clickEmail('root@example.com')
    await clickEmail('via@example.com')
    await clickEmail('second@example.com')

    expect(getUserOverview.mock.calls.map((c) => c[0])).toEqual([11, 22, 33])
  })

  it('keeps the invites type on the invites endpoint', async () => {
    mountTable('invites')
    await flushPromises()

    expect(listInviteRecords).toHaveBeenCalledTimes(1)
    expect(listSecondLevelRecords).not.toHaveBeenCalled()
  })
})
