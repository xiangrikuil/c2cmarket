# Contact Usage Scopes And Safe Configuration Audit

Date: 2026-08-12
Author: Codex
Updated: 2026-08-18

## Scenario: Capability-Bound Contact Purposes

### 1. Scope / Trigger

- Trigger: changing contact-method CRUD, merchant/carpool contact selection, linux.do contact projection, contact configuration events, OpenAPI contact DTOs, or frontend contact forms.
- Owners: `internal/module/contact`, `internal/store/postgres/contact.go`, server/core contact boundaries, migration `000093`, OpenAPI contact schemas, and frontend profile/publish adapters.
- Goal: make a contact method's intended transaction purpose durable and enforceable without exposing its value in operation audit.

### 2. Signatures

```go
const (
    UsageScopeCarpoolOwner = "carpool_owner"
    UsageScopeAPIMerchant  = "api_merchant"
    UsageScopeBuyer        = "buyer"
    UsageScopeDispute      = "dispute"
)

type ContactMethodInput struct {
    UserID      string
    Type        string
    Label       string
    Value       string
    UsageScopes []string
    IsDefault   bool
    Enabled     bool
    RequestID   string
}
```

```yaml
ContactUsageScope: carpool_owner | api_merchant | buyer | dispute
ContactMethodRequest.usageScopes: required non-empty unique array
ContactMethod.usageScopes: required canonical array
```

```sql
contact_methods.usage_scopes text[] NOT NULL
CHECK (usage_scopes <@ ARRAY['carpool_owner','api_merchant','buyer','dispute'])
CHECK (type <> 'wechat' OR usage_scopes = ARRAY['carpool_owner','api_merchant','buyer','dispute'])
```

### 3. Contracts

- PostgreSQL persists the exact canonical scope set and returns it on every contact read. Canonical order is `carpool_owner`, `api_merchant`, `buyer`, `dispute`; duplicate input values collapse to one value. Enabled WeChat is the explicit server-owned exception and always persists all four scopes.
- Legacy rows migrate with all four scopes so existing transaction references keep working. New non-WeChat backend callers that omit scopes use only `buyer` plus `dispute`; the public OpenAPI create request remains structurally non-empty, while WeChat ignores the submitted selection and uses the single-mapping contract below.
- A non-WeChat update that omits `usageScopes` preserves the stored scopes. An explicit empty or unknown set is invalid. A WeChat update always restores all four scopes, and the database constraint prevents narrowing; no handler may hard-code wider scopes for an ordinary contact than the row stores.
- Adding or retaining `api_merchant` on a non-WeChat method requires `api_service.publish`; adding or retaining `carpool_owner` requires `carpool.publish`. WeChat bypasses this request-purpose check because its scopes are server-owned and do not grant publish capability; publish endpoints still enforce the real capability independently.
- Scope capability checks happen before idempotency acquisition and are repeated at the reusable service boundary with freshly loaded identity facts.
- API-service/quota and carpool transaction writes accept only the actor's enabled WeChat with the required stored scope. Ownership, type, current-version, and transaction-scope checks apply together; the WeChat-specific single-snapshot rules are defined below.
- The system-managed linux.do bridge is projected from the immutable binding with all four scopes and cannot be created, converted, edited, or deleted through public contact CRUD.
- WeChat is required after registration. Once bound, public CRUD may modify only its value and metadata; it cannot disable, convert, or delete the record. The frontend must not expose a WeChat scope selector or removal action.
- Contact values stay in versioned private storage and transaction snapshots. Configuration audit events expose only the action, actor/request IDs, aggregate version, and changed field names; they never contain the value, masked value, fingerprint, email, or provider handle.
- Contact create/update/default/verify/delete mutation, configuration event, and idempotency completion use one repository transaction. Actual disclosure reads remain separate dedicated access-log facts.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/empty/unknown explicit scope array | `422 VALIDATION_FAILED` on `usageScopes`; no mutation/event |
| Student requests `api_merchant` or `carpool_owner` | `403 CAPABILITY_REQUIRED` before idempotency or storage |
| Student creates or updates WeChat with any valid scope set | Persist all four canonical scopes; publish capability remains unchanged |
| Existing WeChat is disabled, converted, deleted, or narrowed in SQL | `409 INVALID_STATE_TRANSITION` for CRUD; database constraint rejects scope narrowing |
| Merchant/carpool selects a contact lacking its required scope | `422 VALIDATION_FAILED` on the contact field |
| Contact is disabled, foreign, missing, or wrong type | Existing stable not-found/validation result; do not reveal another user's row |
| Public CRUD targets the system linux.do method | `409 INVALID_STATE_TRANSITION` |
| Same completed create/update command is replayed | Return stored completion; do not create a new version or event |
| Event serialization/completion fails | Roll back contact mutation, version, default changes, and event |

### 5. Good / Base / Bad Cases

- Good: a student stores an email for `buyer` and `dispute`, binds one WeChat with all four server-owned scopes, and still cannot perform seller actions.
- Good: a linux.do-bound seller configures one enabled WeChat; API publication separately validates seller capability and freezes only that WeChat version.
- Base: a migrated existing non-WeChat contact has all four scopes and remains usable until its owner narrows the set explicitly; migrated WeChat remains fixed to all four.
- Bad: accept `usageScopes` in JSON, drop it before the service, return hard-coded scopes for an ordinary contact, or trust a client-provided subset for WeChat.
- Bad: treat owning a contact as enough for every transaction purpose or infer a scope from its type/label.
- Bad: copy contact values, masked values, fingerprints, or request bodies into `domain_events` or the unified administrator log.

### 6. Tests Required

- Unit tests assert canonical ordering/deduplication, omitted-create defaults, omitted-update preservation, explicit-empty/unknown rejection, clone safety, and linux.do all-scope projection.
- Handler/service tests assert student seller-scope denial for ordinary contacts, student WeChat all-scope normalization, valid buyer/dispute round trips, truthful response scopes, and no direct-service bypass.
- PostgreSQL integration tests assert migration defaults, array constraints, create/update/read round trips, scope-aware merchant/carpool selection, and rollback/replay counts for mutation/version/event/completion.
- Audit tests assert one safe configuration action per successful mutation and scan serialized output for contact values, emails, fingerprints, credentials, and arbitrary metadata.
- OpenAPI/generated/frontend tests assert the exact four-value union, capability-aware non-WeChat scope options, and no scope selector for WeChat; production real mode must not fall back to mock contact data.

### 7. Wrong vs Correct

#### Wrong

```go
response.UsageScopes = []string{"carpool_owner", "api_merchant", "buyer", "dispute"}
```

This lies about persisted intent for an ordinary contact and can let a later workflow treat a buyer-only method as a seller contact. WeChat may return all four only because the service and database persist that exact server-owned set.

#### Correct

```go
response.UsageScopes = append([]string(nil), method.UsageScopes...)
```

Normalize and authorize the request before mutation, persist the canonical array, validate the selected workflow scope again, and project exactly the stored value.

## Scenario: Single WeChat Transaction Contact

### 1. Scope / Trigger

- Trigger: changing WeChat contact CRUD, authenticated onboarding, carpool publish/apply, API service/quota publish or purchase, transaction contact snapshots, or migration `000115`.
- Goal: every in-scope actor uses one account-wide enabled WeChat while linux.do remains an identity/trust signal only.

### 2. Signatures

```go
const MethodTypeWechat = "wechat"

func AllUsageScopes() []string
func (*contact.Service) WechatVersionForOwnerAndScope(methodID, ownerID, requiredScope string) (ContactMethod, ContactMethodVersion, bool)
func WechatRequiredError(field, detail string) *domain.AppError
```

```sql
CREATE UNIQUE INDEX ux_contact_methods_one_enabled_wechat
ON contact_methods(user_id)
WHERE type = 'wechat' AND enabled = true;
```

```text
AppShell query key: ['my-contact-methods', authenticatedUserId]
Session dismissal key: c2cmarket.wechat-onboarding-dismissed.v1:<userId>
```

### 3. Contracts

- An account may have at most one enabled `wechat` method. Create/update writes normalize enabled WeChat to `AllUsageScopes()` regardless of submitted scopes; the account UI does not render WeChat scope controls.
- Automatic WeChat scopes are contact metadata, not authorization. Existing `carpool.publish` and `api_service.publish` capability checks remain mandatory and independent.
- Carpool owners/applicants and API merchants/buyers must supply their own enabled WeChat with a live current version. New in-scope snapshots freeze exactly one WeChat per participant and never add linux.do, email, Telegram, or a second contact.
- Linux.do synchronization, publish eligibility, profile identity, and source-topic rules remain intact. Linux.do is not selected or disclosed as a new transaction contact.
- `AppShell` prompts an authenticated user with no enabled WeChat after contact state resolves. Dismissal is per user and browser session, permits browsing, and never bypasses backend mutation guards. Contact-query invalidation after save closes the prompt.
- Copy says `configured` rather than `verified`; WeChat has no external verification claim. Updating WeChat affects future snapshots only because historical snapshots retain their frozen version.
- Migration `000115` normalizes existing WeChat scopes and adds the partial unique index. It does not rewrite historical versions or transaction snapshots and fails on duplicate enabled rows instead of choosing a value silently.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing, disabled, wrong-type, foreign, or versionless transaction contact | `422 CONTACT_METHOD_REQUIRED`, field code `wechat_required`; do not reveal ownership details |
| Create or enable a second WeChat | `409 INVALID_STATE_TRANSITION`, field `type`, code `duplicate`; PostgreSQL maps `ux_contact_methods_one_enabled_wechat` to the same error |
| WeChat request submits empty, partial, duplicate, or unauthorized seller scopes | Persist all four canonical scopes; do not grant seller capability |
| API merchant request contains zero contact IDs | `422 CONTACT_METHOD_REQUIRED`, field code `wechat_required` on `ownerContactMethodIds` |
| API merchant request contains more than one contact ID | `422 VALIDATION_FAILED`, field code `invalid_count` on `ownerContactMethodIds` |
| User dismisses onboarding | Continue browsing; publish/apply/purchase mutations remain guarded |
| User updates the configured WeChat value | Create a new contact version; existing snapshots remain unchanged |

### 5. Good / Base / Bad Cases

- Good: an API merchant with publish capability and one enabled WeChat publishes a service; the buyer intent freezes one merchant WeChat and one buyer WeChat.
- Good: a student buyer configures WeChat, dismisses no longer appears after query refresh, and the account still lacks seller capabilities.
- Base: an authenticated user without WeChat dismisses onboarding and browses public listings; a later purchase returns the field-level WeChat guard.
- Bad: select linux.do as the carpool owner contact because it is system-managed and already has every scope.
- Bad: trust a frontend checkbox selection or add a second WeChat column to users/business tables instead of using encrypted contact versions.
- Bad: catch the unique violation as a generic `500` or merge duplicate enabled values during migration.

### 6. Tests Required

- Contact unit/handler tests cover create/update normalization, second-enabled-WeChat rejection, idempotent paths, and unchanged capability enforcement.
- Migration/PostgreSQL tests cover scope normalization, partial uniqueness, concurrent integrity, typed unique-error mapping, and no historical snapshot rewrites.
- Carpool/API unit and PostgreSQL integration tests cover missing/wrong-type/foreign/disabled contacts, exactly-one merchant ID, owner/buyer success, immutable versions, and WeChat-only disclosure.
- Frontend tests cover authenticated-only querying, user-keyed cache isolation, per-session dismissal, save invalidation, no WeChat scope selector, fixed merchant contact presentation, and real/mock parity.
- Run `go test ./...`, `go vet ./...`, `./scripts/ci-postgres-integration.sh`, frontend tests/OpenAPI checks/typecheck/build, migration-doc checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
method, version, ok := contacts.VersionForOwnerAndScope(id, userID, UsageScopeBuyer)
```

This accepts any enabled scoped type, including linux.do or email, and can place it in a new in-scope transaction snapshot.

#### Correct

```go
method, version, ok := contacts.WechatVersionForOwnerAndScope(id, userID, UsageScopeBuyer)
```

Use the typed WeChat resolver at service boundaries and the matching locked PostgreSQL resolver inside transaction writes, then freeze only that version.
