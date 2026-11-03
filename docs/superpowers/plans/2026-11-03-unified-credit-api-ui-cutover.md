# Unified Credit API, UI, And Cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Switch customer/admin APIs and UI to one credit account, expose CNY cost only to administrators, add a guarded maintenance cutover tool, and remove active mixed-unit behavior.

**Architecture:** Backend DTOs serialize decimal credits as strings from `credit_accounts`; frontend models retain strings and use `decimal.js` for formatting/calculation. Admin pricing separates customer credit sale prices from provider CNY cost. A dry-run-first CLI performs cutover only after reconciliation and async-drain gates pass.

**Tech Stack:** Go, Gin, PostgreSQL, Vue 3, TypeScript, Pinia, `decimal.js`, Vitest, Wire.

**Spec:** `docs/superpowers/specs/2026-11-03-unified-credit-billing-design.md`

## Global Constraints

- Complete the first four unified-credit plans before enabling cutover.
- Admin and customer balances read the same `credit_accounts` row.
- New API credit values are decimal strings; frontend code must not coerce them to `number` for arithmetic.
- Customer UI uses `积分`; provider cost UI uses `人民币`/`¥` and remains admin-only.
- The cutover CLI defaults to dry-run and never supplies default migration factors.
- Legacy columns remain compatibility mirrors for one release; this plan does not drop them.

---

### Task 1: Expose canonical credit balances and economics APIs

**Files:**
- Modify: `backend/internal/service/user_balance.go`
- Modify: `backend/internal/handler/user_balance.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/handler/admin/user_handler.go`
- Modify: `backend/internal/handler/organization_handler.go`
- Modify: `backend/internal/service/organization.go`
- Modify: `backend/internal/handler/usage_handler.go`
- Modify: `backend/internal/handler/admin/usage_handler.go`
- Create: `backend/internal/handler/user_balance_credit_test.go`
- Create: `backend/internal/handler/organization_credit_test.go`
- Create: `backend/internal/handler/usage_credit_contract_test.go`
- Create: `backend/internal/handler/admin/user_credit_balance_test.go`
- Create: `backend/internal/handler/admin/usage_credit_contract_test.go`

**Interfaces:**
- Customer/admin user and organization responses return `available_credits` and `held_credits` as strings.
- Usage responses return `charged_credits` as a string.
- Admin-only usage responses additionally return CNY economics strings.

- [ ] **Step 1: Write API contract tests**

```go
func TestUserBalanceReturnsCanonicalCredits(t *testing.T) {
    // Seed one credit account with 32.8 available and 1.2 held.
    // Require exact JSON strings and absence of balance/display_balance.
}
```

Add equivalent admin user and organization assertions. Add a customer usage test proving `provider_cost_cny` is absent and an admin usage test proving it is present.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/handler/... -run 'CanonicalCredits|CreditBalance|UsageEconomics' -count=1`

Expected: FAIL because DTOs still expose legacy floats.

- [ ] **Step 3: Replace balance reads**

Read `CreditAccountService.Get` in customer, admin, and organization handlers. The canonical `/user/balance` response contains only the two credit fields. During the observation release, the broader current-user DTO keeps deprecated `balance`, `display_balance`, and `frozen_balance` aliases, but maps all of them directly from canonical available/held credits so they cannot diverge.

- [ ] **Step 4: Split customer and admin usage DTOs**

Customer DTOs include charged credits and pricing snapshot. Admin DTOs add provider cost, recognized revenue, estimated gross profit, and cost snapshot. Mount economics only under the existing admin-authenticated route group.

- [ ] **Step 5: Run handler tests**

Run: `cd backend && go test ./internal/handler/... ./internal/server/routes -run 'Credit|Usage|Organization|Admin.*User' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/user_balance.go backend/internal/handler/user_balance.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/handler/admin/user_handler.go backend/internal/handler/organization_handler.go backend/internal/service/organization.go backend/internal/handler/usage_handler.go backend/internal/handler/admin/usage_handler.go backend/internal/handler/user_balance_credit_test.go backend/internal/handler/organization_credit_test.go backend/internal/handler/usage_credit_contract_test.go backend/internal/handler/admin/user_credit_balance_test.go backend/internal/handler/admin/usage_credit_contract_test.go
git commit -m "feat(api): expose canonical credit balances"
```

### Task 2: Migrate frontend balance and shared credit formatting

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/pnpm-lock.yaml`
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/utils/credits.ts`
- Create: `frontend/src/utils/__tests__/credits.spec.ts`
- Remove after migration: `frontend/src/utils/balance.ts`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/user/profile/ProfileInfoCard.vue`
- Modify: `frontend/src/views/admin/UsersView.vue`
- Modify: `frontend/src/components/admin/user/UserBalanceModal.vue`
- Modify: `frontend/src/components/admin/user/UserBalanceHistoryModal.vue`
- Create: `frontend/src/components/layout/__tests__/AppHeader.credit.spec.ts`
- Modify: `frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/UsersView.spec.ts`

**Interfaces:**
- `CreditAmount` is a string type alias at API boundaries.
- `formatCredits(value: CreditAmount, places?: number): string` uses `decimal.js`.
- User types contain `available_credits` and `held_credits`; no display-balance preference helper remains.

- [ ] **Step 1: Add decimal.js as a direct dependency**

Run: `pnpm --dir frontend add decimal.js@10.6.0`

Expected: `package.json` and lockfile list the direct dependency without changing unrelated packages.

- [ ] **Step 2: Write formatting tests**

Cover `2088.00000000`, `32.80000000`, very large values, eight-decimal values, invalid input fallback, and addition of available plus held without IEEE-754 loss.

- [ ] **Step 3: Run tests and verify they fail**

Run: `pnpm --dir frontend exec vitest run src/utils/__tests__/credits.spec.ts`

Expected: FAIL because `credits.ts` does not exist.

- [ ] **Step 4: Implement credit types/formatting and migrate core views**

Replace `userDisplayBalance`, `Number(...)`, `toFixed`, and money-format helpers on customer credits. The admin user list and customer header must render the same API field.

- [ ] **Step 5: Run focused frontend tests**

Run: `pnpm --dir frontend exec vitest run src/utils/__tests__/credits.spec.ts src/components/user/profile/__tests__/ProfileInfoCard.spec.ts src/views/admin/__tests__/UsersView.spec.ts`

Expected: PASS with canonical-balance assertions in all three views.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml frontend/src/types/index.ts frontend/src/utils/credits.ts frontend/src/utils/__tests__/credits.spec.ts frontend/src/utils/balance.ts frontend/src/components/layout/AppHeader.vue frontend/src/components/layout/__tests__/AppHeader.credit.spec.ts frontend/src/components/user/profile/ProfileInfoCard.vue frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts frontend/src/views/admin/UsersView.vue frontend/src/components/admin/user frontend/src/views/admin/__tests__/UsersView.spec.ts
git commit -m "feat(frontend): display canonical credit balances"
```

### Task 3: Separate customer credit price and provider CNY cost in channel admin

**Files:**
- Modify: `backend/internal/handler/admin/channel_handler.go`
- Modify: `backend/internal/handler/admin/channel_handler_test.go`
- Modify: `frontend/src/api/admin/channels.ts`
- Modify: `frontend/src/components/admin/channel/types.ts`
- Modify: `frontend/src/components/admin/channel/PricingEntryCard.vue`
- Modify: `frontend/src/components/admin/channel/IntervalRow.vue`
- Modify: `frontend/src/views/admin/ChannelsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/channels.ts`
- Modify: `frontend/src/i18n/locales/en/admin/channels.ts`
- Modify/Create tests under `frontend/src/components/admin/channel/__tests__/`

**Interfaces:**
- Channel DTOs expose explicit `*_price_credits` customer fields and `provider_*_cost_cny` internal fields.
- Form sections are labeled `客户积分售价` and `供应商成本（人民币）`.

- [ ] **Step 1: Write backend DTO contract tests**

Assert create/update/list round-trip both units without mapping one to the other. Reject negative values and reject a credit-price field submitted with a currency marker.

- [ ] **Step 2: Write frontend form tests**

Assert both sections render, video price help says `积分/秒`, provider cost says `元/秒`, and submitted JSON keeps values in distinct properties.

- [ ] **Step 3: Run tests and verify they fail**

Run: `cd backend && go test ./internal/handler/admin -run 'Channel.*Credit.*CNY' -count=1`

Run: `pnpm --dir frontend exec vitest run src/components/admin/channel/__tests__`

Expected: FAIL because current DTO/form fields are ambiguous.

- [ ] **Step 4: Implement backend mapping and validation**

Use decimal strings for new price/cost fields. Legacy numeric fields are read only when the global flag is false and are never auto-copied into the new catalog.

- [ ] **Step 5: Implement split admin controls**

Reuse existing pricing-entry structure, but present two unambiguous sections without nested cards. Use numeric inputs with fixed unit suffixes; retain all existing model, tier, and interval controls.

- [ ] **Step 6: Run tests**

Run: `cd backend && go test ./internal/handler/admin -run 'Channel' -count=1`

Run: `pnpm --dir frontend exec vitest run src/components/admin/channel/__tests__`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/handler/admin/channel_handler.go backend/internal/handler/admin/channel_handler_test.go frontend/src/api/admin/channels.ts frontend/src/components/admin/channel frontend/src/views/admin/ChannelsView.vue frontend/src/i18n/locales/zh/admin/channels.ts frontend/src/i18n/locales/en/admin/channels.ts
git commit -m "feat(admin): separate credit pricing from CNY provider cost"
```

### Task 4: Migrate quota, usage, organization, and media views

**Files:**
- Modify: `frontend/src/api/usage.ts`
- Modify: `frontend/src/api/organizations.ts`
- Modify: `frontend/src/api/batchImage.ts`
- Modify: `frontend/src/api/admin/users.ts`
- Modify: `frontend/src/api/admin/usage.ts`
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/views/KeyUsageView.vue`
- Modify: `frontend/src/views/user/SubscriptionsView.vue`
- Modify: `frontend/src/views/user/OrganizationsView.vue`
- Modify: `frontend/src/views/user/UsageView.vue`
- Modify: `frontend/src/views/user/BatchImageGuideView.vue`
- Modify: `frontend/src/views/admin/UsageView.vue`
- Modify: `frontend/src/views/user/__tests__/KeysView.spec.ts`
- Modify: `frontend/src/views/__tests__/KeyUsageView.spec.ts`
- Modify: `frontend/src/views/user/__tests__/UsageView.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- Create: `frontend/src/views/user/__tests__/SubscriptionsView.credit.spec.ts`
- Create: `frontend/src/views/user/__tests__/OrganizationsView.credit.spec.ts`
- Create: `frontend/src/views/user/__tests__/BatchImageGuideView.credit.spec.ts`

**Interfaces:**
- All customer limits, usage, holds, and charges are credit strings.
- Only admin usage views accept provider-cost/profit CNY fields.

- [ ] **Step 1: Update API types and fixtures first**

Replace `_usd`, `actual_cost`, and display-balance customer fields with explicit credit fields in new contracts. Keep legacy optional fields only in compatibility parsers.

- [ ] **Step 2: Write cross-view consistency tests**

Use the same fixture (`available_credits=2088`, one 24-credit video) and assert header/user/organization/usage/key/subscription/media screens all show credits and never `$`.

- [ ] **Step 3: Run focused tests and verify they fail**

Run: `pnpm --dir frontend exec vitest run src/views/user/__tests__/KeysView.spec.ts src/views/__tests__/KeyUsageView.spec.ts src/views/user/__tests__/UsageView.spec.ts`

Expected: FAIL until views use the new contracts.

- [ ] **Step 4: Migrate customer views**

Use `formatCredits` throughout. Rename visible `费用` to `消耗积分` where it means customer charge. Holds show `冻结积分`; subscriptions and quotas show `已用积分 / 积分额度`.

- [ ] **Step 5: Migrate admin usage economics**

Display charged credits separately from provider cost CNY, recognized revenue CNY, and estimated gross profit CNY. Never sum credits and CNY in one chart axis or total.

- [ ] **Step 6: Run frontend tests, typecheck, and lint**

Run: `pnpm --dir frontend run test:run`

Run: `pnpm --dir frontend run typecheck`

Run: `pnpm --dir frontend run lint:check`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api/usage.ts frontend/src/api/organizations.ts frontend/src/api/batchImage.ts frontend/src/api/admin/users.ts frontend/src/api/admin/usage.ts frontend/src/views/user/KeysView.vue frontend/src/views/KeyUsageView.vue frontend/src/views/user/SubscriptionsView.vue frontend/src/views/user/OrganizationsView.vue frontend/src/views/user/UsageView.vue frontend/src/views/user/BatchImageGuideView.vue frontend/src/views/admin/UsageView.vue frontend/src/views/user/__tests__/KeysView.spec.ts frontend/src/views/__tests__/KeyUsageView.spec.ts frontend/src/views/user/__tests__/UsageView.spec.ts frontend/src/views/admin/__tests__/UsageView.spec.ts frontend/src/views/user/__tests__/SubscriptionsView.credit.spec.ts frontend/src/views/user/__tests__/OrganizationsView.credit.spec.ts frontend/src/views/user/__tests__/BatchImageGuideView.credit.spec.ts frontend/src/types/index.ts
git commit -m "feat(frontend): unify usage and quotas in credits"
```

### Task 5: Add guarded cutover CLI and reconciliation metrics

**Files:**
- Create: `backend/cmd/credit-cutover/main.go`
- Create: `backend/cmd/credit-cutover/main_test.go`
- Create: `backend/cmd/credit-cutover/README.md`
- Modify: `backend/internal/service/credit_migration.go`
- Modify: `backend/internal/repository/credit_migration_repo.go`
- Create: `backend/internal/service/credit_metrics.go`
- Modify: `backend/internal/service/ops_metrics_collector.go`
- Modify: `backend/internal/service/ops_metrics_collector_test.go`

**Interfaces:**
- CLI flags: `--execute`, `--cutover-id`, `--legacy-credit-basis-cny-per-credit`, and `--legacy-quota-credits-per-usd`.
- Dry-run is the default.
- Execute requires zero open holds, zero legacy frozen balances, valid price catalog, explicit positive factors, and zero preview errors.

- [ ] **Step 1: Write CLI safety tests**

Cover default dry-run, missing execute parameters, open holds, frozen legacy balances, incomplete price catalog, reconciliation failure, successful execute, and rerun with the same cutover ID.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./cmd/credit-cutover ./internal/service ./internal/repository -run 'CreditCutover|CutoverReadiness' -count=1`

Expected: FAIL because the command/readiness checks do not exist.

- [ ] **Step 3: Implement dry-run and execute modes**

Dry-run prints counts/totals/issues and performs no writes. Execute acquires a PostgreSQL advisory lock, rechecks readiness, runs the migration transaction, sets direct legacy mirrors, runs blocking reconciliation, and exits nonzero without enabling traffic when any check fails.

- [ ] **Step 4: Add metrics and alerts**

Expose counters/gauges for reconciliation failures, insufficient-credit rejects, duplicate idempotency hits, open holds, provider-cost missing rows, and admin/customer sampled balance mismatches.

- [ ] **Step 5: Document the maintenance runbook**

Include exact preview, async-drain, execute, verify, enable-config, resume, and maintenance rollback commands. State that database ledger rows are never deleted on rollback.

- [ ] **Step 6: Run tests**

Run: `cd backend && go test ./cmd/credit-cutover ./internal/service ./internal/repository -run 'CreditCutover|CreditMigration|CreditReconciliation' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/cmd/credit-cutover/main.go backend/cmd/credit-cutover/main_test.go backend/cmd/credit-cutover/README.md backend/internal/service/credit_migration.go backend/internal/service/credit_metrics.go backend/internal/repository/credit_migration_repo.go backend/internal/service/ops_metrics_collector.go backend/internal/service/ops_metrics_collector_test.go
git commit -m "feat(ops): add guarded unified credit cutover"
```

### Task 6: Final verification and active legacy-path removal

**Files:**
- Modify: `docs/PAYMENT.md`
- Modify: `docs/PAYMENT_CN.md`
- Modify: `docs/platform-model-pricing.md`
- Create: `docs/UNIFIED_CREDIT_OPERATIONS.md`

**Interfaces:**
- No active code path performs ratio conversion or treats USD as a customer quota.
- Legacy fields remain database mirrors only and are marked for later physical removal.

- [ ] **Step 1: Run forbidden-pattern scans**

Run: `rg -n 'display_balance\s*/\s*balance|display_balance.*balance|credits_per_settlement_unit|daily_usage_usd|weekly_usage_usd|monthly_usage_usd' backend/internal frontend/src`

Run: `rg -n 'Used quota amount in USD|Quota limit in USD|customer.*USD|用户.*USD' backend frontend/src`

Expected: no active billing/UI matches; historical migration comments and explicitly named compatibility readers are documented exceptions.

- [ ] **Step 2: Enforce the prior-task removal gate**

The scans must show that prior tasks already made the credit path authoritative and deleted ratio arithmetic and mixed-unit customer quote code. If an active-code match remains, stop execution and amend the owning earlier plan/task with the exact file and test rather than applying an unplanned cleanup here.

- [ ] **Step 3: Run all verification suites**

Run: `cd backend && go test ./...`

Run: `pnpm --dir frontend run test:run`

Run: `pnpm --dir frontend run typecheck`

Run: `pnpm --dir frontend run lint:check`

Run: `pnpm --dir frontend run build`

Expected: PASS.

- [ ] **Step 4: Verify acceptance examples**

Run the end-to-end fixtures and require:

```text
pay 1288 CNY -> available credits 2088.00000000
hold 15 * 1.6 -> available 2064.00000000, held 24.00000000
capture 24 -> available 2064.00000000, held 0.00000000
admin balance == customer balance == 2064.00000000
```

- [ ] **Step 5: Update operational and pricing documentation**

Document recharge credits, customer credit pricing, provider CNY cost, profit semantics, API decimal strings, cutover, reconciliation, and rollback.

- [ ] **Step 6: Commit**

```bash
git add docs/PAYMENT.md docs/PAYMENT_CN.md docs/platform-model-pricing.md docs/UNIFIED_CREDIT_OPERATIONS.md
git commit -m "feat(billing): complete unified credit cutover"
```
