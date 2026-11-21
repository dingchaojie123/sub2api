<template>
  <BaseDialog :show="show" :title="t('admin.groups.lineRoutesTitle')" width="extra-wide" @close="handleClose">
    <div v-if="group" class="space-y-4">
      <!-- 分组 + 概览 -->
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-base font-semibold text-gray-900 dark:text-white">{{ group.name }}</span>
        <template v-if="!loading && rows.length > 0">
          <span class="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            {{ t('admin.groups.lineRoutesCount', { count: rows.length }) }}
          </span>
          <span
            v-if="routedCount > 0"
            class="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
            data-testid="line-route-routed-count"
          >
            {{ t('admin.groups.lineRoutesRoutedCount', { count: routedCount }) }}
          </span>
        </template>
      </div>

      <!-- 总开关：关闭时所有分组的路由都不生效 -->
      <div
        :class="[
          'flex items-center justify-between gap-3 rounded-lg border px-3.5 py-2.5',
          routesEnabled
            ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-900/40 dark:bg-emerald-900/20'
            : 'border-amber-300 bg-amber-50 dark:border-amber-700/60 dark:bg-amber-900/20'
        ]"
        data-testid="line-route-switch"
      >
        <div class="min-w-0">
          <div class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.groups.lineRoutesSwitchTitle') }}
            <span
              :class="[
                'rounded-full px-2 py-0.5 text-xs font-medium',
                routesEnabled ? 'bg-emerald-500 text-white' : 'bg-amber-500 text-white'
              ]"
              data-testid="line-route-switch-state"
            >
              {{ routesEnabled ? t('admin.groups.lineRoutesSwitchOn') : t('admin.groups.lineRoutesSwitchOff') }}
            </span>
          </div>
          <div class="mt-1 text-xs text-gray-600 dark:text-gray-300">
            {{ routesEnabled ? t('admin.groups.lineRoutesSwitchOnHint') : t('admin.groups.lineRoutesSwitchOffHint') }}
          </div>
        </div>
        <button
          type="button"
          :class="['btn btn-sm flex-shrink-0 whitespace-nowrap', routesEnabled ? 'btn-secondary' : 'btn-primary']"
          data-testid="line-route-switch-toggle"
          :disabled="switchBusy || switchUnknown"
          @click="toggleSwitch"
        >
          {{ routesEnabled ? t('admin.groups.lineRoutesSwitchDisable') : t('admin.groups.lineRoutesSwitchEnable') }}
        </button>
      </div>

      <!-- 说明 -->
      <div
        class="rounded-lg border border-sky-100 bg-sky-50 px-3.5 py-2.5 text-xs leading-relaxed text-sky-800 dark:border-sky-900/40 dark:bg-sky-900/20 dark:text-sky-200"
      >
        {{ t('admin.groups.lineRoutesHint') }}
      </div>

      <div v-if="loading" class="flex justify-center py-10">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <div v-else-if="rows.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.groups.lineRoutesNoLines') }}
      </div>

      <!-- 每条线路一张卡片：线路 → 目标线路 → 保存 -->
      <ul v-else class="max-h-[60vh] space-y-2.5 overflow-y-auto pr-1">
        <li
          v-for="row in rows"
          :key="row.line_name"
          data-testid="line-route-row"
          :class="[
            'flex flex-col gap-3 rounded-xl border p-3.5 transition-colors md:flex-row md:items-center',
            row.target_line_name
              ? 'border-amber-300 bg-amber-50/60 dark:border-amber-700/60 dark:bg-amber-900/10'
              : 'border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800'
          ]"
        >
          <!-- 线路 -->
          <div class="min-w-0 md:w-5/12">
            <div class="break-all text-sm font-medium leading-snug text-gray-900 dark:text-white">
              {{ row.line_name }}
            </div>
            <div class="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs">
              <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                {{ row.platform }}
              </span>
              <span class="rounded bg-emerald-50 px-1.5 py-0.5 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400">
                {{ t('admin.groups.lineRoutesAccountsBadge', { count: row.accounts }) }}
              </span>
              <span
                v-if="row.hidden"
                class="rounded bg-amber-100 px-1.5 py-0.5 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
              >
                {{ t('admin.groups.lineRoutesHidden') }}
              </span>
              <span
                v-if="row.target_line_name"
                class="rounded bg-amber-500 px-1.5 py-0.5 font-medium text-white"
              >
                {{ t('admin.groups.lineRoutesRouted') }}
              </span>
              <span
                v-if="isDirty(row)"
                class="rounded bg-primary-50 px-1.5 py-0.5 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
              >
                {{ t('admin.groups.lineRoutesUnsaved') }}
              </span>
            </div>
          </div>

          <!-- 箭头 -->
          <div class="hidden flex-shrink-0 text-gray-300 dark:text-dark-500 md:block" aria-hidden="true">
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
            </svg>
          </div>

          <!-- 目标线路 -->
          <div class="min-w-0 flex-1">
            <select
              v-model="drafts[row.line_name]"
              class="input w-full"
              :aria-label="t('admin.groups.lineRoutesRoute')"
              data-testid="line-route-select"
            >
              <option value="">{{ t('admin.groups.lineRoutesSelf') }}</option>
              <option v-for="candidate in optionsFor(row)" :key="candidate.line_name" :value="candidate.line_name">
                {{ candidate.line_name }} ({{ t('admin.groups.lineRoutesAccountsBadge', { count: candidate.accounts }) }})
              </option>
            </select>
            <div
              v-if="row.target_line_name && row.target_accounts === 0"
              class="mt-1.5 text-xs text-red-600 dark:text-red-400"
            >
              {{ t('admin.groups.lineRoutesBroken') }}
            </div>
          </div>

          <!-- 保存 -->
          <button
            type="button"
            class="btn btn-primary btn-sm flex-shrink-0 whitespace-nowrap md:min-w-[4.5rem]"
            data-testid="line-route-save"
            :disabled="savingLine !== '' || !isDirty(row)"
            @click="save(row)"
          >
            {{ savingLine === row.line_name ? t('admin.groups.lineRoutesSaving') : t('admin.groups.lineRoutesSave') }}
          </button>
        </li>
      </ul>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { GroupLineRoute, GroupLineRouteCandidate } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const rows = ref<GroupLineRoute[]>([])
// line name -> the target chosen in the dropdown ('' = the line serves itself)
const drafts = ref<Record<string, string>>({})
// the line whose save request is in flight ('' = none)
const savingLine = ref('')

// The global switch. Until it has been read it is unknown, and the toggle stays disabled.
const routesEnabled = ref(false)
const switchUnknown = ref(true)
const switchBusy = ref(false)

const loadSwitch = async () => {
  try {
    routesEnabled.value = await adminAPI.groups.getLineRoutesSwitch()
    switchUnknown.value = false
  } catch (error) {
    console.error('Failed to load line routes switch:', error)
    switchUnknown.value = true
  }
}

const toggleSwitch = async () => {
  if (switchBusy.value || switchUnknown.value) return
  switchBusy.value = true
  try {
    routesEnabled.value = await adminAPI.groups.setLineRoutesSwitch(!routesEnabled.value)
    appStore.showSuccess(t('admin.groups.lineRoutesSwitchSaved'))
  } catch (error: any) {
    console.error('Failed to update line routes switch:', error)
    appStore.showError(error?.message || t('admin.groups.lineRoutesSwitchFailed'))
  } finally {
    switchBusy.value = false
  }
}

const routedCount = computed(() => rows.value.filter((row) => row.target_line_name).length)

// A row is dirty when the dropdown differs from what the server has.
const isDirty = (row: GroupLineRoute) => (drafts.value[row.line_name] ?? '') !== row.target_line_name

const applyRows = (lines: GroupLineRoute[]) => {
  rows.value = lines
  const next: Record<string, string> = {}
  for (const line of lines) {
    next[line.line_name] = line.target_line_name
  }
  drafts.value = next
}

// The dropdown offers the candidates, plus the current target when it is no longer one (for
// example it lost its accounts), so the select never shows a blank for an existing route.
const optionsFor = (row: GroupLineRoute): GroupLineRouteCandidate[] => {
  if (row.target_line_name && !row.candidates.some((c) => c.line_name === row.target_line_name)) {
    return [
      ...row.candidates,
      { line_name: row.target_line_name, platform: '', accounts: row.target_accounts }
    ]
  }
  return row.candidates
}

const load = async () => {
  if (!props.group) return
  loading.value = true
  try {
    applyRows(await adminAPI.groups.getGroupLineRoutes(props.group.id))
  } catch (error) {
    console.error('Failed to load line routes:', error)
    appStore.showError(t('admin.groups.lineRoutesLoadFailed'))
    applyRows([])
  } finally {
    loading.value = false
  }
}

const save = async (row: GroupLineRoute) => {
  if (!props.group || savingLine.value !== '') return
  savingLine.value = row.line_name
  try {
    applyRows(
      await adminAPI.groups.setGroupLineRoute(props.group.id, row.line_name, drafts.value[row.line_name] ?? '')
    )
    appStore.showSuccess(t('admin.groups.lineRoutesSaved'))
  } catch (error: any) {
    console.error('Failed to save line route:', error)
    // The server explains a rejected route (no account in the group, version mismatch, ...).
    appStore.showError(error?.message || t('admin.groups.lineRoutesSaveFailed'))
  } finally {
    savingLine.value = ''
  }
}

const handleClose = () => {
  emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      load()
      loadSwitch()
    }
  }
)
</script>
