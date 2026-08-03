-- Roll back explicit moderation information requests.
-- Date: 2026-08-03
-- Author: Codex

DROP TRIGGER IF EXISTS trg_moderation_info_supplements_append_only ON moderation_info_supplements;
DROP FUNCTION IF EXISTS reject_moderation_info_supplement_mutation();

DROP INDEX IF EXISTS ix_moderation_info_supplements_submitter;
DROP TABLE IF EXISTS moderation_info_supplements;

DROP INDEX IF EXISTS ix_moderation_info_requests_requested_user;
DROP INDEX IF EXISTS ux_moderation_info_requests_open_dispute;
DROP INDEX IF EXISTS ux_moderation_info_requests_open_report;
DROP TABLE IF EXISTS moderation_info_requests;
