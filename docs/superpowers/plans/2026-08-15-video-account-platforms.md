# Video Account Platforms Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add K-Ling, Happy-Hourse, and Seedance as API Key video-account platforms in `admin/accounts`, without adding gateway routing.

**Architecture:** Keep the existing OpenAI-compatible provider metadata untouched and add a separate video-account metadata family. Account forms use that family for empty Base URL defaults, Bearer API-key credentials, and manual model configuration; account presentation consumes shared IDs, translations, icons, and colors.

**Tech Stack:** Vue 3, TypeScript, Vitest, vue-i18n, Tailwind CSS.

---

### Task 1: Define independent video-account metadata and account types

**Files:**
- Modify: `frontend/src/constants/platforms.ts`
- Modify: `frontend/src/constants/__tests__/platforms.spec.ts`
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Write the failing metadata test**

Add a second test block in `frontend/src/constants/__tests__/platforms.spec.ts`:

```ts
import {
  VIDEO_ACCOUNT_PLATFORMS,
  getVideoAccountPlatformMetadata,
  isVideoAccountPlatform
} from '../platforms'

it('defines video account platforms with empty Base URL defaults and Bearer auth', () => {
  expect(VIDEO_ACCOUNT_PLATFORMS).toEqual(['kling', 'happyhourse', 'seedance'])

  for (const platform of VIDEO_ACCOUNT_PLATFORMS) {
    const metadata = getVideoAccountPlatformMetadata(platform)
    expect(metadata.defaultBaseUrl).toBe('')
    expect(metadata.authScheme).toBe('bearer')
    expect(metadata.accountType).toBe('apikey')
    expect(isVideoAccountPlatform(platform)).toBe(true)
  }
})
```

- [ ] **Step 2: Run the metadata test to verify it fails**

Run:

```bash
cd frontend && npm run test:run -- src/constants/__tests__/platforms.spec.ts
```

Expected: FAIL because video-account exports do not exist.

- [ ] **Step 3: Add video-account metadata and type unions**

In `frontend/src/constants/platforms.ts`, add a separate metadata family:

```ts
export const VIDEO_ACCOUNT_PLATFORMS = ['kling', 'happyhourse', 'seedance'] as const
export type VideoAccountPlatform = (typeof VIDEO_ACCOUNT_PLATFORMS)[number]

export interface VideoAccountPlatformMetadata {
  id: VideoAccountPlatform
  label: string
  defaultBaseUrl: ''
  baseUrlPlaceholder: string
  apiKeyPlaceholder: string
  accountType: 'apikey'
  authScheme: 'bearer'
}

export const VIDEO_ACCOUNT_PLATFORM_METADATA: Record<
  VideoAccountPlatform,
  VideoAccountPlatformMetadata
> = {
  kling: {
    id: 'kling',
    label: 'K-Ling',
    defaultBaseUrl: '',
    baseUrlPlaceholder: 'https://your-kling-api.example.com/v1',
    apiKeyPlaceholder: 'sk-...',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  happyhourse: {
    id: 'happyhourse',
    label: 'Happy-Hourse',
    baseUrlPlaceholder: 'https://your-happyhourse-api.example.com/v1',
    apiKeyPlaceholder: 'sk-...',
    defaultBaseUrl: '',
    accountType: 'apikey',
    authScheme: 'bearer'
  },
  seedance: {
    id: 'seedance',
    label: 'Seedance',
    baseUrlPlaceholder: 'https://your-seedance-api.example.com/v1',
    apiKeyPlaceholder: 'sk-...',
    defaultBaseUrl: '',
    accountType: 'apikey',
    authScheme: 'bearer'
  }
}

export function isVideoAccountPlatform(platform: string): platform is VideoAccountPlatform {
  return (VIDEO_ACCOUNT_PLATFORMS as readonly string[]).includes(platform)
}

export function getVideoAccountPlatformMetadata(
  platform: VideoAccountPlatform
): VideoAccountPlatformMetadata {
  return VIDEO_ACCOUNT_PLATFORM_METADATA[platform]
}
```

Extend `AccountPlatform` in `frontend/src/types/index.ts` with:

```ts
  | 'kling'
  | 'happyhourse'
  | 'seedance'
```

Do not extend `GroupPlatform`: these accounts have no group routing in this scope.

- [ ] **Step 4: Run the metadata test to verify it passes**

Run:

```bash
cd frontend && npm run test:run -- src/constants/__tests__/platforms.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit the metadata slice**

```bash
git add frontend/src/constants/platforms.ts frontend/src/constants/__tests__/platforms.spec.ts frontend/src/types/index.ts
git commit -m "feat: define video account platforms"
```

### Task 2: Create and edit API Key video accounts

**Files:**
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/composables/useModelWhitelist.ts`
- Modify: `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`
- Modify: `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

- [ ] **Step 1: Write failing create-account tests**

In `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`, add a helper:

```ts
async function fillVideoApiKeyAccount(
  wrapper: ReturnType<typeof mountModal>,
  platform: 'kling' | 'happyhourse' | 'seedance'
) {
  await selectButtonByText(wrapper, `admin.accounts.platforms.${platform}`)
  await flushPromises()
  await wrapper.get('input[data-tour="account-form-name"]').setValue(`${platform} account`)
  await wrapper.get('input[type="text"]:not([data-tour="account-form-name"])')
    .setValue(`https://${platform}.example.com/v1`)
  await wrapper.get('input[type="password"]').setValue(`${platform}-key`)
}
```

Add a parameterized test:

```ts
it.each(['kling', 'happyhourse', 'seedance'] as const)(
  'creates %s as a Bearer API key video account',
  async (platform) => {
    const wrapper = mountModal()
    await fillVideoApiKeyAccount(wrapper, platform)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
      platform,
      type: 'apikey',
      credentials: {
        base_url: `https://${platform}.example.com/v1`,
        api_key: `${platform}-key`,
        auth_mode: 'bearer'
      }
    })
  }
)
```

Add a blank-Base-URL test for `kling` that asserts no create request is made.

- [ ] **Step 2: Run the create-account test to verify it fails**

Run:

```bash
cd frontend && npm run test:run -- src/components/account/__tests__/CreateAccountModal.spec.ts
```

Expected: FAIL because the video platform selector and credentials are absent.

- [ ] **Step 3: Implement video platform behavior in both account forms**

In `CreateAccountModal.vue`:

```ts
import {
  PROVIDER_PLATFORMS,
  VIDEO_ACCOUNT_PLATFORMS,
  getPlatformMetadata,
  getVideoAccountPlatformMetadata,
  isProviderPlatform,
  isVideoAccountPlatform
} from '@/constants/platforms'

const videoAccountPlatformOptions = VIDEO_ACCOUNT_PLATFORMS.map(
  platform => getVideoAccountPlatformMetadata(platform)
)
```

Render `videoAccountPlatformOptions` in the existing `data-tour="account-form-platform"`
row using the same `min-w-[8.5rem] shrink-0 whitespace-nowrap` button classes as the
other platforms.

For a video account platform:

```ts
if (isVideoAccountPlatform(form.platform)) {
  return t('admin.accounts.videoPlatform.baseUrlHint')
}
```

Use `getVideoAccountPlatformMetadata()` for the Base URL and API Key placeholders.
On platform change, force `accountCategory` to `apikey`, set
`modelRestrictionMode` to `mapping`, and clear preset whitelist models. Require a
non-empty Base URL alongside the API Key. Add `auth_mode: 'bearer'` to the
credentials payload.

In `EditAccountModal.vue`, use the same metadata for hints and placeholders;
require a Base URL when editing any video account platform; and set
`newCredentials.auth_mode = 'bearer'` for those platforms. Preserve any
administrator-defined `model_mapping`.

In `frontend/src/composables/useModelWhitelist.ts`, return an empty preset list for
`kling`, `happyhourse`, and `seedance`:

```ts
case 'kling':
case 'happyhourse':
case 'seedance':
  return []
```

This keeps model selection manual and avoids surfacing unrelated Claude presets.

- [ ] **Step 4: Add and run edit-account behavior tests**

In `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`, use an
`Account` fixture with `platform: 'seedance'`, `type: 'apikey'`,
`credentials.base_url`, and `credentials_status.has_api_key: true`. Assert that:

```ts
expect(wrapper.get('input[type="text"]').attributes('placeholder'))
  .toBe('https://your-seedance-api.example.com/v1')
```

After submit, assert the update credentials retain the supplied Base URL and
contain `auth_mode: 'bearer'`.

Run:

```bash
cd frontend && npm run test:run -- src/components/account/__tests__/CreateAccountModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit the account-form slice**

```bash
git add frontend/src/components/account/CreateAccountModal.vue frontend/src/components/account/EditAccountModal.vue frontend/src/composables/useModelWhitelist.ts frontend/src/components/account/__tests__/CreateAccountModal.spec.ts frontend/src/components/account/__tests__/EditAccountModal.spec.ts
git commit -m "feat: add video account forms"
```

### Task 3: Render and filter video account platforms

**Files:**
- Modify: `frontend/src/components/common/PlatformIcon.vue`
- Modify: `frontend/src/components/common/PlatformTypeBadge.vue`
- Modify: `frontend/src/utils/platformColors.ts`
- Modify: `frontend/src/components/admin/account/AccountTableFilters.vue`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Create: `frontend/src/components/admin/account/__tests__/AccountTableFilters.spec.ts`
- Create: `frontend/src/utils/__tests__/platformColors.spec.ts`

- [ ] **Step 1: Write failing presentation tests**

Create `frontend/src/components/admin/account/__tests__/AccountTableFilters.spec.ts`:

```ts
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AccountTableFilters from '../AccountTableFilters.vue'

const SelectStub = defineComponent({
  props: { options: { type: Array, default: () => [] } },
  template: '<div>{{ options.map(option => option.label).join("|") }}</div>'
})

it('offers all video account platforms', () => {
  const wrapper = mount(AccountTableFilters, {
    props: {
      searchQuery: '',
      filters: {},
      groups: []
    },
    global: {
      stubs: { SearchInput: true, Select: SelectStub },
      mocks: { $t: (key: string) => key }
    }
  })

  expect(wrapper.text()).toContain('admin.accounts.platforms.kling')
  expect(wrapper.text()).toContain('admin.accounts.platforms.happyhourse')
  expect(wrapper.text()).toContain('admin.accounts.platforms.seedance')
})
```

Create `frontend/src/utils/__tests__/platformColors.spec.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { platformLabel } from '../platformColors'

describe('platformLabel', () => {
  it('labels video account platforms', () => {
    expect(platformLabel('kling')).toBe('K-Ling')
    expect(platformLabel('happyhourse')).toBe('Happy-Hourse')
    expect(platformLabel('seedance')).toBe('Seedance')
  })
})
```

- [ ] **Step 2: Run the presentation tests to verify they fail**

Run:

```bash
cd frontend && npm run test:run -- src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/utils/__tests__/platformColors.spec.ts
```

Expected: FAIL because the platform options and labels are not present.

- [ ] **Step 3: Implement visual presentation and filtering**

Update `PlatformIcon.vue` to accept `AccountPlatform` so account-only platform IDs
can render without adding them to `GroupPlatform`. Add one recognizable,
monochrome SVG mark per video platform, with the existing generic fallback still
intact.

Add labels and balanced color mappings for all three IDs in
`PlatformTypeBadge.vue` and every `Record<Platform, string>` in
`platformColors.ts`. Extend `isPlatform()` and `platformLabel()`:

```ts
case 'kling': return 'K-Ling'
case 'happyhourse': return 'Happy-Hourse'
case 'seedance': return 'Seedance'
```

Append the three i18n-based options to `AccountTableFilters.vue`:

```ts
{ value: 'kling', label: t('admin.accounts.platforms.kling') },
{ value: 'happyhourse', label: t('admin.accounts.platforms.happyhourse') },
{ value: 'seedance', label: t('admin.accounts.platforms.seedance') }
```

Add translated platform names and the shared `videoPlatform.baseUrlHint` /
`videoPlatform.apiKeyHint` texts to both account locale files.

- [ ] **Step 4: Run presentation tests to verify they pass**

Run:

```bash
cd frontend && npm run test:run -- src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/utils/__tests__/platformColors.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit the presentation slice**

```bash
git add frontend/src/components/common/PlatformIcon.vue frontend/src/components/common/PlatformTypeBadge.vue frontend/src/utils/platformColors.ts frontend/src/components/admin/account/AccountTableFilters.vue frontend/src/i18n/locales/en/admin/accounts.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/components/admin/account/__tests__/AccountTableFilters.spec.ts frontend/src/utils/__tests__/platformColors.spec.ts
git commit -m "feat: display video account platforms"
```

### Task 4: Verify the isolated account-management scope

**Files:**
- Modify: `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`

- [ ] **Step 1: Add a non-routing regression test**

Extend the horizontal-selector assertion:

```ts
expect(platformButtons).toHaveLength(13)
for (const platform of ['kling', 'happyhourse', 'seedance']) {
  expect(wrapper.text()).toContain(`admin.accounts.platforms.${platform}`)
}
```

Do not update OpenAI-compatible import, key-use, group, quota, or gateway tests:
the video IDs must remain outside those capability lists.

- [ ] **Step 2: Run the focused frontend suite**

Run:

```bash
cd frontend && npm run test:run -- src/constants/__tests__/platforms.spec.ts src/components/account/__tests__/CreateAccountModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/admin/account/__tests__/AccountTableFilters.spec.ts src/utils/__tests__/platformColors.spec.ts
```

Expected: PASS.

- [ ] **Step 3: Run static checks and production build**

Run:

```bash
cd frontend && npm run typecheck
cd frontend && npm run build
```

Expected: both commands exit with status 0. Existing non-fatal Vite chunk-size
warnings may remain.

- [ ] **Step 4: Review the diff for scope containment**

Run:

```bash
git diff --check
git diff -- frontend/src/constants/platforms.ts frontend/src/components/account/CreateAccountModal.vue frontend/src/components/account/EditAccountModal.vue frontend/src/composables/useModelWhitelist.ts frontend/src/components/common/PlatformIcon.vue frontend/src/components/common/PlatformTypeBadge.vue frontend/src/utils/platformColors.ts frontend/src/components/admin/account/AccountTableFilters.vue frontend/src/i18n/locales/en/admin/accounts.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/types/index.ts
```

Expected: no whitespace errors and no changes to gateway routing, group routing,
quota-platform lists, pricing, or billing behavior.

- [ ] **Step 5: Commit the verification updates**

```bash
git add frontend/src/components/account/__tests__/CreateAccountModal.spec.ts
git commit -m "test: cover video account platform scope"
```
