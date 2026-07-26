-- 回退信誉交易排除与追加审计记录。
-- 日期：2026-07-24
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_reputation_exclusion_events_append_only
ON reputation_transaction_exclusion_events;

DROP FUNCTION IF EXISTS reject_reputation_exclusion_event_mutation();
DROP TABLE IF EXISTS reputation_transaction_exclusion_events;
DROP TABLE IF EXISTS reputation_transaction_exclusions;
