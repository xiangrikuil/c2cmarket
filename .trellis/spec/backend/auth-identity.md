# OAuth Identity And Administrator Bootstrap

Date: 2026-07-26
Author: Codex

## Scenario: Immutable OAuth Ownership And Proven First-Admin Bootstrap

### 1. Scope / Trigger

- Trigger: changes to OAuth profile normalization, `users`, `auth_identities`,
  provider-specific bindings, OAuth login permissions, native administrator
  bootstrap, or migrations that affect those records.
- Owners: `internal/module/auth`, `internal/store/postgres/auth.go`,
  `internal/app/app.go`, `internal/domain/errors.go`, and migration `000062`.
- Goal: provider display handles are profile data, not account ownership proof;
  only `(provider, provider_subject)` owns an OAuth-to-local-user mapping.

### 2. Signatures

```go
auth.Service.LoginWithOAuthProfile(
    ctx context.Context,
    profile auth.OAuthProfile,
) (auth.User, auth.Session, *domain.AppError)

auth.Repository.UpsertOAuthUser(
    ctx context.Context,
    profile auth.OAuthProfile,
    now time.Time,
) (auth.OAuthUserResult, *domain.AppError)

auth.Service.BootstrapAdmin(
    ctx context.Context,
    input auth.BootstrapAdminInput,
) (auth.BootstrapAdminResult, *domain.AppError)
```

```text
OAuth identity key:
  lower(trim(provider)) + NUL + trim(provider_subject)

First collision handle:
  <bounded-base>-<sha256(provider + NUL + subject)[0:8]>

Bootstrap marker:
  admin_bootstrap_runs.bootstrap_key = "initial-admin-v1"
```

```sql
admin_bootstrap_runs(
  bootstrap_key text primary key,
  user_id uuid unique not null references users(id),
  username_snapshot text unique not null,
  created_at timestamptz not null
)
```

### 3. Contracts

- OAuth login queries `(provider, provider_subject)` before considering a
  username. An existing identity always keeps its original `user_id` and local
  `users.username`.
- Existing login may refresh display name, avatar, last-active/login timestamps,
  and provider-specific non-secret profile data. It must not update
  `auth_identities.user_id`.
- First login creates a new user even when a normal user, administrator, or
  another provider already uses the proposed handle. Handle conflicts use the
  shared deterministic candidate generator; they never reuse the conflicting
  row.
- New user, identity, and linux.do binding writes commit in one transaction.
  Identity insertion uses `ON CONFLICT ... DO NOTHING`. A concurrent loser rolls
  back its temporary user and reloads the committed winner.
- Only `provider="linux_do"` writes `linux_do_bindings`. OAuth profile data never
  grants `user_permissions`; administrator authority is independent of OAuth.
- Bootstrap is create-only. With no marker, any existing administrator or
  occupied target username returns `ADMIN_BOOTSTRAP_CONFLICT` without mutation.
- A matching marker rerun verifies the snapshot, user, active status, admin
  permission, and password credential, then returns without updating the
  password. Damaged marked state returns `ADMIN_BOOTSTRAP_INCONSISTENT`.
- User, permission, password credential, and marker creation commit in one
  transaction. Bootstrap logs may include user ID and normalized username, but
  never password, salt, hash, or credential material.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing provider, subject, or provider username | `422 VALIDATION_FAILED` |
| Proposed OAuth handle is occupied | Generate a stable alternative; do not expose the conflicting account |
| Existing identity arrives with a renamed provider username | Return the original user and local username |
| Two first logins race for one identity | One `Created=true`; all calls return the same user |
| Provider binding write fails | `INTERNAL_ERROR`; user and identity inserts roll back |
| Bootstrap target username is occupied | `ADMIN_BOOTSTRAP_CONFLICT`; no account mutation |
| Any unproven administrator exists | `ADMIN_BOOTSTRAP_CONFLICT`; no new administrator |
| Marker exists with a different requested username | `ADMIN_BOOTSTRAP_CONFLICT`; existing password unchanged |
| Marker points to missing/inactive/non-admin/no-credential state | `ADMIN_BOOTSTRAP_INCONSISTENT` |

### 5. Good / Base / Bad Cases

- Good: linux.do subject `42` first logs in as `alice`; a local `alice` already
  exists, so OAuth receives a stable suffixed handle and a separate user ID.
- Base: the same subject later reports username `alice-new`; login returns the
  original user, keeps the local handle, and refreshes display/profile fields.
- Good: a configured bootstrap restarts with the same username; the marker is
  verified and the stored password hash remains byte-for-byte unchanged.
- Bad: OAuth upserts `users` by username and then changes
  `auth_identities.user_id`; an attacker can adopt a local or admin account.
- Bad: Bootstrap reuses an existing username, adds `admin`, or upserts the
  password credential.

### 6. Tests Required

- Auth unit tests: rename stability, normal/admin collision, provider and subject
  isolation, non-linux.do binding absence, no OAuth admin grant, deterministic
  bounded handles, concurrent first login, bootstrap conflicts, provenance
  rerun, unchanged password, and damaged provenance.
- PostgreSQL integration tests: real unique constraints for collision and race,
  one committed user for concurrent identity creation, binding-failure rollback,
  create-only Bootstrap, occupied normal/OAuth username, foreign administrator,
  unchanged credential on rerun, inconsistent marker, and mid-transaction
  rollback.
- Migration tests: version 62 files, marker primary key, unique user/snapshot,
  user foreign key, rollback table removal, README entry, and
  `ExpectedMigrationVersion`.
- Full gate: `go test -count=1 ./...`, `go vet ./...`,
  `node scripts/check-migrations-doc.mjs`, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```sql
INSERT INTO users (username, ...)
VALUES ($1, ...)
ON CONFLICT (username) DO UPDATE ...;

INSERT INTO auth_identities (user_id, provider, provider_subject, ...)
VALUES ($user_id, $provider, $subject, ...)
ON CONFLICT (provider, provider_subject)
DO UPDATE SET user_id = EXCLUDED.user_id;
```

#### Correct

```sql
-- Query the immutable identity owner first.
SELECT user_id
FROM auth_identities
WHERE provider = $1 AND provider_subject = $2
FOR UPDATE;

-- First login creates a new user; a handle collision selects another candidate.
INSERT INTO users (username, ...)
VALUES ($candidate, ...)
ON CONFLICT (username) DO NOTHING
RETURNING id;

-- The identity owner is never reassigned.
INSERT INTO auth_identities (user_id, provider, provider_subject, ...)
VALUES ($new_user_id, $provider, $subject, ...)
ON CONFLICT (provider, provider_subject) DO NOTHING
RETURNING id;
```
