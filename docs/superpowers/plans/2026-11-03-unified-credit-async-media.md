# Unified Credit Async Media Settlement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert batch image, Jimeng video, and PP video task billing to explicit credit holds, captures, releases, and independent provider CNY cost.

**Architecture:** Async services calculate estimated/actual credits through the shared customer-price resolver and settle through the existing usage-billing repository, which invokes the credit transaction helpers and updates related quotas atomically. `credit_holds` is the source of truth for open obligations. Provider cost is stored independently on terminal task and usage-log records.

**Tech Stack:** Go, PostgreSQL, Ent, `shopspring/decimal`, async workers/pollers, Testify, SQLMock.

**Spec:** `docs/superpowers/specs/2026-11-03-unified-credit-billing-design.md`

## Global Constraints

- Complete the ledger, funding, and synchronous usage/pricing plans first.
- Hold/capture/release use stable task-derived keys and remain idempotent across poll retries.
- Success captures actual credits; failure/cancellation releases all remaining held credits.
- Resolution tier selection is shared across all video providers.
- Provider CNY cost never changes held or captured credits.
- Existing user modifications in PP-video files must be read and preserved; never reset or overwrite them.

---

### Task 1: Add explicit async credit and cost persistence

**Files:**
- Create: `backend/migrations/211_async_media_credit_fields.sql`
- Create: `backend/migrations/async_media_credit_fields_migration_test.go`
- Modify: `backend/ent/schema/batch_image_job.go`
- Modify: `backend/ent/schema/batch_image_item.go`
- Modify: generated files under `backend/ent/`
- Modify: `backend/internal/service/jimeng_video_billing.go`
- Modify: `backend/internal/service/pp_video_task.go`

**Interfaces:**
- Adds nullable `estimated_credits`, `held_credits`, `charged_credits`, and `provider_cost_cny` to batch-image jobs/items, Jimeng tasks, and PP-video tasks.
- Adds `credit_hold_key` with a unique partial index on each task family.

- [ ] **Step 1: Write migration contract tests**

Assert all three task families receive explicit unit fields and that legacy `estimated_total_cost`, `hold_amount`, and `actual_cost` remain unchanged/read-only.

- [ ] **Step 2: Run the migration test and verify it fails**

Run: `cd backend && go test ./migrations -run TestAsyncMediaCreditFieldsMigrationContract -count=1`

Expected: FAIL because migration 211 does not exist.

- [ ] **Step 3: Add migration and model fields**

Use `NUMERIC(20,8)` and nullable fields for legacy tasks. New tasks created under credit billing require a hold key and estimated credits before upstream submission.

- [ ] **Step 4: Generate and test schema**

Run: `cd backend && make generate`

Run: `cd backend && go test ./migrations ./ent/schema -run 'AsyncMediaCredit|BatchImage' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/211_async_media_credit_fields.sql backend/migrations/async_media_credit_fields_migration_test.go backend/ent backend/internal/service/jimeng_video_billing.go backend/internal/service/pp_video_task.go
git commit -m "feat(media): add explicit async credit settlement fields"
```

### Task 2: Add transactional async credit commands to usage billing

**Files:**
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/repository/usage_billing_repo_unit_test.go`
- Modify: `backend/internal/repository/usage_billing_repo_integration_test.go`

**Interfaces:**
- Produces `ReserveCreditHold`, `CaptureCreditHold`, and `ReleaseCreditHold` repository methods.
- Commands contain decimal `EstimatedCredits`, `ActualCredits`, and `ProviderCostCNY`, plus owner, API key, organization/member, task, hold, request, and fingerprint IDs.

- [ ] **Step 1: Write repository tests**

Cover personal and organization holds, lower capture, higher capture with sufficient/insufficient available credits, release, subscription allowance reservation, related customer quota updates, replay, and conflicting payload reuse.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/repository -run 'CreditHold|CreditCapture|CreditRelease' -count=1`

Expected: FAIL because new commands do not exist.

- [ ] **Step 3: Implement using shared credit transaction helpers**

Each method claims usage-billing dedup and mutates account/hold/ledger/customer quotas in one transaction. Do not call standalone credit-service methods from inside the repository and do not open nested transactions.

- [ ] **Step 4: Return canonical results**

Return `AvailableCredits`, `HeldCredits`, `Applied`, and terminal hold status as decimals. A replay returns the original terminal mutation result.

- [ ] **Step 5: Run repository tests**

Run: `cd backend && go test ./internal/repository -run 'CreditHold|CreditCapture|CreditRelease|UsageBilling' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/usage_billing.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_unit_test.go backend/internal/repository/usage_billing_repo_integration_test.go
git commit -m "feat(billing): add transactional async credit settlement"
```

### Task 3: Migrate batch-image hold and settlement

**Files:**
- Modify: `backend/internal/service/batch_image.go`
- Modify: `backend/internal/service/batch_image_public.go`
- Modify: `backend/internal/service/batch_image_billing_hold.go`
- Modify: `backend/internal/service/batch_image_settlement.go`
- Modify: `backend/internal/service/batch_image_billing_recovery.go`
- Modify: `backend/internal/repository/batch_image_repo.go`
- Modify: `backend/internal/service/batch_image_public_test.go`
- Modify: `backend/internal/service/batch_image_settlement_test.go`
- Modify: `backend/internal/service/batch_image_billing_recovery_test.go`
- Modify: `backend/internal/repository/batch_image_repo_integration_test.go`

**Interfaces:**
- Hold key: `batch-image:<batch-id>`.
- Creation stores `estimated_credits` and `held_credits` before enqueue.
- Terminal settlement stores `charged_credits` and `provider_cost_cny`.

- [ ] **Step 1: Write credit-mode lifecycle tests**

Cover estimate/hold, partial success lower capture, full success, all failed release, cancellation release, worker replay, and recovery of a job whose upstream result exists but capture was interrupted.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'BatchImage.*Credit' -count=1`

Expected: FAIL because batch image still uses legacy balance amounts.

- [ ] **Step 3: Replace enabled pricing and hold**

Use `CustomerCreditPricingResolver.QuoteImage`; apply the batch customer discount/hold policy in credits; reserve through `ReserveCreditHold`. Disabled behavior remains unchanged until global cutover.

- [ ] **Step 4: Replace terminal settlement and recovery**

Capture actual successful-image credits or release on terminal zero-success/failure/cancel. Persist provider cost separately. Recovery checks the hold table rather than inferring frozen balance.

- [ ] **Step 5: Run batch-image tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'BatchImage' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/batch_image.go backend/internal/service/batch_image_public.go backend/internal/service/batch_image_billing_hold.go backend/internal/service/batch_image_settlement.go backend/internal/service/batch_image_billing_recovery.go backend/internal/service/batch_image_public_test.go backend/internal/service/batch_image_settlement_test.go backend/internal/service/batch_image_billing_recovery_test.go backend/internal/repository/batch_image_repo.go backend/internal/repository/batch_image_repo_integration_test.go
git commit -m "feat(batch-image): settle tasks with unified credits"
```

### Task 4: Migrate Jimeng video hold and settlement

**Files:**
- Modify: `backend/internal/service/jimeng_video_billing.go`
- Modify: `backend/internal/service/jimeng_video_gateway.go`
- Modify: `backend/internal/service/jimeng_video_poller.go`
- Modify: `backend/internal/repository/jimeng_video_task_repo.go`
- Modify: `backend/internal/service/jimeng_video_billing_test.go`
- Modify: `backend/internal/service/jimeng_video_poller_test.go`
- Modify: `backend/internal/repository/jimeng_video_task_repo_test.go`

**Interfaces:**
- Hold key: `jimeng-video:<task-id>`.
- Customer charge uses shared video resolution/duration pricing.
- Provider CNY cost uses the actual selected Jimeng channel/account snapshot.

- [ ] **Step 1: Add failing credit lifecycle tests**

Cover requested-duration hold, actual-duration lower/higher capture, missing actual-duration fallback, failure/cancel release, poll replay, and independent provider cost.

- [ ] **Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'JimengVideo.*Credit' -count=1`

Expected: FAIL because Jimeng still carries float cost/hold fields.

- [ ] **Step 3: Implement enabled credit path**

Quote credits before upstream submission, reserve the stable hold, store all snapshots, and settle terminal state idempotently. A successful task with missing duration uses requested/default duration exactly once.

- [ ] **Step 4: Run Jimeng tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'JimengVideo' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/jimeng_video_billing.go backend/internal/service/jimeng_video_gateway.go backend/internal/service/jimeng_video_poller.go backend/internal/repository/jimeng_video_task_repo.go backend/internal/service/jimeng_video_billing_test.go backend/internal/service/jimeng_video_poller_test.go backend/internal/repository/jimeng_video_task_repo_test.go
git commit -m "feat(jimeng): settle video tasks with unified credits"
```

### Task 5: Migrate PP video and unify resolution pricing

**Files:**
- Modify carefully: `backend/internal/service/pp_video.go`
- Modify carefully: `backend/internal/service/pp_video_billing.go`
- Modify carefully: `backend/internal/service/pp_video_gateway.go`
- Modify carefully: `backend/internal/service/pp_video_poller.go`
- Modify carefully: `backend/internal/repository/pp_video_task_repo.go`
- Modify carefully: `backend/internal/handler/pp_video.go`
- Modify carefully: `backend/internal/service/pp_video_billing_test.go`
- Modify carefully: `backend/internal/service/pp_video_ctmo_test.go`
- Modify carefully: `backend/internal/service/pp_video_guangying_test.go`
- Modify carefully: `backend/internal/service/pp_video_opone_test.go`
- Modify carefully: `backend/internal/service/pp_video_compshare_test.go`
- Modify carefully: `backend/internal/service/pp_video_poller_test.go`
- Modify carefully: `backend/internal/repository/pp_video_task_repo_test.go`
- Modify carefully: `backend/internal/handler/pp_video_test.go`

**Interfaces:**
- Hold key: `pp-video:<task-id>`.
- `ResolveVideoCreditUnitPrice(group, resolution)` is provider-independent.
- 15 seconds at 1.6 credits/second and multiplier 1 produces exactly 24 credits.

- [ ] **Step 1: Inspect and preserve the dirty PP-video diff**

Run: `git diff -- backend/internal/handler/pp_video.go backend/internal/repository/pp_video_task_repo.go backend/internal/service/pp_video.go backend/internal/service/pp_video_gateway.go backend/internal/service/pp_video_poller.go`

Expected: understand all user changes before editing; do not discard or reformat unrelated hunks.

- [ ] **Step 2: Add provider-matrix pricing tests**

Test Guangying, CTMO, OPONE, 88API, Kling, Seedance, MiniMax, and other enabled PP providers across supported resolution tiers. Assert every provider uses the same customer tier resolver and no provider silently falls back to default when a configured tier exists.

- [ ] **Step 3: Add hold/capture/release tests**

Cover the same lifecycle and idempotency cases as Jimeng, including generated milliseconds, count multiplier, task replay, and independent provider cost.

- [ ] **Step 4: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler -run 'PPVideo.*Credit|VideoCreditUnitPrice' -count=1`

Expected: FAIL because PP video still settles legacy balance cost and provider-specific tier selection is inconsistent.

- [ ] **Step 5: Implement shared credit resolution and settlement**

Use requested/default duration for hold and generated duration for capture. Route all platforms through `ResolveVideoCreditUnitPrice`. Preserve provider request/response behavior from the pre-existing dirty diff.

- [ ] **Step 6: Run all PP-video tests**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler -run 'PPVideo|Guangying|CTMO|OPONE|Seedance|Compshare' -count=1`

Expected: PASS.

- [ ] **Step 7: Review the final diff against the starting dirty diff**

Run: `git diff --check`

Run: `git diff -- backend/internal/handler/pp_video.go backend/internal/repository/pp_video_task_repo.go backend/internal/service/pp_video.go backend/internal/service/pp_video_gateway.go backend/internal/service/pp_video_poller.go`

Expected: user changes remain present; only credit-settlement changes were added.

- [ ] **Step 8: Commit only the intended PP-video hunks**

Stage with explicit paths/hunks so unrelated pre-existing changes are not accidentally committed.

```bash
git commit -m "feat(video): settle PP tasks with unified credits"
```

### Task 6: Verify async drain and end-to-end lifecycle

**Files:**
- Create: `backend/internal/integration/async_credit_billing_e2e_test.go`
- Modify: `backend/internal/service/credit_migration.go`
- Modify: `backend/internal/repository/credit_migration_repo.go`

**Interfaces:**
- Reconciliation reports every open `credit_holds` obligation and can assert a zero-open-hold cutover gate.

- [ ] **Step 1: Add drain-gate tests**

Require cutover readiness to fail with any open hold or nonzero legacy frozen balance and to pass only when both are zero.

- [ ] **Step 2: Add end-to-end lifecycle tests**

Seed 2088 credits, submit a 15-second 1.6-credit/second video, assert 24 held, capture success, final 2064 available, zero held, one terminal hold, one hold ledger row, one capture row, and no duplicate on poll replay.

- [ ] **Step 3: Run async and backend suites**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler ./internal/integration -run 'Credit|BatchImage|JimengVideo|PPVideo' -count=1`

Run: `cd backend && go test ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/integration/async_credit_billing_e2e_test.go backend/internal/service/credit_migration.go backend/internal/repository/credit_migration_repo.go
git commit -m "test(billing): verify async credit holds and cutover drain"
```
