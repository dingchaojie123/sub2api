import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ForgotPasswordView from '@/views/auth/ForgotPasswordView.vue'

const {
  showSuccessMock,
  showErrorMock,
  getPublicSettingsMock,
  forgotPasswordMock,
} = vi.hoisted(() => ({
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  forgotPasswordMock: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showSuccess: (...args: unknown[]) => showSuccessMock(...args),
    showError: (...args: unknown[]) => showErrorMock(...args),
  }),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
  forgotPassword: (...args: unknown[]) => forgotPasswordMock(...args),
}))

function mountView() {
  return mount(ForgotPasswordView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        TurnstileWidget: true,
      },
    },
  })
}

describe('ForgotPasswordView', () => {
  beforeEach(() => {
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    getPublicSettingsMock.mockReset()
    forgotPasswordMock.mockReset()

    getPublicSettingsMock.mockResolvedValue({
      turnstile_enabled: false,
      turnstile_site_key: '',
    })
    forgotPasswordMock.mockResolvedValue({
      message: 'ok',
    })
  })

  it('requests a password reset link for a valid email', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(forgotPasswordMock).toHaveBeenCalledWith({
      email: 'user@example.com',
      turnstile_token: undefined,
    })
    expect(wrapper.text()).toContain('auth.resetEmailSent')
    expect(showSuccessMock).toHaveBeenCalledWith('auth.resetEmailSent')
  })

  it('shows a client-side validation error for an invalid email', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#email').setValue('invalid-email')
    await wrapper.get('form').trigger('submit')

    expect(forgotPasswordMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('auth.invalidEmail')
  })

  it('surfaces API errors returned by the request', async () => {
    forgotPasswordMock.mockRejectedValueOnce({ message: 'Password reset is not configured' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('Password reset is not configured')
    expect(wrapper.text()).not.toContain('auth.resetEmailSent')
  })
})
