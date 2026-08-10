# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Backend code must be optimized for long-term maintenance. Handlers, services, repositories, and domain code should expose clear contracts and fail explicitly when required data or dependencies are unavailable.

All backend changes must follow [Maintainability Contract](../guides/maintainability-contract.md).

---

## Forbidden Patterns

- Broad error handling that converts failures into empty success responses.
- Silent default values for required request fields, configuration, or persistence results.
- Compatibility branches for API versions or payload shapes that are not explicitly supported.
- Repositories or services returning mock data after a real dependency fails.
- Handler logic that mixes transport parsing, business rules, persistence, and response formatting in one large function.

---

## Required Patterns

- Validate required inputs at the boundary and return clear errors.
- Keep business logic outside HTTP handlers when the behavior grows beyond simple routing.
- Return typed domain errors or explicit status decisions instead of ambiguous booleans.
- Log operational failures at the layer that has enough context to explain them.
- Delete obsolete fallback branches when replacing data contracts or integrations.
- Treat `backend/go.mod` as the source of truth for the Go toolchain version. Dockerfile base images, CI setup, README prerequisites, and Docker-based local verification must stay aligned with that version.

---

## Testing Requirements

- Test the normal successful path.
- Test required error paths for boundary validation and dependency failures.
- Do not add tests for speculative fallback behavior unless the fallback is part of a documented contract.

---

## Code Review Checklist

- Does the code fail explicitly when required data is missing?
- Are errors returned or logged where they can be acted on?
- Are fallback branches tied to a documented requirement and covered by focused tests?
- Does the package boundary make future changes cheaper, not harder?

## Scenario: Repeatable Full Smoke Suite

### 1. Scope / Trigger

- Trigger: adding or changing `scripts/*-smoke.mjs`, `scripts/run-smokes.mjs`, or an API/state transition exercised by smoke automation.

### 2. Signatures

```text
node scripts/run-smokes.mjs
API_BASE_URL=http://127.0.0.1:8080
FRONTEND_ORIGIN=http://127.0.0.1:5173
SMOKE_RUN_ID=<optional run-scoped identifier>
```

The runner executes exactly the maintained smoke script list and returns a non-zero process status only after every script has run when one or more scripts fail.

### 3. Contracts

- Each run derives one unique suffix and uses it consistently for usernames, slugs, titles, emails/contact values, and idempotency keys that have uniqueness constraints.
- Smoke scripts create the smallest legal state-machine prerequisites through supported APIs or the documented local-only fixture helper.
- Local fixture helpers may seed owner-scoped records only when the public API intentionally cannot create the required verified state. They must not relax production authorization, validation, or outbound HTTP policy.
- Assertions track the current OpenAPI fields and domain states. Do not preserve obsolete request fields, redirects, or status names as compatibility expectations.
- The runner records script name, duration, exit code, and failure detail, prints one aggregate summary, and never stops after the first failed script.
- The suite must be repeatable against the same development PostgreSQL database without fixed fixture collisions.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| All scripts pass | Aggregate reports every script as passed; process exits 0. |
| One script fails | Remaining scripts still run; aggregate identifies the failure; process exits non-zero. |
| Existing rows from a prior run remain | New run uses distinct identifiers and still passes. |
| A request contract changes | Update the smoke payload/assertion in the same change. |
| Verified external-probe prerequisite is required locally | Seed an owner-scoped verified record without sending an uncontrolled external request. |

### 5. Good / Base / Bad Cases

- Good: two consecutive full runs execute all 11 scripts and both report 11/11 passed.
- Base: one intentional fixture error fails one script, while the runner still executes and reports the other ten.
- Bad: the runner uses `execFileSync` without failure collection and exits at the first error, or scripts reuse fixed usernames and depend on a clean database.

### 6. Tests Required

- Unit/source test the runner's all-script aggregation and non-zero final result when any child fails.
- Run the complete suite twice against the same local backend/database.
- Keep OpenAPI route/type, migration documentation, and Compose exposure guards in the final quality gate.
- Record every failed attempt and its resolved contract drift in the task verification artifacts.

### 7. Wrong vs Correct

#### Wrong

```js
for (const script of scripts) execFileSync(node, [script], { stdio: 'inherit' })
```

#### Correct

```js
const results = []
for (const script of scripts) results.push(await runSmoke(script))
printSummary(results)
if (results.some(result => !result.ok)) process.exitCode = 1
```
