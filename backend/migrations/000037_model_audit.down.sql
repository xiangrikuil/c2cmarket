DROP INDEX IF EXISTS ix_model_audit_monitors_enabled;
DROP TABLE IF EXISTS model_audit_scheduled_monitors;

DROP INDEX IF EXISTS ix_model_audit_passive_target_created;
DROP TABLE IF EXISTS model_audit_passive_call_features;

DROP INDEX IF EXISTS ix_model_audit_probe_scores_run;
DROP TABLE IF EXISTS model_audit_probe_scores;

DROP INDEX IF EXISTS ix_model_audit_samples_run_probe;
DROP TABLE IF EXISTS model_audit_samples;

ALTER TABLE IF EXISTS model_audit_targets
DROP CONSTRAINT IF EXISTS model_audit_targets_last_run_id_fkey;

DROP INDEX IF EXISTS ix_model_audit_runs_target_created;
DROP TABLE IF EXISTS model_audit_runs;

DROP INDEX IF EXISTS ix_model_audit_baselines_model;
DROP TABLE IF EXISTS model_audit_baselines;

DROP INDEX IF EXISTS ix_model_audit_targets_enabled;
DROP TABLE IF EXISTS model_audit_targets;
