# Backend Migrations

日期：2026-06-21
执行者：Codex

This directory contains PostgreSQL-oriented SQL contract files for C2CMarket.

The first backend contract baseline is split into focused `golang-migrate`
versions:

| Version | Scope |
| --- | --- |
| `000001_extensions_and_identity` | `pgcrypto`, users, auth sessions, linux.do bindings, permissions, restrictions, merchant profiles |
| `000002_catalog_and_policy` | product categories/plans, publish policy, policy history, versioned risk notices |
| `000003_idempotency_events_notifications_audit` | idempotency keys, domain events, notifications, admin audit logs |
| `000004_official_price` | official price leads and official price records |
| `000005_contact_methods` | contact methods and encrypted contact method versions |
| `000006_contact_sessions` | contact windows and access logs |
| `000007_seed_catalog_risk_and_policy` | initial catalog, risk notice, and publish policy seed data |
| `000008_contact_and_foundation_integrity` | contact FK/trigger integrity, idempotency constraints, official price constraints and indexes |
| `000009_carpool_contract` | carpool listings, listing risk acknowledgements, applications, application risk acknowledgements |
| `000010_carpool_reservation_and_integrity` | reservation deadlines, buyer-seat semantics, owner contact selection, contact-session consistency, risk acknowledgement version integrity |
| `000011_carpool_membership_lifecycle` | buyer/owner join confirmations, joined applications, and active carpool memberships |
| `000012_carpool_membership_cycle_lifecycle` | cycle terms, completion confirmations, completed memberships, buyer leave, owner remove |
| `000013_api_market_services` | API model catalog, API service owner/admin lifecycle, service access modes, supported models, fixed packages |
| `000014_api_market_purchase_intents` | API purchase intents, frozen buyer/owner contact method versions, intent lifecycle |
| `000015_api_intent_direct_contacts` | removes legacy API intent contact-window columns and enforces direct frozen contact disclosure status model |
| `000016_api_intent_contract_hardening` | API purchase intent selected access mode, contact type/label snapshots, contact-version identity constraints, status timestamp constraints |
| `000017_profile_public_contact` | profile privacy fields, public username index, merchant profile public slug index |
| `000018_announcements` | announcements, per-user receipts, admin announcement audit logs |
| `000019_demands` | demand posts, publisher/admin review lifecycle, public active demand indexes |
| `000020_favorites` | user favorites for public carpool listings and public API services |
| `000021_reviews` | completed carpool membership buyer-to-owner reviews and public review listings |
| `000022_reports_disputes_appeals` | user reports, dispute cases, appeals, and append-only dispute events |
| `000023_api_intent_contact_access_logs` | API purchase intent direct contact disclosure audit logs without plaintext contact values |
| `000024_search_trigram_indexes` | `pg_trgm` extension and GIN trigram indexes for public search fields |
| `000025_native_admin_login` | native username/password credential table without fixed password seeds |
| `000026_account_identity_profile` | account profile fields for password, email verification, and custom avatar URL |
| `000027_api_service_instant_orders` | API service orderability settings, API orders, order events, and payment-instruction access logs |
| `000028_api_order_dispute_targets` | report/dispute/appeal target constraint support for API order disputes |
| `000029_feedback_tickets` | user feedback tickets, supplements, admin handling lifecycle, and unread receipt tracking |
| `000030_carpool_quota_fields` | carpool listing service multiplier and average quota disclosure fields |
| `000031_email_registration_verification` | email registration challenge, verification, and auth identity contract |
| `000032_carpool_cancel_exit_lifecycle` | buyer application cancel, owner acceptance withdrawal status constraints, and cancelled contact-session history |
| `000033_product_plan_quota_unit_carpool` | product-plan quota units and carpool listing quota-unit snapshots |
| `000034_api_model_provider_catalog` | managed API model providers and provider-backed model catalog |
| `000035_password_argon2_admin_bootstrap` | Argon2id password algorithm support and fixed admin seed cleanup |
| `000036_search_trigram_alignment` | merchant-profile trigram expression alignment for display-name-only public search |

The current runnable Go slice supports both in-memory tests and PostgreSQL runtime.
When `DATABASE_URL` is configured, users, auth sessions, idempotency, product
catalog reads, official price leads/records, contact methods, contact sessions,
contact access logs, carpool listings, carpool cycle terms, carpool applications,
join confirmations, memberships, completion confirmations, API model catalog
reads, API service publishing/review/moderation reads and writes, API
purchase-intent creation/lifecycle reads and writes, native username/password
login credentials, profile privacy fields,
merchant profile public reads, announcements, demands, favorites, completed
carpool membership reviews, reports, dispute cases, appeals, dispute events, and
API purchase-intent contact access logs are backed by PostgreSQL.

Official price approval is the baseline multi-row transaction: the runtime writes
the lead update, price record, domain event, admin audit log, notification, and
completed idempotency response cache together.

Carpool application accept follows the same transaction rule: the runtime locks
the application/listing rows, checks seat availability, creates the 30-minute
contact window, freezes contact method versions, writes the application event and
notification, and completes the idempotency response cache in one commit.

Carpool join confirmation follows the same transaction rule: the runtime records
the buyer/owner confirmation, creates the active membership only after both sides
confirm, increments the listing buyer-member cache, writes event/notification,
and completes the idempotency response cache in one commit.

Carpool membership completion and exit follow the same transaction rule: the
runtime records buyer/owner completion confirmations, marks the membership
completed only after both sides confirm, or ends active membership through buyer
leave / owner remove with a required reason. These actions decrement the listing
buyer-member cache, write event/notification, and complete the idempotency
response cache in one commit. They do not implement platform payment, refund,
compensation, or guarantee handling.

API service publishing is split from API purchase intents. Version 13 stores
only service descriptions, non-sensitive access-mode notes, supported model
snapshots, merchant-declared pricing, and fixed package descriptions. Public API
service reads are limited to `review_status='approved'`,
`publication_status='online'`, and `moderation_status='clear'`; public responses
do not include owner contact method IDs, review internals, or merchant internal
notes. `distribution_system='sub2api'` service models must keep
`merchant_multiplier=1.0000` at both service-validation and database CHECK
levels.

API purchase intents are version 14 plus the direct-contact cleanup in version
15 and contract hardening in version 16. Creating an intent for a public API service creates the intent row, freezes
buyer and owner contact method version IDs, writes a domain event and owner
notification, and completes idempotency metadata in one transaction. The intent
table stores non-sensitive service and pricing snapshots plus frozen contact
version references only; full contact values are not copied into snapshots,
events, notifications, audit logs, or `idempotency_keys.response_body_json`.
Successful API intent creation and buyer/owner detail reads decrypt the frozen
contact version for the authorized participant and must use `Cache-Control:
no-store`. API purchase intents no longer create or reference `contact_sessions`;
contact sessions remain for carpool and development contact-window flows.

Version 23 stores direct contact disclosure audit rows for API purchase intents.
Rows record only `api_purchase_intent_id`, `viewer_user_id`,
`viewed_contact_owner_side`, `request_id`, and `accessed_at`. They must not store
plaintext contact values, masked contact values, credentials, payment evidence,
or fulfillment data. API purchase-intent creation records the buyer viewing the
merchant contact; buyer detail records merchant-contact reads; owner detail
records buyer-contact reads.

Version 24 enables PostgreSQL `pg_trgm` and adds GIN trigram indexes over public
search text expressions for API services/models, carpool listings, demands,
product-plan text used by official price search, public users/linux.do
usernames, merchant profiles, and API model catalog rows. These indexes are
performance support only; they do not change search visibility predicates or
response DTOs. Use `scripts/explain-search.sql` from the repository root to
verify that global search predicates keep matching the expression indexes.

Version 36 realigns the merchant-profile trigram index to `lower(display_name)`
so store-alias API service search can use the index while preserving the public
search contract that matches and displays public merchant display names only.

Version 21 stores `carpool_reviews`. Reviews are constrained to
`source_type='carpool_membership'`, `reviewer_role='buyer'`, and
`reviewee_role='owner'`. A constraint trigger verifies the source membership is
completed and that reviewer/reviewee match the membership buyer/owner. The unique
`(source_type, source_id, reviewer_user_id)` constraint makes repeated review
submission an update of the same review rather than a second public record.

Version 22 stores `reports`, `dispute_cases`, `appeals`, `dispute_events`, and
`moderation_audit_logs`.
Report and appeal creation plus admin actions are idempotent, versioned where
applicable, and append dispute events in the same transaction. Public dispute
reads come only from `dispute_cases.public_summary`, `public_result_code`, and
`public_result`; they must not expose reporter IDs, admin IDs, contact values,
internal notes, evidence descriptions, payment, refund, compensation, escrow,
guarantee, fulfillment, or credential-delivery semantics.

## Contact Retention And Destruction

Contact method deletion retires the mutable contact method surface. Historical
business rows keep frozen contact method version references where the product
requires a dispute/audit trail. Carpool contact sessions and API purchase intents
can continue to resolve their frozen versions only through authorized business
reads, and those reads must use `Cache-Control: no-store` and write access logs
where applicable. Access logs and domain events store identifiers and side
metadata only, never plaintext contact values.

Physical destruction of historical contact ciphertext is intentionally not
implemented in this migration set because it must be coordinated with dispute
retention policy, encrypted version references, and key-rotation operations.
Future destructive retention work should add explicit `destroyed_at` semantics
and a key-rotation/destruction runbook rather than deleting rows implicitly.

`000007_seed_catalog_risk_and_policy.down.sql` removes only fixed seed UUIDs. If
business rows already reference those seed plans, PostgreSQL foreign keys are
expected to block rollback instead of deleting referenced catalog data.

## Docker Compose

The repository root `compose.yaml` provides a PostgreSQL service and a one-shot
`migrate` service based on `migrate/migrate`.

Start PostgreSQL and run migrations:

```bash
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```

Repeat migration runs are safe when the schema is already up to date.

Reset the local database and re-run migrations:

```bash
docker compose down -v
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```
