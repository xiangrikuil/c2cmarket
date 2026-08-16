# Announcement Publication Lifecycle

> Executable publication-time, persistence, status, and user-detail contracts for announcements.

## Scenario: Publish A Draft Or Historical Announcement

### 1. Scope / Trigger

- Trigger: any change to the admin announcement publish action, announcement time fields, display-status derivation, audit persistence, the frontend Mock adapter, or user-facing announcement time metadata.
- This contract prevents an old draft timestamp from making a newly published announcement appear historical or immediately expired.

### 2. Signatures

```typescript
publishAnnouncement(id: string): Promise<Announcement>
```

```go
func (s *Service) PublishAnnouncement(ctx context.Context, user auth.User, id string) (Announcement, *domain.AppError)
func ResolvePublishTransition(item Announcement, now time.Time) (status string, publishAt time.Time, appErr *domain.AppError)
type Repository interface {
	PublishAnnouncement(ctx context.Context, input ActionInput, now time.Time) (Announcement, *domain.AppError)
}
func (s *Store) PublishAnnouncement(ctx context.Context, input ActionInput, now time.Time) (Announcement, *domain.AppError)
```

```http
POST /api/v1/admin/announcements/{id}/publish
Content-Type: application/json

{}
```

PostgreSQL updates `announcements.status`, `publish_at`, `updated_by_user_id`, `updated_at`, and `version`, then inserts the matching `announcement_audit_logs` row in the same transaction. User-visible content updates additionally persist `content_updated_at`; management-only mutations preserve it.

### 3. Contracts

- Capture one `now` per publish action and reuse it for transition resolution, `updatedAt`, the immediate `publishAt`, and the audit timestamp.
- Compute `effectivePublishAt = max(now, publishAt)` once for publication validation.
- When `publishAt > now` and `expireAt` is absent or later than `publishAt`, store `status=scheduled`, preserve the configured `publishAt`, set `updatedAt=now`, increment `version`, and record the scheduled-publication audit reason.
- When `publishAt <= now` and `expireAt` is absent or later than `now`, store `status=published`, replace stale `publishAt` with `now`, set `updatedAt=now`, increment `version`, and record the immediate-publication audit reason.
- When `expireAt <= effectivePublishAt`, reject before mutation. The announcement status, times, version, and audit log remain unchanged.
- The frontend Mock adapter, in-memory service, and PostgreSQL repository must implement the same transition table.
- PostgreSQL must resolve the transition while holding a row lock and commit the announcement update and audit insert atomically.
- Stored status is publication intent. Every read derives effective status in this order: explicit `draft` / `offline` / `archived`, elapsed `expireAt` as `expired`, future `publishAt` as `scheduled`, otherwise `published`. User visibility and admin filters must not wait for a scheduler write.
- `contentUpdatedAt` is required in the database, Go model, HTTP response, OpenAPI schema, generated frontend type, Mock data, and stored Mock normalization.
- Create and duplicate initialize `contentUpdatedAt` to the new-record action time. Historical migration backfills it from `publishAt`.
- Only normalized title, summary, Markdown body, category, level, CTA label, or CTA URL changes advance `contentUpdatedAt`.
- Pinning, channels, dismissibility, publish/offline state, sorting, audit reasons, and administrator-only metadata update `updatedAt` where applicable but preserve `contentUpdatedAt`.
- User-facing detail always labels `publishAt` as `发布于`. It labels `contentUpdatedAt` as `更新于` only when `contentUpdatedAt > publishAt`; admin audit action names must not appear in the user detail.

### 4. Validation & Error Matrix

| Condition | Result | Mutation |
|---|---|---|
| User is not an administrator | Existing authorization error | None |
| Announcement does not exist | `404 OBJECT_NOT_FOUND` | None |
| `publishAt > now` | `scheduled`; preserve `publishAt` | Update announcement and append audit atomically |
| `publishAt <= now`, `expireAt` absent or `expireAt > now` | `published`; set `publishAt=now` | Update announcement and append audit atomically |
| `publishAt <= now`, `expireAt <= now` | `422 VALIDATION_FAILED`, field `expireAt`, rule `elapsed`, detail `结束时间已过，请调整结束时间后再发布。` | None |
| `publishAt > now`, `expireAt <= publishAt` | `422 VALIDATION_FAILED`, field `expireAt`, rule `elapsed`, detail `结束时间必须晚于计划发布时间，请调整后再发布。` | None |

### 5. Good/Base/Bad Cases

- Good: publish a draft saved ten days ago with no end time; it enters the admin Published group with `publishAt == updatedAt == action time`.
- Base: publish an announcement planned for tomorrow; it enters Scheduled and keeps tomorrow's `publishAt`, while `updatedAt` records today's action.
- Bad: publish an old draft whose end time already elapsed; return the `expireAt` validation error and preserve the draft unchanged.
- Bad scheduled window: plan publication for tomorrow with an end time tonight; reject rather than create an announcement that expires before it can publish.
- Good status projection: the same stored publication intent derives Scheduled before `publishAt`, Published at `publishAt`, and Expired at `expireAt`, without a database write.
- Good user projection: immediately published content shows only `发布于`; a later visible content edit makes `contentUpdatedAt > publishAt` and adds `更新于`.
- Base management edit: pinning or changing channels advances `updatedAt` but preserves `contentUpdatedAt`, so the user detail does not claim a content update.

### 6. Tests Required

- Frontend Mock unit tests assert immediate, future-scheduled, read-time Scheduled/Published/Expired boundaries, invalid future windows, and unchanged state after rejection.
- Go service unit tests assert stored status, `DisplayStatus`, exact action timestamps, admin-list visibility, scheduled-time preservation, boundary transitions, and field-level validation errors.
- Frontend Mock and Go service tests assert that visible-content edits advance `contentUpdatedAt`, while pin/channel/dismissibility-only edits and publish/offline actions preserve it.
- Migration contract tests assert additive non-null `content_updated_at`, conservative `publish_at` backfill, rollback SQL, and expected migration version.
- PostgreSQL-focused tests or repository package tests must cover row-lock transition resolution and atomic update/audit behavior whenever the repository implementation changes.
- User-detail source/component tests assert `发布于`, conditional `更新于`, and absence of backend audit wording.
- Browser smoke publishes an old draft, verifies the Published count/list placement and action-time display, then opens the user detail and verifies its time labels.

### 7. Wrong vs Correct

#### Wrong

```go
item.Status = StatusPublished
item.UpdatedAt = now
// Stale draft PublishAt survives and display status may derive as expired.
```

#### Correct

```go
status, publishAt, appErr := ResolvePublishTransition(item, now)
if appErr != nil {
	return Announcement{}, appErr
}
item.Status = status
item.PublishAt = publishAt
item.UpdatedAt = now
```

#### Wrong

```go
item.UpdatedAt = now
item.ContentUpdatedAt = now // A pin/channel/status-only edit now leaks as a user-visible update.
```

#### Correct

```go
if UserVisibleContentChanged(item, form) {
	item.ContentUpdatedAt = now
}
item.UpdatedAt = now
```

## Scenario: Deliver Important And Critical Announcements Globally

### 1. Scope / Trigger

- Trigger: any change to announcement severity, channels, audience, receipt
  actions, public projections, global delivery UI, or delivery revision rules.
- This contract keeps anonymous, authenticated, Mock, PostgreSQL, OpenAPI, and
  root-layout behavior aligned without adding another realtime transport.

### 2. Signatures

```http
GET /api/v1/announcements/public-active?channel=global_bar|modal
POST /api/v1/me/announcements/{id}/seen
POST /api/v1/me/announcements/{id}/read
POST /api/v1/me/announcements/{id}/dismiss
POST /api/v1/me/announcements/{id}/acknowledge
```

```go
type Audience struct {
	Type    string
	Roles   []string
	UserIDs []string
}

type Receipt struct {
	AnnouncementVersion int64
	FirstSeenAt         *time.Time
	ReadAt              *time.Time
	DismissedAt         *time.Time
	AcknowledgedAt      *time.Time
}
```

Migration 109 adds `announcements.requires_ack`,
`announcement_receipts.acknowledged_at`, and the
`announcement_recipients(announcement_id, user_id, announcement_version,
snapshotted_at)` publication snapshot.

### 3. Contracts

- Valid levels are `normal`, `important`, and `critical`; valid channels are
  `message_center`, `home_banner`, `global_bar`, and `modal`.
- `message_center` is mandatory. Normal announcements cannot use global or
  acknowledgement channels. Important announcements may use `global_bar` but
  not `modal` or acknowledgement. Critical announcements require
  `global_bar + modal + requiresAck=true + isDismissible=false`.
- Audiences are exactly `{type:"all"}`, `{type:"roles",roles:[buyer|merchant|admin]}`,
  or `{type:"specific_users",userIds:[uuid...]}`. Targeted audiences resolve
  into `announcement_recipients` when scheduled or published. A published
  audience edit rebuilds the snapshot atomically at the new delivery revision.
- Public active reads return only all-user important or critical announcements
  for the requested global channel. Public DTOs include `isPinned` because the
  anonymous client must reproduce the server's delivery order, but omit target
  IDs, receipts, operators, and audit metadata.
- Delivery order is severity descending, pinned first, publication time
  descending, then ID ascending. Only one unacknowledged critical modal renders
  at a time. The global bar excludes the item currently in the modal.
- `seen` records the first successful bar/modal render; `read` records detail
  access; `dismiss` closes a dismissible bar; `acknowledge` is the only action
  that satisfies a required acknowledgement. Read alone never records seen.
- Anonymous acknowledgement and dismissal are browser-local by announcement ID
  plus delivery revision. Login ignores anonymous acknowledgement and requires
  the durable backend receipt.
- The delivery `version` advances for normalized user-visible content,
  delivery-channel, acknowledgement, dismissibility, audience, or publish
  changes. Pin, schedule, expiry, operator, and offline-only changes preserve
  it. `contentUpdatedAt` retains its narrower content-only contract.
- The root `GlobalAnnouncementLayer` owns all routes. Receipt failures keep the
  surface visible and retain the exact failed action (`seen` or `dismiss`) for
  retry. Shells reserve `--global-announcement-height` so the bar never covers
  navigation.

### 4. Validation & Error Matrix

| Condition | Result | Mutation |
|---|---|---|
| Normal uses `global_bar`, `modal`, or acknowledgement | `422 VALIDATION_FAILED` | None |
| Important uses `modal` or acknowledgement | `422 VALIDATION_FAILED` | None |
| Critical omits a required channel/ack or is dismissible | `422 VALIDATION_FAILED` | None |
| Role audience has no valid roles | `422 VALIDATION_FAILED`, `audience.roles` | None |
| Specific-user audience is empty or contains an unknown user | `422 VALIDATION_FAILED`, `audience.userIds` | None |
| Public request asks for a non-global channel | `422 VALIDATION_FAILED`, `channel` | None |
| Non-recipient reads or acknowledges a targeted announcement | Existing not-found/visibility error | None |
| Dismiss a non-dismissible announcement | `422 VALIDATION_FAILED` | None |
| Acknowledge a non-critical/non-required announcement | `422 VALIDATION_FAILED` | None |
| Receipt request carries a stale delivery revision in storage | Current revision replaces effective receipt state | Current revision only |

### 5. Good/Base/Bad Cases

- Good: a pinned critical all-user announcement is the first anonymous and
  authenticated modal on every route; Escape, outside click, and detail
  navigation do not acknowledge it.
- Good: acknowledging critical A advances to critical B; refresh does not show A
  again for that authenticated user and revision.
- Base: an important global bar can be dismissed and stays dismissed for its
  current revision; a failed write leaves it visible with retry.
- Good targeting: a scheduled buyer announcement snapshots buyer recipients at
  publication time and never leaks explicit user IDs through user/public DTOs.
- Bad: sorting a public DTO without `isPinned` silently changes anonymous modal
  order even when the PostgreSQL query returned the correct order.
- Bad: incrementing the delivery revision for a pin-only edit makes an already
  acknowledged critical notice falsely pending again.

### 6. Tests Required

- Service tests cover the level/channel matrix, audience matching, deterministic
  ordering, receipt action meanings, and material versus management-only
  revision changes.
- Store and migration tests cover migration 109, current-version receipt upsert,
  recipient snapshot creation/rebuild, and targeted visibility.
- Handler/OpenAPI tests cover the public endpoint, `isPinned` ordering metadata,
  acknowledgement action, and absence of target IDs/receipts/admin fields.
- Frontend tests cover Mock parity, local receipt revision invalidation, serial
  critical selection, exact retry action, public detail reuse, and editor
  invariants.
- Browser checks cover anonymous detail, authenticated acknowledgement across
  refresh, serial modals, dismiss failure/retry, admin audience controls, and
  shell overlap at 390x844, 1440x900, and 1920x1080.

### 7. Wrong vs Correct

#### Wrong

```typescript
// Public items omit isPinned, so equal-time announcements reorder by ID.
sortAnnouncementsForDelivery(publicItems)
```

#### Correct

```typescript
// PublicAnnouncement includes isPinned and uses the shared deterministic order.
sortAnnouncementsForDelivery(publicItems)
```

#### Wrong

```go
receipt.ReadAt = &now
receipt.FirstSeenAt = &now // Detail access falsely claims the global surface rendered.
```

#### Correct

```go
receipt.ReadAt = &now // Only seen/dismiss/acknowledge may initialize FirstSeenAt.
```
