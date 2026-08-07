import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const {
  listRedeemCodes,
  getLotteryPool,
  bindLotteryPool,
  getAllGroups,
  showSuccess,
  showError,
  showInfo
} = vi.hoisted(() => ({
  listRedeemCodes: vi.fn(),
  getLotteryPool: vi.fn(),
  bindLotteryPool: vi.fn(),
  getAllGroups: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list: listRedeemCodes,
      generate: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      batchUpdate: vi.fn(),
      exportCodes: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    },
    lottery: {
      getPool: getLotteryPool,
      bindPool: bindLotteryPool
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key} ${JSON.stringify(params)}` : key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key">
            <slot :name="'header-' + column.key" :column="column">{{ column.label }}</slot>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  setup(
    props: { options: Array<{ value: unknown; label: string }> },
    { emit }: { emit: (event: string, ...args: unknown[]) => void }
  ) {
    const onChange = (event: Event) => {
      const raw = (event.target as HTMLSelectElement).value
      const option = props.options.find((item) => String(item.value ?? '') === raw)
      const value = option ? option.value : raw
      emit('update:modelValue', value)
      emit('change', value, option ?? null)
    }
    return { onChange }
  },
  template: `
    <select v-bind="$attrs" :value="modelValue ?? ''" @change="onChange">
      <option v-for="option in options" :key="String(option.value ?? '')" :value="option.value ?? ''">
        {{ option.label }}
      </option>
    </select>
  `
}

describe('admin RedeemView lottery pool', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''

    listRedeemCodes.mockReset()
    getLotteryPool.mockReset()
    bindLotteryPool.mockReset()
    getAllGroups.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    showInfo.mockReset()

    listRedeemCodes.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'BALANCE-30',
          type: 'balance',
          value: 30,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 2,
          code: 'BALANCE-10',
          type: 'balance',
          value: 10,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 3,
          code: 'BALANCE-5',
          type: 'balance',
          value: 5,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 4,
          code: 'BALANCE-2',
          type: 'balance',
          value: 2,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 5,
          code: 'BALANCE-300',
          type: 'balance',
          value: 300,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 6,
          code: 'BALANCE-20',
          type: 'balance',
          value: 20,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        }
      ],
      total: 6,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getLotteryPool.mockResolvedValue([
      {
        prize_name: 'First Prize',
        value: 30,
        available: 3,
        assigned: 1
      }
    ])
    bindLotteryPool.mockResolvedValue({ bound: 1 })
    getAllGroups.mockResolvedValue([])
  })

  it('shows pool summary and binds only selected eligible redeem codes', async () => {
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.get('[data-test="lottery-pool-summary"]').text()).toContain('First Prize')
    expect(wrapper.get('[data-test="lottery-pool-summary"]').text()).toContain('30')
    for (const checkbox of wrapper.findAll('[data-test="select-code"]')) {
      await checkbox.setValue(true)
    }

    expect(wrapper.get('[data-test="lottery-eligible-count"]').text()).toContain('"count":4')
    expect(wrapper.get('[data-test="lottery-ineligible-hint"]').text()).toContain('"count":2')

    await wrapper.get('[data-test="lottery-bind-selected"]').trigger('click')
    await flushPromises()

    expect(bindLotteryPool).toHaveBeenCalledWith([1, 2, 3, 4])
    expect(listRedeemCodes).toHaveBeenCalledTimes(2)
    expect(getLotteryPool).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('admin.redeem.lottery.bindSuccess {"count":1}')
  })
})
