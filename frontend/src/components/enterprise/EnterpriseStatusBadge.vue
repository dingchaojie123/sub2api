<template>
  <span class="badge" :class="badgeClass" :title="title">
    {{ t(`enterprise.status.${status}`) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EnterpriseStatus } from '@/api/enterprise'

const props = defineProps<{
  status: EnterpriseStatus
  title?: string
}>()

const { t } = useI18n()

const badgeClass = computed(() => {
  switch (props.status) {
    case 'approved':
      return 'badge-success'
    case 'pending':
      return 'badge-warning'
    case 'rejected':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
})
</script>
