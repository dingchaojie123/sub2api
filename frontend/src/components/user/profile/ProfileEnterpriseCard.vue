<template>
  <div v-if="store.loaded" class="card p-6" data-testid="profile-enterprise-card">
    <!-- 企业账号：展示 / 编辑企业信息 -->
    <template v-if="enterprise">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('enterprise.profile.cardTitle') }}
            </h3>
            <EnterpriseStatusBadge :status="enterprise.verify_status" />
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('enterprise.profile.cardDescription') }}
          </p>
        </div>
        <button v-if="!editing" type="button" class="btn btn-secondary btn-sm" @click="startEdit">
          {{ t('enterprise.profile.edit') }}
        </button>
      </div>

      <p
        v-if="enterprise.verify_status === 'incomplete'"
        class="mt-4 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('enterprise.profile.incompleteHint') }}
      </p>
      <p
        v-else-if="enterprise.verify_status === 'rejected'"
        class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ t('enterprise.profile.rejectedHint') }}
        <span v-if="enterprise.reject_reason" class="block pt-1">
          {{ t('enterprise.profile.rejectReason') }}：{{ enterprise.reject_reason }}
        </span>
      </p>
      <p
        v-else-if="enterprise.verify_status === 'approved'"
        class="mt-4 rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300"
      >
        {{ t('enterprise.profile.lockedHint') }}
      </p>

      <!-- 只读展示 -->
      <dl v-if="!editing" class="mt-5 grid gap-x-6 gap-y-4 sm:grid-cols-2">
        <div v-for="row in readonlyRows" :key="row.label" :class="row.wide ? 'sm:col-span-2' : ''">
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ row.label }}</dt>
          <dd class="mt-1 break-all text-sm text-gray-900 dark:text-white" :class="row.mono ? 'font-mono' : ''">
            {{ row.value || '-' }}
          </dd>
        </div>
      </dl>

      <!-- 编辑 -->
      <form v-else class="mt-5 space-y-4" @submit.prevent="save">
        <EnterpriseForm
          v-model="form"
          :errors="fieldErrors"
          :disabled="saving"
          :lock-identity="enterprise.verify_status === 'approved'"
          id-prefix="profile-enterprise"
        />
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="cancelEdit">
            {{ t('enterprise.profile.cancel') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ submitLabel }}
          </button>
        </div>
      </form>
    </template>

    <!-- 个人账号：升级入口 -->
    <template v-else>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('enterprise.profile.upgradeTitle') }}
          </h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('enterprise.profile.upgradeDescription') }}
          </p>
        </div>
        <button v-if="!editing" type="button" class="btn btn-primary btn-sm" @click="startEdit">
          {{ t('enterprise.profile.upgradeButton') }}
        </button>
      </div>

      <form v-if="editing" class="mt-5 space-y-4" @submit.prevent="save">
        <EnterpriseForm
          v-model="form"
          :errors="fieldErrors"
          :disabled="saving"
          id-prefix="profile-upgrade"
        />
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="cancelEdit">
            {{ t('enterprise.profile.cancel') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="saving">
            {{ t('enterprise.profile.upgradeSubmit') }}
          </button>
        </div>
      </form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import EnterpriseForm from '@/components/enterprise/EnterpriseForm.vue'
import EnterpriseStatusBadge from '@/components/enterprise/EnterpriseStatusBadge.vue'
import { enterpriseAPI } from '@/api/enterprise'
import { useAppStore } from '@/stores/app'
import { useWorkspaceAccessStore } from '@/stores/workspaceAccess'
import {
  emptyEnterpriseInput,
  enterpriseErrorMessage,
  normalizeEnterpriseInput,
  validateEnterpriseInput,
  type EnterpriseFieldErrors
} from '@/utils/enterprise'

const { t, te } = useI18n()
const appStore = useAppStore()
const store = useWorkspaceAccessStore()

const enterprise = computed(() => store.enterprise)
const editing = ref(false)
const saving = ref(false)
const form = reactive(emptyEnterpriseInput())
const fieldErrors = ref<EnterpriseFieldErrors>({})

const readonlyRows = computed(() => {
  const e = enterprise.value
  if (!e) return []
  return [
    { label: t('enterprise.fields.companyName'), value: e.company_name, wide: true },
    { label: t('enterprise.fields.creditCode'), value: e.credit_code, mono: true },
    { label: t('enterprise.fields.contactName'), value: e.contact_name },
    { label: t('enterprise.fields.contactPhone'), value: e.contact_phone },
    { label: t('enterprise.fields.address'), value: e.address, wide: true }
  ]
})

// 未认证、驳回或待补全的信息保存后都会进入「待认证」，按钮文案提示这一点。
const submitLabel = computed(() => {
  const status = enterprise.value?.verify_status
  return status === 'rejected' || status === 'incomplete'
    ? t('enterprise.profile.saveAndSubmit')
    : t('enterprise.profile.save')
})

function fillForm() {
  const e = enterprise.value
  Object.assign(form, e
    ? {
        company_name: e.company_name,
        credit_code: e.credit_code,
        address: e.address,
        contact_name: e.contact_name,
        contact_phone: e.contact_phone
      }
    : emptyEnterpriseInput())
}

function startEdit() {
  fillForm()
  fieldErrors.value = {}
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  fieldErrors.value = {}
}

async function save() {
  fieldErrors.value = validateEnterpriseInput(form)
  if (Object.keys(fieldErrors.value).length > 0) return

  saving.value = true
  try {
    const payload = normalizeEnterpriseInput(form)
    if (enterprise.value) {
      await enterpriseAPI.update(payload)
      appStore.showSuccess(t('enterprise.profile.saved'))
    } else {
      await enterpriseAPI.upgrade(payload)
      appStore.showSuccess(t('enterprise.profile.upgraded'))
    }
    await store.fetch(true)
    editing.value = false
  } catch (error: unknown) {
    appStore.showError(enterpriseErrorMessage(error, t, te, t('enterprise.registerFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await store.fetch(true)
  // 存量团队创建者信息待补全：直接进入编辑状态，引导补全
  if (enterprise.value?.verify_status === 'incomplete') {
    startEdit()
  }
})
</script>
