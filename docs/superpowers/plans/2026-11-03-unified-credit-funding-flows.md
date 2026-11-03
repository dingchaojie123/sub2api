# Unified Credit Funding Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route recharge, grants, redeem codes, admin adjustments, affiliate transfers, refunds, and organization funding through the unified credit ledger while preserving the disabled legacy path before cutover.

**Architecture:** Existing business services keep ownership of payment and authorization rules, but delegate every enabled credit mutation to `CreditAccountService`. Payment orders store explicit cash CNY and granted-credit snapshots. The global billing flag selects exactly one writer; ledger idempotency keys derive from stable business IDs.

**Tech Stack:** Go, Ent, PostgreSQL migrations, `shopspring/decimal`, Wire, Testify.

**Spec:** `docs/superpowers/specs/2026-11-03-unified-credit-billing-design.md`

## Global Constraints

- Complete `2026-11-03-unified-credit-ledger-foundation.md` first.
- Recharge 1288 CNY grants exactly 2088 credits.
- Enabled flows never call legacy balance update methods after a credit mutation.
- Disabled flows retain existing behavior until the global maintenance cutover.
- Stable business IDs form idempotency keys; random keys are forbidden.
- Existing uncommitted PP-video files remain untouched.

---

### Task 1: Add explicit payment cash and credit snapshots

**Files:**
- Create: `backend/migrations/209_payment_credit_snapshots.sql`
- Create: `backend/migrations/payment_credit_snapshots_migration_test.go`
- Modify: `backend/ent/schema/payment_order.go`
- Modify: generated files under `backend/ent/paymentorder/` and `backend/ent/`
- Modify: `backend/internal/service/payment_amounts.go`
- Create: `backend/internal/service/payment_amounts_credit_test.go`

**Interfaces:**
- Produces payment fields `cash_amount_cny`, `granted_credits`, `payment_amount`, `payment_currency`, `exchange_rate_to_cny`, and `exchange_rate_at`.
- Produces `PaymentOrderGrantedCredits(*ent.PaymentOrder) decimal.Decimal` and `PaymentOrderCashAmountCNY(*ent.PaymentOrder) decimal.Decimal`.

- [ ] **Step 1: Write migration and product snapshot tests**

```go
func TestPaymentOrderGrantedCredits_1288Product(t *testing.T) {
    order := &ent.PaymentOrder{OrderType: payment.OrderTypeBalance, Amount: 1288}
    got := PaymentOrderGrantedCredits(order)
    require.True(t, got.Equal(decimal.RequireFromString("2088")))
}
```

Assert the migration adds numeric scale-8 CNY/credit fields and does not rewrite completed historical orders.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./migrations -run 'PaymentOrderGrantedCredits|PaymentCreditSnapshots' -count=1`

Expected: FAIL because explicit fields/helpers do not exist.

- [ ] **Step 3: Add schema and Ent fields**

Add nullable fields for legacy rows. New balance orders must populate both cash and credits before payment submission. Preserve existing `amount`, `pay_amount`, and provider snapshot fields for gateway compatibility.

- [ ] **Step 4: Implement decimal snapshot helpers**

Read explicit snapshots first, then the existing product snapshot, then legacy fallback. Convert only at the API edge; all multiplication and comparison use decimal values.

- [ ] **Step 5: Regenerate Ent and run tests**

Run: `cd backend && make generate`

Run: `cd backend && go test ./internal/service ./migrations -run 'PaymentOrder.*Credits|PaymentCreditSnapshots|BalanceRecharge' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/209_payment_credit_snapshots.sql backend/migrations/payment_credit_snapshots_migration_test.go backend/ent backend/internal/service/payment_amounts.go backend/internal/service/payment_amounts_credit_test.go
git commit -m "feat(payment): snapshot cash and granted credits"
```

### Task 2: Route payment fulfillment and refunds through credits

**Files:**
- Modify: `backend/internal/service/payment_service.go`
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/payment_refund.go`
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Modify: `backend/internal/service/payment_refund_test.go`
- Modify: `backend/internal/service/wire.go`

**Interfaces:**
- Adds `PaymentService.SetCreditAccountService(*CreditAccountService)`.
- Uses recharge idempotency key `recharge:payment-order:<order-id>`.
- Uses refund idempotency key `refund:payment-order:<order-id>:<refund-sequence>`.

- [ ] **Step 1: Write enabled fulfillment tests**

Assert a completed 1288 order calls `Grant` with amount 2088 and revenue basis 1288, writes no legacy `UpdateBalance`, and replay returns the prior result without a second grant.

- [ ] **Step 2: Write enabled refund tests**

Cover full refund with enough credits, rejection when credits are insufficient, forced refund flooring at zero with unrecovered value in metadata, and replay safety.

- [ ] **Step 3: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service -run 'Payment.*Credit|Refund.*Credit' -count=1`

Expected: FAIL because payment has no credit dependency.

- [ ] **Step 4: Implement the global-flag branch**

When enabled, payment completion creates one credit grant using explicit snapshots. When disabled, call the unchanged legacy code. Never run both. Attach order ID, product ID, cash CNY, granted credits, payer, and provider trade ID as ledger metadata.

- [ ] **Step 5: Implement refund compensation**

Resolve the credits represented by the refund fraction using decimal arithmetic. Normal refund first verifies availability. Forced refund deducts up to available credits and records `requested_credits`, `deducted_credits`, and `unrecovered_credits`.

- [ ] **Step 6: Wire and test**

Run: `cd backend && make generate`

Run: `cd backend && go test ./internal/service -run 'Payment|Refund' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/payment_service.go backend/internal/service/payment_fulfillment.go backend/internal/service/payment_refund.go backend/internal/service/payment_fulfillment_test.go backend/internal/service/payment_refund_test.go backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat(payment): fulfill and refund unified credits"
```

### Task 3: Migrate redeem, admin, affiliate, and authentication grants

**Files:**
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/service/redeem_service_redeem_test.go`
- Modify: `backend/internal/service/admin_user.go`
- Create: `backend/internal/service/admin_user_credit_test.go`
- Modify: `backend/internal/service/affiliate_service.go`
- Modify: `backend/internal/repository/affiliate_repo.go`
- Modify: `backend/internal/repository/affiliate_repo_integration_test.go`
- Modify: `backend/internal/service/auth_service.go`
- Modify: `backend/internal/service/auth_oauth_first_bind.go`
- Create: `backend/internal/service/credit_grant_sources_test.go`

**Interfaces:**
- Redeem key: `redeem:<redeem-code-id>:<user-id>`.
- Admin key: `admin-adjustment:<adjustment-id>`; admin identity and reason are mandatory metadata.
- Affiliate key: `affiliate-transfer:<affiliate-ledger-id>`.
- Signup/bind key: `auth-grant:<identity-id>:<grant-kind>`.

- [ ] **Step 1: Add failing tests for each enabled grant source**

Each test asserts exactly one typed credit ledger command and zero calls to `UpdateBalance`/`ApplyRedeemBalanceAdjustment` when enabled. Retain one disabled-path regression per source.

- [ ] **Step 2: Run the focused tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Redeem.*Credit|Admin.*Credit|Affiliate.*Credit|Auth.*CreditGrant' -count=1`

Expected: FAIL because the services still mutate legacy balances.

- [ ] **Step 3: Inject the credit service without widening unrelated constructors**

Follow existing setter/provider patterns where constructor fan-out is high. Wire must always attach the dependency in production. Tests may omit it only when `credit_billing_enabled=false`.

- [ ] **Step 4: Replace enabled mutations**

Use `Grant`, `Deduct`, or `Transfer` with stable keys and source-specific metadata. Affiliate conversion transfers its actual CNY revenue basis only when the source ledger has one; promotional affiliate grants carry zero basis.

- [ ] **Step 5: Run focused and integration tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Redeem|AdminUser|Affiliate|Auth.*Grant' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/redeem_service.go backend/internal/service/redeem_service_redeem_test.go backend/internal/service/admin_user.go backend/internal/service/admin_user_credit_test.go backend/internal/service/affiliate_service.go backend/internal/repository/affiliate_repo.go backend/internal/repository/affiliate_repo_integration_test.go backend/internal/service/auth_service.go backend/internal/service/auth_oauth_first_bind.go backend/internal/service/credit_grant_sources_test.go
git commit -m "feat(billing): migrate credit grant and adjustment flows"
```

### Task 4: Replace organization funding and return with atomic transfers

**Files:**
- Modify: `backend/internal/service/organization.go`
- Modify: `backend/internal/repository/organization_repo.go`
- Modify: `backend/internal/repository/organization_repo_test.go`
- Create: `backend/internal/repository/organization_repo_credit_integration_test.go`

**Interfaces:**
- Organization funding calls `TransferCreditsCommand` from user owner to organization owner.
- Correlation key: `organization-fund:<organization-id>:<operation-id>`.
- Organization deletion/return key: `organization-return:<organization-id>:<operation-id>`.

- [ ] **Step 1: Write transfer behavior tests**

Cover funding 10 credits, preserving proportional CNY basis, insufficient personal credits, replay, and organization deletion returning all available credits while rejecting open holds.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Organization.*Credit|Organization.*Fund' -count=1`

Expected: FAIL because organization funding performs ratio conversion SQL.

- [ ] **Step 3: Move authorization to service and mutation to credit transfer**

Retain owner/admin authorization and audit logging. Delete enabled-path SQL that calculates `amount*balance/display_balance`. The organization repository remains responsible for membership and audit records, not credit arithmetic.

- [ ] **Step 4: Run organization tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Organization' -count=1`

Expected: PASS, with an assertion that no executed SQL contains `balance/display_balance`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/organization.go backend/internal/repository/organization_repo.go backend/internal/repository/organization_repo_test.go backend/internal/repository/organization_repo_credit_integration_test.go
git commit -m "feat(organization): fund workspaces with credit transfers"
```

### Task 5: Wire funding flows and run regression gates

**Files:**
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire_gen.go` (generated)

**Interfaces:**
- All enabled credit-producing flows receive one shared `CreditAccountService`.

- [ ] **Step 1: Regenerate Wire**

Run: `cd backend && make generate`

Expected: production constructors/setters receive the credit service without duplicate providers.

- [ ] **Step 2: Run formatting and backend tests**

Run: `cd backend && gofmt -w internal/service/payment_service.go internal/service/payment_fulfillment.go internal/service/payment_refund.go internal/service/payment_amounts.go internal/service/redeem_service.go internal/service/admin_user.go internal/service/affiliate_service.go internal/service/auth_service.go internal/service/auth_oauth_first_bind.go internal/service/organization.go internal/repository/affiliate_repo.go internal/repository/organization_repo.go`

Run: `cd backend && go test ./...`

Expected: PASS.

- [ ] **Step 3: Scan enabled writers**

Run: `rg -n 'UpdateBalance|ApplyRedeemBalanceAdjustment|display_balance|balance/display_balance' backend/internal/service backend/internal/repository`

Expected: remaining matches are documented disabled compatibility paths, balance reads, or tests; every enabled funding writer calls the credit service.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "chore(billing): wire unified credit funding flows"
```
