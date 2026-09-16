<template>
  <DataTable :columns="columns" :data="orders" :loading="loading">
    <template #cell-id="{ value }">
      <span class="font-mono text-sm">#{{ value }}</span>
    </template>
    <template #cell-out_trade_no="{ value }">
      <span class="text-sm text-gray-900 dark:text-white">{{ value }}</span>
    </template>
    <template v-if="showUser" #cell-user_email="{ value, row }">
      <div class="text-sm">
        <span class="text-gray-900 dark:text-white">{{ value || row.user_name || '#' + row.user_id }}</span>
        <span v-if="row.user_notes" class="ml-1 text-xs text-gray-400">({{ row.user_notes }})</span>
      </div>
    </template>
    <template #cell-pay_amount="{ value, row }">
      <div class="text-sm">
        <span class="font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol(row) }}{{ value.toFixed(2) }}</span>
        <span v-if="row.fee_rate > 0" class="ml-1 text-xs text-gray-400" :title="t('payment.orders.fee') + ': ' + row.fee_rate + '%'">
          ({{ t('payment.orders.fee') }} {{ row.fee_rate }}%)
        </span>
        <div v-if="shouldShowCreditedAmount(row)" class="text-xs text-gray-500">
          {{ t('payment.orders.creditedAmount') }}: {{ formatCreditedAmount(row) }}
        </div>
      </div>
    </template>
    <template #cell-payment_type="{ value }">
      <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.methods.' + value, value) }}</span>
    </template>
    <template #cell-status="{ value }">
      <OrderStatusBadge :status="value" />
    </template>
    <template #cell-created_at="{ value }">
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(value) }}</span>
    </template>
    <template #cell-actions="{ row }">
      <slot name="actions" :row="row" />
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentOrder } from '@/types/payment'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import { currencySymbol, formatPaymentAmount } from '@/components/payment/currency'

const i18n = useI18n()
const { t } = i18n

const props = defineProps<{
  orders: PaymentOrder[]
  loading: boolean
  showUser?: boolean
}>()

function formatDate(dateStr: string) { return new Date(dateStr).toLocaleString() }

const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

function paymentAmountSymbol(order: PaymentOrder): string {
  return currencySymbol(order.currency)
}

function formatBalanceAmount(value: number): string {
  if (!Number.isFinite(value)) return '0.00'
  return new Intl.NumberFormat(localeCode.value, {
    minimumFractionDigits: Number.isInteger(value) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(value)
}

function creditedAmountForOrder(order: PaymentOrder): number {
  return typeof order.display_amount === 'number' && Number.isFinite(order.display_amount)
    ? order.display_amount
    : order.amount
}

function shouldShowCreditedAmount(order: PaymentOrder): boolean {
  if (order.order_type === 'balance') {
    return Math.abs(creditedAmountForOrder(order) - order.pay_amount) > 0.000001
  }
  return Math.abs(order.amount - order.pay_amount) > 0.000001
}

function formatCreditedAmount(order: PaymentOrder): string {
  if (order.order_type === 'balance') {
    return `${formatBalanceAmount(creditedAmountForOrder(order))} ${t('payment.balanceUnit')}`
  }
  return formatPaymentAmount(order.amount, 'USD', localeCode.value)
}

const columns = computed((): Column[] => {
  const cols: Column[] = [
    { key: 'id', label: t('payment.orders.orderId') },
    { key: 'out_trade_no', label: t('payment.orders.orderNo') },
  ]
  if (props.showUser) {
    cols.push({ key: 'user_email', label: t('payment.admin.colUser') })
  }
  cols.push(
    { key: 'pay_amount', label: t('payment.orders.payAmount') },
    { key: 'payment_type', label: t('payment.orders.paymentMethod') },
    { key: 'status', label: t('payment.orders.status') },
    { key: 'created_at', label: t('payment.orders.createdAt') },
    { key: 'actions', label: t('common.actions') },
  )
  return cols
})
</script>
