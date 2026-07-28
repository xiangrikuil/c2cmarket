# Technical Design

Implementation baseline: `origin/staging` at
`a2d65e931346fce7417c5fb7122bda32ce1c5a16`.

## Architecture And Boundaries

The change stays in the frontend publish workflow. `ApiServicePublishPage.vue`
remains the generic `/api-market/new` route and gains an explicit
`SellingMode | null` state. A null mode renders only the dedicated mode
selection surface. A selected mode renders the existing progressive editor and
preview. No backend payload contract changes: the selected mode drives the
existing `billingMode` and navigation behavior before submission.

`SellingMode` expands from `free | limited` to
`free | package | limited`. Query normalization will accept those three values,
map legacy `after=quota` to `limited`, and return null for absent or invalid
values. The route query is the durable source for refresh/deep-link behavior;
component state follows it through a watcher. Returning to the chooser removes
the mode query while preserving the existing unsaved-changes guard.

## UI Composition

`SellingModeSelector.vue` becomes the standalone first screen and renders three
equal peer choices. Each choice contains a Lucide icon, literal mode name, short
buyer behavior, and the minimum distinguishing facts. Cards remain restrained
in size and use subtle semantic accents; primary actions keep the project brand
button treatment. Desktop uses three columns and mobile uses one column.

The progressive form no longer renders `SellingModeSelector` inside step one.
Free mode renders `PriceInventorySection`. Package mode renders a focused fixed
package editor extracted from the current `BillingModeSection`; the nested
radio group is removed. Limited mode preserves its completed sales-mode step
and existing base-service continuation.

## Data Flow And Compatibility

Mode mapping:

```text
mode=free     -> sellingMode=free    -> billingMode=metered_credit
mode=package  -> sellingMode=package -> billingMode=fixed_package
mode=limited  -> sellingMode=limited -> base service -> quota workflow
after=quota   -> sellingMode=limited -> legacy continuation preserved
```

Changing mode resets step completion and mode-specific validation errors, then
applies the correct billing defaults. Fixed-package initialization creates one
package only when none exists. Existing form data is not silently converted
between modes.

The recommended navigation policy is intent-aware: generic
`发布 API 服务` opens the chooser; contextual `发布限时额度包` actions keep their
direct limited-workflow route. This avoids forcing a seller to repeat an
already-explicit choice.

## Risks And Rollback

The main regression risk is stale mode state after browser navigation or legacy
`after=quota` entry. Query normalization and route-watcher tests cover it.
Another risk is package stock/form behavior moving out of the billing selector;
the package editor keeps the existing package mutations and validation intact.

Rollback is frontend-only: revert the selector, page branching, tests, and spec
sync. No stored data or backend migration is involved.
