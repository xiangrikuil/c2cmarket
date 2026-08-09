# Ordered Production Release Checklist

Date: 2026-07-26
Author: Codex

This checklist records operator evidence. Do not mark an item complete from an
assumption or a previous release.

## 1. Freeze The Release Input

- [ ] Release ref resolves to the approved full Git SHA.
- [ ] Worktree is clean; no staged, unstaged, or untracked release input.
- [ ] CI `release-gate` passed for that exact SHA.
- [ ] Backend uses Go `1.26.5`; frontend uses Node 24 and pnpm 10.
- [ ] Source archive checksum, backend image digest, OCI labels, and SBOM are recorded.
- [ ] Gitleaks, Trivy filesystem/image, govulncheck, frontend high audit, tests,
      race, OpenAPI, migration, header, and Compose checks passed.

## 2. Validate Configuration

- [ ] Production uses prebuilt `BACKEND_IMAGE`; Compose has no backend build context.
- [ ] PostgreSQL is private and the backend host port is loopback-only.
- [ ] `FRONTEND_ORIGIN`, allowed origins, OAuth callback, SMTP sender, and
      trusted immediate proxy are correct.
- [ ] `METRICS_BEARER_TOKEN`, verification pepper, contact encryption keyring,
      and fingerprint keyring are distinct and supplied from a secret manager.
- [ ] `CONTACT_KEY_VERSION` exists in both keyrings; every stored old version
      remains available.
- [ ] Database pool budget and statement/lock/idle transaction timeouts fit the
      production PostgreSQL connection limit.
- [ ] Frontend build uses `VITE_API_MODE=real` and does not enable mocks.

## 3. Protect Data

- [ ] A fresh custom-format backup and checksum exist in remote storage.
- [ ] Backup checksum and `pg_restore --list` validation passed.
- [ ] Latest isolated PostgreSQL 18 restore drill meets RPO/RTO.
- [ ] Migrations and rollback implications from 62 through current migration 85 were reviewed.
- [ ] No contact re-encryption apply job is part of the deployment.

## 4. Stage The Release

- [ ] Staging migrated from its current version through 85 with `dirty=false`.
- [ ] Staging `/health`, `/readyz`, `/version`, and authenticated `/metrics` pass.
- [ ] Staging OAuth first/repeat login, email verification, limiter responses,
      contact reads, SSE, maintenance, and representative transactions pass.
- [ ] Security headers reach the browser from both API and Pages.
- [ ] Alert routing was tested without putting tokens or contact values in labels.

## 5. Deploy Production

- [ ] Start/verify PostgreSQL before migration.
- [ ] Apply forward migrations once; require `schema_migrations=85:false`.
- [ ] Start the backend from the approved immutable image with `--no-build`.
- [ ] Require `/health`, `/readyz`, and `/version` to match the approved image.
- [ ] Scrape `/metrics` through the restricted authenticated path.
- [ ] Deploy the exact frontend artifact built from the approved SHA.
- [ ] Verify OAuth callback/origin/cookie/CSRF behavior from the public frontend.
- [ ] Verify CSP, frame, permissions, content-type, referrer, and HSTS headers.

## 6. Observe And Close

- [ ] Watch 5xx rate, readiness, migration, pool, slow query, limiter, decrypt,
      maintenance, outbound, OAuth/email, realtime, and idempotency signals.
- [ ] Run controlled non-destructive production checks; never use dev auth or
      fake OAuth against production.
- [ ] If Bootstrap created the first administrator, clear both Bootstrap
      variables and recreate the backend.
- [ ] Record release SHA, image digest, frontend artifact, migration result,
      backup, dashboards, checks, operator, and timestamps.
- [ ] Keep the previous immutable image and frontend artifact available until
      the observation window closes.

## 7. Rollback Decision

- [ ] Stop rollout and route traffic away when readiness, migration, identity,
      data integrity, or decrypt checks fail.
- [ ] Before migrations, restore the previous immutable image and frontend.
- [ ] After migrations, prefer application rollback with the upgraded schema
      when compatibility permits; do not run down migrations automatically.
- [ ] After any `-apply` contact re-encryption, never remove migration 64 format
      metadata or old keys as part of rollback.
- [ ] For suspected data corruption, preserve the failed database and follow
      [`backup-restore.md`](./backup-restore.md).
