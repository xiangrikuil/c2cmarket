# Journal - xiangrikuil (Part 1)

> AI development session journal
> Started: 2026-07-06

---



## Session 1: Auth Hardening Bootstrap

**Date**: 2026-07-06
**Task**: Auth Hardening Bootstrap
**Package**: backend
**Branch**: `main`

### Summary

Committed Argon2id password hashing, legacy rehash, env-driven first-admin bootstrap, migration cleanup, and backend spec updates.

### Main Changes

- Added `argon2id_v1` for new password credentials and kept `sha256_salted_v1` as legacy verification-only.
- Rehashed successful legacy password logins to Argon2id before session creation completes.
- Replaced the fixed admin password seed with explicit `C2C_BOOTSTRAP_ADMIN_USERNAME` / `C2C_BOOTSTRAP_ADMIN_PASSWORD` startup bootstrap.
- Added migration cleanup, environment examples, compose wiring, backend tests, and backend spec updates.

### Git Commits

| Hash | Message |
|------|---------|
| `af95f14` | (see git log) |

### Testing

- [OK] `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...`
- [OK] `git diff --check`
- [OK] fixed admin hash/salt literal scan returned no matches

### Status

[OK] **Completed**

### Next Steps

- Continue the parent maintenance roadmap with P0 request/proxy hardening.
