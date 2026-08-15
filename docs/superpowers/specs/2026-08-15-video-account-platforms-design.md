# Video Account Platforms Design

## Goal

Add three independent video-platform account types to the admin account-management
experience:

- `kling` displayed as K-Ling
- `happyhourse` displayed as Happy-Hourse
- `seedance` displayed as Seedance

Administrators can create and edit these accounts with an API key, a required
Base URL, and manual model configuration.

## Scope

This work is limited to account management.

- Add the three platforms to the account creation selector.
- Keep the selector as a single horizontally scrollable row.
- Support API Key account creation and editing.
- Keep Base URL empty by default and require the administrator to supply it.
- Support existing model whitelist and model-mapping controls.
- Add platform labels, icons, colors, type definitions, and account-list filters.
- Ensure the backend accepts and returns the platform values where account data is
  stored and displayed.

## Out Of Scope

- Video generation request routing.
- Upstream task creation, polling, media download, or error translation.
- Video pricing, billing, user quotas, group routing, or model auto-discovery.
- Automatic credential validation or upstream billing probes.
- The separately mentioned `pp-视频` platform.

## Architecture

### Platform Metadata

Extend the frontend platform metadata module with a distinct video-account
category. Each entry has:

- stable platform ID
- display label
- empty default Base URL
- Base URL and API Key placeholders
- API Key account type
- model-platform identifier for manual configuration

The category remains separate from `PROVIDER_PLATFORMS`. Those existing entries
are OpenAI-compatible text providers; treating the video providers as part of
that category would incorrectly enable OpenAI gateway behavior.

### Account Forms

`CreateAccountModal.vue` and `EditAccountModal.vue` consume shared metadata:

1. The platform selector renders the three video providers after existing
   providers without allowing wrapping or shrinking.
2. Selecting one switches directly to API Key credentials.
3. The Base URL starts empty, shows a platform-neutral example placeholder, and
   must be non-empty before creation.
4. The API Key is saved as the standard API Key credential payload.
5. Existing manual model whitelist and mapping controls stay available.

No upstream endpoint is called while creating or editing these accounts.

### Account Presentation

The frontend account types, icons, colors, badges, filter options, and
translations recognize all three platform IDs. This makes existing accounts
readable and filterable after a page refresh.

The backend platform constants and account-facing validation/display paths
recognize the IDs as account platforms only. They are intentionally excluded
from OpenAI-compatible platform predicates, gateway route dispatch, group
routing, quota platform lists, and video-pricing capability checks.

## Data Flow

1. An administrator selects K-Ling, Happy-Hourse, or Seedance.
2. The form uses the platform metadata to select API Key mode and empty Base
   URL defaults.
3. The administrator provides account name, Base URL, and API Key, and may
   configure model restrictions or mappings.
4. The existing admin account API persists `platform`, `type`, `credentials`,
   and model configuration.
5. Account list and edit views render the saved platform through centralized
   metadata, icon, color, and translation helpers.

## Validation And Errors

- Creation rejects a blank account name, Base URL, or API Key using the form's
  existing validation behavior.
- Backend errors continue to use the existing account API error presentation.
- Because there is no connection probe in scope, a syntactically valid
  Base URL/API Key pair can be saved even if the upstream later rejects it.

## Testing

- Platform metadata tests cover IDs, labels, and empty Base URL defaults.
- Create-account modal tests verify selector presence, horizontal scrolling,
  API Key-only selection, required Base URL, and submitted account payloads.
- Edit-account tests verify platform-specific placeholders and retained model
  configuration behavior.
- Type checking and the focused frontend test suite verify UI integrations.

