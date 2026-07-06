# P0 password hashing and admin bootstrap implementation plan

## Checklist

- [x] Add auth model/input types for bootstrap as needed.
- [x] Add `argon2id_v1` password hashing and verification helpers in `internal/module/auth`.
- [x] Replace new password writes in `SetPassword` with Argon2id.
- [x] Update `LoginWithPassword` to verify both algorithms and rehash legacy credentials on successful login.
- [x] Add auth service bootstrap behavior for memory and repository-backed modes.
- [x] Extend the auth repository interface and PostgreSQL store with transactional first-admin bootstrap.
- [x] Add config fields for `C2C_BOOTSTRAP_ADMIN_USERNAME` and `C2C_BOOTSTRAP_ADMIN_PASSWORD`.
- [x] Wire bootstrap in `internal/app` startup without logging secrets.
- [x] Update migration `000025` to stop seeding a fixed admin password and allow `argon2id_v1`.
- [x] Add a new migration for existing databases to allow `argon2id_v1` and remove the old fixed admin seed credential without embedding the old fixed hash.
- [x] Update backend tests for:
  - Argon2id login success;
  - legacy sha256 login plus automatic rehash;
  - wrong password fail without rehash/session;
  - SetPassword writes Argon2id;
  - bootstrap creates first admin;
  - bootstrap does not overwrite an existing admin credential.
- [x] Update `backend/go.mod` / `backend/go.sum` for `golang.org/x/crypto/argon2`.

## Validation

Primary validation:

```bash
cd backend && go test ./...
```

Repository-level validation:

```bash
rg -n "<old fixed admin hash>|<old fixed admin salt>" backend
rg -n "Algorithm: PasswordAlgorithmSHA256SaltedV1|PasswordAlgorithmSHA256SaltedV1," backend/internal/module/auth
git diff --check
```

If local `go` remains unavailable, try Docker with a Go image already present or record the exact blocker. Do not claim backend tests passed unless they actually run successfully.

## Validation Results

- `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine gofmt -w ...`: passed.
- `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...`: passed.
- `git diff --check`: passed.
- Fixed admin hash/salt literal scan across `backend`: no matches.

## Risk Points

- `auth.Repository` is used by the PostgreSQL store and test fake only; update both together.
- Bootstrap must be no-op after an admin password credential exists.
- Legacy rehash must happen only after password verification and account eligibility checks pass.
- The migration must not keep the old fixed admin password hash in source.
- The core service should stay a facade; avoid adding HTTP/server surface for bootstrap.

## Rollback Points

- Auth helper changes are localized to `backend/internal/module/auth/service.go` and tests.
- PostgreSQL bootstrap behavior is localized to `backend/internal/store/postgres/auth.go`.
- Startup wiring is localized to `backend/internal/config/config.go` and `backend/internal/app/app.go`.
- Migration changes can be inspected independently before running database migrations.
