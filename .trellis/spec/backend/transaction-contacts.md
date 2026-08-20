# Transaction Contact Selection And Safe Disclosure

Date: 2026-08-20
Author: Codex

## Scenario: Explicit Per-Transaction Contact Selection

### 1. Scope / Trigger

- Trigger: changing contact CRUD, account-email reuse, API or carpool transaction writes, contact snapshots/windows, registration navigation, OpenAPI contact DTOs, or frontend transaction forms.
- Owners: `internal/module/contact`, `internal/module/core`, API intent/market/quota and carpool services, PostgreSQL contact repositories, migration `000117`, OpenAPI schemas, and frontend contact selectors/adapters.
- Goal: let a user explicitly choose one eligible WeChat or verified email for each transaction action without exposing the private account email or granting seller capabilities.

### 2. Signatures

```go
func (*contact.Service) TransactionVersionForOwner(
    methodID string,
    ownerID string,
) (contact.ContactMethod, contact.ContactMethodVersion, bool)
```

```text
ContactMethodRequest:
  type: wechat | email | linuxdo | telegram | other
  label: string
  displayValue: string
  isDefault: boolean
  enabled: boolean

API service publish/update: ownerContactMethodId
API purchase intent: buyerContactMethodId
API quota order: buyerContactMethodId
Carpool publish/update: ownerContactMethodId
Carpool application: buyerContactMethodId
```

```sql
-- Migration 117 removes account-level authorization metadata.
contact_methods has no usage_scopes column

CREATE UNIQUE INDEX ux_contact_methods_one_enabled_wechat
ON contact_methods(user_id)
WHERE type = 'wechat' AND enabled = true;
```

### 3. Contracts

- An eligible transaction contact is owned by the actor, enabled, and has a current immutable version. Its type is either `wechat`, or `email` with `verified_at IS NOT NULL`.
- `linuxdo` remains system-managed identity metadata. `telegram` and `other` remain stored contact types but are not valid for V1 transaction selection.
- Every transaction mutation accepts one explicit contact-method ID. Adapters must not list contacts and silently choose a default, first item, or WeChat fallback.
- The business record freezes the selected contact method and version. Later contact edits, disabling, or deletion affect new transactions only; existing snapshots and authorized contact windows retain the frozen version.
- Account email is private account-recovery data. It becomes a transaction contact only after the user confirms disclosure and the backend creates or reuses a separate `email` contact record.
- Account-email verification reuse is server-authoritative: core loads the current user profile, normalizes the email, checks `email_verified_at`, and injects the trusted verification time internally. Public JSON never accepts `verified` or `verifiedAt`.
- Creating the same enabled normalized email for the same user returns the existing contact. A custom email starts unverified and must complete the contact-email challenge before it becomes eligible.
- WeChat may be disabled or soft-deleted. Enabling a second WeChat still conflicts with `ux_contact_methods_one_enabled_wechat`; WeChat has no external-verification claim.
- Contact selection does not grant `carpool.publish`, `api_service.publish`, or any other capability. Business services enforce actor capability and object state independently.
- Registration and OAuth success return to the normalized internal target. Missing WeChat must not create onboarding dialogs, query parameters, storage markers, account-completeness tasks, or navigation redirects.
- Contact values remain private outside authorized transaction windows. Public market/profile responses never expose account email, contact values, or owner contact IDs.
- Configuration audit events contain safe action metadata and changed field names only. Disclosure reads remain dedicated access-log facts.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Contact ID missing from a transaction request | `422 CONTACT_METHOD_REQUIRED` on the business contact field |
| Contact is missing, foreign, disabled, versionless, unsupported, or an unverified email | `422 VALIDATION_FAILED` with “请选择有效的交易联系方式”; do not reveal which ownership check failed |
| Enabled owned WeChat with current version | Eligible; freeze exactly its submitted version |
| Enabled owned verified email with current version | Eligible; freeze exactly its submitted version |
| Client submits `verified`/`verifiedAt` | Field is absent from the public contract; never trust it |
| Confirmed account email matches an existing enabled normalized email contact | Reuse the existing contact and version; do not create duplicates |
| User disables or deletes WeChat | Succeeds unless another independent business invariant rejects the command; old snapshots remain readable |
| User enables a second WeChat | `409 INVALID_STATE_TRANSITION` mapped from the partial unique index |
| Seller lacks the publish capability | Existing capability error, even with a valid contact |
| Student/OAuth registration succeeds without WeChat | Return to normalized `returnTo`; no onboarding side effect |

### 5. Good / Base / Bad Cases

- Good: a buyer confirms use of the verified account email, receives a separate verified contact, explicitly selects it for an API purchase, and the intent freezes that version.
- Good: a carpool owner publishes with WeChat while an applicant selects verified email; acceptance reveals only each participant's frozen selection.
- Base: a user has no eligible contact and can still register, browse, and manage the account; publish/apply/purchase forms remain blocked until a contact is selected.
- Base: a user has both email and WeChat; the selector shows both and does not preselect when multiple choices exist.
- Bad: derive transaction email directly from `users.email`, auto-select the first contact, or prefer WeChat in an adapter.
- Bad: restore `usage_scopes`, `ownerContactMethodIds`, `WechatRequiredError`, or a `wechat_required` field code.
- Bad: treat a configured WeChat as account completeness or interrupt login/navigation to request it.

### 6. Tests Required

- Contact unit tests cover enabled WeChat, verified email, disabled/foreign/versionless contacts, unverified email, unsupported types, and immutable version lookup.
- Core tests cover authoritative account-email verification reuse, normalized duplicate reuse, custom-email verification reset, idempotency, and safe audit fields.
- API intent/quota/carpool/service tests cover required explicit IDs, both eligible types, invalid ownership/state/type, capability independence, and frozen version IDs.
- PostgreSQL integration tests run against migration 117 and assert the `usage_scopes` column/function/constraints are absent while the single-enabled-WeChat index and historical versions remain.
- OpenAPI/generated-type checks assert singular required contact IDs and the absence of `usageScopes`, `ContactUsageScope`, and `ownerContactMethodIds`.
- Frontend tests cover no contact, only email, only WeChat, both contacts, unverified email, account-email disclosure confirmation, explicit payload IDs, no registration onboarding, and real/mock parity.
- Run `go test ./...`, `go vet ./...`, `./scripts/ci-postgres-integration.sh`, frontend tests/typecheck/build, OpenAPI generation/check, migration-doc checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
methods := contacts.ListForOwner(userID)
method := firstEnabledWechat(methods)
intent, err := orders.Create(serviceID, method.ID)
```

This silently broadens disclosure and prevents the user from choosing verified email.

#### Correct

```go
method, version, ok := contacts.TransactionVersionForOwner(input.BuyerContactMethodID, userID)
if !ok {
    return domain.ValidationError("buyerContactMethodId", "请选择有效的交易联系方式")
}
intent, err := orders.Create(serviceID, method.ID, version.ID)
```

The UI sends the explicit choice, the service validates it against authoritative state, and the transaction freezes the selected version.

## Design Decision: Transaction Context Replaces Account-Level Usage Scopes

**Context**: Account-level buyer/dispute/seller scope checkboxes duplicated the authorization already expressed by submitting a business action and made one WeChat binding apply to every future context.

**Decision**: Migration 117 removes `contact_methods.usage_scopes`. The business field carrying the contact ID defines the purpose, while capability checks and contact eligibility remain separate.

**Extension**: A future contact type becomes eligible by extending the central backend resolver and frontend predicate together, then adding cross-layer tests. Do not add a new account-level purpose array.
