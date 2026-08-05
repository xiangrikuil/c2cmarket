# Production Operations

Date: 2026-07-26
Author: Codex

## Runtime Endpoints

| Endpoint | Purpose | Expected exposure |
| --- | --- | --- |
| `GET /health` | Process liveness; no database dependency | Public or proxy health check |
| `GET /readyz` | PostgreSQL reachability and migration `80:false` | Operator/internal |
| `GET /version` | Version, Git SHA, build time, migration target | Operator/internal |
| `GET /metrics` | Prometheus/OpenMetrics runtime data | Internal plus bearer authentication |

Production metrics requests require:

```bash
curl --fail --silent --show-error \
  --header "Authorization: Bearer ${METRICS_BEARER_TOKEN}" \
  http://127.0.0.1:<BACKEND_PORT>/metrics
```

Do not put the token in a shared shell history or monitoring label. A missing or
invalid token returns `401` with `WWW-Authenticate`.

## PostgreSQL Connection Budget

Set pool values per application instance, then verify:

```text
(backend_instances * DB_MAX_CONNS)
  + migration_connections
  + backup_connections
  + operator_reserve
  <= PostgreSQL max_connections
```

`DB_MIN_CONNS` is a warm floor, not a reservation outside `DB_MAX_CONNS`.
Start conservatively, preserve an operator reserve, and increase only with pool
wait evidence. Also configure max lifetime, idle lifetime, health period,
statement timeout, lock timeout, and idle-in-transaction timeout from the
environment examples. Never disable these timeouts to mask a slow query.

## Metrics And Initial Alerts

All application metrics use the `c2c_market_` namespace and bounded labels.
Initial paging alerts should include:

- `c2c_market_database_ready == 0` for 2 minutes.
- `c2c_market_database_migration_dirty == 1` immediately.
- Current migration version differs from expected.
- `c2c_market_database_observability_up == 0` for 5 minutes.
- Pool total/max above 85% with sustained acquire wait growth.
- `c2c_market_database_slow_active_queries > 0` for 5 minutes.
- Any increase in `contact_decrypt_total{result="unknown_key"}`.
- Repeated `maintenance_runs_total{result="failure"}` without a later success.
- Sustained increases in rate-limit `limited` or `capacity_limited` decisions.
- OAuth callback or email verification 5xx failures.
- Realtime listener failures or an unexpected drop in active SSE connections.

Warning alerts should cover outbound rejection spikes, idempotency conflicts,
maintenance duration growth, and 5xx request rate by chi route pattern. Do not
group or label metrics by raw URL, user ID, request ID, contact value, or token.

## Maintenance And Retention

The maintenance runner executes at startup and every `MAINTENANCE_INTERVAL`.
A PostgreSQL advisory transaction lock permits one active runner across
instances. Each data class processes at most `MAINTENANCE_BATCH_SIZE` rows per
run.

Defaults:

| Data | Retention/action |
| --- | --- |
| Expired or revoked auth sessions | 168 hours |
| Consumed or expired email challenges | 24 hours |
| Read notifications | 2160 hours |
| Unread notifications | 8760 hours |
| Unreferenced domain events | 8760 hours |
| Expired contact windows | Mark expired; keep encrypted history and access audit |
| Administrator and moderation audit | Retained |

Changes to retention require legal/product review, a bounded integration test,
and a backup/restore assessment. Monitor skipped runs; a skip caused by another
healthy instance holding the advisory lock is expected, while repeated failures
are not.

## Logs And Triage

Search logs by request ID and fixed event fields. A request ID containing
control characters is rejected and replaced before logging. Treat any
multi-line or non-JSON application request record as a logging defect.

Triage order:

1. Check `/health`, then `/readyz`, then `/version`.
2. Compare image OCI revision to `/version.gitCommit`.
3. Check migration current/expected/dirty metrics.
4. Check database pool saturation and slow active queries.
5. Check maintenance, outbound, limiter, decrypt, OAuth, email, and realtime
   counters for the affected time window.
6. Use redacted logs for the request ID; never dump environment files or
   database rows containing ciphertext or credentials.

Follow [`ops/deployment-runbook.md`](./ops/deployment-runbook.md) for release
and rollback, and [`backup-restore.md`](./backup-restore.md) for restore drills.
