import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const { listRedeemCodes, generateRedeemCodes, getLotteryPool, getAllGroups, showSuccess, showError, showInfo } =
  vi.hoisted(() => ({
    listRedeemCodes: vi.fn(),
    generateRedeemCodes: vi.fn(),
    getLotteryPool: vi.fn(),
    getAllGroups: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list: listRedeemCodes,
      generate: generateRedeemCodes,
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
      bindPool: vi.fn(),
      unbindPool: vi.fn()
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
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key">{{ column.label }}</th>
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
  template: `<select v-bind="$attrs"></select>`
}

const mountView = () =>
  mount(RedeemView, {
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

type Wrapper = ReturnType<typeof mountView>

const openGenerateDialog = async (wrapper: Wrapper) => {
  const button = wrapper.findAll('button').find((b) => b.text() === 'admin.redeem.generateCodes')
  expect(button).toBeTruthy()
  await button!.trigger('click')
  await flushPromises()
}

const inputValue = (wrapper: Wrapper, selector: string) =>
  (wrapper.get(selector).element as HTMLInputElement).value

const enableDiscount = async (wrapper: Wrapper) => {
  await wrapper.get('[data-test="discount-mode-toggle"]').setValue(true)
  await flushPromises()
}

describe('admin RedeemView discount codes', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''

    listRedeemCodes.mockReset()
    generateRedeemCodes.mockReset()
    getLotteryPool.mockReset()
    getAllGroups.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    showInfo.mockReset()

    listRedeemCodes.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'CODE-1',
          type: 'balance',
          value: 1666.67,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null,
          notes: '张三 · 实付 1000，面值 1666.67（6折）'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    generateRedeemCodes.mockResolvedValue([
      {
        id: 11,
        code: 'NEW-CODE-1',
        type: 'balance',
        value: 1666.67,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-01-02T00:00:00Z'
      }
    ])
    getLotteryPool.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('shows the notes column so the customer and amount paid are visible in the list', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.redeem.columns.notes')
    expect(wrapper.text()).toContain('张三 · 实付 1000，面值 1666.67（6折）')
  })

  it('calculates the credit value from the amount paid and the discount', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await flushPromises()

    expect(inputValue(wrapper, '[data-test="discount-credits"]')).toBe('1666.67')
    expect(wrapper.get('[data-test="discount-summary"]').text()).toBe('admin.redeem.discount.summary')
  })

  it('submits pay_amount, the credit value and a note describing the discount', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await wrapper.get('[data-test="generate-notes"]').setValue('张三')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledTimes(1)
    expect(generateRedeemCodes).toHaveBeenCalledWith(
      1,
      'balance',
      1666.67,
      undefined,
      undefined,
      undefined,
      { payAmount: 1000, notes: '张三 · 实付 1000，面值 1666.67（6折）' }
    )
    expect(wrapper.get('[data-test="generated-summary"]').text()).toBe(
      'admin.redeem.discount.resultSummary'
    )
  })

  it('lets the credit value be adjusted by hand and records the effective discount', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await wrapper.get('[data-test="discount-credits"]').setValue('1667')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledWith(
      1,
      'balance',
      1667,
      undefined,
      undefined,
      undefined,
      { payAmount: 1000, notes: '实付 1000，面值 1667（6折）' }
    )
  })

  it('leaves regular balance codes untouched when discount mode is off', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)

    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledWith(
      1,
      'balance',
      10,
      undefined,
      undefined,
      undefined,
      { notes: '' }
    )
    expect(wrapper.find('[data-test="generated-summary"]').exists()).toBe(false)
  })

  it('rejects a discount code without an amount paid', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.redeem.discount.payAmountRequired')
    expect(generateRedeemCodes).not.toHaveBeenCalled()
  })

  it('warns and blocks submit when the discount is entered as a ratio (0.6 instead of 6)', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('0.6')
    await flushPromises()

    expect(wrapper.get('[data-test="discount-rate-warning"]').text()).toBe(
      'admin.redeem.discount.rateLooksLikeRatio'
    )
    // 不会算出 16666.67 这样的离谱面值
    expect(inputValue(wrapper, '[data-test="discount-credits"]')).toBe('0')

    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.redeem.discount.rateInvalid')
    expect(generateRedeemCodes).not.toHaveBeenCalled()

    // 改成 6 之后警告消失，面值恢复正确
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await flushPromises()
    expect(wrapper.find('[data-test="discount-rate-warning"]').exists()).toBe(false)
    expect(inputValue(wrapper, '[data-test="discount-credits"]')).toBe('1666.67')
  })

  it('rejects an out-of-range discount', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('11')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.redeem.discount.rateInvalid')
    expect(generateRedeemCodes).not.toHaveBeenCalled()
  })

  it('rejects a credit value below the amount paid', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await wrapper.get('[data-test="discount-credits"]').setValue('900')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.redeem.discount.creditsBelowPay')
    expect(generateRedeemCodes).not.toHaveBeenCalled()
  })

  it('restores the plain value input when discount mode is turned off', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openGenerateDialog(wrapper)
    await enableDiscount(wrapper)

    await wrapper.get('[data-test="discount-pay-amount"]').setValue('1000')
    await wrapper.get('[data-test="discount-rate"]').setValue('6')
    await wrapper.get('[data-test="discount-mode-toggle"]').setValue(false)
    await flushPromises()

    expect(wrapper.find('[data-test="discount-credits"]').exists()).toBe(false)

    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'balance', 10, undefined, undefined, undefined, {
      notes: ''
    })
  })
})
