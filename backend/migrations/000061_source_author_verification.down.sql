-- 回滚资源级原帖作者验证、审计事件和信誉快照失效联动。
-- 日期：2026-07-24
-- 执行者：Codex

DROP TRIGGER IF EXISTS trg_api_services_source_reputation_dirty ON api_services;
DROP TRIGGER IF EXISTS trg_carpool_listings_source_reputation_dirty ON carpool_listings;
DROP FUNCTION IF EXISTS dirty_reputation_for_source_resource();

DROP TRIGGER IF EXISTS trg_source_author_verifications_reputation_dirty ON source_author_verifications;
DROP FUNCTION IF EXISTS dirty_reputation_for_source_author_verification();

DROP TRIGGER IF EXISTS trg_source_author_verification_events_append_only ON source_author_verification_events;
DROP FUNCTION IF EXISTS reject_source_author_verification_event_mutation();

DROP TABLE IF EXISTS source_author_verification_events;
DROP TABLE IF EXISTS source_author_verifications;
