# Error Handling

> How errors are handled in this project.

---

## Overview

Backend errors use typed domain errors and HTTP Problem Details. Handlers convert `domain.AppError` into `application/problem+json`; application services return explicit domain errors instead of ambiguous booleans or silent defaults.

---

## Error Types

- `domain.AppError` owns HTTP status, stable `code`, title, detail, and optional field errors.
- `domain.FieldError` owns field-level validation details returned under `errors`.
- Stable codes live in `backend/internal/domain/errors.go`.

---

## Error Handling Patterns

- Validate syntax/unknown JSON fields in `internal/server` or shared helpers under `internal/validator`.
- Validate business fields and state transitions in the owning module service. During the transition, this remains under `internal/module/core`.
- Return `*domain.AppError` from application services for expected domain failures.
- Do not echo raw contact values, credentials, tokens, or credential-looking request content in error details.
- Do not convert storage or validation failures into empty success responses.

---

## API Error Responses

Response content type:

```text
application/problem+json; charset=utf-8
```

Shape:

```json
{
  "type": "https://c2cmarket.local/problems/validation-failed",
  "title": "Invalid JSON",
  "status": 400,
  "code": "VALIDATION_FAILED",
  "detail": "请求 JSON 格式不正确或包含未知字段。",
  "instance": "/api/v1/official-price-leads",
  "requestId": "req_xxx",
  "errors": [
    {"field": "sourceUrl", "code": "secret_query", "message": "来源 URL 不能包含认证参数。"}
  ]
}
```

Baseline mapping:

| Scenario | HTTP | Code |
| --- | ---: | --- |
| Missing session | 401 | `SESSION_EXPIRED` |
| Missing CSRF | 403 | `CSRF_TOKEN_INVALID` |
| Permission denied | 403 | `PERMISSION_DENIED` |
| Object hidden/not found | 404 | `OBJECT_NOT_FOUND` |
| Invalid state transition | 409 | `INVALID_STATE_TRANSITION` |
| Duplicate active carpool application | 409 | `ACTIVE_APPLICATION_EXISTS` |
| Existing active carpool membership | 409 | `ACTIVE_MEMBERSHIP_EXISTS` |
| No available buyer seat | 409 | `SEAT_UNAVAILABLE` |
| Join confirmation expired | 409 | `JOIN_CONFIRMATION_EXPIRED` |
| Membership action on non-active membership | 409 | `MEMBERSHIP_NOT_ACTIVE` |
| Idempotency key body conflict | 409 | `IDEMPOTENCY_KEY_REUSED` |
| Rate limit exceeded | 429 | `RATE_LIMITED` |
| Version conflict | 412 | `VERSION_CONFLICT` |
| Field validation | 422 | `VALIDATION_FAILED` |
| JSON body too large | 413 | `VALIDATION_FAILED` |
| Secret-looking evidence | 422 | `SECRET_CONTENT_DETECTED` |

---

## Scenario: Carpool Self-Application Rejection

### 1. Scope / Trigger

- Trigger: `POST /api/v1/carpools/{id}/applications` receives a session user who owns the selected public listing.
- The database preserves the final `buyer_user_id <> owner_user_id` integrity constraint, but the service must reject this business state before repository persistence.

### 2. Signatures

```text
carpool.Service.CreateApplication(ctx, user, CreateApplicationInput)
POST /api/v1/carpools/{id}/applications
```

### 3. Contracts

- `CreateApplicationInput.BuyerUserID` is assigned from the authenticated session; it is never trusted from JSON.
- `Listing.OwnerUserID` comes from the public-listing repository result.
- The frontend listing DTO preserves `ownerUserId` and compares it with the authenticated profile ID before enabling the application action.

### 4. Validation & Error Matrix

| Condition | HTTP | Code | Detail |
| --- | ---: | --- | --- |
| `BuyerUserID == OwnerUserID` | 409 | `INVALID_STATE_TRANSITION` | `不能申请自己的车源。` |
| Different user with valid application input | 201 | — | Create application normally |

### 5. Good / Base / Bad Cases

- Good: a different authenticated buyer creates an application and the normal application transaction runs.
- Base: the frontend disables a listing owned by the current profile without submitting a mutation.
- Bad: a direct API call attempts self-application; it receives Problem Details and must not reach the database check constraint.

### 6. Tests Required

- Router regression: owner request returns 409 with `INVALID_STATE_TRANSITION` and the Chinese detail.
- PostgreSQL integration regression: the same owner request returns 409 before the application insert.
- Frontend helper regression: a listing with matching `ownerUserId` is disabled for the current profile ID.

### 7. Wrong vs Correct

#### Wrong

```text
UI compares display names or omits ownerUserId → repository insert → generic persistence error
```

#### Correct

```text
UI compares stable user IDs → service returns typed conflict → repository is not called
```

---

## Common Mistakes

- In Go, use `http.StatusUnprocessableEntity`; `http.Status422UnprocessableEntity` is not a standard-library constant.
- `http.MaxBytesReader` requires a response writer and should not be called with `nil`; use `io.LimitReader` in helpers that only have a request body.
- Strict JSON decoding is required for public submit endpoints and shared JSON helpers so authority fields, empty bodies, malformed JSON, oversized bodies, and trailing JSON values fail loudly instead of being silently ignored.
- Rate limits must use `http.StatusTooManyRequests` with stable code `RATE_LIMITED` and Problem Details content type; do not return ad hoc plaintext 429 bodies.
