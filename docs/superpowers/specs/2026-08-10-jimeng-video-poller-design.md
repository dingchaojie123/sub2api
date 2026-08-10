# Jimeng Video Poller Design

## Goal

Add a background scanner for Jimeng video tasks so held video balance is captured only after an upstream terminal success, and released after failure, cancellation, refund, or timeout.

## Current Behavior

Jimeng video generation already creates a local task, reserves an estimated balance hold, stores the upstream task id after submit, and settles billing when a client status query observes a terminal result.

The missing piece is recovery when clients stop querying. A task can remain `held` and `processing` forever, or a stale `submitting` task can keep funds frozen if submission never reaches a persisted upstream task id.

## Proposed Behavior

Run a dedicated background runtime that periodically claims a bounded page of tasks where:

- `billing_status = 'held'`
- `status IN ('submitting', 'processing')`

The scanner uses database leases (`poll_lease_owner`, `poll_lease_until`) and `FOR UPDATE SKIP LOCKED` so multiple app instances do not process the same task at the same time.

For each claimed task:

- `submitting` without an upstream task id: keep waiting until `submitted_at` or `created_at` exceeds the submit timeout, then mark failed and release the hold.
- `submitting` or `processing` with an upstream task id: query Jimeng upstream using the task's bound account.
- Upstream processing: persist latest status/body and keep the hold.
- Upstream success: persist success and capture the hold once, then record usage without deducting balance again.
- Upstream failure, cancellation, timeout, or `refunded`: persist failure and release the hold once.
- Transient query errors: keep the task held for the next polling cycle. If task age exceeds the max processing timeout, mark failed and release.

## Service Shape

Create a focused Jimeng poller service, separate from the batch image worker:

- Repository methods claim tasks, release leases, mark poll attempts, and update timeout failures.
- Service method polls one task without depending on Gin request context.
- Runtime starts from server wiring and stops through existing cleanup.

The poller reuses existing billing idempotency IDs:

- hold: `jimeng_video_hold:<local_task_id>`
- capture: `jimeng_video_capture:<local_task_id>`
- release: `jimeng_video_release:<local_task_id>`
- accounting: `jimeng_video_accounting:<local_task_id>`

These IDs make repeated poll attempts idempotent and prevent duplicate capture, release, or usage rows.

## Configuration

Add Jimeng video poller settings with conservative defaults:

- enabled: true
- poll interval: 30 seconds
- batch size: 25
- lease TTL: 2 minutes
- upstream request timeout: 30 seconds
- submit timeout: 10 minutes
- max processing age: 2 hours

## Tests

Cover:

- claim skips tasks with an active lease
- processing responses do not settle billing
- success captures and records usage once
- failed, cancelled, timeout, and refunded statuses release the hold
- transient upstream errors keep the hold and retry later
- stale submitting tasks without upstream task id release after timeout
