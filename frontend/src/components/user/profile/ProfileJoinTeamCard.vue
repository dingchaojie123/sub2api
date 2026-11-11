<template>
  <div v-if="store.loaded && !store.hasTeamAccess" class="card p-6" data-testid="profile-join-team-card">
    <div class="flex items-start gap-3">
      <div class="rounded-xl bg-primary-50 p-2.5 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
        <Icon name="users" size="md" />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('enterprise.join.title') }}
        </h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ t('enterprise.join.description') }}
        </p>

        <form class="mt-4 flex flex-col gap-3 sm:flex-row" @submit.prevent="join">
          <input
            v-model.trim="token"
            type="text"
            autocomplete="off"
            :disabled="busy"
            class="input flex-1"
            :placeholder="t('enterprise.join.placeholder')"
          />
          <button type="submit" class="btn btn-primary sm:w-32" :disabled="busy || !token">
            {{ busy ? t('enterprise.join.joining') : t('enterprise.join.submit') }}
          </button>
        </form>
        <p class="input-hint">{{ t('enterprise.join.hint') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import { organizationsAPI } from '@/api/organizations'
import { useAppStore } from '@/stores/app'
import { useWorkspaceAccessStore } from '@/stores/workspaceAccess'
import { enterpriseErrorMessage } from '@/utils/enterprise'
import { extractInviteToken } from '@/utils/organizationInvite'

/**
 * 未加入团队的个人账号没有「团队空间」菜单，这里提供加入团队的入口：
 * 粘贴管理员发来的邀请链接或令牌即可。加入成功后菜单出现并跳转到团队空间。
 */
const { t, te } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const store = useWorkspaceAccessStore()

const token = ref('')
const busy = ref(false)

async function join() {
  const value = extractInviteToken(token.value)
  if (!value || busy.value) return
  busy.value = true
  try {
    await organizationsAPI.accept(value)
    token.value = ''
    await store.fetch(true)
    appStore.showSuccess(t('enterprise.join.success'))
    await router.push('/organizations')
  } catch (error: unknown) {
    appStore.showError(enterpriseErrorMessage(error, t, te, t('enterprise.join.failed')))
  } finally {
    busy.value = false
  }
}
</script>
