-- 回滚版本化信誉快照、历史和事实变更失效触发器。
-- 日期：2026-07-24
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_linux_do_bindings_reputation_dirty ON linux_do_bindings;
DROP FUNCTION IF EXISTS dirty_reputation_for_linux_do_binding();

DROP TRIGGER IF EXISTS trg_user_restrictions_reputation_dirty ON user_restrictions;
DROP FUNCTION IF EXISTS dirty_reputation_for_restriction();

DROP TRIGGER IF EXISTS trg_dispute_outcomes_reputation_dirty ON dispute_reputation_outcomes;
DROP FUNCTION IF EXISTS dirty_reputation_for_outcome();

DROP TRIGGER IF EXISTS trg_dispute_cases_reputation_dirty ON dispute_cases;
DROP FUNCTION IF EXISTS dirty_reputation_for_dispute();

DROP TRIGGER IF EXISTS trg_transaction_reviews_reputation_dirty ON transaction_reviews;
DROP FUNCTION IF EXISTS dirty_reputation_for_review();

DROP TRIGGER IF EXISTS trg_reputation_exclusions_state_dirty ON reputation_transaction_exclusions;
DROP FUNCTION IF EXISTS dirty_reputation_for_exclusion();

DROP TRIGGER IF EXISTS trg_api_order_events_reputation_dirty ON api_order_events;
DROP FUNCTION IF EXISTS dirty_reputation_for_api_order_event();

DROP TRIGGER IF EXISTS trg_api_orders_reputation_dirty ON api_orders;
DROP TRIGGER IF EXISTS trg_carpool_memberships_reputation_dirty ON carpool_memberships;
DROP TRIGGER IF EXISTS trg_carpool_applications_reputation_dirty ON carpool_applications;
DROP FUNCTION IF EXISTS dirty_reputation_for_participant_row();

DROP FUNCTION IF EXISTS mark_transaction_reputation_dirty(text, uuid, timestamptz);
DROP FUNCTION IF EXISTS mark_user_reputation_dirty(uuid, timestamptz);

DROP TRIGGER IF EXISTS trg_user_reputation_history_append_only ON user_reputation_history;
DROP FUNCTION IF EXISTS reject_user_reputation_history_mutation();

DROP TABLE IF EXISTS user_reputation_history;
DROP TABLE IF EXISTS user_reputation_states;
