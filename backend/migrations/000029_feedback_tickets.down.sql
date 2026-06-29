-- Roll back feedback loop ticket contract.
-- 日期：2026-06-26
-- 执行者：Codex

DROP INDEX IF EXISTS ix_feedback_events_actor_created;
DROP INDEX IF EXISTS ix_feedback_events_ticket_created;
DROP TABLE IF EXISTS feedback_events;

DROP INDEX IF EXISTS ix_feedback_tickets_unread_submitter;
DROP INDEX IF EXISTS ix_feedback_tickets_admin_status_updated;
DROP INDEX IF EXISTS ix_feedback_tickets_submitter_updated;
DROP TABLE IF EXISTS feedback_tickets;
