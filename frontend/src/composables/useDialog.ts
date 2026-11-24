import { reactive } from 'vue'

/**
 * 站内确认 / 输入弹窗，用来替代浏览器原生的 window.confirm / window.prompt。
 *
 * 用法（和原生写法几乎一样，只是要 await）：
 *   if (!(await confirmDialog('确认删除？', { danger: true }))) return
 *   const value = await promptDialog('输入每月额度', { defaultValue: '0' })
 *   if (value === null) return // 用户取消
 *
 * 弹窗由 App.vue 里挂载的 <GlobalDialog /> 渲染。多个请求会排队依次显示。
 */

export interface ConfirmDialogOptions {
  title?: string
  confirmText?: string
  cancelText?: string
  /** 危险操作：确认按钮显示为红色 */
  danger?: boolean
}

export interface PromptDialogOptions extends ConfirmDialogOptions {
  defaultValue?: string
  placeholder?: string
  inputType?: 'text' | 'password' | 'number'
}

type DialogKind = 'confirm' | 'prompt'

interface DialogRequest {
  kind: DialogKind
  message: string
  options: PromptDialogOptions
  resolve: (value: boolean | string | null) => void
}

export interface DialogState {
  open: boolean
  kind: DialogKind
  message: string
  title: string
  confirmText: string
  cancelText: string
  danger: boolean
  inputType: 'text' | 'password' | 'number'
  placeholder: string
  value: string
}

/** 仅供 GlobalDialog.vue 读取/绑定 */
export const dialogState = reactive<DialogState>({
  open: false,
  kind: 'confirm',
  message: '',
  title: '',
  confirmText: '',
  cancelText: '',
  danger: false,
  inputType: 'text',
  placeholder: '',
  value: ''
})

let current: DialogRequest | null = null
const queue: DialogRequest[] = []

function showNext() {
  if (current || queue.length === 0) return
  const next = queue.shift() as DialogRequest
  current = next
  dialogState.kind = next.kind
  dialogState.message = next.message
  dialogState.title = next.options.title ?? ''
  dialogState.confirmText = next.options.confirmText ?? ''
  dialogState.cancelText = next.options.cancelText ?? ''
  dialogState.danger = next.options.danger ?? false
  dialogState.inputType = next.options.inputType ?? 'text'
  dialogState.placeholder = next.options.placeholder ?? ''
  dialogState.value = next.options.defaultValue ?? ''
  dialogState.open = true
}

function enqueue(kind: DialogKind, message: string, options: PromptDialogOptions) {
  return new Promise<boolean | string | null>((resolve) => {
    queue.push({ kind, message, options, resolve })
    showNext()
  })
}

/** 站内确认框：确认 → true，取消 / 关闭 → false */
export function confirmDialog(message: string, options: ConfirmDialogOptions = {}): Promise<boolean> {
  return enqueue('confirm', message, options) as Promise<boolean>
}

/** 站内输入框：确认 → 输入的字符串，取消 / 关闭 → null */
export function promptDialog(message: string, options: PromptDialogOptions = {}): Promise<string | null> {
  return enqueue('prompt', message, options) as Promise<string | null>
}

/** 由 GlobalDialog.vue 调用：结束当前弹窗并处理排队中的下一个 */
export function settleDialog(confirmed: boolean) {
  const request = current
  if (!request) return
  const value = dialogState.value
  current = null
  dialogState.open = false
  if (request.kind === 'confirm') {
    request.resolve(confirmed)
  } else {
    request.resolve(confirmed ? value : null)
  }
  // 等关闭动画结束后再显示下一个，避免两个弹窗闪烁叠在一起
  if (queue.length > 0) {
    setTimeout(showNext, 200)
  }
}

/** 兼容 composable 风格的调用 */
export function useDialog() {
  return { confirmDialog, promptDialog }
}
