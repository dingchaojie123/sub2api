<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('enterprise.admin.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('enterprise.admin.description') }}</p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <input
          v-model="search"
          type="text"
          class="input w-full sm:w-80"
          :placeholder="t('enterprise.admin.searchPlaceholder')"
          @input="onSearchInput"
        />
        <select v-model="status" class="input w-44" @change="resetAndLoad">
          <option value="">{{ t('enterprise.admin.allStatus') }}</option>
          <option v-for="item in statusOptions" :key="item" :value="item">
            {{ t(`enterprise.status.${item}`) }}
          </option>
        </select>
        <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.company') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.creditCode') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.contact') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.account') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.status') }}</th>
                <th class="px-4 py-3 font-medium">{{ t('enterprise.admin.columns.submittedAt') }}</th>
                <th class="px-4 py-3 text-right font-medium">{{ t('enterprise.admin.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="!loading && items.length === 0">
                <td colspan="7" class="px-4 py-12 text-center text-gray-500 dark:text-gray-400">
                  {{ t('enterprise.admin.empty') }}
                </td>
              </tr>
              <tr v-for="item in items" :key="item.user_id" class="text-gray-700 dark:text-gray-300">
                <td class="max-w-[16rem] truncate px-4 py-3 font-medium text-gray-900 dark:text-white" :title="item.company_name">
                  {{ item.company_name || '-' }}
                </td>
                <td class="px-4 py-3 font-mono text-xs">{{ item.credit_code || '-' }}</td>
                <td class="px-4 py-3">
                  <div>{{ item.contact_name || '-' }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.contact_phone }}</div>
                </td>
                <td class="px-4 py-3">{{ item.email }}</td>
                <td class="px-4 py-3">
                  <EnterpriseStatusBadge :status="item.verify_status" :title="item.reject_reason || undefined" />
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-xs">{{ formatTime(item.created_at) }}</td>
                <td class="whitespace-nowrap px-4 py-3 text-right">
                  <button class="link-button mr-3" @click="detail = item">{{ t('enterprise.admin.view') }}</button>
                  <button
                    v-if="item.verify_status !== 'approved'"
                    class="link-button mr-3 disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="item.verify_status === 'incomplete'"
                    :title="item.verify_status === 'incomplete' ? t('enterprise.admin.cannotApproveIncomplete') : undefined"
                    @click="approveTarget = item"
                  >
                    {{ t('enterprise.admin.approve') }}
                  </button>
                  <button
                    v-if="item.verify_status !== 'rejected' && item.verify_status !== 'incomplete'"
                    class="text-sm font-medium text-red-600 hover:underline dark:text-red-400"
                    @click="openReject(item)"
                  >
                    {{ t('enterprise.admin.reject') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="total > 0"
          :page="page"
          :page-size="pageSize"
          :total="total"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </div>
    </div>

    <!-- 详情 -->
    <BaseDialog :show="!!detail" :title="detail?.company_name || t('enterprise.admin.title')" width="narrow" @close="detail = null">
      <dl v-if="detail" class="space-y-3 text-sm">
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.fields.creditCode') }}</dt>
          <dd class="mt-0.5 font-mono">{{ detail.credit_code || '-' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.admin.address') }}</dt>
          <dd class="mt-0.5 break-all">{{ detail.address || '-' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.fields.contactName') }}</dt>
          <dd class="mt-0.5">{{ detail.contact_name || '-' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.admin.phone') }}</dt>
          <dd class="mt-0.5">{{ detail.contact_phone || '-' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.admin.columns.account') }}</dt>
          <dd class="mt-0.5">{{ detail.email }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">{{ t('enterprise.admin.columns.status') }}</dt>
          <dd class="mt-0.5"><EnterpriseStatusBadge :status="detail.verify_status" /></dd>
        </div>
        <div v-if="detail.reject_reason">
          <dt class="text-xs text-gray-500">{{ t('enterprise.profile.rejectReason') }}</dt>
          <dd class="mt-0.5 text-red-600 dark:text-red-400">{{ detail.reject_reason }}</dd>
        </div>
      </dl>
    </BaseDialog>

    <!-- 通过确认 -->
    <ConfirmDialog
      :show="!!approveTarget"
      :title="t('enterprise.admin.approve')"
      :message="t('enterprise.admin.approveConfirm', { name: approveTarget?.company_name || '' })"
      :confirm-text="t('enterprise.admin.approve')"
      :cancel-text="t('enterprise.profile.cancel')"
      @confirm="approve"
      @cancel="approveTarget = null"
    />

    <!-- 驳回 -->
    <BaseDialog :show="!!rejectTarget" :title="t('enterprise.admin.rejectTitle')" width="narrow" @close="closeReject">
      <form id="reject-enterprise-form" class="space-y-2" @submit.prevent="reject">
        <label class="input-label" for="reject-reason">{{ t('enterprise.admin.rejectReasonLabel') }}</label>
        <textarea
          id="reject-reason"
          v-model="rejectReason"
          rows="4"
          maxlength="200"
          class="input w-full"
          :placeholder="t('enterprise.admin.rejectReasonPlaceholder')"
        />
        <p v-if="rejectError" class="input-error-text">{{ rejectError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeReject">
            {{ t('enterprise.profile.cancel') }}
          </button>
          <button form="reject-enterprise-form" type="submit" class="btn btn-danger" :disabled="busy">
            {{ t('enterprise.admin.reject') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import EnterpriseStatusBadge from '@/components/enterprise/EnterpriseStatusBadge.vue'
import { adminEnterpriseAPI, type EnterpriseAdminItem, type EnterpriseStatus } from '@/api/enterprise'
import { useAppStore } from '@/stores/app'
import { enterpriseErrorMessage } from '@/utils/enterprise'

const { t, te } = useI18n()
const appStore = useAppStore()

const statusOptions: EnterpriseStatus[] = ['pending', 'approved', 'rejected', 'incomplete']

const items = ref<EnterpriseAdminItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const status = ref<EnterpriseStatus | ''>('pending')
const search = ref('')
const loading = ref(false)
const busy = ref(false)

const detail = ref<EnterpriseAdminItem | null>(null)
const approveTarget = ref<EnterpriseAdminItem | null>(null)
const rejectTarget = ref<EnterpriseAdminItem | null>(null)
const rejectReason = ref('')
const rejectError = ref('')

const formatTime = (value: string) => new Date(value).toLocaleString()

async function load() {
  loading.value = true
  try {
    const data = await adminEnterpriseAPI.list({
      page: page.value,
      page_size: pageSize.value,
      status: status.value || undefined,
      search: search.value.trim() || undefined
    })
    items.value = data.items
    total.value = data.total
  } catch (error: unknown) {
    appStore.showError(enterpriseErrorMessage(error, t, te, t('enterprise.admin.loadFailed')))
  } finally {
    loading.value = false
  }
}

function resetAndLoad() {
  page.value = 1
  void load()
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(resetAndLoad, 300)
}

function onPageChange(value: number) {
  page.value = value
  void load()
}

function onPageSizeChange(value: number) {
  pageSize.value = value
  resetAndLoad()
}

async function approve() {
  const target = approveTarget.value
  if (!target || busy.value) return
  busy.value = true
  try {
    await adminEnterpriseAPI.review(target.user_id, 'approve')
    appStore.showSuccess(t('enterprise.admin.approved'))
    approveTarget.value = null
    await load()
  } catch (error: unknown) {
    appStore.showError(enterpriseErrorMessage(error, t, te, t('enterprise.admin.loadFailed')))
  } finally {
    busy.value = false
  }
}

function openReject(item: EnterpriseAdminItem) {
  rejectTarget.value = item
  rejectReason.value = ''
  rejectError.value = ''
}

function closeReject() {
  rejectTarget.value = null
  rejectReason.value = ''
  rejectError.value = ''
}

async function reject() {
  const target = rejectTarget.value
  if (!target || busy.value) return
  const reason = rejectReason.value.trim()
  if (!reason) {
    rejectError.value = t('enterprise.admin.rejectReasonRequired')
    return
  }
  busy.value = true
  try {
    await adminEnterpriseAPI.review(target.user_id, 'reject', reason)
    appStore.showSuccess(t('enterprise.admin.rejected'))
    closeReject()
    await load()
  } catch (error: unknown) {
    appStore.showError(enterpriseErrorMessage(error, t, te, t('enterprise.admin.loadFailed')))
  } finally {
    busy.value = false
  }
}

onMounted(load)
onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<style scoped>
.link-button { @apply text-sm font-medium text-primary-600 hover:underline dark:text-primary-400; }
</style>
