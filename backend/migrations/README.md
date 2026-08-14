# Backend Migrations

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
| `000019_demands` | demand posts, publisher lifecycle, admin moderation compatibility, public active demand indexes |
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
| `000037_model_audit` | model audit targets, encrypted API key storage, baselines, runs, samples, probe scores, passive call features, and scheduled monitors |
| `000038_api_service_quota_expires_at` | fixed expiration timestamp for metered API quota service listings |
| `000039_demands_publish_immediately` | converts pending demand posts to active after demand posting stops requiring admin review |
| `000040_carpool_listing_region` | persisted carpool listing opening region code and display name |
| `000041_api_service_source_url` | optional linux.do source topic for API quota services |
| `000042_carpool_distribution_admin_account` | public carpool distribution method and administrator-account availability signals |
| `000043_remove_usdt_payment_method` | removes USDT from API service payment method options and order payment-method constraints |
| `000044_api_order_delivery_credentials` | API order payment QR snapshots and encrypted in-platform delivery credential storage |
| `000045_product_category_icon` | optional admin-uploaded PNG/WebP category icon data URL |
| `000046_realtime_invalidation` | commit-aware PostgreSQL notifications for user inbox changes and administrator work queues |
| `000047_api_purchase_intent_ordered_status` | releases order-backed API purchase intents from the active-intent uniqueness constraint |
| `000048_report_schema_upgrade` | upgrades earlier report/dispute schemas with canonical targets, result codes, deduplication, and moderation audit rows |
| `000049_api_order_quota_inventory` | service-level metered quota inventory plus immutable quota, rate, and pricing snapshots on API orders |
| `000050_api_order_payment_issue` | buyer-reported API order payment issues with merchant resolution tracking and notification hooks |
| `000051_api_limited_packages` | stable limited-package inventory, package/model associations, and order expiry snapshots |
| `000052_api_purchase_intent_ordered_constraint_cleanup` | removes the legacy anonymous intent constraint and preserves the ordered intent state |
| `000053_auth_session_renewal` | seven-day idle sessions, twenty-four-hour renewal throttling, and thirty-day absolute expiry |
| `000054_api_quota_offers` | seller-declared API quota batches, fixed USD offers, scheduled inventory units, round claims, and immutable order snapshots |
| `000055_api_quota_credentials` | encrypted pre-imported buyer-specific credential inventory for limited quota offers |
| `000056_api_quota_system_slots` | nullable fixed Beijing sale-slot keys for simplified scheduled quota publication |
| `000057_reputation_transaction_exclusions` | reversible transaction exclusions with append-only administrator audit events |
| `000058_reputation_governance` | explicit dispute subjects, reversible reputation outcomes, role/action restrictions, and append-only governance audit events |
| `000059_transaction_reviews` | completed carpool/API bidirectional sealed reviews, publication freeze, append-only revisions, and legacy review migration |
| `000060_reputation_engine` | rebuildable role/scope reputation snapshots, append-only history, and fact-driven invalidation |
| `000061_source_author_verification` | resource-level source-author verification, append-only audit events, and reputation invalidation |
| `000062_auth_identity_bootstrap_hardening` | immutable first-admin bootstrap provenance for create-only, fail-closed initialization |
| `000063_verification_data_lifecycle` | keyed email verification challenges, bounded idempotency records, and lifecycle-maintenance indexes |
| `000064_contact_cipher_aad` | explicit legacy/AAD cipher formats for versioned contact, audit, order, and quota secrets |
| `000065_remove_demands` | removes the prelaunch demand-post table, its indexes, and demand idempotency residue |
| `000066_api_service_multiplier_reconciliation` | removes historical fixed-one Sub2API service-model constraints so the positive merchant-declared multiplier contract is consistent |
| `000067_api_account_payment_settings` | account-level mutually exclusive WeChat Pay and Alipay settings for future API service snapshots |
| `000068_api_order_delivery_review` | API-order delivery review deadlines and explicit buyer-confirmed or automatic completion sources |
| `000069_carpool_usage_signals` | structured carpool quota, reset, region, connection, channel, and payment signals |
| `000070_admin_user_directory_governance` | administrator account-directory governance audit lookup support |
| `000071_api_service_promotions` | administrator-owned API service promotion schedules and lifecycle facts |
| `000072_api_service_commercial_facts` | merchant-declared account-pool and refund-commitment facts |
| `000073_growth_analytics` | privacy-safe user attribution, daily activity, and first-publication facts |
| `000074_promotion_rewards` | referral campaigns, invite relations, and single-use promotion coupons |
| `000075_api_order_public_numbers` | immutable human-facing API order numbers |
| `000076_api_delivery_credential_destruction` | explicit irreversible destruction state for retained API delivery credential payloads |
| `000077_moderation_info_requests` | participant-targeted moderation information requests and immutable supplements |
| `000078_account_appeal_sessions` | dedicated account-governance appeal sessions for suspended or banned users |
| `000079_api_market_probe_and_quota_limits` | API SKU quota usage policies, immutable purchase snapshots, authorized platform probes, and probe samples |
| `000080_api_prompt_audit_and_publish_contract` | nullable seller prompt-audit declarations and immutable purchase/order snapshots, plus independent historical performance fields |
| `000081_api_probe_connections_and_model_keys` | reusable seller probe connections, frozen order delivery targets, and canonical API model keys |
| `000082_api_probe_real_model_health` | real streaming model probes, attempt and usage facts, model-change history, and immutable latency rules |
| `000083_carpool_daily_weekly_quota` | renames carpool weekly/monthly quota fields to daily/weekly while preserving existing values |
| `000084_api_order_dispute_status_projection` | adds negotiation and remediation phases while keeping API-order dispute state as a synchronized projection |
| `000085_api_order_dispute_negotiation` | adds structured API-order requests, immutable participant messages, and bilateral settlement proposals |
| `000086_announcement_content_updated_at` | separates user-visible announcement content updates from general record mutation time |
| `000087_api_order_dispute_remedies` | adds auditable post-ruling remedy requirements, claims, beneficiary responses, neutral timeouts, and administrator-confirmed overdue facts |
| `000088_api_order_dispute_sanctions` | links seller restrictions to overdue remedies and indexes the 180-day confirmed-breach window |
| `000089_api_order_after_sales_contacts` | freezes ordered API-service contact selections and records after-sales issue occurrence time |
| `000090_linuxdo_contact_single_mapping` | consolidates enabled linux.do contacts to the identity-bound account mapping while preserving historical transaction snapshots |
| `000091_student_identity_and_auth_link` | durable student identities, purpose-isolated registration challenges, and recent-password linux.do linking |
| `000092_operation_audit_projection` | unified operation-audit projection indexes and source event references |
| `000093_contact_usage_scopes` | canonical contact usage scopes for buyer, dispute, carpool owner, and API merchant contexts |
| `000094_account_governance_session_foundation` | immutable governance actions, restricted-business sessions, dedicated restricted/appeal OAuth state tables, exact suspension expiry jobs, and purpose-bound administrator reauthentication |
| `000095_account_governance_business_disposition` | durable governance disposition jobs, unique cross-action resource outcomes, structured governance cancellation facts, and non-restoring sales-stop links |
| `000096_api_order_launch_hardening` | immutable merchant-confirm and delivery deadlines, constrained late-payment recovery, buyer pending-order capacity, and rush-round fulfillment confirmation |
| `000097_dynamic_catalog_lifecycle` | three-state catalog lifecycle, Grok/xAI seed data, immutable core identity, and API-order catalog risk holds |
| `000098_password_reset` | purpose-isolated active password-reset challenge uniqueness for immutable student-email recovery |

The current runnable Go slice supports both in-memory tests and PostgreSQL runtime.
When `DATABASE_URL` is configured, users, auth sessions, idempotency, product
catalog reads, official price leads/records, contact methods, contact sessions,
contact access logs, carpool listings, carpool cycle terms, carpool applications,
join confirmations, memberships, completion confirmations, API model catalog
reads, API service publishing/review/moderation reads and writes, API
purchase-intent creation/lifecycle reads and writes, native username/password
login credentials, profile privacy fields,
merchant profile public reads, announcements, favorites, unified
transaction reviews, reports, dispute cases, appeals, dispute events, and
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
notes. Version 13 initially constrained `distribution_system='sub2api'` service
models to `merchant_multiplier=1.0000`. Version 51 removes that constraint: the
default remains `1.0000`, while merchants may declare any positive multiplier
that reflects their actual upstream conversion.

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
search text expressions for API services/models, carpool listings, the historical demand table,
product-plan text used by official price search, public users/linux.do
usernames, merchant profiles, and API model catalog rows. These indexes are
performance support only; the demand index is removed with the table in Version
65. They do not change search visibility predicates or response DTOs. Use
`scripts/explain-search.sql` from the repository root to
verify that global search predicates keep matching the expression indexes.

Version 36 realigns the merchant-profile trigram index to `lower(display_name)`
so store-alias API service search can use the index while preserving the public
search contract that matches and displays public merchant display names only.

Version 38 adds `api_services.quota_expires_at` for metered USD quota listings.
Metered API quota services must have a fixed future expiration timestamp, and
public orderable predicates exclude expired quota listings.

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

Version 48 upgrades databases that applied the earlier Version 22 definition
before report canonical-target fields, dispute result codes, and moderation audit
rows were added. It backfills non-sensitive report snapshots and canonical
targets, archives earlier duplicate active reports before creating the canonical
unique index, and then adds the missing constraints, indexes, and audit table.
The upgrade is idempotent for fresh databases whose current Version 22 baseline
already contains those objects. Its down migration is intentionally non-destructive
because those objects are owned by the current Version 22 baseline.

Version 49 separates a metered service's total available USD allowance from its
per-order maximum. Creating an API order reserves allowance atomically and
freezes the requested allowance, CNY exchange rate, and pricing JSON on the
order. Cancelling or expiring an unpaid order releases the reservation; paid
orders retain it through delivery and completion.

Version 50 adds the `payment_issue` API-order state and structured mismatch
reasons (`not_received`, `amount_mismatch`, `remark_mismatch`). A seller may
report an issue only after the buyer submits payment information. The buyer can
then supplement and resubmit that information, returning the order to
`payment_submitted`; the existing quota reservation remains held throughout.

Version 51 turns fixed API packages into limited-duration inventory. Packages
store panel allowance and total/available stock, reference a subset of stable
service-model rows, and reserve one unit atomically when an order is created.
Unpaid cancellation or timeout releases the unit; payment confirmation consumes
it. Fixed-package orders freeze a delivery-based expiry timestamp. This version
also removes the historical Sub2API fixed-`1.0000` model-multiplier constraint;
`1.0000` remains the default rather than a forced value.

Version 52 repairs databases that still retain an earlier anonymous
`api_purchase_intents` status constraint alongside the canonical named
constraint. It drops both legacy and canonical variants, then recreates one
named constraint that accepts the order-backed `ordered` state. The down
migration is intentionally non-destructive because restoring the anonymous
constraint would reintroduce the production failure this migration removes.

Version 54 adds seller-declared quota batches and fixed USD quota offers without
changing the existing Sub2API free-amount purchase path. Scheduled sales allocate
one durable inventory row per copy, and `(sale_round_id, buyer_user_id)` claims
enforce the cross-offer one-copy limit. Quota-offer orders freeze price, allowance,
multiplier, cutoff, expiration, performance declaration, and delivery-mode fields.
At this historical step, the database enforces the one-hour hard sale cutoff and
restores the fixed `1.0000` Sub2API service-model constraint. Version 66 removes
that obsolete constraint while preserving the positive multiplier contract.

Version 55 adds encrypted pre-imported credential inventory for quota offers.
Rows use the existing one-time delivery shapes, keyed fingerprints, explicit
available/reserved/delivered/retired states, and one current reservation per
order. Raw API keys and passwords are never stored in plaintext and do not
belong in public, list, notification, event, log, or idempotency payloads.

Version 53 adds throttled sliding renewal for login sessions. New sessions have
a seven-day idle expiry and a thirty-day absolute expiry. Authenticated requests
may renew at most once every twenty-four hours through a conditional PostgreSQL
update; revoked, idle-expired, and absolute-expired sessions are never extended.

Version 59 replaces new `carpool_reviews` writes with `transaction_reviews` and
`transaction_review_revisions`. Completed carpool memberships and completed API
orders support one buyer-to-seller and one seller-to-buyer review within fourteen
days. The first review remains sealed; the paired submission publishes and
freezes both rows atomically, while an eligible read materializes a lone expired
review at its deadline. Published content is immutable, administrator removal
changes only audited removal state, and active reputation exclusions suppress
eligibility/public reads. Existing `carpool_reviews` are copied without changing
their IDs, content, or timestamps and receive one `migrated` revision.

Version 60 adds the rebuildable `user_reputation_states` cache and append-only
`user_reputation_history`. Snapshots are keyed by user, buyer/seller role, and
overall/carpool/API scope. They retain the `reputation-v1` rule version, source
fact timestamp, dirty marker, and next time-driven recalculation boundary.
Database triggers invalidate existing snapshots when transaction participants,
reviews and their platform prior, disputes, outcomes, restrictions, exclusions,
or linux.do bindings change.

Version 61 adds resource-level source-author verification for carpool listings
and API services. Each resource keeps one versioned verification record with
`not_submitted`, `pending`, `verified`, `mismatch`, or `expired` status, while
every administrator change appends an immutable audit event. Verification and
source-resource changes invalidate the seller's reputation snapshots so
mismatches and time-based expiry are reflected in subsequent reads.

Version 62 adds immutable provenance for the one supported first-admin bootstrap
run. Runtime bootstrap is create-only: a proven matching rerun returns without
changing credentials, while occupied usernames, foreign administrators, and
inconsistent provenance fail closed.

Version 63 invalidates legacy unkeyed bind-email challenges and enforces one
active challenge per user. It adds the `failed` idempotency state, finite
processing/completed/failed retention, and indexes used by the bounded
PostgreSQL lifecycle runner. The runner deletes terminal sessions, terminal
verification challenges, expired idempotency rows, aged notifications, and
unreferenced domain events in bounded batches; it expires ended contact
windows without deleting contact history, access logs, administrator audits,
or dispute audits. The down migration cannot restore invalidated challenges or
response bodies truncated above 64 KiB.

Version 65 removes the prelaunch demand-post table after clearing demand
idempotency resource references. Its down migration recreates only the empty
historical table and indexes; deleted development data is not recoverable.

Version 66 removes the canonical or legacy anonymous checks that forced
Sub2API service-model multipliers to `1.0000`. Version 54 remains immutable as
historical migration state; the new forward migration makes existing and fresh
databases converge on the current positive merchant-declared multiplier
contract without rewriting business data. Rolling Version 66 down can fail
explicitly when non-`1.0000` Sub2API model rows already exist.

Version 67 (`000067_api_account_payment_settings`) adds account-level API
payment settings for WeChat Pay and Alipay. It normalizes legacy API service
snapshots to at most one enabled payment method, adds database constraints that
keep both account and service settings mutually exclusive, and backfills each
owner from the most recently updated enabled service snapshot. Account changes
apply only to future service snapshots; existing services and orders remain
unchanged.

Version 68 (`000068_api_order_delivery_review`) adds a 24-hour buyer review
window after API-order delivery, a one-time review reminder marker, and an
explicit `buyer_confirmed` or `auto_completed` completion source. Existing
delivered orders receive a fresh 24-hour window at migration time; existing
completed orders retain their delivery history and are marked as buyer
confirmed.

Version 69 (`000069_carpool_usage_signals`) adds nullable weekly quota,
official-reset, VPS region, mainland direct-connection, opening-channel, and
payment-method fields to carpool listings. New listing writes require all
signals in service validation; nullable columns preserve explicit `未声明`
rendering for development rows created before this contract.

Version 70 (`000070_admin_user_directory_governance`) adds the partial recent
audit lookup index used by the administrator account-detail surface. Account
status and administrator-permission changes continue to use the existing users,
permissions, sessions, domain events, notifications, audit, and idempotency
tables; the migration does not rewrite account data.

Version 71 (`000071_api_service_promotions`) adds administrator-owned API
service promotion schedules with half-open time ranges, stop facts, optimistic
versions, and indexes for placement capacity and same-service overlap checks.
Promotion history is independent from API service review, publication,
reputation, badges, natural ordering, payments, and analytics storage.

Version 72 (`000072_api_service_commercial_facts`) adds a single
merchant-declared API account-pool type with an optional custom public label
and a structured merchant full-refund commitment. Historical services keep a
null account pool until revised; new service validation requires one. It also
renames recommended concurrency to merchant-declared maximum concurrency
across API services and limited-quota order snapshots without changing the
stored numeric values. The platform snapshots the merchant promise but does
not escrow, fund, or execute refunds.

Version 73 (`000073_growth_analytics`) adds a stable random analytics identifier
for registered users, immutable registration attribution facts, and daily
activity rows. It also records the first publication time for carpool listings
and API services with database triggers that preserve the original timestamp,
so growth windows do not shift when a listing or service is edited later.

Version 74 (`000074_promotion_rewards`) adds a disabled-by-default API-service
referral campaign, stable invite codes, immutable referral relations, and
single-use promotion coupons with activation facts. Reward promotions use a
separate rotating pool and do not consume administrator promotion capacity or
change API-service review, reputation, badges, stock, price, or natural order.

Version 75 (`000075_api_order_public_numbers`) adds immutable public API order
numbers in the `API-YYYYMMDD-XXXXXXXXXX` format. Existing orders are backfilled
from each order's Asia/Shanghai creation date with stable collision handling;
new writes use cryptographically random suffixes. The UUID remains the internal
primary key, route key, and relation key.

Version 76 (`000076_api_delivery_credential_destruction`) adds explicit live
and destroyed states to order delivery credentials and pre-imported quota
credentials. Eligible maintenance runs irreversibly null credential payloads
and fingerprints after the configured retention period while preserving only
non-sensitive audit facts. Open disputes and submitted appeals hold order
credential destruction; available and reserved inventory is never selected.

Version 77 (`000077_moderation_info_requests`) adds participant-targeted
administrator information requests and immutable, idempotent user supplements.
Only the requested active case participant may answer an open request, and the
answer marks the request as answered without resolving the report or dispute.

Version 78 (`000078_account_appeal_sessions`) adds a dedicated fifteen-minute
session for suspended or banned users who prove an existing linux.do identity.
Only opaque session and CSRF hashes are stored. The migration also adds the
`account_governance` appeal target with no report/dispute source and enforces at
most one submitted account-governance appeal per user.

Version 79 (`000079_api_market_probe_and_quota_limits`) adds explicit 5-hour
and UTC+8 calendar-day USD usage policies to free-quota services, fixed
packages, and limited-quota offers, with matching purchase-intent and order
snapshots. It also stores one encrypted, seller-dedicated platform probe
configuration per API service, exact-origin authorization audit events, and
five-minute bounded probe samples. Probe results remain observational and do
not change listing, ordering, dispute, fulfillment, or recommendation state.

Version 80 (`000080_api_prompt_audit_and_publish_contract`) adds nullable
seller prompt-audit declarations to API services and freezes the declaration
on purchase intents and orders. New service writes require an explicit true or
false value while historical rows remain null. It also removes the grouped
performance-declaration constraint so new writes can keep maximum concurrency
without inventing seller TTFT or confirmation timestamps; the historical
columns and their existing values remain intact.

Version 81 (`000081_api_probe_connections_and_model_keys`) replaces
service-scoped probe configuration with reusable seller-owned connections.
Services and orders retain exact Base URL snapshots, while API model catalog,
service, package, and order projections use canonical model keys without
decorative display-name variants.

Version 82 (`000082_api_probe_real_model_health`) upgrades connection checks
from `/models`-only samples to real streaming model probes. It records
per-attempt TTFT, retry, usage, and cost facts, preserves model-change history,
and adds immutable model/protocol/environment latency-rule versions. Existing
local probe connections are disabled for explicit revalidation and incompatible
legacy samples are cleared before the new schema constraints apply.

Version 83 (`000083_carpool_daily_weekly_quota`) corrects the carpool quota
period contract. The former weekly value becomes the daily value, and the
former monthly value becomes the weekly value without rewriting amounts.

Version 84 (`000084_api_order_dispute_status_projection`) extends dispute-case
and API-order dispute status constraints for the V1 negotiation and remediation
workflow. Existing rows are not rewritten; the API-order status remains a
projection of the linked dispute lifecycle.

Version 85 (`000085_api_order_dispute_negotiation`) adds structured API-order
requests, immutable participant messages, and bilateral settlement proposals.
Existing dispute rows are not rewritten, and a proposal can close a dispute
only after the counterparty confirms the same pending proposal.

Version 87 (`000087_api_order_dispute_remedies`) adds auditable post-ruling
remedy records for API-order disputes. Responsibility, action, amount, due
dates, fulfillment claims, counterparty responses, neutral confirmation
timeouts, and administrator-confirmed overdue facts remain separate from the
dispute case status and API-order lifecycle.

Version 88 (`000088_api_order_dispute_sanctions`) links an administrator-applied
API-order seller restriction directly to its overdue remedy, prevents a second
restriction from consuming the same overdue fact, and indexes confirmed seller
breaches for the rolling 180-day recommendation window.

Version 89 (`000089_api_order_after_sales_contacts`) adds ordered API-service
contact associations, immutable purchase-intent contact-version snapshots, and
an optional API-order dispute issue occurrence time. Existing services and
intents are backfilled from their legacy single-contact fields without exposing
plaintext contact values.

Version 90 (`000090_linuxdo_contact_single_mapping`) makes the account's
linux.do binding the single enabled linux.do transaction-contact mapping. It
repoints mutable API-service and carpool references, disables deterministic
duplicates, and adds a per-user partial unique index while preserving frozen
purchase-intent contact versions and historical snapshots.

Version 91 (`000091_student_identity_and_auth_link`) adds the default-off
student-registration singleton, immutable exact institution domains,
append-only lifetime student-email claims, purpose-isolated registration
challenge constraints, recent password reauthentication, and one-time
session-bound linux.do link state. Its down migration refuses to remove a
durable student identity after any claim exists.

Version 92 (`000092_operation_audit_projection`) evolves the existing probe
model-change history into one append-only probe operation ledger while
preserving legacy model facts and compatibility writes. It also adds cursor
indexes for the eight allowlisted authorities consumed by the administrator-only
unified operation-history projection. The projection never copies source rows
into a second universal audit table and does not expose raw metadata, reasons,
credentials, contact values, or request bodies.

Version 93 (`000093_contact_usage_scopes`) persists canonical contact-method
usage scopes. Existing methods retain all four historical scopes
(`carpool_owner`, `api_merchant`, `buyer`, and `dispute`), while newly inserted
rows default to the buyer/dispute pair. A database check enforces a non-empty,
deduplicated, allowlisted, canonically ordered scope array. The guarded down
migration refuses to erase policy once a row carries a post-migration scope set.

Version 96 (`000096_api_order_launch_hardening`) adds immutable merchant-confirm
and delivery deadlines, a constrained late-payment recovery fact, buyer pending
order capacity indexes, and seller fulfillment confirmation for fixed rush
rounds. Historical orders and non-system rounds remain readable with null fields.

## Contact Retention And Destruction

Contact method deletion retires the mutable contact method surface. Historical
business rows keep frozen contact method version references where the product
requires a dispute/audit trail. Carpool contact sessions and API purchase intents
can continue to resolve their frozen versions only through authorized business
reads, and those reads must use `Cache-Control: no-store` and write access logs
where applicable. Access logs and domain events store identifiers and side
metadata only, never plaintext contact values.

Physical destruction of historical contact-method ciphertext remains outside
this migration set because it must be coordinated with frozen contact-version
references and key-rotation operations. API order delivery credentials are a
narrower exception: Version 76 adds explicit `destroyed_at` semantics and
irreversibly clears eligible order and pre-imported credential payloads while
preserving non-sensitive audit facts.

`000007_seed_catalog_risk_and_policy.down.sql` removes only fixed seed UUIDs. If
business rows already reference those seed plans, PostgreSQL foreign keys are
expected to block rollback instead of deleting referenced catalog data.

Version 97 (`000097_dynamic_catalog_lifecycle`) replaces catalog `active`
booleans with `active | deprecated | blocked` lifecycle state, adds immutable
core identity markers, seeds Grok/xAI, and persists independent API-order
catalog risk holds. Its down migration refuses rollback after blocked state or
risk-hold data has been used.

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
