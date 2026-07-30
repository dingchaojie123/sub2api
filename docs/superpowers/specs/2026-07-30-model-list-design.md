# Model List Page Design

## Goal

Add a user-facing frontend page that displays Corgi model pricing by group. The page is a static display page for the pricing data provided in the request; it does not call a backend API and does not add admin editing behavior.

## Route And Navigation

- Add authenticated user route `/models`.
- Add a sidebar item labeled `模型列表` / `Model List` near the existing usage and available-channel entries.
- The route metadata should use i18n title and description keys.

## Layout

- Use the approved A layout: a two-column pricing browser.
- Header area:
  - Title: `模型与价格`.
  - Subtitle: `按你可用的分组查看模型价格。`.
  - Refresh icon button that resets the selected group to the default group because the data is static.
- Main content:
  - Left group list showing group name, provider, and model count.
  - Right pricing table for the selected group.
- Responsive behavior:
  - Desktop: left group list and right table side by side.
  - Mobile: group selector becomes horizontally scrollable above the table; the table remains horizontally scrollable.

## Pricing Data

Use static local data for these groups:

- `gpt pro`, provider `openai`, no forced multiplier badge.
- `gpt plus`, provider `openai`, no forced multiplier badge.
- `gpt-image-2生图`, provider `openai`, multiplier `2x`.
- `Claude-kiro`, provider `anthropic`, multiplier `0.3x`.
- `Claude-max-1.1`, provider `anthropic`, multiplier `2.2x`.

Each table has these columns:

- Model
- Platform input
- Platform output
- Official input
- Official output

The exact price strings should match the request text. Platform input values use green styling, platform output values use red/pink styling, and official price values use orange styling.

## Component Scope

- Create `frontend/src/views/user/ModelListView.vue`.
- Keep the static data in the view unless tests or maintainability clearly benefit from a tiny local helper inside the same file.
- Avoid adding backend APIs, stores, or new feature flags.
- Avoid changing existing lottery/sidebar changes beyond the minimum needed to insert the new navigation item.

## Tests And Verification

- Add a focused component test for `ModelListView`:
  - renders the default `gpt pro` group.
  - switches to `Claude-kiro` and shows its provider/multiplier/prices.
  - verifies table column labels and at least one key model price from the supplied data.
- Update existing route/sidebar/i18n tests if required by the new route or nav label.
- Run frontend typecheck/build and relevant tests before completion.
