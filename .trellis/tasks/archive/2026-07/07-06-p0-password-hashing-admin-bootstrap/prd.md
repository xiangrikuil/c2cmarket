# P0 password hashing and admin bootstrap

## Goal

Upgrade password storage to Argon2id with legacy sha256 verification/rehash and replace fixed admin password initialization with a safe bootstrap path.

## Confirmed Facts

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 1.1.
- Current auth code stores password credentials in `user_password_credentials` with `sha256_salted_v1`.
- `auth.Service.LoginWithPassword` currently accepts only `sha256_salted_v1`.
- `auth.Service.SetPassword` currently creates new `sha256_salted_v1` credentials.
- Migration `000025_native_admin_login` currently creates the credential table and seeds a fixed admin password credential.
- The backend has only an API process entrypoint (`backend/cmd/api/main.go`), not a CLI framework.
- Production startup already reads environment config through `backend/internal/config` and composes dependencies through `backend/internal/app`.

## Requirements

- Add password algorithm `argon2id_v1` for all new password credentials.
- Keep `sha256_salted_v1` verification only as a legacy compatibility path.
- On successful login with a legacy `sha256_salted_v1` credential, automatically rehash and persist the credential as `argon2id_v1` before completing login.
- New and reset password writes must use `argon2id_v1`; application code must not create new `sha256_salted_v1` credentials.
- Wrong passwords must fail without creating sessions or rehashing credentials.
- Remove fixed admin password credential seeding from migration source.
- Add an environment-based first-admin bootstrap path:
  - `C2C_BOOTSTRAP_ADMIN_USERNAME`
  - `C2C_BOOTSTRAP_ADMIN_PASSWORD`
- Bootstrap must create or promote the requested admin only when no admin password credential already exists.
- Re-running bootstrap after an admin password credential exists must not overwrite the existing admin credential.
- Bootstrap and password handling must not log plaintext passwords, password hashes, salts, session tokens, or credential-looking values.
- PostgreSQL schema must accept both `argon2id_v1` and legacy `sha256_salted_v1` rows so old users can log in and migrate.
- Existing native password behavior for linux.do-bound users must remain intact. The bootstrap admin is the only intended native-password path for an unbound first admin.

## Acceptance Criteria

- [x] Argon2id password credentials can log in and create sessions.
- [x] Legacy `sha256_salted_v1` credentials can log in and are upgraded to `argon2id_v1`.
- [x] Wrong passwords fail with `INVALID_CREDENTIALS` and do not create sessions or rehash.
- [x] Setting or changing a password writes `argon2id_v1`, never `sha256_salted_v1`.
- [x] Environment bootstrap creates/promotes the first admin with an Argon2id credential.
- [x] Re-running environment bootstrap does not overwrite an existing admin password credential.
- [x] Migration source no longer contains the fixed admin password hash.
- [x] `go test ./...` passes from `backend/`, or the local blocker is recorded if the Go toolchain/Docker remains unavailable.

## Notes

- Environment bootstrap is selected instead of a CLI because this repository has no command framework today; using config/app/auth/store keeps the change narrow.
- Legacy sha256 support is for migration only and should not appear in new write paths.
