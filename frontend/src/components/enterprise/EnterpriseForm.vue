<template>
  <div class="space-y-4">
    <div>
      <label :for="`${idPrefix}-company-name`" class="input-label">
        {{ t('enterprise.fields.companyName') }}
      </label>
      <input
        :id="`${idPrefix}-company-name`"
        v-model="model.company_name"
        type="text"
        maxlength="100"
        autocomplete="organization"
        :disabled="disabled || lockIdentity"
        class="input"
        :class="{ 'input-error': errors.company_name }"
        :placeholder="t('enterprise.placeholders.companyName')"
      />
      <p v-if="errors.company_name" class="input-error-text">{{ errorText('company_name') }}</p>
    </div>

    <div>
      <label :for="`${idPrefix}-credit-code`" class="input-label">
        {{ t('enterprise.fields.creditCode') }}
      </label>
      <input
        :id="`${idPrefix}-credit-code`"
        v-model="model.credit_code"
        type="text"
        maxlength="24"
        autocomplete="off"
        :disabled="disabled || lockIdentity"
        class="input font-mono uppercase"
        :class="{ 'input-error': errors.credit_code }"
        :placeholder="t('enterprise.placeholders.creditCode')"
      />
      <p v-if="errors.credit_code" class="input-error-text">{{ errorText('credit_code') }}</p>
    </div>

    <div>
      <label :for="`${idPrefix}-address`" class="input-label">
        {{ t('enterprise.fields.address') }}
      </label>
      <input
        :id="`${idPrefix}-address`"
        v-model="model.address"
        type="text"
        maxlength="200"
        autocomplete="street-address"
        :disabled="disabled"
        class="input"
        :class="{ 'input-error': errors.address }"
        :placeholder="t('enterprise.placeholders.address')"
      />
      <p v-if="errors.address" class="input-error-text">{{ errorText('address') }}</p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label :for="`${idPrefix}-contact-name`" class="input-label">
          {{ t('enterprise.fields.contactName') }}
        </label>
        <input
          :id="`${idPrefix}-contact-name`"
          v-model="model.contact_name"
          type="text"
          maxlength="50"
          autocomplete="name"
          :disabled="disabled"
          class="input"
          :class="{ 'input-error': errors.contact_name }"
          :placeholder="t('enterprise.placeholders.contactName')"
        />
        <p v-if="errors.contact_name" class="input-error-text">{{ errorText('contact_name') }}</p>
      </div>
      <div>
        <label :for="`${idPrefix}-contact-phone`" class="input-label">
          {{ t('enterprise.fields.contactPhone') }}
        </label>
        <input
          :id="`${idPrefix}-contact-phone`"
          v-model="model.contact_phone"
          type="tel"
          maxlength="40"
          autocomplete="tel"
          :disabled="disabled"
          class="input"
          :class="{ 'input-error': errors.contact_phone }"
          :placeholder="t('enterprise.placeholders.contactPhone')"
        />
        <p v-if="errors.contact_phone" class="input-error-text">{{ errorText('contact_phone') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EnterpriseInput } from '@/api/enterprise'
import type { EnterpriseFieldErrors } from '@/utils/enterprise'

/**
 * 企业信息表单（注册、升级、编辑共用）。
 * v-model 直接修改传入的 reactive 对象；校验由父组件完成，错误通过 errors 传入。
 */
const model = defineModel<EnterpriseInput>({ required: true })

const props = withDefaults(
  defineProps<{
    errors?: EnterpriseFieldErrors
    disabled?: boolean
    /** 认证通过后锁定公司名称与信用代码 */
    lockIdentity?: boolean
    idPrefix?: string
  }>(),
  { errors: () => ({}), disabled: false, lockIdentity: false, idPrefix: 'enterprise' }
)

const { t } = useI18n()

function errorText(field: keyof EnterpriseInput): string {
  const key = props.errors?.[field]
  return key ? t(`enterprise.errors.${key}`) : ''
}
</script>
