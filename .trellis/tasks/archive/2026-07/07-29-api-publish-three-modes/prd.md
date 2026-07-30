# 发布 API 额度三模式选择

## Goal

Make sellers choose the buyer-facing API quota sales mode before entering the
publish form, so the form never presents nested or conflicting sales-mode
choices.

## Requirements

- The generic `/api-market/new` entry must initially show exactly three peer
  sales modes with no preselected default:
  - `自由额度`: the buyer enters a CNY amount and receives the corresponding USD
    allowance at the seller's `¥ / $1` rate.
  - `固定额度包`: the seller defines fixed package price, allowance, stock, model
    scope, and delivery-relative validity.
  - `限时额度包`: the seller defines or reuses a base service, then configures
    fixed allowance, total price, inventory, release timing, and absolute
    expiry in the existing limited-quota workflow.
- Mode selection must be a dedicated first screen. The progressive publish form
  and buyer preview must not render until a mode is selected.
- The three choices must be peer options. `固定额度包` must not remain nested
  under `自由额度`, and the UI must stop using `限时流量包` for this mode.
- The selected mode must be represented in the URL so refresh, deep linking,
  and browser history preserve it.
- `free` must configure `metered_credit`; `package` must configure
  `fixed_package`; `limited` must preserve the existing limited base-service
  continuation and quota-offer workflow.
- A seller may return to the mode chooser before publication. Existing unsaved
  changes protection must remain authoritative when form data is dirty.
- A published product cannot be converted to another sales mode through this
  publish flow; the seller creates a new product instead.
- Distribution system, model provider/category, models, delivery, payment, and
  merchant identity remain downstream form decisions, not sales modes.
- Existing account-recovery return targets and the legacy
  `/api-market/new?after=quota` continuation must remain usable.
- The mode screen must use existing shadcn-vue primitives, Lucide icons, brand
  button treatment, and the marketplace visual hierarchy. It must be responsive
  without oversized cards or page-level horizontal overflow.

## Acceptance Criteria

- [x] `/api-market/new` shows three unselected peer choices and no publish
      stepper/form/preview.
- [x] Selecting `自由额度` enters the free-amount workflow and shows only
      free-amount pricing and inventory fields.
- [x] Selecting `固定额度包` enters the fixed-package workflow and shows package
      configuration without a second billing-mode selector.
- [x] Selecting `限时额度包` enters the existing limited base-service workflow
      and continues to the existing quota-offer configuration route.
- [x] Refreshing a selected mode restores the same workflow from the URL;
      invalid mode values fall back to the chooser.
- [x] The form header exposes a clear way to return to the chooser, subject to
      the existing unsaved-changes guard.
- [x] User-facing copy consistently uses `自由额度`, `固定额度包`, and
      `限时额度包`; the touched publish surface no longer says `限时流量包`.
- [x] Existing `after=quota` navigation, free publication, fixed-package
      publication, and limited continuation tests remain green.
- [x] Source/unit regressions cover query normalization, all three choices,
      mode-to-billing mapping, and removal of the nested billing selector.
- [x] Full Vitest, Nuxt typecheck, real-backend Nuxt build, and browser checks
      at `1440x900` and `390x844` pass.

## Out Of Scope

- Backend schema or API changes.
- Changing quota pricing, stock, reservation, payment, delivery, or expiry
  semantics.
- Redesigning the existing limited-quota package form after its mode is chosen.
- Converting already-published products between modes.

## Confirmed Entry Policy

- Should contextual actions that already say `发布限时额度包` continue to enter
  the limited workflow directly, while only the generic `发布 API 服务` action
  shows the three-mode chooser? Yes. The contextual action already captures the
  seller's intent and showing the chooser again would add a redundant decision.
