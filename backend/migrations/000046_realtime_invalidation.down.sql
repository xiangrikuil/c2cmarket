-- Remove realtime invalidation triggers without touching business data.
-- 日期：2026-07-11
-- 执行者：Codex

DROP TRIGGER IF EXISTS appeals_realtime_admin ON appeals;
DROP TRIGGER IF EXISTS dispute_cases_realtime_admin ON dispute_cases;
DROP TRIGGER IF EXISTS reports_realtime_admin ON reports;
DROP TRIGGER IF EXISTS feedback_tickets_realtime_admin ON feedback_tickets;
DROP TRIGGER IF EXISTS api_services_realtime_admin ON api_services;
DROP TRIGGER IF EXISTS carpool_listings_realtime_admin ON carpool_listings;
DROP TRIGGER IF EXISTS official_price_leads_realtime_admin ON official_price_leads;
DROP TRIGGER IF EXISTS notifications_realtime_user ON notifications;

DROP FUNCTION IF EXISTS c2c_notify_realtime_admin();
DROP FUNCTION IF EXISTS c2c_notify_realtime_user();
