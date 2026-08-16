-- Add explicit dispute finality, a fixed appeal window, and adverse-party snapshots.
-- Date: 2026-08-15
-- Executor: Codex

ALTER TABLE dispute_cases
ADD COLUMN final_reason text NOT NULL DEFAULT '',
ADD COLUMN appeal_expires_at timestamptz,
ADD COLUMN adversely_affected_user_ids uuid[] NOT NULL DEFAULT '{}'::uuid[];

UPDATE dispute_cases
SET final_reason = CASE WHEN status = 'closed' THEN 'legacy_closed' ELSE 'legacy_resolved' END,
    appeal_expires_at = COALESCE(closed_at, resolved_at, updated_at) + interval '30 days',
    adversely_affected_user_ids = CASE
      WHEN subject_user_id IS NOT NULL THEN ARRAY[subject_user_id]
      ELSE array_remove(ARRAY[primary_user_id, counterparty_user_id], NULL)
    END
WHERE active = false AND status IN ('resolved', 'closed');

ALTER TABLE dispute_cases
ADD CONSTRAINT ck_dispute_cases_final_reason
  CHECK (final_reason = '' OR final_reason ~ '^[a-z][a-z0-9_]{1,63}$'),
ADD CONSTRAINT ck_dispute_cases_appeal_window CHECK (
  (active = true AND appeal_expires_at IS NULL AND final_reason = '' AND cardinality(adversely_affected_user_ids) = 0)
  OR (
    active = false AND status IN ('resolved', 'closed')
    AND appeal_expires_at IS NOT NULL AND final_reason <> ''
    AND cardinality(adversely_affected_user_ids) > 0
  )
  OR (
    active = false AND status NOT IN ('resolved', 'closed')
    AND appeal_expires_at IS NULL AND final_reason = ''
    AND cardinality(adversely_affected_user_ids) = 0
  )
),
ADD CONSTRAINT ck_dispute_cases_appeal_after_final CHECK (
  appeal_expires_at IS NULL
  OR appeal_expires_at = COALESCE(closed_at, resolved_at, updated_at) + interval '30 days'
);

CREATE INDEX ix_dispute_cases_appeal_expiry
ON dispute_cases(appeal_expires_at, id)
WHERE appeal_expires_at IS NOT NULL;

ALTER TABLE dispute_reputation_outcomes
DROP CONSTRAINT IF EXISTS dispute_reputation_outcomes_dispute_case_id_key,
ADD CONSTRAINT ux_dispute_reputation_outcomes_case_subject UNIQUE (dispute_case_id, subject_user_id);
