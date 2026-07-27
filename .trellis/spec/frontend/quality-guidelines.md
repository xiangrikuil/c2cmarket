# Quality Guidelines

> Code quality standards for frontend development.

Date: 2026-07-27

Executor: Codex

---

## Overview

Frontend code must be optimized for long-term maintenance. Keep UI state, routing, data fetching, and component responsibilities explicit. The main user flow should be readable without tracing through layers of defensive fallback behavior.

All frontend changes must follow [Maintainability Contract](../guides/maintainability-contract.md).

---

## Forbidden Patterns

- Broad `try/catch` blocks that replace failed requests with silent empty lists, fake success, or mock production data.
- Component-level fallback data that hides API, parsing, routing, or store failures.
- Production builds that can silently fall back to mock/demo data when required Nuxt runtime API configuration is missing.
- Multiple nested compatibility branches for data shapes that the backend does not officially return.
- "Just in case" props, defaults, or watchers that are not required by current behavior.
- Large components mixing page layout, API calls, data transformation, and mutation logic.

---

## Required Patterns

- Keep page components focused on composition and workflow.
- Put reusable request logic in query/composable modules instead of duplicating it in pages.
- Surface failed operations through an explicit UI state or error path.
- Production Nuxt builds must fail during config loading unless `NUXT_PUBLIC_API_MODE=real`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL` are all configured; development may still use mock/demo mode intentionally.
- Nuxt development backend proxies must match API paths narrowly. Use `/api/` or an equivalent anchored matcher for backend API routes; do not use a broad `/api` proxy key because it also captures application routes such as `/api-market/new`.
- Prefer typed data contracts over optional chains spread across components.
- Remove obsolete UI states, feature flags, and compatibility branches when replacing behavior.

---

## Testing Requirements

- Test the normal user path for every feature.
- Test only required fallback paths; do not create tests that preserve speculative behavior.
- When a fallback is necessary, assert that the failure is visible and does not masquerade as success.
- When changing the Nuxt development proxy or adding a route that starts with an API-like prefix, smoke direct deep links with `curl http://localhost:<port>/<route>` and verify they return `text/html`, while real backend paths such as `/api/v1/...` still return JSON through the proxy.

## Legacy Compatibility Disclosure

- Runtime aliases, fallback branches, and transitional data shapes must not be retained silently.
- Before retaining compatibility code, Codex must explicitly tell the user the exact alias or branch, its current consumer, the removal risk, and either a removal timeline or the concrete reason it must remain.
- Remove compatibility code by default when no supported deployed consumer exists. Do not infer a supported consumer from historical documentation alone.
- Tests and source scans must reject pre-Nuxt public runtime-variable aliases in current source, CI, environment templates, operational documentation, and active specs.
- Nuxt's supported Vite builder integration, Vitest, and their required dependencies are normal toolchain usage, not legacy compatibility code.

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
  },
  "scripts": {
    "postinstall": "nuxt prepare"
  }
}
```

CI must install pnpm 10 and Node 24.11 before running frontend checks.
`pnpm --dir frontend install --frozen-lockfile` must run the standard
`nuxt prepare` lifecycle and generate `frontend/.nuxt/tsconfig.json`.

### 3. Contracts

- `frontend/package.json` dependencies and devDependencies must not use `latest`.
- Replace `latest` with explicit ranges from the current lockfile/resolved version unless a task explicitly approves a dependency upgrade.
- Keep `frontend/pnpm-lock.yaml` importer specifiers aligned with `package.json`.
- Keep `"postinstall": "nuxt prepare"` in `frontend/package.json`. Clean
  checkouts must not rely on stale `.nuxt` output before typecheck, OpenAPI
  generation, or other commands that load `frontend/tsconfig.json`.
- Frontend production build verification must set all three Nuxt API variables documented above; do not relax the Nuxt config guard to make a bare build pass.
- README/frontend setup docs must mention pnpm, not npm, for this project.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `package.json` contains `latest` | Reject the change before commit. |
| Lockfile importer specifier differs from `package.json` | `pnpm install --frozen-lockfile` fails; update the lockfile without upgrading packages. |
| Clean install omits `nuxt prepare` | Reject the change; `.nuxt/tsconfig.json` is absent and clean OpenAPI/type commands cannot load the Nuxt TypeScript configuration. |
| Node or pnpm is outside `engines` | Local install/checks may fail fast; use the supported toolchain. |
| Production build omits any required Nuxt API variable | Nuxt config fails the build instead of producing a mock-backed artifact. |

### 5. Good/Base/Bad Cases

- Good: `@tanstack/vue-query` uses `^5.101.0` because the lockfile already resolves `5.101.0`.
- Base: CI uses `pnpm/action-setup@v4` with `version: 10`,
  `actions/setup-node@v4` with `node-version: 24.11`, and the package lifecycle
  generates Nuxt types during frozen installation.
- Bad: a maintenance task changes `"vue": "latest"` to another `latest`-like
  floating range, removes `postinstall`, or removes the real-mode build guard
  to satisfy `pnpm build`.

### 6. Tests Required

- Move existing `.nuxt` output outside the checkout, run
  `pnpm --dir frontend install --frozen-lockfile` with Node `>=24.11 <25` and
  pnpm `>=10 <11`, then assert `frontend/.nuxt/tsconfig.json` exists.
- From that clean-install boundary, run `node scripts/check-openapi-types.mjs`
  and require an exact generated snapshot match.
- `pnpm --dir frontend typecheck`.
- Real-mode Nuxt build with `NUXT_PUBLIC_API_MODE`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL` configured.
- `pnpm --dir frontend test`.
- Source scan: `rg -n '"latest"|specifier: latest' frontend/package.json frontend/pnpm-lock.yaml` must find no matches.
- Compatibility scan: `rg -n 'V[I]TE_[A-Z0-9_]+' frontend .github README.md frontend/README.md docs/ops .env.example .env.production.example .env.staging.example .trellis/spec` must find no matches.

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

#### Wrong: depend on generated state left by a previous command

```json
"scripts": {
  "build": "nuxt build"
}
```

#### Correct: materialize Nuxt TypeScript state during installation

```json
"scripts": {
  "build": "nuxt build",
  "postinstall": "nuxt prepare"
}
```
