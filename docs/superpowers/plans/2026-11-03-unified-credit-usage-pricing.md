# Unified Credit Usage And Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce explicit customer credit pricing, independent provider CNY cost, credit-based quotas, and atomic synchronous usage settlement.

**Architecture:** A new decimal customer-pricing resolver produces `ChargedCredits`; a separate provider-cost resolver produces `ProviderCostCNY`. The existing unified usage-billing transaction consumes the credit result and updates the account ledger, subscription/API-key/platform/organization meters, and dedup record atomically. Legacy USD/cost fields remain read-only compatibility data.

**Tech Stack:** Go, PostgreSQL, Ent, `database/sql`, `shopspring/decimal`, Wire, Redis cache compatibility, Vue API contracts tested later.

**Spec:** `docs/superpowers/specs/2026-11-03-unified-credit-billing-design.md`

## Global Constraints

- Complete the ledger foundation and funding plans first.
- Customer prices and quotas use credits; upstream account controls and provider cost use CNY.
- Customer charge and provider cost are calculated independently and stored in separate fields.
- No enabled path may read `ActualCost` as a customer credit amount.
- Synchronous settlement remains one database transaction with existing request deduplication.
- Existing uncommitted PP-video files are out of scope for this plan.

---

### Task 1: Add explicit credit pricing, quota, usage, and CNY cost columns

**Files:**
- Create: `backend/migrations/210_credit_usage_pricing.sql`
- Create: `backend/migrations/credit_usage_pricing_migration_test.go`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/ent/schema/api_key.go`
- Modify: `backend/ent/schema/user_subscription.go`
- Modify: `backend/ent/schema/user_platform_quota.go`
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/ent/schema/account.go`
- Modify: generated files under `backend/ent/`

**Interfaces:**
- Adds `_credits` customer price/limit/usage columns to channel pricing, pricing intervals, groups, API keys, subscriptions, user-platform quotas, organization members, and usage logs.
- Adds `provider_cost_cny`, `recognized_revenue_cny`, `estimated_gross_profit_cny`, and JSON pricing/cost snapshots to usage logs.
- Adds explicit `_cny` upstream-account quota/usage columns.

- [ ] **Step 1: Write a migration contract test**

Assert representative columns exist for every unit family and comments contain either `credits` or `CNY`. Assert the migration never copies a legacy USD value without multiplying by the explicit cutover parameter in the later cutover job.

- [ ] **Step 2: Run the migration test and verify it fails**

Run: `cd backend && go test ./migrations -run TestCreditUsagePricingMigrationContract -count=1`

Expected: FAIL because migration 210 does not exist.

- [ ] **Step 3: Add forward-only schema fields**

Add nullable new columns so deployment does not change behavior. Channel/group customer columns include all current token, cache, image, request, web-search, and video tiers. Provider CNY columns live on channel model pricing and intervals, not customer group multipliers.

- [ ] **Step 4: Update Ent schemas and generate**

Use Ent float fields only as persistence compatibility at existing API edges; new billing arithmetic converts database numeric text to decimal before calculation. Generated getters are not allowed inside pricing arithmetic.

Run: `cd backend && make generate`

- [ ] **Step 5: Run schema tests**

Run: `cd backend && go test ./migrations ./ent/schema ./internal/repository -run 'CreditUsagePricing|MigrationsSchema' -count=1`

Expected: PASS or integration skip when PostgreSQL is unavailable.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/210_credit_usage_pricing.sql backend/migrations/credit_usage_pricing_migration_test.go backend/ent
git commit -m "feat(billing): add explicit credit and CNY accounting fields"
```

### Task 2: Implement decimal customer credit pricing

**Files:**
- Create: `backend/internal/service/credit_pricing.go`
- Create: `backend/internal/service/credit_pricing_test.go`
- Modify: `backend/internal/service/channel.go`
- Modify: `backend/internal/service/group.go`
- Modify: `backend/internal/repository/channel_repo_pricing.go`
- Modify: `backend/internal/repository/channel_repo.go`
- Modify: `backend/internal/repository/group_repo.go`
- Modify: `backend/internal/service/model_pricing_resolver.go`

**Interfaces:**
- Produces `CustomerCreditPricingResolver`.
- Produces decimal `CustomerPriceSnapshot` and `CustomerCharge`.

```go
type CustomerCharge struct {
    BaseCredits    decimal.Decimal
    ChargedCredits decimal.Decimal
    BillingMode    BillingMode
    Formula        string
    Units          decimal.Decimal
    UnitPrice      decimal.Decimal
    Snapshot       CustomerPriceSnapshot
}

type CustomerCreditPricingResolver interface {
    QuoteTokens(context.Context, CreditTokenUsage) (CustomerCharge, error)
    QuoteRequest(context.Context, CreditRequestUsage) (CustomerCharge, error)
    QuoteImage(context.Context, CreditImageUsage) (CustomerCharge, error)
    QuoteVideo(context.Context, CreditVideoUsage) (CustomerCharge, error)
}
```

- [ ] **Step 1: Write table-driven pricing tests**

Cover input/output/cache/image tokens, context intervals, per-request, image tiers, web search, video `480p/720p/1080p/2k/4k`, group multiplier, image/video independent multiplier, and half-up eight-decimal rounding.

```go
func TestCustomerCreditPricingVideo15Seconds(t *testing.T) {
    got := quoteVideo(t, "1.6", 15, 1, "1")
    require.Equal(t, "24.00000000", got.ChargedCredits.StringFixed(8))
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service -run '^TestCustomerCreditPricing' -count=1`

Expected: FAIL because the resolver does not exist.

- [ ] **Step 3: Add decimal credit fields to service models and repositories**

Use `*decimal.Decimal` in new pricing structures. Repository scanners use `decimal.NullDecimal`; do not route values through legacy `*float64` fields. Keep legacy fields populated only for disabled compatibility paths.

- [ ] **Step 4: Implement the resolver**

Resolve one customer price catalog by group/channel/model, apply only customer multipliers, and round once. Missing credit pricing returns `ErrCreditPricingUnavailable`; it never falls back to hard-coded USD prices.

- [ ] **Step 5: Run focused pricing tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'CustomerCreditPricing|Channel.*CreditPricing|Group.*CreditPrice' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/credit_pricing.go backend/internal/service/credit_pricing_test.go backend/internal/service/channel.go backend/internal/service/group.go backend/internal/repository/channel_repo_pricing.go backend/internal/repository/channel_repo.go backend/internal/repository/group_repo.go backend/internal/service/model_pricing_resolver.go
git commit -m "feat(billing): calculate customer usage in credits"
```

### Task 3: Implement independent provider CNY cost calculation

**Files:**
- Create: `backend/internal/service/provider_cost.go`
- Create: `backend/internal/service/provider_cost_test.go`
- Modify: `backend/internal/service/account_stats_pricing.go`
- Modify: `backend/internal/repository/channel_repo_account_stats_pricing.go`

**Interfaces:**
- Produces `ProviderCostResolver.Calculate(context.Context, ProviderUsage) (ProviderCost, error)`.

```go
type ProviderCost struct {
    AmountCNY       decimal.Decimal
    OriginalAmount decimal.Decimal
    OriginalCurrency string
    ExchangeRateToCNY decimal.Decimal
    Snapshot        ProviderCostSnapshot
}
```

- [ ] **Step 1: Write provider-cost independence tests**

Use one request with `ChargedCredits=24` and `ProviderCostCNY=9.6`. Change customer multiplier and assert provider cost remains 9.6. Change provider cost multiplier and assert customer credits remain 24.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service -run '^TestProviderCost' -count=1`

Expected: FAIL because the resolver does not exist.

- [ ] **Step 3: Implement configured CNY and FX paths**

Prefer explicit CNY channel cost. When original cost is another currency, require amount, currency, rate, source, and timestamp; reject an incomplete snapshot. Return a nullable/missing-cost result for analytics repair without mutating customer settlement.

- [ ] **Step 4: Keep account statistics compatibility separate**

Existing account statistics fields remain legacy. Add explicit provider-cost snapshots instead of reusing customer pricing structures.

- [ ] **Step 5: Run focused tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'ProviderCost|AccountStatsPricing' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/provider_cost.go backend/internal/service/provider_cost_test.go backend/internal/service/account_stats_pricing.go backend/internal/repository/channel_repo_account_stats_pricing.go
git commit -m "feat(billing): record provider cost independently in CNY"
```

### Task 4: Convert synchronous unified billing to atomic credit settlement

**Files:**
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/openai_gateway_usage.go`
- Modify: `backend/internal/repository/credit_account_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo_unit_test.go`
- Modify: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Modify: `backend/internal/service/gateway_record_usage_test.go`
- Modify: `backend/internal/service/openai_gateway_record_usage_test.go`

**Interfaces:**
- Replaces new-path command fields with `ChargedCredits`, `SubscriptionCredits`, `APIKeyQuotaCredits`, `APIKeyRateLimitCredits`, and `AccountCostCNY`, all `decimal.Decimal`.
- Produces `UsageBillingApplyResult.NewAvailableCredits *decimal.Decimal`.

- [ ] **Step 1: Refactor credit repository transaction helpers**

Extract unexported `creditMutationQueries` methods accepting `*sql.Tx`. Standalone credit mutations and `usageBillingRepository` both use these helpers; no SQL is duplicated and no nested transaction is opened.

- [ ] **Step 2: Write atomic settlement tests**

Cover personal credit, organization credit, subscription allowance, API key total/rolling quota, platform quota, organization member quota, request replay, fingerprint conflict, insufficient credit rollback, and account CNY quota.

```go
func TestUsageBillingCreditFailureRollsBackAllMeters(t *testing.T) {
    // Seed 1 credit, request 2 credits, and assert account, ledger,
    // API-key usage, platform usage, and dedup row are all unchanged.
}
```

- [ ] **Step 3: Run focused tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'UsageBilling.*Credit|Gateway.*CreditBilling' -count=1`

Expected: FAIL because the command still uses float balance costs.

- [ ] **Step 4: Build the command from `CustomerCharge`**

When the global flag is enabled, construct all customer meters from `ChargedCredits`. Construct upstream account control from `ProviderCostCNY`. Keep the disabled legacy command unchanged until removal.

- [ ] **Step 5: Apply all effects in the existing transaction**

Claim request dedup, deduct credit or subscription allowance, update customer quotas, update organization member usage, update upstream CNY quota, append ledger, and commit. Any failure rolls back every effect.

- [ ] **Step 6: Update cache and notification payloads**

Cache canonical available credits only. Low-balance notifications label values as credits. Redis is never authoritative and receives updates after commit.

- [ ] **Step 7: Run settlement tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'UsageBilling|RecordUsage|CreditBilling' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/usage_billing.go backend/internal/service/gateway_usage_billing.go backend/internal/service/openai_gateway_usage.go backend/internal/repository/credit_account_repo.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_unit_test.go backend/internal/repository/usage_billing_repo_integration_test.go backend/internal/service/gateway_record_usage_test.go backend/internal/service/openai_gateway_record_usage_test.go
git commit -m "feat(billing): settle synchronous usage in credits"
```

### Task 5: Store explicit usage economics and simplify customer quotes

**Files:**
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Modify: `backend/internal/repository/usage_log_repo_integration_test.go`
- Modify: `backend/internal/service/media_quote.go`
- Modify: `backend/internal/service/media_quote_test.go`
- Modify: `backend/internal/pkg/usagestats/usage_log_types.go`

**Interfaces:**
- Usage logs expose `charged_credits`, `provider_cost_cny`, `recognized_revenue_cny`, and `estimated_gross_profit_cny`.
- Customer quote removes `SettlementAmount` and `CreditsPerSettlementUnit`; it returns decimal-string `estimated_credits`.

- [ ] **Step 1: Write usage log and quote contract tests**

Assert new logs persist both 24 credits and 9.6 CNY without populating one from the other. Assert customer quote JSON has `estimated_credits` and no `settlement_amount` or conversion ratio.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'MediaQuote.*Credits|UsageLog.*Economics' -count=1`

Expected: FAIL because contracts still expose dual units.

- [ ] **Step 3: Implement explicit persistence and DTOs**

Write new fields for enabled requests. Preserve legacy `total_cost`/`actual_cost` only for disabled requests and historical reads. Mark legacy rows by missing `charged_credits`.

- [ ] **Step 4: Replace media quote conversion**

Quote directly from the customer credit resolver. Do not load account balances to derive a conversion ratio.

- [ ] **Step 5: Run tests**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/pkg/usagestats -run 'MediaQuote|UsageLog|Credit' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/usage_log.go backend/internal/repository/usage_log_repo.go backend/internal/repository/usage_log_repo_integration_test.go backend/internal/service/media_quote.go backend/internal/service/media_quote_test.go backend/internal/pkg/usagestats/usage_log_types.go
git commit -m "feat(billing): persist credit charges and CNY provider cost"
```

### Task 6: Verify synchronous billing end to end

**Files:**
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire_gen.go` (generated)
- Create: `backend/internal/integration/credit_billing_e2e_test.go`

**Interfaces:**
- Production gateway receives both customer-price and provider-cost resolvers.

- [ ] **Step 1: Wire and generate**

Run: `cd backend && make generate`

Expected: all new resolvers compile and are injected once.

- [ ] **Step 2: Add end-to-end synchronous test**

Seed 2088 credits, configure a deterministic token credit price and CNY provider cost, submit one usage settlement, and assert account, ledger, all customer quotas, usage log, and provider cost.

- [ ] **Step 3: Run backend tests**

Run: `cd backend && go test ./...`

Expected: PASS.

- [ ] **Step 4: Scan for unit leaks**

Run: `rg -n 'ActualCost.*(Balance|Quota)|BalanceCost|SubscriptionCost|credits_per_settlement_unit|display_balance.*/.*balance' backend/internal`

Expected: matches exist only in explicitly marked disabled legacy compatibility code and legacy DTO tests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/wire.go backend/cmd/server/wire_gen.go backend/internal/integration/credit_billing_e2e_test.go
git commit -m "test(billing): verify synchronous unified credit settlement"
```
