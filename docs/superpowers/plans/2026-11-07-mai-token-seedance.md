# Mai Token Seedance Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an isolated `mai-token-seedance` provider that supports the complete `sdtj-2.5-480p` JSON and multipart contract and settles against upstream `usage.billing_seconds`.

**Architecture:** Register Mai Token as a distinct PP Video platform, then route its requests through provider-specific JSON and multipart adapters while reusing existing task ownership, polling, and billing infrastructure. Persist provider billing seconds separately from output/reference durations and withhold successful media until local settlement completes.

**Tech Stack:** Go 1.x, Gin, PostgreSQL migrations, `gjson`, Vue 3, TypeScript, Vitest, `testify`.

**Spec:** `docs/superpowers/specs/2026-11-07-mai-token-seedance-design.md`

## Global Constraints

- Platform identifier: `mai-token-seedance`; display name: `Mai Token Seedance`.
- Default upstream base URL: `https://api.mai-token.com`.
- Only model `sdtj-2.5-480p` is eligible.
- Client creation remains `POST /v1/videos/generations`; upstream creation is `POST /v1/videos`.
- Duration defaults to 4 and must be an integer from 4 through 30.
- Resolution is fixed to `480p`.
- Final billable units come only from integer `usage.billing_seconds`; never add or round reference duration again.
- Never mix accounts, models, pricing, quotas, task lookup, or content delivery with another Seedance platform.
- Preserve all unrelated dirty-worktree changes and stage only files named in the current task.

---

### Task 1: Register the Isolated Platform

**Files:**
- Modify: `backend/internal/domain/constants.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/model/error_passthrough_rule.go`
- Modify: `backend/internal/service/pp_video.go`
- Modify: `backend/internal/service/upstream_models.go`
- Test: `backend/internal/service/domain_constants_platform_test.go`
- Test: `backend/internal/service/pp_video_mai_token_test.go`
- Test: `backend/internal/service/pp_video_task_query_test.go`

**Interfaces:**
- Produces: `domain.PlatformMaiTokenSeedance`, `service.PlatformMaiTokenSeedance`, `MaiTokenSeedanceDefaultBaseURL`, `MaiTokenSeedanceModel`.
- Produces: exact model filtering through `filterUpstreamModelsForPlatform` and `filterPPVideoPublicModelsForPlatform`.

- [ ] **Step 1: Write failing platform and model-isolation tests**

```go
func TestMaiTokenSeedancePlatformRegistration(t *testing.T) {
	t.Parallel()
	require.True(t, IsPPVideoPlatform(PlatformMaiTokenSeedance))
	require.Equal(t, "mai-token-seedance", PlatformMaiTokenSeedance)
	require.Equal(t, "https://api.mai-token.com", MaiTokenSeedanceDefaultBaseURL)
}

func TestMaiTokenSeedanceFiltersExactModel(t *testing.T) {
	t.Parallel()
	models := []string{"sdtj-2.5-480p", "seedance-2.5", "gpt-image-2"}
	require.Equal(t, []string{"sdtj-2.5-480p"}, filterUpstreamModelsForPlatform(PlatformMaiTokenSeedance, models))
	require.Equal(t, []string{"sdtj-2.5-480p"}, filterPPVideoPublicModelsForPlatform(PlatformMaiTokenSeedance, models))
}
```

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `cd backend && go test ./internal/service -run 'TestMaiTokenSeedancePlatformRegistration|TestMaiTokenSeedanceFiltersExactModel' -count=1`

Expected: FAIL because the platform constants and filtering cases do not exist.

- [ ] **Step 3: Add constants, supported-platform membership, and exact filtering**

```go
const PlatformMaiTokenSeedance = "mai-token-seedance"

const (
	MaiTokenSeedanceDefaultBaseURL = "https://api.mai-token.com"
	MaiTokenSeedanceModel          = "sdtj-2.5-480p"
)
```

Add the platform to `AllPlatforms`, PP Video task-query platform coverage, and the default error pass-through platform set. In model filtering, return only an exact case-sensitive `sdtj-2.5-480p` match; do not use broad `seedance` substring matching.

- [ ] **Step 4: Run platform tests**

Run: `cd backend && go test ./internal/service ./internal/model -run 'MaiToken|Platform' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the platform boundary**

```bash
git add backend/internal/domain/constants.go backend/internal/service/domain_constants.go backend/internal/model/error_passthrough_rule.go backend/internal/service/pp_video.go backend/internal/service/upstream_models.go backend/internal/service/domain_constants_platform_test.go backend/internal/service/pp_video_mai_token_test.go backend/internal/service/pp_video_task_query_test.go
git commit -m "feat(video): register Mai Token Seedance platform"
```

---

### Task 2: Normalize and Validate Mai Token JSON

**Files:**
- Create: `backend/internal/service/pp_video_mai_token.go`
- Modify: `backend/internal/service/pp_video.go`
- Test: `backend/internal/service/pp_video_mai_token_test.go`

**Interfaces:**
- Consumes: `PlatformMaiTokenSeedance`, `MaiTokenSeedanceModel` from Task 1.
- Produces: `normalizeMaiTokenSeedancePayload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error)`.

- [ ] **Step 1: Add table-driven failing tests for supported input forms**

Cover native arrays, `contract_version=video-v1` references, `options.generate_audio`, legacy `content`, all documented aliases, omitted duration/resolution, explicit false booleans, preserved order/duplicates, and `aspect_ratio` to `ratio` normalization.

```go
func TestPrepareMaiTokenSeedanceVideoV1Request(t *testing.T) {
	body := []byte(`{"contract_version":"video-v1","model":"sdtj-2.5-480p","prompt":"lake","references":[{"type":"image","role":"reference_image","source":"https://example.com/a.jpg"}],"options":{"generate_audio":false}}`)
	prepared, public, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, body)
	require.NoError(t, err)
	require.Equal(t, MaiTokenSeedanceModel, public.Model)
	require.Equal(t, int64(4000), public.DurationMilliseconds)
	require.JSONEq(t, `{"model":"sdtj-2.5-480p","prompt":"lake","duration":4,"resolution":"480p","referenceImages":["https://example.com/a.jpg"],"generate_audio":false}`, string(prepared))
}
```

- [ ] **Step 2: Run supported-format tests and confirm failure**

Run: `cd backend && go test ./internal/service -run 'TestPrepareMaiTokenSeedance' -count=1`

Expected: FAIL because no Mai Token normalization branch exists.

- [ ] **Step 3: Implement canonical JSON conversion**

Create focused helpers in `pp_video_mai_token.go`:

```go
func normalizeMaiTokenSeedancePayload(payload map[string]any, body []byte, public *PPVideoPublicRequest) (map[string]any, error)
func maiTokenReferenceLists(body []byte) (images, videos, audios []string, first, last string, err error)
func maiTokenRejectUnsupportedFields(body []byte) error
func validateMaiTokenHTTPURL(field, raw string, httpsOnly bool) error
```

Return a new map containing only upstream-supported fields. Set `duration=4` and `resolution="480p"` before billing metadata is read. Preserve explicit `false` for `generate_audio` and `watermark`. Consume `contract_version`, `references`, `content`, `options`, and aliases without forwarding them.

- [ ] **Step 4: Add failing validation tests**

Table cases must include wrong model, empty prompt, fractional/string/out-of-range duration, non-480p resolution, invalid boolean types, last frame without first frame, frame/reference mixing, 31 images, 11 videos, 11 audios, 51 total items, local/data/unsupported URLs, and every rejected feature (`seed`, `draft`, callback fields, `return_last_frame`, `n`, `count`).

```go
func TestPrepareMaiTokenSeedanceRejectsUnsupportedInputs(t *testing.T) {
	_, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric,
		[]byte(`{"model":"sdtj-2.5-480p","prompt":"lake","duration":4,"seed":1}`))
	require.ErrorContains(t, err, "seed")
}
```

- [ ] **Step 5: Implement validation and operation gating**

In the Mai Token switch branch, reject every operation other than `PPVideoOperationGeneric`. Validate all cardinality and exclusivity rules before returning the normalized body.

- [ ] **Step 6: Run JSON adapter tests**

Run: `cd backend && go test ./internal/service -run 'MaiTokenSeedance' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit JSON adaptation**

```bash
git add backend/internal/service/pp_video_mai_token.go backend/internal/service/pp_video.go backend/internal/service/pp_video_mai_token_test.go
git commit -m "feat(video): normalize Mai Token Seedance requests"
```

---

### Task 3: Add Streaming Multipart Preparation

**Files:**
- Create: `backend/internal/service/pp_video_mai_token_multipart.go`
- Create: `backend/internal/service/pp_video_mai_token_multipart_test.go`
- Modify: `backend/internal/handler/pp_video.go`
- Modify: `backend/internal/handler/pp_video_test.go`
- Modify: `backend/internal/service/pp_video_gateway.go`

**Interfaces:**
- Consumes: normalized Mai Token JSON from Task 2.
- Produces: `PPVideoSubmissionBody` and `MaiTokenMultipartSubmission`.
- Produces: `PrepareMaiTokenMultipartSubmission(req *http.Request) (*PPVideoSubmissionBody, error)`.
- Produces: `(*PPVideoSubmissionBody).PayloadHash() string`, `Open() (io.ReadCloser, string, error)`, and `Cleanup() error`.
- Produces: `ForwardPPVideoSubmissionBuffered(ctx context.Context, c *gin.Context, account *Account, operation PPVideoOperation, taskID string, submission *PPVideoSubmissionBody, publicRequests ...PPVideoPublicRequest) (*OpenAIForwardResult, error)`; existing `ForwardPPVideoBuffered` remains a JSON-compatible wrapper.

- [ ] **Step 1: Write failing multipart spool, hash, and cleanup tests**

```go
func TestPrepareMaiTokenMultipartSubmissionSpoolsAndHashesFiles(t *testing.T) {
	reqA := newMaiTokenMultipartTestRequest(t, "boundary-a", map[string]string{"model":"sdtj-2.5-480p","prompt":"lake","duration":"4"}, []testUpload{{Field:"first_image", Name:"first.jpg", Body:"jpeg-bytes"}})
	reqB := newMaiTokenMultipartTestRequest(t, "boundary-b", map[string]string{"model":"sdtj-2.5-480p","prompt":"lake","duration":"4"}, []testUpload{{Field:"first_image", Name:"first.jpg", Body:"jpeg-bytes"}})
	a, err := PrepareMaiTokenMultipartSubmission(reqA)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Cleanup() })
	b, err := PrepareMaiTokenMultipartSubmission(reqB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Cleanup() })
	require.Equal(t, a.PayloadHash(), b.PayloadHash())
}
```

Also verify repeated `referenceImages`, first/last files, rejected video/audio files, per-file metadata, reconstructed upstream parts, cleanup on parse failure, and cleanup after success.

- [ ] **Step 2: Run multipart tests and confirm failure**

Run: `cd backend && go test ./internal/service -run 'MaiTokenMultipart' -count=1`

Expected: FAIL because the types and parser do not exist.

- [ ] **Step 3: Implement request-scoped temp-file spooling**

```go
type MaiTokenUpload struct {
	FieldName   string
	FileName    string
	ContentType string
	TempPath    string
	Size        int64
	SHA256      string
}

type PPVideoSubmissionBody struct {
	JSON      []byte
	Multipart *MaiTokenMultipartSubmission
}
```

Use `req.MultipartReader`, `os.CreateTemp("", "sub2api-mai-token-*")`, `io.MultiWriter(file, sha256.New())`, and a cleanup method that removes only the explicit recorded paths. Enforce allowed file field names while reading. Feed normalized scalar fields and URL arrays through Task 2's JSON normalizer.

- [ ] **Step 4: Implement deterministic hashing and multipart reconstruction**

Hash normalized JSON followed by each upload's field name, ordinal, size, and SHA-256 digest. `Open` must create an `io.Pipe`, write normalized scalar fields and URL array fields, reopen each temp file, and close both the multipart writer and pipe with the first error.

- [ ] **Step 5: Add handler tests for content-type dispatch**

Verify multipart is accepted only when the authenticated group platform is `mai-token-seedance`; JSON behavior for every existing platform stays unchanged; audit and prompt recording use sanitized canonical JSON rather than binary file contents.

- [ ] **Step 6: Wire the submission object through the handler and gateway**

Add `ForwardPPVideoSubmissionBuffered` and make `ForwardPPVideoBuffered` wrap `body` as `&PPVideoSubmissionBody{JSON: body}`. In `handlePPVideo`, detect multipart only for Mai Token, defer cleanup immediately, use the canonical JSON for selection/quote/audit, and use `PayloadHash()` for idempotency.

- [ ] **Step 7: Run service and handler tests**

Run: `cd backend && go test ./internal/service ./internal/handler -run 'MaiTokenMultipart|PPVideo' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit multipart support**

```bash
git add backend/internal/service/pp_video_mai_token_multipart.go backend/internal/service/pp_video_mai_token_multipart_test.go backend/internal/service/pp_video_gateway.go backend/internal/handler/pp_video.go backend/internal/handler/pp_video_test.go
git commit -m "feat(video): accept Mai Token multipart requests"
```

---

### Task 4: Implement Upstream Create, Status, Models, and Content

**Files:**
- Modify: `backend/internal/service/pp_video.go`
- Modify: `backend/internal/service/pp_video_gateway.go`
- Modify: `backend/internal/service/upstream_models.go`
- Modify: `backend/internal/service/pp_video_task_content.go`
- Modify: `backend/internal/handler/pp_video.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Test: `backend/internal/service/pp_video_mai_token_test.go`
- Test: `backend/internal/service/pp_video_task_content_test.go`
- Test: `backend/internal/handler/pp_video_test.go`
- Test: `backend/internal/server/routes/gateway_test.go`

**Interfaces:**
- Consumes: platform constants and submission bodies from Tasks 1-3.
- Produces: Mai Token cases in `PPVideoUpstreamPath`, `PPVideoStatusUpstreamPath`, `PPVideoRequestHeaders`, `ParsePPVideoResponse`, `ForwardPPVideoTaskContent`, and upstream model requests.

- [ ] **Step 1: Write failing protocol path and response tests**

```go
func TestMaiTokenSeedanceProtocol(t *testing.T) {
	create, err := PPVideoUpstreamPath(PlatformMaiTokenSeedance, PPVideoOperationGeneric, "")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", create)
	status, err := PPVideoStatusUpstreamPath(PlatformMaiTokenSeedance, PPVideoOperationGeneric, "task/one")
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/task%2Fone", status)
	headers, err := PPVideoRequestHeaders(PlatformMaiTokenSeedance, "mai-key")
	require.NoError(t, err)
	require.Equal(t, "Bearer mai-key", headers.Get("Authorization"))
}
```

Add `httptest` transport coverage for the default base URL, `/v1/models`, JSON and multipart content types, stable upstream idempotency key, queued/in-progress/completed/failed parsing, status task-ID fallback, and GET/HEAD content with Range/If-Range.

- [ ] **Step 2: Run protocol tests and confirm failure**

Run: `cd backend && go test ./internal/service ./internal/handler ./internal/server/routes -run 'MaiToken|PPVideoRecommended' -count=1`

Expected: FAIL because Mai Token paths and route dispatch are absent.

- [ ] **Step 3: Add exact upstream paths and headers**

Creation is `/v1/videos`; status and content use `url.PathEscape(taskID)`. Add Bearer authentication and JSON content type unless `PPVideoSubmissionBody.Open` returns a multipart type. Set upstream `Idempotency-Key` to the stable local task ID for creation.

- [ ] **Step 4: Add response parsing without exposing provider URLs**

Parse IDs from `id` and `task_id`; map `queued` and `in_progress` to processing, `completed` to succeeded, and `failed` to failed. Read output/reference durations into their existing distinct fields. Normalize successful public URLs to `/v1/videos/{task_id}/content` and strip provider URLs from non-settled responses.

- [ ] **Step 5: Enable isolated routing and authenticated content**

Add Mai Token to generation/status route dispatch and the content allowlist. Extend `ppVideoTaskUsesAuthenticatedContent` and require the task platform/account platform pair to match exactly.

- [ ] **Step 6: Run protocol and route tests**

Run: `cd backend && go test ./internal/service ./internal/handler ./internal/server/routes -run 'MaiToken|PPVideo' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit upstream protocol support**

```bash
git add backend/internal/service/pp_video.go backend/internal/service/pp_video_gateway.go backend/internal/service/upstream_models.go backend/internal/service/pp_video_task_content.go backend/internal/handler/pp_video.go backend/internal/server/routes/gateway.go backend/internal/service/pp_video_mai_token_test.go backend/internal/service/pp_video_task_content_test.go backend/internal/handler/pp_video_test.go backend/internal/server/routes/gateway_test.go
git commit -m "feat(video): connect Mai Token video lifecycle"
```

---

### Task 5: Persist Provider Billing Seconds and Quota Namespace

**Files:**
- Create: `backend/migrations/210_pp_video_provider_billing_seconds.sql`
- Create: `backend/migrations/mai_token_seedance_migration_test.go`
- Create: `backend/internal/repository/mai_token_seedance_migration_integration_test.go`
- Modify: `backend/internal/service/pp_video.go`
- Modify: `backend/internal/service/pp_video_task.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/pp_video_gateway.go`
- Modify: `backend/internal/service/pp_video_poller.go`
- Modify: `backend/internal/repository/pp_video_task_repo.go`
- Modify: `backend/internal/repository/pp_video_task_repo_test.go`
- Test: `backend/internal/service/pp_video_mai_token_test.go`
- Test: `backend/internal/service/pp_video_poller_test.go`

**Interfaces:**
- Produces: `PPVideoResponse.ProviderBillingSeconds int64`.
- Produces: `OpenAIForwardResult.VideoProviderBillingSeconds int64`.
- Produces: `PPVideoTask.ProviderBillingSeconds int64` and matching create/update repository parameters.

- [ ] **Step 1: Write failing migration contract tests**

```go
func TestMigration210AddsMaiTokenBillingSecondsAndQuota(t *testing.T) {
	raw, err := FS.ReadFile("210_pp_video_provider_billing_seconds.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "provider_billing_seconds BIGINT NOT NULL DEFAULT 0")
	require.Contains(t, sql, "provider_billing_seconds >= 0")
	require.Contains(t, sql, "'mai-token-seedance'")
}
```

Also add an integration-tagged repository migration test modeled on `xingapi_seedance_migration_integration_test.go` to prove the quota constraint preserves deployed custom values and accepts `mai-token-seedance`.

- [ ] **Step 2: Run migration tests and confirm failure**

Run: `cd backend && go test ./migrations -run 'Migration210' -count=1`

Expected: FAIL because migration 210 is absent.

- [ ] **Step 3: Add the forward-only migration**

```sql
ALTER TABLE pp_video_tasks
    ADD COLUMN IF NOT EXISTS provider_billing_seconds BIGINT NOT NULL DEFAULT 0;

ALTER TABLE pp_video_tasks
    DROP CONSTRAINT IF EXISTS pp_video_tasks_provider_billing_seconds_check;
ALTER TABLE pp_video_tasks
    ADD CONSTRAINT pp_video_tasks_provider_billing_seconds_check
    CHECK (provider_billing_seconds >= 0);
```

In the same migration, rebuild `user_platform_quotas_platform_check` using the preservation pattern from migration 208 and include `mai-token-seedance`.

- [ ] **Step 4: Write failing parser and repository round-trip tests**

Verify `usage.output_video_duration=5`, `usage.reference_video_duration=5.06195`, and `usage.billing_seconds=11` remain three independent values through parse, gateway result, task update, scan, and poller update.

- [ ] **Step 5: Thread the integer field through all layers**

Add the field immediately after input/generated duration fields in service structs and repository projections. Update every insert, update, `RETURNING`, scan, fake repository, and poller parameter in the same order. Parse only an integer or exact JSON integer from `usage.billing_seconds`; negative, missing, fractional, and overflow values become zero.

- [ ] **Step 6: Run migration, repository, parser, and poller tests**

Run: `cd backend && go test ./migrations ./internal/repository ./internal/service -run 'Migration210|ProviderBillingSeconds|MaiToken' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit persistence**

```bash
git add backend/migrations/210_pp_video_provider_billing_seconds.sql backend/migrations/mai_token_seedance_migration_test.go backend/internal/repository/mai_token_seedance_migration_integration_test.go backend/internal/service/pp_video.go backend/internal/service/pp_video_task.go backend/internal/service/openai_gateway_service.go backend/internal/service/pp_video_gateway.go backend/internal/service/pp_video_poller.go backend/internal/repository/pp_video_task_repo.go backend/internal/repository/pp_video_task_repo_test.go backend/internal/service/pp_video_mai_token_test.go backend/internal/service/pp_video_poller_test.go
git commit -m "feat(video): persist Mai Token billing seconds"
```

---

### Task 6: Gate Completion on Local Settlement

**Files:**
- Modify: `backend/internal/service/pp_video_billing.go`
- Modify: `backend/internal/service/pp_video_poller.go`
- Modify: `backend/internal/service/pp_video.go`
- Modify: `backend/internal/handler/pp_video.go`
- Test: `backend/internal/service/pp_video_billing_test.go`
- Test: `backend/internal/service/pp_video_poller_test.go`
- Test: `backend/internal/handler/pp_video_test.go`
- Test: `backend/internal/service/pp_video_mai_token_test.go`

**Interfaces:**
- Consumes: `PPVideoTask.ProviderBillingSeconds` from Task 5.
- Produces: `ppVideoTaskBillingPending(task *PPVideoTask) bool`.
- Produces: `normalizeStoredPPVideoBillingPendingResponse(task *PPVideoTask) []byte`.
- Changes: Mai Token `ppVideoRawSettlementCost` uses provider billing seconds.

- [ ] **Step 1: Write failing authoritative-unit billing tests**

```go
func TestMaiTokenSettlementUsesProviderBillingSecondsOnly(t *testing.T) {
	task := &PPVideoTask{Platform: PlatformMaiTokenSeedance, BillingFormula: PPVideoBillingFormulaPerSecond, BillingUnitPrice: 0.25, VideoCount: 1, GeneratedVideoDurationMilliseconds: 5000, InputVideoDurationMilliseconds: 5062, ProviderBillingSeconds: 11}
	require.InDelta(t, 2.75, ppVideoRawSettlementCost(task), 1e-12)
}
```

Add cases proving requested duration is used only for the hold, output/reference durations do not alter final units, and existing provider calculations are unchanged.

- [ ] **Step 2: Run billing tests and confirm failure**

Run: `cd backend && go test ./internal/service -run 'MaiTokenSettlement|BillingPending' -count=1`

Expected: FAIL because settlement still uses generated duration and successful stored responses can replay before capture.

- [ ] **Step 3: Use provider billing seconds for final cost**

In `ppVideoRawSettlementCost`, branch on `PlatformMaiTokenSeedance`; require a positive field and call `ppVideoTaskBillingUnits(task, ProviderBillingSeconds*1000)` without adding input duration. Keep the snapshotted unit price and existing multiplier path.

- [ ] **Step 4: Write failing settlement-pending response tests**

Cover completed upstream responses with missing billing seconds, extra balance charge failure, `billing_status=settling`, repeated settlement attempts, and content access before/after `captured`. Assert pending responses contain `status:"in_progress"`, `billing_pending:true`, and no `result.url`, `metadata.url`, or `video_url`.

- [ ] **Step 5: Implement billing-pending masking and retry behavior**

Treat upstream completed-without-valid-billing-seconds as processing and persist its response. When local capture/accounting fails, keep `billing_status=settling`; stored-response replay must return the masked pending representation. The poller retries settlement idempotently without another create request. Exempt Mai Token processing tasks from the generic processing-age failure path. Content requires `billing_status=captured`.

- [ ] **Step 6: Run billing, poller, and handler tests**

Run: `cd backend && go test ./internal/service ./internal/handler -run 'MaiToken|BillingPending|Settlement' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit settlement gating**

```bash
git add backend/internal/service/pp_video_billing.go backend/internal/service/pp_video_poller.go backend/internal/service/pp_video.go backend/internal/handler/pp_video.go backend/internal/service/pp_video_billing_test.go backend/internal/service/pp_video_poller_test.go backend/internal/handler/pp_video_test.go backend/internal/service/pp_video_mai_token_test.go
git commit -m "feat(video): gate Mai Token results on settlement"
```

---

### Task 7: Expose Admin and Pricing Configuration

**Files:**
- Modify: `frontend/src/constants/platforms.ts`
- Modify: `frontend/src/constants/__tests__/platforms.spec.ts`
- Modify: `frontend/src/constants/__tests__/channel.spec.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/components/common/PlatformIcon.vue`
- Modify: `frontend/src/components/common/PlatformTypeBadge.vue`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/groupsImagePricing.ts`
- Modify: `frontend/src/views/admin/__tests__/groupsImagePricing.spec.ts`
- Modify: `frontend/src/composables/useModelWhitelist.ts`
- Modify: `frontend/src/composables/__tests__/useModelWhitelist.spec.ts`

**Interfaces:**
- Consumes: platform/model identifiers from backend Tasks 1-2.
- Produces: admin account/group options and the fixed `['sdtj-2.5-480p']` model list.

- [ ] **Step 1: Write failing frontend platform tests**

```ts
expect(VIDEO_ACCOUNT_PLATFORMS).toContain('mai-token-seedance')
expect(CHANNEL_PRICING_PLATFORMS).toContain('mai-token-seedance')
expect(getVideoAccountPlatformMetadata('mai-token-seedance')).toMatchObject({
  label: 'Mai Token Seedance',
  defaultBaseUrl: 'https://api.mai-token.com',
  accountType: 'apikey',
  authScheme: 'bearer'
})
expect(getModelsByPlatform('mai-token-seedance')).toEqual(['sdtj-2.5-480p'])
```

Assert it is a PP Video pricing platform, does not use group fallback prices, and appears separately from all other Seedance entries.

- [ ] **Step 2: Run frontend tests and confirm failure**

Run: `cd frontend && pnpm test:run src/constants/__tests__/platforms.spec.ts src/constants/__tests__/channel.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/views/admin/__tests__/groupsImagePricing.spec.ts`

Expected: FAIL because the platform is not registered.

- [ ] **Step 3: Add types, metadata, labels, icons, and pricing behavior**

Add `'mai-token-seedance'` to both `GroupPlatform` and `AccountPlatform`, platform arrays, group option/filter lists, Seedance icon condition, and type-badge label/style. Add a dedicated `maiTokenSeedanceModels` constant containing only `sdtj-2.5-480p`. Configure channel pricing rather than group-price fallback.

- [ ] **Step 4: Run frontend focused tests and typecheck**

Run: `cd frontend && pnpm test:run src/constants/__tests__/platforms.spec.ts src/constants/__tests__/channel.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/views/admin/__tests__/groupsImagePricing.spec.ts && pnpm typecheck`

Expected: PASS.

- [ ] **Step 5: Commit frontend configuration**

```bash
git add frontend/src/constants/platforms.ts frontend/src/constants/__tests__/platforms.spec.ts frontend/src/constants/__tests__/channel.spec.ts frontend/src/types/index.ts frontend/src/components/common/PlatformIcon.vue frontend/src/components/common/PlatformTypeBadge.vue frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/groupsImagePricing.ts frontend/src/views/admin/__tests__/groupsImagePricing.spec.ts frontend/src/composables/useModelWhitelist.ts frontend/src/composables/__tests__/useModelWhitelist.spec.ts
git commit -m "feat(admin): configure Mai Token Seedance"
```

---

### Task 8: Add User Pricing and API Documentation

**Files:**
- Create: `docs/video-api/mai-token-seedance.md`
- Modify: `docs/PP_VIDEO_GENERATION_API.md`
- Modify: `frontend/src/views/user/apiDocsCatalog.ts`
- Modify: `frontend/src/views/user/modelPricingCatalog.ts`
- Modify: `frontend/src/views/user/__tests__/apiDocsCatalog.spec.ts`
- Modify: `frontend/src/views/user/__tests__/apiDocsRender.spec.ts`
- Modify: `frontend/src/views/user/__tests__/ModelListView.spec.ts`

**Interfaces:**
- Consumes: public request/response behavior from Tasks 2, 4, and 6.
- Produces: in-app page slug `mai-token-seedance` and a separate price catalog entry.

- [ ] **Step 1: Write failing catalog and render tests**

```ts
expect(apiDocPages.some(page => page.slug === 'mai-token-seedance')).toBe(true)
expect(modelSquareCatalog.some(model => model.model === 'sdtj-2.5-480p')).toBe(true)
```

Render the page and assert it documents duration 4-30, fixed 480p, JSON and multipart examples, reference limits, billing pending, status polling, and authenticated content download.

- [ ] **Step 2: Run documentation tests and confirm failure**

Run: `cd frontend && pnpm test:run src/views/user/__tests__/apiDocsCatalog.spec.ts src/views/user/__tests__/apiDocsRender.spec.ts src/views/user/__tests__/ModelListView.spec.ts`

Expected: FAIL because the page and model entry are absent.

- [ ] **Step 3: Add the dedicated documentation page**

Document only the site's public API and public model; do not publish `api.mai-token.com`, upstream credentials, or raw provider URLs. Include native JSON, `video-v1`, first-frame multipart, polling, content, validation, and billing examples that match the implemented behavior.

- [ ] **Step 4: Register the page and pricing card**

Import the Markdown in `apiDocsCatalog.ts`, add it to the video group, link it from `PP_VIDEO_GENERATION_API.md`, and add a `sdtj-2.5-480p` model card with 480p and per-second pricing metadata.

- [ ] **Step 5: Run documentation tests**

Run: `cd frontend && pnpm test:run src/views/user/__tests__/apiDocsCatalog.spec.ts src/views/user/__tests__/apiDocsRender.spec.ts src/views/user/__tests__/ModelListView.spec.ts`

Expected: PASS.

- [ ] **Step 6: Commit documentation**

```bash
git add docs/video-api/mai-token-seedance.md docs/PP_VIDEO_GENERATION_API.md frontend/src/views/user/apiDocsCatalog.ts frontend/src/views/user/modelPricingCatalog.ts frontend/src/views/user/__tests__/apiDocsCatalog.spec.ts frontend/src/views/user/__tests__/apiDocsRender.spec.ts frontend/src/views/user/__tests__/ModelListView.spec.ts
git commit -m "docs(video): document Mai Token Seedance"
```

---

### Task 9: End-to-End Regression Verification

**Files:**
- Modify only if a verification failure exposes a defect in files from Tasks 1-8.

**Interfaces:**
- Consumes: all preceding tasks.
- Produces: verified isolated provider integration with no known failing affected tests.

- [ ] **Step 1: Format changed Go files**

Run: `cd backend && gofmt -w internal/domain/constants.go internal/model/error_passthrough_rule.go internal/service/domain_constants.go internal/service/pp_video.go internal/service/pp_video_mai_token.go internal/service/pp_video_mai_token_multipart.go internal/service/pp_video_gateway.go internal/service/pp_video_task.go internal/service/pp_video_task_content.go internal/service/pp_video_billing.go internal/service/pp_video_poller.go internal/service/openai_gateway_service.go internal/service/upstream_models.go internal/handler/pp_video.go internal/server/routes/gateway.go internal/repository/pp_video_task_repo.go`

Expected: command succeeds.

- [ ] **Step 2: Run affected backend packages**

Run: `cd backend && go test ./internal/domain ./internal/model ./internal/service ./internal/handler ./internal/server/routes ./internal/repository ./migrations -count=1`

Expected: PASS.

- [ ] **Step 3: Run frontend tests and typecheck**

Run: `cd frontend && pnpm test:run src/constants/__tests__/platforms.spec.ts src/constants/__tests__/channel.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/views/admin/__tests__/groupsImagePricing.spec.ts src/views/user/__tests__/apiDocsCatalog.spec.ts src/views/user/__tests__/apiDocsRender.spec.ts src/views/user/__tests__/ModelListView.spec.ts && pnpm typecheck`

Expected: PASS.

- [ ] **Step 4: Run static diff checks**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; status contains only intentional feature changes plus the user's pre-existing unrelated changes.

- [ ] **Step 5: Review isolation and secret handling manually**

Inspect the final diff and confirm `mai-token-seedance` never enters another provider's scheduler case, public responses contain no upstream API key/base URL/raw result URL, multipart temp paths are request-scoped and cleaned, and content access requires ownership plus captured billing.

- [ ] **Step 6: Record the verification result**

If a check fails, return to the task that owns the failing file, add a regression test there, fix it, rerun that task's checks, and commit using that task's explicit file list. Task 9 itself creates no commit when all checks pass.
