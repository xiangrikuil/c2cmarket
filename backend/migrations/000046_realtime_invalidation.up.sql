-- Commit-aware realtime invalidation signals for user inbox and admin queues.
-- 日期：2026-07-11
-- 执行者：Codex

CREATE FUNCTION c2c_notify_realtime_user()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify(
    'c2c_market_realtime',
    json_build_object(
      'v', 1,
      'audience', 'user',
      'userId', NEW.user_id::text
    )::text
  );
  RETURN NEW;
END;
$$;

CREATE FUNCTION c2c_notify_realtime_admin()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify(
    'c2c_market_realtime',
    json_build_object('v', 1, 'audience', 'admin')::text
  );
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER notifications_realtime_user
AFTER INSERT OR UPDATE OF read_at ON notifications
FOR EACH ROW
EXECUTE FUNCTION c2c_notify_realtime_user();

CREATE TRIGGER official_price_leads_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON official_price_leads
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER carpool_listings_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON carpool_listings
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER api_services_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON api_services
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER feedback_tickets_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON feedback_tickets
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER reports_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON reports
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER dispute_cases_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON dispute_cases
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();

CREATE TRIGGER appeals_realtime_admin
AFTER INSERT OR UPDATE OR DELETE ON appeals
FOR EACH STATEMENT
EXECUTE FUNCTION c2c_notify_realtime_admin();
