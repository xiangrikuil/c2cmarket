# Contact Usage Scopes And Safe Configuration Audit

Date: 2026-08-12
Author: Codex

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
```

### 3. Contracts

- PostgreSQL persists the exact canonical scope set and returns it on every contact read. Canonical order is `carpool_owner`, `api_merchant`, `buyer`, `dispute`; duplicate input values collapse to one value.
- Legacy rows migrate with all four scopes so existing transaction references keep working. New backend callers that omit scopes use only `buyer` plus `dispute`; the public OpenAPI create request requires an explicit non-empty array.
- An update that omits `usageScopes` preserves the stored scopes. An explicit empty or unknown set is invalid; no handler may hard-code a wider response than the row actually stores.
- Adding or retaining `api_merchant` requires `api_service.publish`; adding or retaining `carpool_owner` requires `carpool.publish`. Buyer/dispute scopes remain available to an authenticated student buyer.
- Scope capability checks happen before idempotency acquisition and are repeated at the reusable service boundary with freshly loaded identity facts.
- API-service and quota publication accept only an enabled owner contact containing `api_merchant`; carpool-owner publication accepts only `carpool_owner`; buyer and dispute snapshots accept only the matching scope. Ownership/type checks still apply.
- The system-managed linux.do bridge is projected from the immutable binding with all four scopes and cannot be created, converted, edited, or deleted through public contact CRUD.
- Contact values stay in versioned private storage and transaction snapshots. Configuration audit events expose only the action, actor/request IDs, aggregate version, and changed field names; they never contain the value, masked value, fingerprint, email, or provider handle.
- Contact create/update/default/verify/delete mutation, configuration event, and idempotency completion use one repository transaction. Actual disclosure reads remain separate dedicated access-log facts.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/empty/unknown explicit scope array | `422 VALIDATION_FAILED` on `usageScopes`; no mutation/event |
| Student requests `api_merchant` or `carpool_owner` | `403 CAPABILITY_REQUIRED` before idempotency or storage |
| Merchant/carpool selects a contact lacking its required scope | `422 VALIDATION_FAILED` on the contact field |
| Contact is disabled, foreign, missing, or wrong type | Existing stable not-found/validation result; do not reveal another user's row |
| Public CRUD targets the system linux.do method | `409 INVALID_STATE_TRANSITION` |
| Same completed create/update command is replayed | Return stored completion; do not create a new version or event |
| Event serialization/completion fails | Roll back contact mutation, version, default changes, and event |

### 5. Good / Base / Bad Cases

- Good: a student stores WeChat for `buyer` and `dispute`, buys API quota, and can use buyer after-sales without seeing seller publish actions.
- Good: a linux.do seller chooses `api_merchant` on one enabled contact; API publication validates that scope and stores only the selected version reference.
- Base: a migrated existing contact has all four scopes and remains usable until its owner narrows the set explicitly.
- Bad: accept `usageScopes` in JSON, drop it before the service, then return a hard-coded four-scope response.
- Bad: treat owning a contact as enough for every transaction purpose or infer a scope from its type/label.
- Bad: copy contact values, masked values, fingerprints, or request bodies into `domain_events` or the unified administrator log.

### 6. Tests Required

- Unit tests assert canonical ordering/deduplication, omitted-create defaults, omitted-update preservation, explicit-empty/unknown rejection, clone safety, and linux.do all-scope projection.
- Handler/service tests assert student seller-scope denial before idempotency, valid buyer/dispute round trips, truthful response scopes, and no direct-service bypass.
- PostgreSQL integration tests assert migration defaults, array constraints, create/update/read round trips, scope-aware merchant/carpool selection, and rollback/replay counts for mutation/version/event/completion.
- Audit tests assert one safe configuration action per successful mutation and scan serialized output for contact values, emails, fingerprints, credentials, and arbitrary metadata.
- OpenAPI/generated/frontend tests assert the exact four-value union and capability-aware scope options; production real mode must not fall back to mock contact data.

### 7. Wrong vs Correct

#### Wrong

```go
response.UsageScopes = []string{"carpool_owner", "api_merchant", "buyer", "dispute"}
```

This lies about persisted intent and can let a later workflow treat a buyer-only method as a seller contact.

#### Correct

```go
response.UsageScopes = append([]string(nil), method.UsageScopes...)
```

Normalize and authorize the request before mutation, persist the canonical array, validate the selected workflow scope again, and project exactly the stored value.
