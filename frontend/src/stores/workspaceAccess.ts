import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { enterpriseAPI, type WorkspaceAccess } from '@/api/enterprise'
import { useAuthStore } from './auth'

/**
 * 当前账号的企业身份与团队空间访问权限。
 *
 * - 企业账号（存在企业档案）：展示团队空间、可创建团队。
 * - 已加入团队的个人账号：展示团队空间，但不能创建团队。
 * - 其他个人账号：不展示团队空间。
 *
 * 这里只负责界面可见性；创建团队的限制由后端强制执行。
 */
export const useWorkspaceAccessStore = defineStore('workspaceAccess', () => {
  const authStore = useAuthStore()

  const access = ref<WorkspaceAccess | null>(null)
  const loaded = ref(false)
  let pending: Promise<WorkspaceAccess | null> | null = null

  const isEnterprise = computed(() => access.value?.account_type === 'enterprise')
  const canCreateTeam = computed(() => access.value?.can_create_team === true)
  const hasTeamAccess = computed(() => access.value?.has_team_access === true)
  const enterprise = computed(() => access.value?.enterprise ?? null)

  async function fetch(force = false): Promise<WorkspaceAccess | null> {
    if (!authStore.isAuthenticated) {
      reset()
      return null
    }
    if (loaded.value && !force) return access.value
    if (pending && !force) return pending

    const request = (async () => {
      try {
        access.value = await enterpriseAPI.workspaceAccess()
        loaded.value = true
      } catch (error) {
        // 获取失败时保持上一次的结果，并保持「未加载」状态，下次访问会自动重试；
        // 后端仍会强制校验，界面不因网络抖动误锁用户。
        console.warn('Failed to load workspace access', error)
      } finally {
        pending = null
      }
      return access.value
    })()
    pending = request
    return request
  }

  function reset(): void {
    access.value = null
    loaded.value = false
    pending = null
  }

  // 登录 / 切换账号 / 退出时同步。
  watch(
    () => (authStore.isAuthenticated ? authStore.user?.id : null),
    (id) => {
      reset()
      if (id) void fetch(true)
    },
    { immediate: true }
  )

  return { access, loaded, isEnterprise, canCreateTeam, hasTeamAccess, enterprise, fetch, reset }
})
