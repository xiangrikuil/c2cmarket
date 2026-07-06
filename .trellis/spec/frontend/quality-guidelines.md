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
- Vite dev-server backend proxies must match API paths narrowly. Use `/api/` or an equivalent anchored matcher for backend API routes; do not use a broad `/api` proxy key because it also captures SPA routes such as `/api-market/new`.
- Prefer typed data contracts over optional chains spread across components.
- Remove obsolete UI states, feature flags, and compatibility branches when replacing behavior.

---

## Testing Requirements

- Test the normal user path for every feature.
- Test only required fallback paths; do not create tests that preserve speculative behavior.
- When a fallback is necessary, assert that the failure is visible and does not masquerade as success.
- When changing Vite proxy config or adding a route that starts with an API-like prefix, smoke direct deep links with `curl http://localhost:<port>/<route>` and verify they return `text/html`, while real backend paths such as `/api/v1/...` still return JSON through the proxy.

---

## Code Review Checklist

- Is the primary UI/data flow obvious?
- Are all fallback branches necessary, visible, and tested?
- Can the component be maintained without understanding unrelated pages?
- Did the change remove outdated branches instead of preserving them indefinitely?

## Scenario: Frontend Dependency And Toolchain Pinning

### 1. Scope / Trigger

- Trigger: changes to `frontend/package.json`, `frontend/pnpm-lock.yaml`, frontend CI setup, or frontend local-development documentation.
- Goal: dependency installs must be repeatable and production builds must keep the real-backend guard.

### 2. Signatures

```json
{
  "engines": {
    "node": ">=24.11 <25",
    "pnpm": ">=10 <11"
  }
}
```

CI must install pnpm 10 and Node 24.11 before running frontend checks.

### 3. Contracts

- `frontend/package.json` dependencies and devDependencies must not use `latest`.
- Replace `latest` with explicit ranges from the current lockfile/resolved version unless a task explicitly approves a dependency upgrade.
- Keep `frontend/pnpm-lock.yaml` importer specifiers aligned with `package.json`.
- Frontend production build verification must set `VITE_API_MODE=real` or `VITE_API_BASE_URL`; do not relax the Vite config guard to make a bare build pass.
- README/frontend setup docs must mention pnpm, not npm, for this project.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `package.json` contains `latest` | Reject the change before commit. |
| Lockfile importer specifier differs from `package.json` | `pnpm install --frozen-lockfile` fails; update the lockfile without upgrading packages. |
| Node or pnpm is outside `engines` | Local install/checks may fail fast; use the supported toolchain. |
| Production build omits real backend mode | Vite config fails the build instead of producing a mock-backed artifact. |

### 5. Good/Base/Bad Cases

- Good: `@tanstack/vue-query` uses `^5.101.0` because the lockfile already resolves `5.101.0`.
- Base: CI uses `pnpm/action-setup@v4` with `version: 10` and `actions/setup-node@v4` with `node-version: 24.11`.
- Bad: a maintenance task changes `"vue": "latest"` to another `latest`-like floating range, or removes the real-mode build guard to satisfy `pnpm build`.

### 6. Tests Required

- `pnpm --dir frontend install --frozen-lockfile` with Node `>=24.11 <25` and pnpm `>=10 <11`.
- `pnpm --dir frontend typecheck`.
- `VITE_API_MODE=real pnpm --dir frontend build`.
- `pnpm --dir frontend test`.
- Source scan: `rg -n '"latest"|specifier: latest' frontend/package.json frontend/pnpm-lock.yaml` must find no matches.

### 7. Wrong vs Correct

#### Wrong

```json
"dependencies": {
  "vue": "latest"
}
```

#### Correct

```json
"dependencies": {
  "vue": "^3.5.38"
}
```
