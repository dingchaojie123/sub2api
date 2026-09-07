import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  createAccountMock,
  probeUpstreamBillingMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  authIsSimpleMode,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  probeUpstreamBillingMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  authIsSimpleMode: { value: true },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get isSimpleMode() {
      return authIsSimpleMode.value
    },
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      probeUpstreamBilling: probeUpstreamBillingMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({}),
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([]),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  props: {
    showManualOption: Boolean,
    showCodexSessionImportOption: Boolean,
    showAgentIdentityOption: Boolean,
    showCodexPatOption: Boolean,
    initialInputMethod: String,
  },
  data: () => ({ inputMethod: 'manual' }),
  emits: ['import-codex-session', 'import-codex-pat'],
  template: `
    <div>
      <button data-testid="import-codex-session" @click="$emit('import-codex-session', 'session-json')">session</button>
      <button data-testid="import-codex-pat" @click="$emit('import-codex-pat', 'pat-token')">pat</button>
    </div>
  `,
})

const GroupSelectorStub = defineComponent({
  name: 'GroupSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: `
    <div data-testid="group-selector">
      <button
        type="button"
        data-testid="set-create-group"
        @click="$emit('update:modelValue', [7])"
      >
        group
      </button>
    </div>
  `,
})

function mountModal() {
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [], groups: [] },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: GroupSelectorStub,
        ModelWhitelistSelector: true,
        QuotaLimitCard: {
          template: '<div data-testid="quota-limit-card"></div>',
        },
      },
    },
  })
}

async function selectButtonByText(wrapper: ReturnType<typeof mountModal>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button).toBeDefined()
  await button?.trigger('click')
}

async function submitApiKeyAccount(
  platform: 'openai' | 'anthropic',
  enableLongContextBilling = false,
  disableUpstreamBillingProbe = false
) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, platform === 'openai' ? 'OpenAI' : 'admin.accounts.claudeConsole')
  if (platform === 'openai') {
    await selectButtonByText(wrapper, 'API Key')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue(`${platform} account`)
  await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
  if (enableLongContextBilling) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  if (disableUpstreamBillingProbe) {
    await wrapper.get('[data-testid="upstream-billing-auto-probe"]').trigger('click')
  }
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  await flushPromises()
  return wrapper
}

async function fillJimengApiKeyAccount(wrapper: ReturnType<typeof mountModal>) {
  await selectButtonByText(wrapper, '即梦')
  await flushPromises()
  await wrapper.get('form#create-account-form input[data-tour="account-form-name"]').setValue('Jimeng account')
  const baseUrlInput = wrapper
    .findAll('input[type="text"]')
    .find(input => input.attributes('placeholder') === 'https://your-jimeng-proxy.example.com/v1')
  expect(baseUrlInput).toBeDefined()
  await baseUrlInput!.setValue('https://jimeng-proxy.example.com/v1')
  await wrapper.get('form#create-account-form input[type="password"]').setValue('jm-key')
}

async function fillProviderApiKeyAccount(
  wrapper: ReturnType<typeof mountModal>,
  platform: 'doubao' | 'qwen' | 'kimi' | 'deepseek',
  baseUrl?: string
) {
  await selectButtonByText(wrapper, `admin.accounts.platforms.${platform}`)
  await flushPromises()
  await wrapper.get('form#create-account-form input[data-tour="account-form-name"]').setValue(`${platform} account`)
  const baseUrlInput = wrapper.get('form#create-account-form input[type="text"]:not([data-tour="account-form-name"])')
  if (baseUrl) {
    await baseUrlInput.setValue(baseUrl)
  }
  await wrapper.get('form#create-account-form input[type="password"]').setValue(`${platform}-key`)
}

async function fillVideoApiKeyAccount(
  wrapper: ReturnType<typeof mountModal>,
  platform: 'kling' | 'happyhourse' | 'seedance' | 'bytedance' | 'wan3' | 'minimax-h3' | 'pixverse-v6'
) {
  await selectButtonByText(wrapper, `admin.accounts.platforms.${platform}`)
  await flushPromises()
  await wrapper.get('form#create-account-form input[data-tour="account-form-name"]').setValue(`${platform} account`)
  await wrapper
    .get('form#create-account-form input[type="text"]:not([data-tour="account-form-name"])')
    .setValue(`https://${platform}.example.com/v1`)
  await wrapper.get('form#create-account-form input[type="password"]').setValue(`${platform}-key`)
}

async function openCodexImportStep(toggleClicks = 0) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, 'OpenAI')
  for (let click = 0; click < toggleClicks; click += 1) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue('Codex import')
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  return wrapper
}

describe('CreateAccountModal OpenAI long-context billing', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    createAccountMock.mockReset().mockResolvedValue({ id: 42, platform: 'openai', type: 'apikey' })
    probeUpstreamBillingMock.mockReset().mockResolvedValue({})
    importCodexSessionMock.mockReset().mockResolvedValue({
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      errors: [],
      warnings: [],
    })
    createOpenAICodexPATMock.mockReset().mockResolvedValue({})
  })

  it('sends false explicitly for normal OpenAI account creation by default', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  it('keeps all platform options in one horizontally scrollable row', () => {
    const wrapper = mountModal()
    const platformSelector = wrapper.get('[data-tour="account-form-platform"]')
    const platformButtons = platformSelector.findAll('button')

    expect(platformSelector.classes()).toContain('flex-nowrap')
    expect(platformSelector.classes()).not.toContain('flex-wrap')
    expect(platformSelector.classes()).toContain('overflow-x-auto')
    expect(platformButtons).toHaveLength(17)
    for (const button of platformButtons) {
      expect(button.classes()).toContain('shrink-0')
      expect(button.classes()).not.toContain('flex-1')
    }
  })

  it('offers group routing for video account platforms in standard mode', async () => {
    authIsSimpleMode.value = false
    const wrapper = mountModal()

    await selectButtonByText(wrapper, 'admin.accounts.platforms.kling')

    expect(wrapper.find('[data-testid="group-selector"]').exists()).toBe(true)
  })

  it('clears group routing before creating a video account after a platform switch', async () => {
    authIsSimpleMode.value = false
    const wrapper = mountModal()

    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('[data-testid="set-create-group"]').trigger('click')
    await fillVideoApiKeyAccount(wrapper, 'kling')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.group_ids).toEqual([])
  })

  it('shows model whitelist for video accounts without quota controls', async () => {
    const wrapper = mountModal()

    await selectButtonByText(wrapper, 'admin.accounts.platforms.kling')

    expect(wrapper.findAll('button').some(button => (
      button.text().includes('admin.accounts.modelWhitelist')
    ))).toBe(true)
    expect(wrapper.findAll('button').some(button => (
      button.text().includes('admin.accounts.modelMapping')
    ))).toBe(true)
    expect(wrapper.find('[data-testid="quota-limit-card"]').exists()).toBe(false)
  })

  it('enables upstream billing probes by default for new OpenAI API key accounts', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('waits for the initial upstream billing probe before refreshing the account list', async () => {
    let resolveProbe: (() => void) | undefined
    probeUpstreamBillingMock.mockImplementationOnce(
      () => new Promise<void>((resolve) => {
        resolveProbe = resolve
      })
    )

    const wrapper = await submitApiKeyAccount('openai')

    expect(probeUpstreamBillingMock).toHaveBeenCalledWith(42)
    expect(wrapper.emitted('created')).toBeUndefined()

    resolveProbe?.()
    await flushPromises()

    expect(wrapper.emitted('created')).toHaveLength(1)
  })

  it('sends an explicit disabled state when the create toggle is turned off', async () => {
    await submitApiKeyAccount('openai', false, true)

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(false)
    expect(probeUpstreamBillingMock).not.toHaveBeenCalled()
  })

  it('exposes Agent Identity in the OpenAI authorization methods', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('OpenAI account')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    expect(flow.props('showManualOption')).toBe(true)
    expect(flow.props('showCodexSessionImportOption')).toBe(true)
    expect(flow.props('showAgentIdentityOption')).toBe(true)
    expect(flow.props('showCodexPatOption')).toBe(true)
    expect(flow.props('initialInputMethod')).toBe('manual')
  })

  it.each([
    ['camelCase', { authMode: 'agentIdentity', agentIdentity: { agentRuntimeId: 'runtime' } }],
    ['nested identity without auth_mode', { agent_identity: { agent_runtime_id: 'runtime' } }],
  ])('accepts backend-compatible %s Agent Identity imports', async (_name, content) => {
    const wrapper = await openCodexImportStep()
    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    flow.vm.inputMethod = 'agent_identity'

    flow.vm.$emit('import-codex-session', JSON.stringify(content))
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
  })

  it('sends true explicitly when OpenAI long-context billing is enabled', async () => {
    await submitApiKeyAccount('openai', true)

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('omits the OpenAI setting for non-OpenAI account creation', async () => {
    await submitApiKeyAccount('anthropic')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBeUndefined()
  })

  it('defaults Jimeng model mapping to the current Seedance model', async () => {
    const wrapper = mountModal()
    await fillJimengApiKeyAccount(wrapper)

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials?.model_mapping).toEqual({
      'by-seedance2.0-933': 'by-seedance2.0-933',
    })
  })

  it('submits Jimeng as an independent API key platform with fixed model mapping', async () => {
    const wrapper = mountModal()
    await fillJimengApiKeyAccount(wrapper)

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
      platform: 'jimeng',
      type: 'apikey',
      credentials: {
        base_url: 'https://jimeng-proxy.example.com/v1',
        api_key: 'jm-key',
        model_mapping: {
          'by-seedance2.0-933': 'by-seedance2.0-933',
        },
      },
    })
  })

  it.each([
    ['doubao', 'https://ark.cn-beijing.volces.com/api/v3'],
    ['qwen', 'https://dashscope.aliyuncs.com/compatible-mode/v1'],
    ['kimi', 'https://api.moonshot.cn/v1'],
    ['deepseek', 'https://api.deepseek.com'],
  ] as const)('creates %s with its official default Base URL', async (platform, defaultBaseUrl) => {
    const wrapper = mountModal()
    await fillProviderApiKeyAccount(wrapper, platform)

    const baseUrlInput = wrapper.get('form#create-account-form input[type="text"]:not([data-tour="account-form-name"])')
    expect((baseUrlInput.element as HTMLInputElement).value).toBe(defaultBaseUrl)

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
      platform,
      type: 'apikey',
      credentials: {
        base_url: defaultBaseUrl,
        api_key: `${platform}-key`,
      },
    })
  })

  it('keeps a custom proxy Base URL for provider platforms', async () => {
    const wrapper = mountModal()
    await fillProviderApiKeyAccount(wrapper, 'deepseek', 'https://proxy.example.com/v1')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
      platform: 'deepseek',
      credentials: {
        base_url: 'https://proxy.example.com/v1',
      },
    })
  })

  it.each(['kling', 'happyhourse', 'seedance'] as const)(
    'creates %s as a Bearer API key video account',
    async (platform) => {
      const wrapper = mountModal()
      await fillVideoApiKeyAccount(wrapper, platform)

      await wrapper.get('form#create-account-form').trigger('submit.prevent')
      await flushPromises()

      expect(createAccountMock).toHaveBeenCalledTimes(1)
      expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
        platform,
        type: 'apikey',
        credentials: {
          base_url: `https://${platform}.example.com/v1`,
          api_key: `${platform}-key`,
          auth_mode: 'bearer'
        },
      })
      expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('model_mapping')
    }
  )

  it.each(['bytedance', 'wan3', 'minimax-h3', 'pixverse-v6'] as const)(
    'creates the new %s video platform as a Bearer API key account',
    async (platform) => {
      const wrapper = mountModal()
      await fillVideoApiKeyAccount(wrapper, platform)

      await wrapper.get('form#create-account-form').trigger('submit.prevent')
      await flushPromises()

      expect(createAccountMock).toHaveBeenCalledTimes(1)
      expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
        platform,
        type: 'apikey',
        credentials: {
          base_url: `https://${platform}.example.com/v1`,
          api_key: `${platform}-key`,
          auth_mode: 'bearer'
        }
      })
      const credentials = createAccountMock.mock.calls[0]?.[0]?.credentials
      if (platform === 'bytedance') {
        expect(credentials).toMatchObject({
          model_mapping: {
            'doubao-seedance-2-0-260128': 'doubao-seedance-2-0-260128'
          }
        })
      } else if (platform === 'wan3') {
        expect(credentials).toMatchObject({
          model_mapping: {
            'wan3.0-video': 'wan3.0-video',
            'wan3.0-video-prime': 'wan3.0-video-prime'
          }
        })
      } else if (platform === 'minimax-h3') {
        expect(credentials).toMatchObject({
          model_mapping: {
            'MiniMax-H3': 'MiniMax-H3',
            'MiniMax-Hailuo-2.3': 'MiniMax-Hailuo-2.3'
          }
        })
      } else if (platform === 'pixverse-v6') {
        expect(credentials).toMatchObject({
          model_mapping: {
            'pixverse-v6': 'pixverse-v6'
          }
        })
      } else {
        expect(credentials).not.toHaveProperty('model_mapping')
      }
    }
  )

  it('uses the PP gateway default Base URL when creating a K-Ling video account', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.platforms.kling')
    await flushPromises()
    await wrapper.get('form#create-account-form input[data-tour="account-form-name"]').setValue('K-Ling account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('kling-key')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      base_url: 'https://app.ppapi.ai/v1',
      api_key: 'kling-key',
      auth_mode: 'bearer'
    })
  })

  it('leaves Codex session import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('leaves Codex PAT import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock).toHaveBeenCalledTimes(1)
    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('sends explicit true for Codex session import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex session import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  it('sends explicit true for Codex PAT import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex PAT import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })
})
