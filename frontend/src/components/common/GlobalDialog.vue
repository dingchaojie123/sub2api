<template>
  <BaseDialog
    :show="dialogState.open"
    :title="title"
    width="narrow"
    :z-index="200"
    :show-close-button="false"
    :close-on-escape="false"
    @close="settleDialog(false)"
  >
    <div class="space-y-4">
      <p class="whitespace-pre-line break-words text-sm text-gray-600 dark:text-gray-400">
        {{ dialogState.message }}
      </p>
      <input
        v-if="dialogState.kind === 'prompt'"
        ref="inputRef"
        v-model="dialogState.value"
        :type="dialogState.inputType"
        :placeholder="dialogState.placeholder"
        class="input w-full"
        autocomplete="off"
        @keydown.enter.prevent="settleDialog(true)"
      />
    </div>

    <template #footer>
      <div class="flex justify-end space-x-3">
        <button
          type="button"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600 dark:focus:ring-offset-dark-800"
          @click="settleDialog(false)"
        >
          {{ cancelText }}
        </button>
        <button
          type="button"
          :class="[
            'rounded-md px-4 py-2 text-sm font-medium text-white focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-dark-800',
            dialogState.danger
              ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
              : 'bg-primary-600 hover:bg-primary-700 focus:ring-primary-500'
          ]"
          @click="settleDialog(true)"
        >
          {{ confirmText }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'
import { dialogState, settleDialog } from '@/composables/useDialog'

const { t } = useI18n()

const inputRef = ref<HTMLInputElement | null>(null)

const title = computed(() => dialogState.title || t('common.confirm'))
const confirmText = computed(() => dialogState.confirmText || t('common.confirm'))
const cancelText = computed(() => dialogState.cancelText || t('common.cancel'))

// 输入框弹出后自动聚焦并全选默认值
watch(
  () => dialogState.open,
  async (open) => {
    if (open && dialogState.kind === 'prompt') {
      await nextTick()
      inputRef.value?.focus()
      inputRef.value?.select()
    }
  }
)

// 全局弹窗可能叠在别的弹窗之上：在 window 捕获阶段拦截 Esc，只关闭本弹窗，
// 不让下层 BaseDialog 同时被关掉。
function onKeydown(event: KeyboardEvent) {
  if (dialogState.open && event.key === 'Escape') {
    event.stopImmediatePropagation()
    event.preventDefault()
    settleDialog(false)
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown, true))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown, true))
</script>
