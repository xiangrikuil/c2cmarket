# Implementation Plan

Implementation baseline: `origin/staging` at
`a2d65e931346fce7417c5fb7122bda32ce1c5a16`.

## Ordered Checklist

1. Load the applicable frontend, marketplace UI, quota-offer, component,
   maintainability, and product-context specs through `trellis-before-dev`.
2. Add a tested publish-mode query normalizer for
   `free | package | limited | null`, including legacy `after=quota`.
3. Expand the shared publish mode type and rebuild
   `SellingModeSelector.vue` as the dedicated responsive three-choice surface.
4. Split fixed-package configuration from the nested billing selector, keeping
   the existing package list, stock, model, price, and duration behavior.
5. Update `ApiServicePublishPage.vue`:
   - render chooser when no mode is selected;
   - synchronize selected mode with the route query;
   - map free/package modes to the correct billing mode;
   - remove the in-form nested mode selectors;
   - preserve limited and `after=quota` continuation behavior;
   - expose a guarded return to mode selection.
6. Update user-facing copy and the quota-offer frontend specification so the
   three names and entry behavior are authoritative.
7. Update source/unit regressions for all modes, URL restoration, legacy
   compatibility, and removal of the nested billing choice.
8. Run focused tests, full Vitest, Nuxt typecheck, real-backend build, and
   responsive browser QA at `1440x900` and `390x844`.
9. Run `trellis-check`, inspect the final diff, commit on
   `codex/api-publish-three-modes`, and keep the branch unpushed unless asked.

## Risky Files And Rollback Points

- `frontend/src/pages/ApiServicePublishPage.vue`: route/form state and workflow
  step transitions. Verify every mode before continuing.
- `frontend/src/components/api-service-publish/SellingModeSelector.vue`: the
  first-screen interaction and responsive layout.
- Fixed-package editor component: preserve existing package mutations exactly
  while removing only the nested billing radio group.
- `.trellis/spec/frontend/api-quota-offers.md`: reconcile old two-mode and
  `after=quota` wording without changing backend quota semantics.

## Validation Commands

```bash
pnpm --dir frontend test
pnpm --dir frontend typecheck
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
git diff --check
```

Browser acceptance:

- `/api-market/new` at `1440x900` and `390x844`: three choices, no editor,
  no horizontal overflow.
- `?mode=free`: free price/inventory and buyer preview.
- `?mode=package`: fixed package editor and buyer preview.
- `?mode=limited` plus legacy `?after=quota`: limited base-service workflow.

## Completion Record

- Implemented all three URL-backed publish modes and the chooser-only generic
  entry.
- Preserved contextual limited-quota entry and legacy `after=quota`
  compatibility.
- Passed all 53 Vitest files / 222 tests, Nuxt typecheck, the real-backend Nuxt
  production build, and `git diff --check`.
- Verified the chooser and all deep links in a browser at `1440x900` and
  `390x844`, including query preservation, dirty-form confirmation, and no
  horizontal overflow.
