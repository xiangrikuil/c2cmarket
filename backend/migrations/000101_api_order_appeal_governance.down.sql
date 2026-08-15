-- Remove appeal governance only while no post-migration finality would be lost.
-- Date: 2026-08-15
-- Executor: Codex

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM dispute_cases
    WHERE final_reason NOT IN ('', 'legacy_closed', 'legacy_resolved')
  ) OR EXISTS (
    SELECT dispute_case_id FROM dispute_reputation_outcomes
    GROUP BY dispute_case_id HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'cannot roll back appeal governance after new finality or multi-subject outcomes exist';
  END IF;
END $$;

ALTER TABLE dispute_reputation_outcomes
DROP CONSTRAINT IF EXISTS ux_dispute_reputation_outcomes_case_subject,
ADD CONSTRAINT dispute_reputation_outcomes_dispute_case_id_key UNIQUE (dispute_case_id);

DROP INDEX IF EXISTS ix_dispute_cases_appeal_expiry;

ALTER TABLE dispute_cases
DROP CONSTRAINT IF EXISTS ck_dispute_cases_appeal_after_final,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_appeal_window,
DROP CONSTRAINT IF EXISTS ck_dispute_cases_final_reason,
DROP COLUMN IF EXISTS adversely_affected_user_ids,
DROP COLUMN IF EXISTS appeal_expires_at,
DROP COLUMN IF EXISTS final_reason;
