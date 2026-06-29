# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

Frontend code must be optimized for long-term maintenance. Keep UI state, routing, data fetching, and component responsibilities explicit. The main user flow should be readable without tracing through layers of defensive fallback behavior.

All frontend changes must follow [Maintainability Contract](../guides/maintainability-contract.md).

---

## Forbidden Patterns

- Broad `try/catch` blocks that replace failed requests with silent empty lists, fake success, or mock production data.
- Component-level fallback data that hides API, parsing, routing, or store failures.
- Production builds that can silently fall back to mock/demo data when `VITE_API_MODE=real` or `VITE_API_BASE_URL` is missing.
- Multiple nested compatibility branches for data shapes that the backend does not officially return.
- "Just in case" props, defaults, or watchers that are not required by current behavior.
- Large components mixing page layout, API calls, data transformation, and mutation logic.

---

## Required Patterns

- Keep page components focused on composition and workflow.
- Put reusable request logic in query/composable modules instead of duplicating it in pages.
- Surface failed operations through an explicit UI state or error path.
- Production Vite builds must fail during config loading unless a real backend is configured through `VITE_API_MODE=real` or `VITE_API_BASE_URL`; development may still use mock/demo mode intentionally.
- Prefer typed data contracts over optional chains spread across components.
- Remove obsolete UI states, feature flags, and compatibility branches when replacing behavior.

---

## Testing Requirements

- Test the normal user path for every feature.
- Test only required fallback paths; do not create tests that preserve speculative behavior.
- When a fallback is necessary, assert that the failure is visible and does not masquerade as success.

---

## Code Review Checklist

- Is the primary UI/data flow obvious?
- Are all fallback branches necessary, visible, and tested?
- Can the component be maintained without understanding unrelated pages?
- Did the change remove outdated branches instead of preserving them indefinitely?
