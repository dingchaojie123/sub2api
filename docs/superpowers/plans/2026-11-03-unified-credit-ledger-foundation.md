# Unified Credit Ledger Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the decimal credit account, immutable ledger, explicit hold lifecycle, migration preview, and reconciliation foundation without changing production billing behavior.

**Architecture:** PostgreSQL raw-SQL repositories own atomic account, hold, and ledger transactions because organization rows are not Ent entities. A service layer validates decimal commands and exposes one interface to later payment and usage plans. The production mutation path remains disabled by one global configuration flag.

**Tech Stack:** Go, PostgreSQL, `database/sql`, `shopspring/decimal`, Wire, Testify, SQLMock, repository integration tests.

**Spec:** `docs/superpowers/specs/2026-11-03-unified-credit-billing-design.md`

## Global Constraints

- Customer amounts are credits stored as `NUMERIC(20,8)` and calculated with `decimal.Decimal`.
- Provider cost is not part of this plan and cannot be written into credit balances.
- `display_balance` is the migration source; `balance` is audit metadata only.
- Production credit mutations reject writes while `billing.credit_billing_enabled=false`.
- Every mutation is atomic, idempotent, and represented by an immutable ledger row.
- Existing uncommitted PP-video files must not be modified by this plan.

---

### Task 1: Add credit account, ledger, and hold schema

**Files:**
- Create: `backend/migrations/208_unified_credit_ledger.sql`
- Create: `backend/migrations/unified_credit_ledger_migration_test.go`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`

**Interfaces:**
- Produces tables `credit_accounts`, `credit_ledger`, and `credit_holds` with the exact fields and constraints in the design spec.
- Produces partial unique indexes `uq_credit_accounts_user` and `uq_credit_accounts_organization`.
- Produces unique idempotency indexes on `credit_ledger.idempotency_key` and `credit_holds.hold_key`.

- [ ] **Step 1: Write the migration contract test**

```go
func TestUnifiedCreditLedgerMigrationContract(t *testing.T) {
    sql := readMigration(t, "208_unified_credit_ledger.sql")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS credit_accounts")
    require.Contains(t, sql, "CHECK ((user_id IS NULL) <> (organization_id IS NULL))")
    require.Contains(t, sql, "available_credits NUMERIC(20,8) NOT NULL DEFAULT 0")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS credit_ledger")
    require.Contains(t, sql, "idempotency_key VARCHAR(160) NOT NULL UNIQUE")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS credit_holds")
    require.Contains(t, sql, "status VARCHAR(16) NOT NULL")
}
```

- [ ] **Step 2: Run the migration test and verify it fails**

Run: `cd backend && go test ./migrations -run TestUnifiedCreditLedgerMigrationContract -count=1`

Expected: FAIL because `208_unified_credit_ledger.sql` does not exist.

- [ ] **Step 3: Add the forward-only migration**

Create all three tables, foreign keys, non-negative checks, owner exclusivity check, status check (`open`, `captured`, `released`), partial owner indexes, request/reference indexes, and comments describing credits versus CNY revenue basis. Do not add backfill DML to the schema migration.

- [ ] **Step 4: Extend schema integration assertions**

Add table/column/index assertions for the three new tables. Verify `available_credits`, `held_credits`, and all basis columns are PostgreSQL numeric values with scale 8.

- [ ] **Step 5: Run migration tests**

Run: `cd backend && go test ./migrations ./internal/repository -run 'TestUnifiedCreditLedgerMigrationContract|TestMigrationsSchema' -count=1`

Expected: PASS, or the repository integration package skips cleanly when `TEST_DATABASE_DSN` is not configured.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/208_unified_credit_ledger.sql backend/migrations/unified_credit_ledger_migration_test.go backend/internal/repository/migrations_schema_integration_test.go
git commit -m "feat(billing): add unified credit ledger schema"
```

### Task 2: Define decimal credit domain contracts and configuration gate

**Files:**
- Create: `backend/internal/service/credit_account.go`
- Create: `backend/internal/service/credit_account_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces `CreditOwner`, `CreditAccount`, `CreditMutation`, `CreditHold`, and command types.
- Produces `CreditAccountRepository` and concrete `CreditAccountService`.
- Produces `BillingConfig.CreditBillingEnabled bool`, `LegacyCreditBasisCNYPerCredit string`, and `LegacyQuotaCreditsPerUSD string`.

The public service contract is:

```go
type CreditAccountRepository interface {
    Get(context.Context, CreditOwner) (*CreditAccount, error)
    Grant(context.Context, GrantCreditsCommand) (*CreditMutation, error)
    Deduct(context.Context, DeductCreditsCommand) (*CreditMutation, error)
    Hold(context.Context, HoldCreditsCommand) (*CreditMutation, error)
    Capture(context.Context, CaptureCreditsCommand) (*CreditMutation, error)
    Release(context.Context, ReleaseCreditsCommand) (*CreditMutation, error)
    Transfer(context.Context, TransferCreditsCommand) (*CreditTransfer, error)
}
```

- [ ] **Step 1: Write validation and gate tests**

Cover invalid owners, zero/negative amounts, missing idempotency keys, missing hold keys, payload mismatch, and mutation rejection while the global flag is false.

```go
func TestCreditAccountServiceGrantDisabled(t *testing.T) {
    svc := NewCreditAccountService(&creditRepoStub{}, &config.Config{})
    _, err := svc.Grant(context.Background(), GrantCreditsCommand{
        Owner: CreditOwner{UserID: ptrInt64(7)},
        Amount: decimal.RequireFromString("18"),
        IdempotencyKey: "recharge:order:1",
    })
    require.ErrorIs(t, err, ErrCreditBillingDisabled)
}
```

- [ ] **Step 2: Run the service test and verify it fails**

Run: `cd backend && go test ./internal/service -run '^TestCreditAccountService' -count=1`

Expected: FAIL because the credit types and service do not exist.

- [ ] **Step 3: Add exact decimal domain types**

Use `decimal.Decimal` fields for credits and CNY basis. Validate scale with a shared `NormalizeCreditAmount` function that rounds half-up to eight decimal places. Define typed errors for disabled billing, invalid owner, invalid amount, insufficient credits, idempotency conflict, hold not found, and hold already finalized.

- [ ] **Step 4: Add configuration fields and defaults**

Set only `billing.credit_billing_enabled=false` by default. The two migration parameter strings default to empty and validation rejects cutover when either is missing or non-positive. Do not parse them into `float64`.

- [ ] **Step 5: Run focused tests**

Run: `cd backend && go test ./internal/config ./internal/service -run 'Credit|BillingConfig' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/credit_account.go backend/internal/service/credit_account_test.go backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat(billing): define decimal credit account contracts"
```

### Task 3: Implement atomic credit account repository

**Files:**
- Create: `backend/internal/repository/credit_account_repo.go`
- Create: `backend/internal/repository/credit_account_repo_test.go`
- Create: `backend/internal/repository/credit_account_repo_integration_test.go`
- Modify: `backend/internal/repository/wire.go`

**Interfaces:**
- Consumes all types from `service/credit_account.go`.
- Produces `NewCreditAccountRepository(*sql.DB) service.CreditAccountRepository`.
- Stores decimals as strings/`NUMERIC`; no SQL argument is derived through `float64`.

- [ ] **Step 1: Write SQLMock tests for grant and deduct**

Assert `SELECT ... FOR UPDATE`, idempotency lookup, account update, ledger insert, and commit occur in one transaction. Assert insufficient-credit deduction rolls back without a ledger insert.

- [ ] **Step 2: Write integration tests for idempotency and concurrency**

```go
func TestCreditAccountRepositoryConcurrentDeductCannotOverspend(t *testing.T) {
    // Seed 10 credits, launch two 7-credit deductions with distinct keys,
    // require exactly one success and a final balance of 3.
}

func TestCreditAccountRepositoryIdempotentReplay(t *testing.T) {
    // Replay the same key and payload; require one ledger row and identical result.
    // Reuse the key with a different amount; require ErrCreditIdempotencyConflict.
}
```

- [ ] **Step 3: Run the tests and verify they fail**

Run: `cd backend && go test ./internal/repository -run '^TestCreditAccountRepository' -count=1`

Expected: FAIL because the repository does not exist.

- [ ] **Step 4: Implement grant and deduct**

Lock the account row, claim/replay the idempotency key, calculate proportional revenue basis using decimal arithmetic, update the snapshot with `version=version+1`, insert after-values into `credit_ledger`, and commit.

- [ ] **Step 5: Implement hold, capture, and release**

Hold inserts an open `credit_holds` row and moves available amount/basis to held. Capture locks account and hold; lower capture returns the remainder, higher capture consumes extra available credits, and terminal replay returns the original ledger result. Release restores the remaining amount/basis and finalizes the hold.

- [ ] **Step 6: Implement atomic transfer**

Lock source and destination accounts in ascending account-ID order, move credits and proportional basis, and insert paired `transfer_out`/`transfer_in` rows sharing one correlation ID.

- [ ] **Step 7: Mirror canonical snapshots to legacy columns**

At the end of each successful mutation transaction, directly assign personal or organization `balance` and `display_balance` to `available_credits`, and both frozen fields to `held_credits`. Add an integration assertion that the four columns are equal after grant, hold, capture, release, deduction, and transfer. Never calculate a ratio.

- [ ] **Step 8: Run repository tests**

Run: `cd backend && go test ./internal/repository -run '^TestCreditAccountRepository' -count=1`

Expected: PASS, including integration tests when PostgreSQL is available.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/repository/credit_account_repo.go backend/internal/repository/credit_account_repo_test.go backend/internal/repository/credit_account_repo_integration_test.go backend/internal/repository/wire.go
git commit -m "feat(billing): implement atomic credit account repository"
```

### Task 4: Add migration preview, controlled backfill, and reconciliation

**Files:**
- Create: `backend/internal/service/credit_migration.go`
- Create: `backend/internal/service/credit_migration_test.go`
- Create: `backend/internal/repository/credit_migration_repo.go`
- Create: `backend/internal/repository/credit_migration_repo_integration_test.go`

**Interfaces:**
- Produces `CreditMigrationRepository.Preview(context.Context) (*CreditMigrationPreview, error)`.
- Produces `CreditMigrationRepository.Backfill(context.Context, CreditMigrationParameters) (*CreditMigrationResult, error)`.
- Produces `CreditMigrationRepository.Reconcile(context.Context, int) ([]CreditReconciliationIssue, error)`.

```go
type CreditMigrationParameters struct {
    LegacyCreditBasisCNYPerCredit decimal.Decimal
    LegacyQuotaCreditsPerUSD      decimal.Decimal
    CutoverID                     string
}
```

- [ ] **Step 1: Write preview/backfill tests**

Seed a user with `balance=22.2`, `display_balance=32.8`, and zero frozen display balance. Require preview and backfill to produce exactly 32.8 available credits and a migration ledger metadata snapshot containing 22.2 and 32.8.

- [ ] **Step 2: Write reconciliation failure tests**

Cover account-versus-latest-ledger mismatch, summed-delta mismatch, held-versus-open-hold mismatch, and missing migration ledger entry.

- [ ] **Step 3: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'CreditMigration|CreditReconciliation' -count=1`

Expected: FAIL because migration/reconciliation components do not exist.

- [ ] **Step 4: Implement read-only preview**

Preview must not insert credit accounts. It returns account counts, total available/held credits, invalid negative legacy values, and owners that violate migration prerequisites.

- [ ] **Step 5: Implement idempotent backfill**

Require non-empty cutover ID and positive explicit parameters. Insert account and `migration` ledger rows with idempotency key `migration:<cutover-id>:<owner-type>:<owner-id>`. Rerunning the same cutover returns the original result; a different payload conflicts.

- [ ] **Step 6: Implement reconciliation query**

Return structured issues without modifying data. Limit results per call and include owner, account ID, issue code, expected value, and actual value.

- [ ] **Step 7: Run focused tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'CreditMigration|CreditReconciliation' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/credit_migration.go backend/internal/service/credit_migration_test.go backend/internal/repository/credit_migration_repo.go backend/internal/repository/credit_migration_repo_integration_test.go
git commit -m "feat(billing): add credit migration and reconciliation"
```

### Task 5: Wire the foundation and run the complete backend gate

**Files:**
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire_gen.go` (generated)
- Modify: `backend/internal/repository/wire.go` if generation reveals a missing provider

**Interfaces:**
- Consumes `NewCreditAccountRepository` and `NewCreditAccountService`.
- Produces singleton credit services for later payment and usage plans.

- [ ] **Step 1: Add Wire providers**

Add repository and service constructors to their provider sets. Do not inject them into existing billing writers yet.

- [ ] **Step 2: Regenerate dependency injection**

Run: `cd backend && make generate`

Expected: `wire_gen.go` contains the new repository/service construction and compilation succeeds.

- [ ] **Step 3: Run formatting and focused tests**

Run: `cd backend && gofmt -w internal/service/credit_*.go internal/repository/credit_*.go migrations/unified_credit_ledger_migration_test.go`

Run: `cd backend && go test ./internal/config ./internal/service ./internal/repository ./migrations -run 'Credit|UnifiedCredit' -count=1`

Expected: PASS.

- [ ] **Step 4: Run the backend regression suite**

Run: `cd backend && go test ./...`

Expected: PASS.

- [ ] **Step 5: Verify production behavior remains disabled**

Run: `cd backend && go test ./internal/service -run TestCreditAccountServiceGrantDisabled -count=1`

Expected: PASS; no existing recharge or usage path references `CreditAccountService` yet.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/wire.go backend/internal/repository/wire.go backend/cmd/server/wire_gen.go
git commit -m "chore(billing): wire credit ledger foundation"
```
