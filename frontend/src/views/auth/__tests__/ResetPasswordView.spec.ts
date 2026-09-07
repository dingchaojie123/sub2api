import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ResetPasswordView from '@/views/auth/ResetPasswordView.vue'

const {
  routeState,
  showSuccessMock,
  showErrorMock,
  resetPasswordMock,
} = vi.hoisted(() => ({
  routeState: {
    query: {
      email: 'user@example.com',
      token: 'reset-token',
    } as Record<string, unknown>,
  },
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  resetPasswordMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
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
  resetPassword: (...args: unknown[]) => resetPasswordMock(...args),
}))

function mountView() {
  return mount(ResetPasswordView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
      },
    },
  })
}

describe('ResetPasswordView', () => {
  beforeEach(() => {
    routeState.query = {
      email: 'user@example.com',
      token: 'reset-token',
    }
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    resetPasswordMock.mockReset()
    resetPasswordMock.mockResolvedValue({
      message: 'ok',
    })
  })

  it('resets the password with the email and token from the link', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#password').setValue('new-password')
    await wrapper.get('#confirmPassword').setValue('new-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(resetPasswordMock).toHaveBeenCalledWith({
      email: 'user@example.com',
      token: 'reset-token',
      new_password: 'new-password',
    })
    expect(wrapper.text()).toContain('auth.passwordResetSuccess')
    expect(showSuccessMock).toHaveBeenCalledWith('auth.passwordResetSuccess')
  })

  it('shows an invalid-link state when email or token is missing', async () => {
    routeState.query = {
      email: 'user@example.com',
    }
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.invalidResetLink')
    expect(resetPasswordMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('auth.invalidResetLink')
  })

  it('translates an expired-token API error', async () => {
    resetPasswordMock.mockRejectedValueOnce({ code: 'INVALID_RESET_TOKEN', message: 'invalid token' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#password').setValue('new-password')
    await wrapper.get('#confirmPassword').setValue('new-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('auth.invalidOrExpiredToken')
    expect(wrapper.text()).not.toContain('auth.passwordResetSuccess')
  })
})
