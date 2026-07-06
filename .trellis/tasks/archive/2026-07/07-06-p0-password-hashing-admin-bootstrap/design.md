# P0 password hashing and admin bootstrap design

## Architecture and Boundaries

The change stays in the backend auth boundary:

- `internal/module/auth` owns password algorithm selection, verification, legacy rehash, and bootstrap service behavior.
- `internal/store/postgres` owns durable bootstrap and credential persistence.
- `internal/config` owns environment parsing for bootstrap variables.
- `internal/app` wires config into the core service at process startup.
- `internal/module/core` remains a thin facade for app wiring; no HTTP route or frontend contract is added.
- `backend/migrations` owns PostgreSQL contract changes for accepted password algorithms and removal of fixed credential seeding.

No OpenAPI change is required because bootstrap is a process startup behavior, not an HTTP endpoint.

## Password Contract

Algorithms:

- `argon2id_v1`: current write algorithm for new, changed, reset, and bootstrap passwords.
- `sha256_salted_v1`: legacy verification only.

Argon2id v1 parameters are fixed in code and versioned by the algorithm name:

- memory: 64 MiB
- iterations: 3
- parallelism: 1
- salt length: 16 bytes
- key length: 32 bytes

Stored format remains the existing `user_password_credentials` columns:

- `password_algorithm`
- `password_salt`
- `password_hash`

`argon2id_v1` stores hex salt and hex derived key. A future parameter change should use a new algorithm value, not reinterpret existing rows.

## Login Data Flow

1. Normalize username and trim password using the existing auth behavior.
2. Load the credential by username.
3. Verify by algorithm:
   - `argon2id_v1`: decode salt/hash and compare derived Argon2id key in constant time.
   - `sha256_salted_v1`: use the legacy hash helper and mark the credential for rehash on success.
   - unknown algorithm: reject as invalid credentials.
4. Reject inactive users before session creation.
5. Require either linux.do binding or admin status for native password login. The admin allowance exists only so the explicit first-admin bootstrap can produce a usable admin account.
6. If the credential was legacy and login is otherwise successful, write an `argon2id_v1` credential for the same user.
7. Create the session.

If rehash persistence fails, login fails rather than creating a session with an un-upgraded credential state.

## Password Set Data Flow

`SetPassword` keeps the existing current-password requirement when a credential already exists. The current credential may be either `argon2id_v1` or legacy `sha256_salted_v1`; the replacement credential is always `argon2id_v1`.

## Bootstrap Data Flow

Startup reads:

- `C2C_BOOTSTRAP_ADMIN_USERNAME`
- `C2C_BOOTSTRAP_ADMIN_PASSWORD`

If password is empty, bootstrap is skipped. If password is present and username is empty, username defaults to `admin`.

The service validates username and password, then:

1. Checks whether any user with admin permission already has a password credential.
2. If one exists, returns without writing or overwriting anything.
3. Otherwise creates or updates the requested user, grants admin permission, and writes an `argon2id_v1` credential.

PostgreSQL performs this in one transaction with table locking around the existence check to prevent concurrent startup races.

## Migration Notes

Fresh databases should create `user_password_credentials` with a check constraint accepting `argon2id_v1` and `sha256_salted_v1`, without inserting a fixed admin credential.

Existing databases need a follow-up migration that:

- relaxes the password algorithm check constraint to include `argon2id_v1`;
- removes the old seeded admin credential by matching the known seed salt and admin username, without embedding the old fixed password hash.

Rollback should restore the previous sha256-only constraint only if no Argon2id credentials remain.

## Security Notes

- Plaintext passwords never leave request/config memory except for hashing.
- Logs must mention at most that bootstrap was created or skipped, plus username; never password, salt, or hash.
- Tests should compute legacy hashes through helpers instead of embedding fixed hash literals.
