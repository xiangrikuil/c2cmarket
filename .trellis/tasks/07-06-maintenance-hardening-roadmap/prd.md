# Maintenance hardening roadmap

## Goal

Parent task for completing the staged C2CMarket long-term maintenance hardening plan from the reviewed Codex maintenance prompt.

## Requirements

- Complete the reviewed maintenance hardening roadmap without a broad, single-shot rewrite.
- Preserve existing business capabilities and public API compatibility unless a child task explicitly updates backend routes, OpenAPI, frontend adapters, smoke scripts, and tests together.
- Split the roadmap into independently verifiable child tasks. Each child owns its acceptance criteria, implementation plan, validation commands, and rollback boundary.
- Prioritize production safety first: verification baseline, password hashing/admin bootstrap, strict JSON parsing, trusted proxy/rate-limit behavior, readiness/shutdown/logging, dependency pinning, and real-only frontend mode.
- Continue later maintenance tasks after the P0 work: frontend API/mock isolation, backend service boundary reduction, database-level pagination, search index/query alignment, documentation drift checks, source packaging, and frontend critical-path tests.
- Do not expand `backend/internal/server.Service` or `backend/internal/module/core.Service` for new behavior unless a child task explicitly documents why no narrower interface is feasible.
- Do not introduce production mock fallback, credential custody, payment/escrow/guarantee semantics, or plaintext/contact/secret logging.

## Acceptance Criteria

- [x] Child task exists and passes for Phase 0 safety net: CI, frontend tests, and OpenAPI/routes guard.
- [x] Child task exists and passes for P0 auth hardening: Argon2id, legacy sha256 verification/rehash, no fixed admin password/hash bootstrap, and tests.
- [x] Child task exists and passes for P0 request/proxy hardening: strict JSON body handling and trusted proxy/rate-limit behavior.
- [x] Child task exists and passes for runtime readiness: expected migration version, graceful shutdown, request ID/logging baseline.
- [x] Child task exists and passes for dependency/toolchain hardening: no `latest` frontend dependencies, engines declared, toolchain docs/CI aligned.
- [x] Child task exists and passes for frontend API/mock isolation with at least one migrated domain slice and no production mock fallback.
- [x] Child task exists and passes for backend service boundary cleanup without growing giant interfaces.
- [x] Child task exists and passes for database-level pagination on the prioritized list endpoints.
- [x] Child task exists and passes for search trigram index/query alignment and verification docs/script.
- [x] Child task exists and passes for migration/docs drift checks, source packaging, final maintenance report, and high-value frontend tests.
- [x] Final verification covers backend tests, frontend typecheck/build/test, CI scripts, documentation checks, and any smoke/manual checks specified by child tasks.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`.
- This parent is a coordination task. Direct implementation should happen in child tasks unless the work is purely roadmap integration.
