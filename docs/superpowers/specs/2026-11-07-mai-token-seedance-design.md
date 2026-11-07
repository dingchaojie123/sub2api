# Mai Token Seedance Provider Design

## Summary

Add Mai Token as a dedicated Seedance upstream platform with the internal
identifier `mai-token-seedance`. The platform supports only
`sdtj-2.5-480p` and is isolated from every existing Seedance provider at the
group, account, scheduling, model, pricing, quota, and error-policy layers.

The integration reuses the established PP Video task, ownership,
idempotency, polling, and balance-hold infrastructure. A provider-specific
adapter handles Mai Token's request formats, upstream endpoints, response
states, content delivery, and billing metadata.

## Goals

- Register Mai Token as an independently configurable video platform.
- Support the complete documented `sdtj-2.5-480p` JSON and multipart input
  surface.
- Call the upstream create, status, models, and content endpoints with Bearer
  authentication.
- Reserve against requested output duration, then settle against the
  upstream's authoritative `usage.billing_seconds`.
- Preserve completed media while withholding it until local billing settles.
- Keep task lookup and media delivery bound to the creating user, API key,
  platform, and upstream account.

## Non-goals

- Mixing Mai Token accounts into another Seedance platform or fallback pool.
- Supporting models other than `sdtj-2.5-480p`.
- Supporting video editing, extension, callbacks, draft mode, seeds,
  `return_last_frame`, or multi-output generation.
- Exposing upstream credentials, task endpoints, or provider URLs to clients.
- Creating a separate task engine or task table for this provider.

## Platform Isolation

The backend and frontend register `mai-token-seedance` as a first-class
platform. Its display name is `Mai Token Seedance` and its default base URL is
`https://api.mai-token.com`.

Only accounts whose platform is `mai-token-seedance` are eligible for a Mai
Token group. Scheduling remains constrained by the existing group and
platform filters. Status polling and content retrieval load the exact account
stored on the task; they do not select a replacement account from another
platform.

The provider has its own:

- domain and service constants;
- frontend platform metadata and account/group types;
- admin account option and default base URL;
- model filtering and whitelist behavior;
- error pass-through registration;
- user platform quota namespace;
- channel pricing entries.

The upstream `/v1/models` response is filtered to the exact model
`sdtj-2.5-480p`. Unknown models are never synchronized into the group.

## API Boundaries

Clients continue to use the site's established video API:

- `POST /v1/videos/generations`
- `GET /v1/videos/{task_id}`
- `GET|HEAD /v1/videos/{task_id}/content`

The adapter calls these Mai Token endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`
- `GET|HEAD /v1/videos/{task_id}/content`
- `GET /v1/models`

The client-facing model remains `sdtj-2.5-480p`; there is no cross-provider
alias or model fallback.

## Request Adapter

### Canonical representation

JSON and multipart requests are parsed into one provider-specific canonical
request. It contains the normalized scalar fields, ordered reference lists,
and optional file parts. This representation is used for validation, quoting,
security audit input, idempotency hashing, and upstream serialization.

Multipart files are streamed to request-scoped temporary files rather than
kept entirely in memory. The site's configured request body limit remains the
total upload limit. Temporary files are removed after submission or on any
error path.

The idempotency hash is computed from normalized field names and values plus
each file's field name, order, size, and content digest. It is independent of
the client's randomly generated multipart boundary.

### Accepted formats

The adapter accepts:

- native top-level fields such as `referenceImages`, `referenceVideos`,
  `referenceAudios`, `first_image`, and `last_image`;
- the `video-v1` `references` array, including `options.generate_audio`;
- the documented legacy `content` array;
- documented aliases for reference arrays, individual media URLs, duration,
  aspect ratio, audio generation, and first-frame input;
- multipart repeated `referenceImages` files plus `first_image` and
  `last_image` files.

`contract_version` is consumed locally and not forwarded. Unified references
are converted into the native upstream reference arrays while preserving
order and duplicate URLs.

### Defaults and validation

Validation occurs before creating a balance hold or calling the upstream.

- `model` must equal `sdtj-2.5-480p`.
- `prompt` must be non-empty.
- Omitted duration becomes 4 seconds before quoting and serialization.
- Duration must be an integer from 4 through 30 seconds.
- Omitted resolution becomes `480p`; any other resolution is rejected.
- `generate_audio` and `watermark` must be booleans when present, and explicit
  `false` values are preserved.
- Ratio is passed through without a local whitelist; the upstream decides
  whether it is supported.
- A last frame requires a first frame.
- First/last-frame mode cannot be mixed with ordinary reference images,
  videos, or audio.
- At most 30 reference images, 10 reference videos, and 10 reference audio
  items are allowed, with at most 50 reference items in total.
- URL media must use HTTP or HTTPS. Local paths, data URIs, and arbitrary URI
  schemes are rejected. Multipart accepts image inputs only.
- `seed`, `draft`, callback fields, `return_last_frame`, `n`, `count`, editing,
  and extension inputs are rejected before billing.

The adapter rebuilds multipart requests with a new valid boundary. JSON inputs
are sent as normalized JSON.

## Task Lifecycle

Creation uses a stable upstream idempotency key derived from the local task.
The adapter maps upstream states as follows:

| Upstream | Local internal state |
| --- | --- |
| `queued` | `processing` |
| `in_progress` | `processing` |
| `completed` | candidate `succeeded`, pending local settlement |
| `failed` | `failed` |

The upstream task ID is stored on the existing PP Video task. Polling uses the
original account and records the latest sanitized response. Temporary status
errors keep the task pending; a failed upstream task releases the local hold.

An upstream `billing_pending=true` response remains processing. The poller
does not treat 10 minutes as an automatic failure threshold for this
platform.

## Billing Model

Channel pricing for `sdtj-2.5-480p` uses the existing per-second video mode
and the 480p price tier. The price and all existing site multipliers are
snapshotted when the task is created.

Creation reserves:

`requested duration seconds * snapshotted per-second sale price * existing multipliers`

The upstream response contains three different measurements that must not be
conflated:

- `usage.output_video_duration` is the output duration shown to clients;
- `usage.reference_video_duration` is informational reference usage;
- `usage.billing_seconds` is the sole final billing unit.

The PP Video task gains a zero-default `provider_billing_seconds` integer
field. Parsing and persistence carry it independently from generated and
reference duration. Final raw cost is:

`provider_billing_seconds * snapshotted per-second sale price`

Existing group and account multipliers then apply through the normal billing
pipeline. The system does not add reference duration again, re-round the
value, or use the provider's monetary cost.

If valid billing seconds are unavailable after generation, or an additional
balance charge cannot be applied, the local public response is
`in_progress` with `billing_pending=true`. The completed response and media
location remain stored, but public result URLs and content delivery are
withheld. Settlement retries are idempotent and never resubmit generation.
Once settlement succeeds, the public state becomes `completed` and content
delivery is enabled.

Subscriptions follow the same authoritative unit calculation and existing
subscription accounting path.

## Public Responses and Content

Public responses retain the site's stable task contract. They expose the
public task ID, fixed public model, normalized status, progress, and sanitized
usage. A successful result URL points back to the site's authenticated content
endpoint, not to a provider URL.

Content access requires all of the following:

- the requesting user and API key own the task;
- the task platform is `mai-token-seedance`;
- local billing has completed;
- the exact stored upstream account is available;
- the upstream response is a successful video or binary content response.

Range and conditional-range headers are forwarded consistently with the
existing content proxy. Provider credentials and raw result URLs are never
returned to clients.

## Error Handling

Request validation errors return a local 400 before a hold is created.
Sanitized upstream 4xx messages may be returned as upstream errors. Transport
errors, malformed responses, and upstream 5xx responses return a generic
gateway error while detailed diagnostics remain in internal logs.

An uncertain create response is never blindly resubmitted with a different
idempotency key. Failed tasks release holds idempotently. Polling errors do not
replace a previously persisted useful response.

## Persistence and Migration

Add migration `210_pp_video_provider_billing_seconds.sql`:

- add `provider_billing_seconds BIGINT NOT NULL DEFAULT 0` to
  `pp_video_tasks` with a non-negative check constraint.

Repository create, update, scan, and query projections include the new field.
Existing tasks retain their current settlement behavior because the new field
is used only for the Mai Token platform.

No separate provider task table is introduced.

## Admin and User Experience

The admin account and group forms expose `Mai Token Seedance` as a distinct
platform. API key accounts default to `https://api.mai-token.com`. Model
selection contains only `sdtj-2.5-480p`, and channel pricing requires a 480p
per-second price before requests are accepted.

The user model-price catalog and API documentation list the new platform and
model separately from all other Seedance providers. Add a dedicated
`docs/video-api/mai-token-seedance.md` page and link it from the video API
overview and the in-app documentation catalog.

## Testing

Backend tests cover:

- platform registration and cross-platform scheduling isolation;
- default URL, Bearer authorization, model endpoint filtering, and exact
  upstream paths;
- native JSON, `video-v1`, legacy content, aliases, and multipart rebuilding;
- boundary-independent request hashing and temporary-file cleanup;
- all validation limits, media conflicts, defaults, and rejected fields;
- state parsing for queued, in-progress, billing-pending, completed, and
  failed responses;
- separate output, reference, and billing duration persistence;
- reserve, refund, additional charge, insufficient-balance retry, and
  idempotent settlement behavior;
- result URL withholding before settlement;
- owner/account/platform checks and authenticated content proxy behavior;
- polling behavior for temporary errors and long-running tasks.

Frontend tests cover platform metadata, account/group option rendering, fixed
model selection, model-price catalog entries, and in-app documentation
registration. Verification includes focused Go tests, the broader affected Go
package tests, frontend unit tests, and frontend type checking.

## Rollout

The migration and application code may deploy before any Mai Token group is
created. The feature remains inactive until an administrator adds a
`mai-token-seedance` account, configures a matching group and channel, and
sets the 480p per-second sale price. Existing video platforms and historical
tasks are unaffected.
