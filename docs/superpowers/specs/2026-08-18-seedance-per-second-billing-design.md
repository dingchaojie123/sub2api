# Seedance Per-Second Billing Design

## Goal

Make Seedance use the same site billing model as Kling and Happy Horse: charge
by generated video duration rather than by upstream Token consumption.

## Billing Rules

- Cost is `actual output seconds * video count * per-second unit price`.
- The initial balance hold uses requested duration and video count.
- Final settlement uses the completed task's actual output duration when
  available, matching the current PP video settlement behavior.
- The unit price is read from the API key's group video pricing:
  - `480p` uses `video_price_480p`
  - `720p` uses `video_price_720p`
  - `1080p` uses `video_price_1080p`
  - `4K` uses `video_price_1080p`
- Existing group, video-rate, and account multipliers continue to apply.

## Configuration

- Seedance shows the existing video price card in the admin group create and
  edit dialogs.
- Seedance no longer requires a channel model Token price for request
  admission, holding, or settlement.
- Existing Seedance Token pricing records are preserved for historical
  integrity, but the PP video billing path ignores them.
- New Seedance channel pricing entries are not restricted to Token-only mode;
  the PP video runtime does not use channel model pricing for per-second
  billing.

## Compatibility

- Public video endpoints, request fields, task status responses, and model
  synchronization remain unchanged.
- Downstream clients do not need to modify their integration.
- `input_video_duration`, output dimensions, and frame rate may still be
  forwarded to an upstream model, but do not affect site billing.

## Documentation And Verification

- Update the public PP video API document to describe the uniform per-second
  billing behavior.
- Replace Seedance Token-formula tests with group video-price tests.
- Cover 480p, 720p, 1080p, and 4K-to-1080p mapping, requested-duration holds,
  actual-duration settlement, and the absence of a channel Token-price
  requirement.
