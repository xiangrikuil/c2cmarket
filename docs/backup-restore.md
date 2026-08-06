# PostgreSQL Backup And Restore

Date: 2026-07-26
Author: Codex

## Policy

The checked-in systemd timer is configured to run the production backup once
per day at 03:30 Asia/Shanghai and uses `Persistent=true` to catch up a missed
run after the timer becomes active again. This is the repository's intended
schedule, not proof that the timer is installed, enabled, or succeeding on the
VPS.

The repository does not define an approved RPO or RTO. Define and approve these
values before launch:

```text
RPO=<MAXIMUM_ACCEPTABLE_DATA_LOSS>
RTO=<MAXIMUM_ACCEPTABLE_RESTORE_TIME>
BACKUP_OWNER=<TEAM_OR_ROLE>
```

The daily timer interval does not by itself establish the RPO. Backup failures,
upload failures, and the age of the newest validated remote object must be
included in the RPO measurement. The current operations policy calls for a
manual isolated restore drill at least monthly; no automated restore-drill job
is checked in.

The repository backup script creates a PostgreSQL custom-format dump, a SHA-256
checksum, uploads both to the configured R2 remote, and only then removes local
copies older than `LOCAL_RETENTION_DAYS` (default 7 days). The 30-day R2
retention target requires a Bucket Lifecycle configured and verified outside
the repository.

Backups contain encrypted contact and credential data. Store contact keyrings,
verification pepper, OAuth/SMTP credentials, and R2 access credentials in a
separate secret manager. A database dump alone is not a complete recoverable
system, but secrets must never be copied into the dump directory.

## Backup Execution

Use the ignored production environment file without printing it:

```bash
ENV_FILE=<IGNORED_PRODUCTION_ENV_PATH> \
COMPOSE_PROJECT=<PRODUCTION_COMPOSE_PROJECT> \
BACKUP_DIR=<RESTRICTED_LOCAL_BACKUP_DIRECTORY> \
R2_REMOTE=<RCLONE_REMOTE> \
R2_BUCKET=<R2_BUCKET> \
R2_PREFIX=<R2_PREFIX> \
scripts/backup-production-postgres.sh
```

An operator or scheduler must fail the backup job when the command exits
non-zero. Alert when no new remote dump and checksum arrive inside the approved
RPO window.

## Backup Validation

For every backup:

1. Confirm both `.dump` and `.dump.sha256` exist remotely and are non-empty.
2. Download them into a restricted temporary directory.
3. Run `shasum -a 256 -c <BACKUP_NAME>.sha256`.
4. Run `pg_restore --list <BACKUP_NAME>.dump` with PostgreSQL 18 tooling.
5. Record dump timestamp, size, checksum result, PostgreSQL tool version, and
   validation result. Do not record database URLs or secret values.

A checksum and archive listing are necessary but not sufficient. Only a
successful isolated restore drill proves logical recoverability.

## Isolated Restore Drill

Never restore a drill over production. Use a new PostgreSQL 18 container,
cluster, and database named clearly for the drill.

```bash
docker run --detach \
  --name c2c-restore-drill \
  --publish 127.0.0.1::5432 \
  --env POSTGRES_USER=c2c_restore \
  --env POSTGRES_PASSWORD=<RESTORE_DRILL_ONLY_PASSWORD> \
  --env POSTGRES_DB=c2c_restore_drill \
  postgres:18-alpine@sha256:96d56f7f57c6aacd1fcb908bc83b345ec5f83231ee486dd66a1baadce274db88
```

After readiness, copy the verified dump into that container and restore it into
the empty drill database with `pg_restore --no-owner --no-privileges`. Then:

1. Query `schema_migrations`; record the restored backup's migration version
   and require `dirty=false`. The version may be older than the current release.
2. Run the release's forward migrations against the drill database, then
   require `version=80` and `dirty=false`. Never edit the restored migration
   row or skip migrations to force the expected value.
3. Run referential-integrity and representative row-count checks.
4. Start the exact release backend image against the drill database using
   restore-drill-only configuration.
5. Require `/readyz` success and confirm `/version` matches the image.
6. Exercise controlled OAuth identity lookup, session validation, encrypted
   contact decryption, and representative read-only list/detail operations.
7. Record measured restore time against RTO and the newest restored timestamp
   against RPO.
8. Destroy the isolated drill resources after evidence is retained.

If contact decryption reports an unknown key version, stop: recover the matching
keyring from the secret manager. Do not rewrite ciphertext or substitute a new
key to make the drill pass.

## Production Recovery

Production recovery requires an incident commander and explicit target
selection. Before restore:

- stop application writers or route traffic away;
- preserve the failed cluster for investigation;
- choose the newest validated backup inside RPO;
- verify checksum and required keyring versions;
- create a new recovery database when possible instead of overwriting the
  failed one;
- record every command, timestamp, and result without credentials.

After restore, require migration `80:false`, `/readyz`, `/version`, metrics,
controlled authentication, and read-only business checks before reopening
traffic. Resume backup scheduling only after the recovered system is stable.
