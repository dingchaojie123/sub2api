# Unified Credit Billing Design

## Status

Approved direction as of 2026-11-03. This document defines the target
architecture and migration contract. It does not authorize an all-at-once
cutover; implementation must follow the staged rollout and reconciliation
gates in this document.

## Goal

Replace the current dual-balance billing model with one customer-facing credit
system:

- customers recharge to receive credits;
- every customer price, allowance, hold, and charge is expressed in credits;
- the admin UI, customer UI, APIs, and billing enforcement read the same credit
  account;
- upstream supplier cost is recorded separately in CNY for cost and profit
  analysis;
- no runtime conversion between an internal balance and a display balance is
  permitted.

## Confirmed Business Decisions

1. The canonical migrated customer balance is the current customer-visible
   `display_balance`, not the current settlement `balance`.
2. The canonical migrated frozen balance is `frozen_display_balance`.
3. A user currently showing `balance = 22.2` and `display_balance = 32.8`
   migrates to exactly `32.8` available credits.
4. Channel customer prices are credit prices. A configured video price of
   `1.6` per second charges exactly `1.6` credits per generated second before
   applicable customer multipliers.
5. Supplier cost remains internal and is normalized to CNY. It never directly
   determines or mutates a customer's credit charge.
6. Historical records keep their original accounting meaning. The migration
   must not fabricate exact historical credit charges when the old system did
   not record enough information to reconstruct them.
7. All customer-credit mutation paths switch authority together. A personal or
   organization account cannot accept some mutations from legacy balance code
   and other mutations from the credit ledger.

## Current-State Problems

The current implementation has two customer balance fields with different
semantics:

- `balance` is treated as the real settlement balance;
- `display_balance` is treated as the customer-visible nominal balance;
- usage deducts `balance`, then proportionally changes `display_balance` using
  the account's current `display_balance / balance` ratio;
- async media repeats that conversion for holds, captures, and releases;
- organizations duplicate the same four-field model;
- subscriptions, API key quotas, and rate limits are still described as USD;
- media quotes return both settlement amount and converted credits;
- generic names such as `cost`, `actual_cost`, and `amount` conceal the unit.

The result is that two customers can consume different visible credits for the
same request because they previously purchased different recharge products.
The customer charge is therefore account-dependent instead of price-dependent.

## Scope

This design covers:

- personal and organization credit accounts;
- recharge, promotional grants, redeem codes, affiliate transfers, admin
  adjustments, refunds, and organization funding;
- synchronous text and image usage;
- asynchronous image and video holds, captures, and releases;
- subscription allowances, API key quotas and rate limits, user-platform
  quotas, and organization member limits;
- usage logs, customer quotes, admin reporting, provider cost, and profit
  reporting;
- customer and admin APIs and all balance-related UI surfaces;
- migration, reconciliation, cutover, rollback, and removal of legacy writes.

It does not redesign payment gateway integrations, subscription product
entitlements unrelated to usage allowance, concurrency limits, storage quotas,
or upstream provider routing.

## Unit Model

### Customer unit: credits

All customer monetary consumption uses credits. Canonical names must include
`credit` or `credits`; ambiguous balance and cost names are not allowed in new
interfaces.

Credit quantities use decimal fixed-point precision `NUMERIC(20,8)`. Billing
domain code uses `shopspring/decimal`; it must not use `float64` for arithmetic,
comparisons, sufficiency checks, holds, captures, or ledger balances.

The system rounds once at the final charge boundary to eight decimal places
using decimal half-up rounding. Intermediate calculations retain decimal
precision. UI presentation may show fewer decimals but must not alter the
stored amount.

### Internal unit: CNY

Provider cost is stored as `provider_cost_cny` with `NUMERIC(20,8)`. If an
upstream reports another currency, the record also stores the original amount,
original currency, exchange rate to CNY, rate source, and rate timestamp.

Provider cost is an analytic fact. Customer charge is a commercial pricing
decision. No formula may derive one from the other inside balance settlement.

### Revenue and profit

Recharge fulfillment records both cash received in CNY and credits granted.
The account carries a weighted-average revenue basis:

- paid recharge adds `cash_amount_cny` to revenue basis and adds the granted
  credits;
- free grants add credits with zero revenue basis;
- a hold moves the proportional revenue basis from available to held;
- release moves that basis back;
- capture recognizes the held basis proportional to credits captured;
- direct consumption recognizes the available basis proportional to credits
  consumed;
- personal-to-organization funding transfers the same proportional basis with
  the credits.

`estimated_gross_profit_cny` is recognized revenue basis minus provider cost.
It is an operational profitability metric, not a replacement for payment or
tax accounting.

The cash basis of credits already present at cutover cannot be reconstructed
exactly from the old dual-balance history. Cutover therefore requires an
explicit `legacy_credit_basis_cny_per_credit` value with no application
default. The migration records that value on every migration ledger entry and
marks resulting profit as estimated. Profit becomes fully based on observed
new-system cash receipts after migrated credits have been consumed.

## Data Model

### `credit_accounts`

One row represents one personal or organization credit account:

```text
id                            BIGSERIAL PRIMARY KEY
user_id                       BIGINT NULL REFERENCES users(id)
organization_id               BIGINT NULL REFERENCES organizations(id)
available_credits             NUMERIC(20,8) NOT NULL DEFAULT 0
held_credits                  NUMERIC(20,8) NOT NULL DEFAULT 0
available_revenue_basis_cny   NUMERIC(20,8) NOT NULL DEFAULT 0
held_revenue_basis_cny        NUMERIC(20,8) NOT NULL DEFAULT 0
version                       BIGINT NOT NULL DEFAULT 0
created_at                    TIMESTAMPTZ NOT NULL
updated_at                    TIMESTAMPTZ NOT NULL
```

A check constraint requires exactly one of `user_id` and `organization_id`.
Partial unique indexes enforce one account per user and one per organization.
Credits and revenue basis cannot be negative. Customer overdraft is not
supported in the new model.

### `credit_ledger`

Every account mutation appends an immutable ledger row in the same database
transaction as the account update:

```text
id                          BIGSERIAL PRIMARY KEY
account_id                  BIGINT NOT NULL REFERENCES credit_accounts(id)
event_type                  VARCHAR(32) NOT NULL
available_delta             NUMERIC(20,8) NOT NULL
held_delta                  NUMERIC(20,8) NOT NULL
available_after             NUMERIC(20,8) NOT NULL
held_after                  NUMERIC(20,8) NOT NULL
available_basis_delta_cny   NUMERIC(20,8) NOT NULL
held_basis_delta_cny        NUMERIC(20,8) NOT NULL
available_basis_after_cny   NUMERIC(20,8) NOT NULL
held_basis_after_cny        NUMERIC(20,8) NOT NULL
recognized_revenue_cny      NUMERIC(20,8) NOT NULL DEFAULT 0
idempotency_key             VARCHAR(160) NOT NULL UNIQUE
correlation_id              VARCHAR(160) NULL
reference_type              VARCHAR(40) NULL
reference_id                VARCHAR(160) NULL
request_id                  VARCHAR(160) NULL
api_key_id                  BIGINT NULL
metadata                    JSONB NOT NULL DEFAULT '{}'
created_at                  TIMESTAMPTZ NOT NULL
```

Allowed event types are `migration`, `recharge`, `grant`, `redeem`,
`affiliate_transfer`, `admin_adjustment`, `usage`, `hold`, `capture`,
`release`, `refund`, `transfer_out`, and `transfer_in`.

Transfer-out and transfer-in entries share a `correlation_id` and commit in one
transaction. Ledger rows are never updated or deleted by application code.
Corrections use compensating entries.

### Usage and async task records

New unambiguous fields are added while legacy fields remain readable:

- `usage_logs.charged_credits`;
- `usage_logs.provider_cost_cny`;
- `usage_logs.recognized_revenue_cny`;
- `usage_logs.estimated_gross_profit_cny`;
- `usage_logs.customer_pricing_snapshot`;
- `usage_logs.provider_cost_snapshot`;
- async jobs/tasks: `estimated_credits`, `held_credits`, `charged_credits`, and
  `provider_cost_cny`.

The old `total_cost` and `actual_cost` fields remain legacy reporting fields
until the retention window expires. New settlement code must not read them as
credits.

### Payment orders

Balance recharge orders gain explicit fields:

- `cash_amount_cny`;
- `granted_credits`;
- `payment_amount` and `payment_currency` when different from CNY;
- `exchange_rate_to_cny` and rate timestamp when conversion is required.

For the existing 1288 product, fulfillment writes
`cash_amount_cny = 1288` and `granted_credits = 2088`.

## Domain Interfaces

The credit subsystem owns all account mutations. Repositories outside this
subsystem cannot update credit account columns directly.

```go
type CreditAmount = decimal.Decimal

type CreditAccountService interface {
    Get(ctx context.Context, owner CreditOwner) (CreditAccount, error)
    Grant(ctx context.Context, cmd GrantCreditsCommand) (CreditMutation, error)
    Deduct(ctx context.Context, cmd DeductCreditsCommand) (CreditMutation, error)
    Hold(ctx context.Context, cmd HoldCreditsCommand) (CreditMutation, error)
    Capture(ctx context.Context, cmd CaptureCreditsCommand) (CreditMutation, error)
    Release(ctx context.Context, cmd ReleaseCreditsCommand) (CreditMutation, error)
    Transfer(ctx context.Context, cmd TransferCreditsCommand) (CreditTransfer, error)
}
```

Every mutation command includes an idempotency key, a business reference, and
structured metadata. `Hold`, `Capture`, and `Release` reference a stable hold
ID. Capture may be lower or higher than the initial hold; a higher capture
atomically consumes the additional available credits or fails without changing
the account.

## Customer Pricing

Customer price configuration is expressed only in credits:

- token models: credits per input, output, cache, and image token;
- per-request models: credits per request;
- generated images: credits per image and resolution tier;
- generated video: credits per second, resolution tier, and video count.

The customer charge formula is:

```text
base credits = configured customer credit price * measured billing units
charged credits = round(base credits * customer group multiplier, 8)
```

Account/provider cost multipliers are not customer multipliers. Existing
fields that are needed for both purposes must be split and renamed.

Video resolution pricing applies consistently to all video providers. `480p`,
`720p`, `1080p`, and supported higher-tier mappings are resolved by one shared
customer price resolver rather than provider-specific branches.

Fallback prices must also be explicit credit prices. Existing hard-coded USD
fallback prices cannot silently become credit prices. Cutover requires an
operator-reviewed credit price catalog for every enabled model.

## Provider Cost

Provider cost calculation runs independently after routing resolves the actual
channel/account. It uses channel-specific CNY cost configuration or upstream
reported usage converted to CNY.

The provider cost snapshot includes:

- provider, channel, and account IDs;
- cost formula and measured units;
- configured unit cost and cost multiplier;
- original currency fields when applicable;
- final `provider_cost_cny`.

Failure to calculate provider cost must not change a successful customer
charge. It records a visible internal reconciliation error and leaves provider
cost nullable for later repair.

## Settlement Flows

### Recharge and grants

Payment confirmation grants exactly the product's `granted_credits`. Gateway
callbacks use the payment order ID as part of the ledger idempotency key.
Redeem codes, affiliate transfers, signup grants, OAuth grants, and admin
adjustments all call the credit service and create typed ledger entries.

### Synchronous usage

The request produces one `charged_credits` value from the customer price
resolver. The existing unified billing transaction then atomically applies:

- subscription allowance usage or personal/organization credit deduction;
- API key quota and rolling-window usage in credits;
- user-platform quota usage in credits;
- organization member usage in credits;
- a credit ledger entry when a credit account pays;
- usage billing deduplication.

Provider cost is stored on the usage log as a separate result.

### Subscriptions and quotas

Subscription daily, weekly, and monthly limits and usage are renamed from
`*_usd` to `*_credits`. API key quota, rate limits, and rolling usage gain
explicit `*_credits` names. Organization member and user-platform quota fields
use the same convention.

A subscription consumes allowance credits rather than account credits. Its
customer charge is still recorded as `charged_credits` for usage reporting,
but no credit ledger deduction occurs. When allowance is exhausted, existing
billing-source fallback rules decide whether the request may use the personal
or organization credit account.

Upstream account quotas are internal cost controls and use CNY fields. They are
not migrated into customer credit quotas.

### Async image and video usage

Task creation calculates estimated credits and creates a credit hold. Terminal
success captures actual charged credits; failure or cancellation releases the
hold. Poll retries reuse the same hold/capture/release idempotency keys.

For a 15-second video at `1.6 credits/second` with count 1 and multiplier 1:

```text
hold = 24 credits
capture = 24 credits
2088 available credits becomes 2064 credits
```

No async path may use `balance`, `display_balance`, or their ratio.

### Refunds

A refund creates a compensating credit entry tied to the payment order. A
normal refund is rejected when the required credits are no longer available.
An explicitly authorized forced refund may reduce credits to zero but must not
create a negative account; any unrecovered value is recorded in refund audit
metadata for operator follow-up.

## Quote and API Contracts

Customer balance APIs return one object:

```json
{
  "available_credits": "2064.00000000",
  "held_credits": "0.00000000"
}
```

Decimal values are serialized as strings at the new API boundary to avoid
JavaScript floating-point corruption. Compatibility fields may be returned
during migration but are derived from the credit account and must always equal
the canonical credit value.

Customer media quotes return `estimated_credits`, `credit_unit`, pricing
snapshot, and expiration. They no longer return a settlement amount or
`credits_per_settlement_unit` conversion.

Admin cost/profit endpoints return CNY fields separately and are mounted only
under the existing administrator-authenticated route group. Customer APIs
never expose provider cost.

## UI Contract

The customer header, profile, usage pages, keys, subscriptions, organizations,
and media task pages label customer amounts as `积分`. They consume the same
canonical balance endpoint.

Admin user and organization views display the same available and held credits.
Channel pricing forms visibly separate:

- `客户积分售价`;
- `供应商成本（人民币）`.

Customer billing UI must not show USD, `$`, or ambiguous `费用` labels. Internal
cost and profit pages use `人民币` or `¥` and never label those values as
customer deductions.

## Migration and Rollout

### Phase 1: Introduce without behavior change

1. Add credit account, ledger, explicit usage fields, and explicit payment
   fields.
2. Add the credit domain service and decimal amount types.
3. Build a read-only migration preview that reports the future account values:
   `available_credits = display_balance` and
   `held_credits = frozen_display_balance`.
4. Rehearse the backfill and reconciliation on a restored production snapshot,
   not on live authoritative accounts.
5. Keep legacy billing fully authoritative. The global
   `credit_billing_enabled` flag remains false and production credit-account
   mutation endpoints reject writes.

### Phase 2: Deploy all new writers dark

Implement grants/recharges, synchronous usage, subscriptions and quotas,
organizations, and async media against the credit service while the global
flag remains false. Tests and non-production environments exercise each flow,
but production requests continue to use legacy settlement.

Customer pricing and provider-cost calculation may have separate read-only
preview flags, but there is only one authority flag for mutations. A request
is processed by exactly one settlement path.

### Phase 3: Pricing and quota cutover

1. Export every enabled customer pricing rule and fallback.
2. Load a separately reviewed credit price catalog. There is no per-user or
   runtime conversion factor.
3. Add credit-named subscription and quota columns.
4. Convert active limits and current-window usage with the mandatory cutover
   parameter `legacy_quota_credits_per_usd`. The application provides no
   default. The selected value and every before/after value are audited. A
   window may instead be reset only for a product whose published terms already
   permit that reset at the cutover time.
5. Keep historical USD fields read-only and mark them `legacy` in reporting.

### Phase 4: Maintenance cutover

1. Stop recharge fulfillment, grants, adjustments, organization transfers,
   usage settlement, and new async image/video task creation.
2. Wait for existing async tasks and holds to reach terminal settlement.
3. Require legacy frozen balances to be zero, or resolve exceptions manually.
4. Lock the affected user and organization rows while the migration transaction
   creates one account and one `migration` ledger entry per owner. The entry
   contains all four legacy balance snapshots and the mandatory
   `legacy_credit_basis_cny_per_credit` value.
5. Convert subscription and quota fields using the approved cutover catalog and
   `legacy_quota_credits_per_usd` value.
6. Set legacy `balance` and `display_balance` to the canonical available
   credits, and both frozen fields to canonical held credits. These fields are
   compatibility mirrors only after this point.
7. Enable `credit_billing_enabled`, run blocking reconciliation, and resume
   traffic only if every account and quota check passes.
8. Re-enable async task creation using credit holds.

After cutover, every successful credit mutation mirrors its final canonical
available and held values to the four legacy columns for one release. The
mirror is a direct assignment, never a ratio calculation. No legacy repository
may write those columns independently.

Rollback during the observation release is a maintenance operation: stop all
credit mutations, verify that compatibility mirrors match credit accounts,
disable the global flag, and deploy the compatibility release that treats the
equal legacy fields as one-to-one credits. Database ledger data is retained.

### Phase 5: Legacy removal

After the observation window has no reconciliation differences:

- stop compatibility writes;
- remove runtime ratio conversion;
- remove old customer balance cache interfaces;
- remove mixed-unit quote fields;
- remove legacy UI and DTO fields;
- retain legacy database columns read-only for one release;
- drop them only in a later reversible database release.

## Reconciliation and Observability

The rollout includes automated checks for:

- credit account snapshot equals the latest ledger `*_after` values;
- ledger deltas sum to the account snapshot;
- held credits equal open hold obligations;
- every migrated payment completion has exactly one recharge ledger entry;
- every charged usage has one billing dedup record and, when account-paid, one
  usage ledger entry;
- no post-cutover SQL writes target legacy balance fields;
- customer/admin API values match for sampled users and organizations;
- provider cost coverage and missing-cost count;
- negative balance attempts, insufficient-credit rejections, duplicate
  idempotency hits, and reconciliation failures.

Any account-level reconciliation mismatch blocks further automated mutation of
that account and raises an operator alert; reads remain available.

## Error Handling

- Insufficient credits fail before an upstream request when cost is known.
- Failed atomic account mutations leave neither a balance change nor a ledger
  row.
- Duplicate idempotency keys return the original mutation result.
- A payload mismatch on a reused idempotency key is a conflict error.
- Cache invalidation failure does not roll back a committed transaction; the
  database remains authoritative and the failure is retried.
- Provider cost calculation failure is isolated from customer settlement and
  queued for internal repair.
- Reconciliation failures are never silently corrected; corrections require a
  compensating ledger entry with an operator identity and reason.

## Testing Strategy

### Unit tests

- decimal charge calculation and final rounding;
- token, request, image, and video pricing;
- all resolution tiers and customer multipliers;
- weighted-average revenue basis movement;
- hold, lower capture, higher capture, release, and idempotent replay;
- subscription and quota consumption in credits;
- provider CNY cost calculation independent of customer charge.

### Repository and integration tests

- concurrent deductions cannot overspend an account;
- account mutation and ledger insertion are atomic;
- duplicate callbacks and pollers cannot double-charge;
- personal-to-organization transfer preserves credits and revenue basis;
- backfill uses display balances exactly and is idempotent;
- reconciliation detects modified or missing ledger data;
- payment refund and forced-refund policies;
- usage billing deduplication across personal, organization, and subscription
  billing sources.

### End-to-end tests

- paying CNY 1288 grants 2088 credits in both admin and customer UIs;
- a 15-second video at 1.6 credits/second holds and captures 24 credits;
- success, failure, cancellation, timeout, and poll retry behavior;
- API key and subscription meters show the same charged credits as usage logs;
- organization funding and member usage remain consistent;
- provider cost appears only in authorized internal reporting.

## Acceptance Criteria

1. Admin and customer balance views read the same credit account and always
   show the same available and held amounts.
2. Recharge grants exactly the configured credits, including 1288 CNY to 2088
   credits.
3. Configured customer prices deduct the exact calculated credits independent
   of a customer's recharge history.
4. No active billing path reads or calculates `display_balance / balance`.
5. All customer allowances, quotas, holds, and usage amounts are credits.
6. Provider cost is independently recorded in CNY and cannot change a customer
   balance.
7. Every credit mutation is atomic, idempotent, and represented by an immutable
   ledger entry.
8. Account snapshots reconcile exactly to their ledgers and open holds.
9. Historical mixed-unit data remains identifiable and is not presented as
   exact new-system credits.
10. The global cutover can be rolled back during maintenance without deleting
    ledger data or charging a request through both settlement paths.

## Implementation Decomposition

Because this is a platform-wide migration, implementation should be split into
independently reviewable plans that preserve a working system after each plan:

1. credit account, ledger, decimal domain, backfill, and reconciliation;
2. recharge, grants, adjustments, refunds, and organization transfers;
3. customer pricing, synchronous usage, subscriptions, quotas, logs, and
   provider CNY cost;
4. async image/video holds and settlement;
5. API/UI cutover, rollout controls, observability, and legacy removal.

Each plan must use test-driven changes and must not depend on dropping legacy
columns to become safe.
