# Model List Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a user-facing `/models` page that displays static model pricing grouped by plan/provider.

**Architecture:** The page is a standalone Vue view with static local pricing data and no backend calls. Router and sidebar changes expose the page to authenticated users, while i18n keys provide Chinese and English navigation/title text.

**Tech Stack:** Vue 3, TypeScript, Vue Router, vue-i18n, Vitest, Vue Test Utils, Tailwind utility classes.

---

## File Structure

- Create `frontend/src/views/user/ModelListView.vue`: static pricing data, selected-group state, responsive A-layout UI, reset button.
- Create `frontend/src/views/user/__tests__/ModelListView.spec.ts`: focused component tests for default data, group switching, and pricing columns.
- Modify `frontend/src/router/index.ts`: add authenticated `/models` route.
- Modify `frontend/src/components/layout/AppSidebar.vue`: add a user/personal navigation item for `/models`.
- Modify `frontend/src/i18n/locales/zh/dashboard.ts`: add `nav.modelList` and `modelList` page strings.
- Modify `frontend/src/i18n/locales/en/dashboard.ts`: add English equivalents.
- Optionally modify existing sidebar/router tests only if the new route or label breaks them.

### Task 1: Add Failing Model List View Test

**Files:**
- Create: `frontend/src/views/user/__tests__/ModelListView.spec.ts`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/views/user/__tests__/ModelListView.spec.ts` with:

```ts
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelListView from '../ModelListView.vue'

const translations: Record<string, string> = {
  'modelList.title': '模型与价格',
  'modelList.description': '按你可用的分组查看模型价格。',
  'modelList.refresh': '重置分组',
  'modelList.groupCount': '{count} 个模型',
  'modelList.columns.model': '模型',
  'modelList.columns.platformInput': '本平台输入',
  'modelList.columns.platformOutput': '本平台输出',
  'modelList.columns.officialInput': '官方输入',
  'modelList.columns.officialOutput': '官方输出',
}

function renderView() {
  return mount(ModelListView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
      mocks: {
        $t: (key: string, params?: Record<string, unknown>) => {
          const value = translations[key] ?? key
          if (!params) return value
          return Object.entries(params).reduce(
            (text, [paramKey, paramValue]) => text.replace(`{${paramKey}}`, String(paramValue)),
            value,
          )
        },
      },
    },
  })
}

describe('ModelListView', () => {
  it('renders gpt pro pricing by default', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型与价格')
    expect(wrapper.text()).toContain('gpt pro')
    expect(wrapper.text()).toContain('openai 分组模型价格')
    expect(wrapper.text()).toContain('codex-auto-review')
    expect(wrapper.text()).toContain('¥1.30 / 1M')
    expect(wrapper.text()).toContain('$30.00 / 1M')
  })

  it('switches groups and renders Claude-kiro pricing', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-group-Claude-kiro"]').trigger('click')

    expect(wrapper.text()).toContain('Claude-kiro')
    expect(wrapper.text()).toContain('anthropic 分组模型价格')
    expect(wrapper.text()).toContain('0.3x')
    expect(wrapper.text()).toContain('claude-haiku-4.5')
    expect(wrapper.text()).toContain('¥0.0750 / 1M')
    expect(wrapper.text()).toContain('$1.25 / 1M')
  })

  it('renders all expected pricing columns', () => {
    const wrapper = renderView()

    expect(wrapper.text()).toContain('模型')
    expect(wrapper.text()).toContain('本平台输入')
    expect(wrapper.text()).toContain('本平台输出')
    expect(wrapper.text()).toContain('官方输入')
    expect(wrapper.text()).toContain('官方输出')
  })

  it('resets to the default group when refresh is clicked', async () => {
    const wrapper = renderView()

    await wrapper.get('[data-testid="model-group-Claude-max-1.1"]').trigger('click')
    expect(wrapper.text()).toContain('Claude-max-1.1')

    await wrapper.get('[data-testid="model-list-refresh"]').trigger('click')
    expect(wrapper.text()).toContain('gpt pro')
    expect(wrapper.text()).toContain('¥1.30 / 1M')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/ModelListView.spec.ts
```

Expected: FAIL because `../ModelListView.vue` does not exist.

### Task 2: Implement Model List View

**Files:**
- Create: `frontend/src/views/user/ModelListView.vue`
- Test: `frontend/src/views/user/__tests__/ModelListView.spec.ts`

- [ ] **Step 1: Create the view with static pricing data and A-layout**

Create `frontend/src/views/user/ModelListView.vue`. Use `AppLayout`, `Icon`, `ref`, `computed`, and `useI18n`. Define local types:

```ts
interface ModelPriceRow {
  model: string
  platformInput: string
  platformOutput: string
  officialInput: string
  officialOutput: string
}

interface ModelPriceGroup {
  id: string
  name: string
  provider: 'openai' | 'anthropic'
  multiplier?: string
  rows: ModelPriceRow[]
}
```

Implementation requirements:

- `pricingGroups` must include every group and row exactly as listed in the user request.
- `selectedGroupId` defaults to `pricingGroups[0].id`.
- `selectedGroup` is computed from `selectedGroupId`.
- `selectGroup(group.id)` switches the active group.
- `resetSelection()` sets `selectedGroupId` back to the first group.
- Add `data-testid="model-list-refresh"` to the refresh button.
- Add each group button test id with `:data-testid="\`model-group-${group.name}\`"`.
- Render title and description through `t('modelList.title')` and `t('modelList.description')`.
- Render subtitle as `{{ selectedGroup.provider }} 分组模型价格`.
- Render multiplier badge only when `selectedGroup.multiplier` exists.
- Table columns use `modelList.columns.*` i18n keys.
- Make the table horizontally scrollable on small screens.

- [ ] **Step 2: Run the focused test**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/ModelListView.spec.ts
```

Expected: PASS.

- [ ] **Step 3: Commit the page and test**

Run:

```bash
git add frontend/src/views/user/ModelListView.vue frontend/src/views/user/__tests__/ModelListView.spec.ts
git commit -m "feat: add model list pricing view"
```

### Task 3: Add Route, Sidebar Entry, And I18n

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

- [ ] **Step 1: Add i18n keys**

In `frontend/src/i18n/locales/zh/dashboard.ts`, add:

```ts
nav: {
  modelList: '模型列表'
}
```

to the existing `nav` object, preserving all existing keys. Also add a top-level sibling:

```ts
modelList: {
  title: '模型与价格',
  description: '按你可用的分组查看模型价格。',
  refresh: '重置分组',
  groupCount: '{count} 个模型',
  columns: {
    model: '模型',
    platformInput: '本平台输入',
    platformOutput: '本平台输出',
    officialInput: '官方输入',
    officialOutput: '官方输出',
  },
}
```

In `frontend/src/i18n/locales/en/dashboard.ts`, add matching keys:

```ts
nav: {
  modelList: 'Model List'
}
```

and:

```ts
modelList: {
  title: 'Models & Pricing',
  description: 'View model pricing by available group.',
  refresh: 'Reset group',
  groupCount: '{count} models',
  columns: {
    model: 'Model',
    platformInput: 'Platform Input',
    platformOutput: 'Platform Output',
    officialInput: 'Official Input',
    officialOutput: 'Official Output',
  },
}
```

- [ ] **Step 2: Add the authenticated route**

In `frontend/src/router/index.ts`, add the route near `/usage` and `/available-channels`:

```ts
{
  path: '/models',
  name: 'ModelList',
  component: () => import('@/views/user/ModelListView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: false,
    title: 'Model List',
    titleKey: 'modelList.title',
    descriptionKey: 'modelList.description'
  }
},
```

- [ ] **Step 3: Add sidebar navigation item**

In `frontend/src/components/layout/AppSidebar.vue`, add the item in `buildSelfNavItems()` after `/usage` and before `/available-channels`:

```ts
{ path: '/models', label: t('nav.modelList'), icon: PriceTagIcon, hideInSimpleMode: true },
```

- [ ] **Step 4: Run route/sidebar related tests**

Run:

```bash
cd frontend
pnpm test:run src/components/layout/__tests__/AppSidebar.spec.ts src/router/__tests__/title.spec.ts src/router/__tests__/guards.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit route/sidebar/i18n changes**

Run:

```bash
git add frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts
git commit -m "feat: expose model list page"
```

### Task 4: Final Verification

**Files:**
- Verify all files touched in Tasks 1-3.

- [ ] **Step 1: Run focused frontend tests**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/ModelListView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts src/router/__tests__/title.spec.ts src/router/__tests__/guards.spec.ts
```

Expected: PASS.

- [ ] **Step 2: Run typecheck**

Run:

```bash
cd frontend
pnpm typecheck
```

Expected: PASS.

- [ ] **Step 3: Run build**

Run:

```bash
cd frontend
pnpm build
```

Expected: PASS. Vite chunk-size warnings are acceptable if there are no TypeScript or build errors.

- [ ] **Step 4: Inspect git status**

Run:

```bash
git status --short
```

Expected: only intentional model-list implementation changes plus pre-existing unrelated working tree changes remain.
