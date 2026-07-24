# Jimeng Upstream Client and Usage Log Query Design

## Goal

Add backend support for a Jimeng video upstream client and account API key probing, while keeping the scope limited to upstream interaction and local usage-log querying.

This design intentionally does not implement a public Jimeng video generation API surface, task persistence, balance holds, settlement, or media asset proxying.

## Inputs

The upstream API document describes a Jimeng video service with:

- Base URL such as `http://64.83.19.242/v1`.
- Bearer API key authentication.
- `POST /v1/video/generations` and compatible `POST /v1/videos/generations`.
- `GET /v1/video/generations/{task_id}` and compatible `GET /v1/videos/{task_id}`.
- Supported model `video-v1`.
- Supported durations `5`, `10`, and `15` seconds.
- Task IDs may appear as `task_id`, `request_id`, `id`, or under `data`.
- Status values need normalization across pending, success, and failure families.

## Scope

Implement:

- A Go service/client for calling Jimeng upstream create/query APIs.
- A non-charging Jimeng API key probe used by account creation/update flows.
- A local `usage_logs` query filter for `platform=jimeng` and `model=video-v1`.

Do not implement:

- Public `/v1/video/generations` request handling for end users.
- Task database tables for Jimeng video jobs.
- User balance freeze/capture/release for Jimeng tasks.
- Media asset URL rewriting or public asset proxy.
- Real video submission during account validation.

## Architecture

### Jimeng Upstream Client

Add a focused client in `backend/internal/service`, for example `jimeng_video_client.go`.

Responsibilities:

- Normalize configured base URLs, accepting either root host or `/v1` URLs.
- Build authenticated requests with `Authorization: Bearer <api_key>` and JSON content type.
- Submit video generation payloads to `/v1/video/generations`.
- Query task status from `/v1/video/generations/{task_id}`.
- Decode flexible task ID and status fields without assuming one exact response shape.
- Return structured errors for authentication failures, invalid responses, and upstream non-2xx responses.

The client should be small and testable with `httptest.Server`.

### Account Probe

Account probing should be safe and non-billable:

- For `platform=jimeng && type=apikey`, probe the configured base URL using `GET /v1/models`.
- If `/v1/models` returns 2xx, mark the key as reachable/valid.
- If it returns 401 or 403, treat the key as invalid.
- If it returns 404 or 405, treat the proxy as reachable but models capability unknown rather than disabling the account.
- Network failures or malformed URLs should surface as probe errors.

This avoids creating a real video task during validation.

### Usage Logs Query

Extend usage log filtering with an optional `platform` parameter.

Expected behavior:

- Admin usage list/stats endpoints can accept `platform=jimeng`.
- Model filtering continues to use the existing requested/upstream/mapping model-source rules.
- `platform` filtering should use the account platform associated with each usage log, via `usage_logs.account_id` and `accounts.platform`.
- The query should support `platform=jimeng&model=video-v1`.
- User-facing usage queries may support the same filter while remaining scoped to the authenticated user.

This reuses existing `usage_logs` rows and does not introduce a separate Jimeng usage table.

## Data Flow

### Probe Flow

1. Admin adds or updates a Jimeng API key account.
2. Backend reads `base_url` and `api_key` from account credentials/extra.
3. Backend calls `GET /v1/models`.
4. Probe result updates account capability/status using existing account probing conventions.

### Upstream Client Flow

1. Internal code constructs `JimengCreateVideoRequest`.
2. Client sends a bearer-authenticated POST request to the upstream.
3. Client extracts a normalized task ID from the response.
4. Internal code can query status later by task ID.

### Usage Query Flow

1. Caller requests usage logs or stats with `platform=jimeng` and optionally `model=video-v1`.
2. Handler validates platform with the existing platform allowlist.
3. Repository adds account-platform filtering to the usage query.
4. Response shape remains compatible with existing usage APIs.

## Error Handling

- Invalid API key probe: map 401/403 to an invalid credential result.
- Unsupported `/v1/models`: map 404/405 to "unknown capability" rather than invalid credentials.
- Invalid task create request: reject missing prompt/image/images and unsupported durations before sending upstream when using the client directly.
- Upstream non-2xx responses: preserve status code and bounded response body for diagnostics.
- Unknown task status: return raw status plus normalized `unknown`.

## Testing

Add unit tests for:

- Base URL normalization.
- Bearer auth headers.
- Create response task ID extraction from top-level and nested fields.
- Status normalization for pending/success/failure variants.
- Probe behavior for 2xx, 401/403, 404/405, and network failures.
- Usage log SQL/filter construction for `platform=jimeng`.

Add focused handler/repository tests for platform filtering, avoiding broad end-to-end tests unless existing patterns make that cheap.

## Decisions

- The initial probe will not submit real video tasks because that can incur cost.
- `/v1/models` support is optional for Jimeng proxies; lack of that endpoint should not automatically disable the account.
- Usage log filtering is based on local account platform, not on upstream response content.
