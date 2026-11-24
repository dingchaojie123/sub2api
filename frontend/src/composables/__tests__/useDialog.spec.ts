import { beforeEach, describe, expect, it } from 'vitest'
import { confirmDialog, dialogState, promptDialog, settleDialog } from '../useDialog'

describe('useDialog', () => {
  beforeEach(() => {
    // 清理上一个用例可能遗留的弹窗
    if (dialogState.open) settleDialog(false)
  })

  it('confirmDialog resolves true when confirmed and false when cancelled', async () => {
    const yes = confirmDialog('确认移除？', { danger: true, title: '移除成员' })
    expect(dialogState.open).toBe(true)
    expect(dialogState.kind).toBe('confirm')
    expect(dialogState.message).toBe('确认移除？')
    expect(dialogState.title).toBe('移除成员')
    expect(dialogState.danger).toBe(true)
    settleDialog(true)
    await expect(yes).resolves.toBe(true)
    expect(dialogState.open).toBe(false)

    const no = confirmDialog('再确认一次')
    settleDialog(false)
    await expect(no).resolves.toBe(false)
  })

  it('promptDialog returns the typed value on confirm and null on cancel', async () => {
    const typed = promptDialog('输入额度', { defaultValue: '5', inputType: 'number' })
    expect(dialogState.kind).toBe('prompt')
    expect(dialogState.value).toBe('5')
    expect(dialogState.inputType).toBe('number')
    dialogState.value = '12'
    settleDialog(true)
    await expect(typed).resolves.toBe('12')

    const cancelled = promptDialog('输入额度')
    dialogState.value = 'ignored'
    settleDialog(false)
    await expect(cancelled).resolves.toBeNull()
  })

  it('queues a second dialog until the first one is settled', async () => {
    const first = confirmDialog('first')
    const second = confirmDialog('second')
    expect(dialogState.message).toBe('first')

    settleDialog(true)
    await expect(first).resolves.toBe(true)

    // 下一个弹窗在关闭动画之后才出现
    await new Promise((resolve) => setTimeout(resolve, 250))
    expect(dialogState.open).toBe(true)
    expect(dialogState.message).toBe('second')
    settleDialog(false)
    await expect(second).resolves.toBe(false)
  })
})
