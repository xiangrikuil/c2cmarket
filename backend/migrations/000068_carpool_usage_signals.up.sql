-- Persist required carpool quota, reset, connection, channel, and payment signals.
-- Date: 2026-08-01
-- Executor: Codex

ALTER TABLE carpool_listings
ADD COLUMN weekly_quota_amount numeric(12,2),
ADD COLUMN follows_official_quota_reset boolean,
ADD COLUMN vps_region text,
ADD COLUMN supports_mainland_china_direct_connection boolean,
ADD COLUMN opening_channel_code text,
ADD COLUMN custom_opening_channel text,
ADD COLUMN payment_method_code text,
ADD COLUMN custom_payment_method text,
ADD CONSTRAINT ck_carpool_listings_weekly_quota_positive
  CHECK (weekly_quota_amount IS NULL OR weekly_quota_amount > 0),
ADD CONSTRAINT ck_carpool_listings_vps_region_not_blank
  CHECK (vps_region IS NULL OR btrim(vps_region) <> ''),
ADD CONSTRAINT ck_carpool_listings_opening_channel_code
  CHECK (opening_channel_code IS NULL OR opening_channel_code IN ('web', 'ios_app_store', 'google_play', 'team_seat', 'other')),
ADD CONSTRAINT ck_carpool_listings_custom_opening_channel
  CHECK (
    opening_channel_code IS NULL
    OR (opening_channel_code = 'other' AND custom_opening_channel IS NOT NULL AND btrim(custom_opening_channel) <> '')
    OR (opening_channel_code <> 'other' AND custom_opening_channel IS NULL)
  ),
ADD CONSTRAINT ck_carpool_listings_payment_method_code
  CHECK (payment_method_code IS NULL OR payment_method_code IN ('credit_card', 'virtual_card', 'apple_pay', 'google_pay', 'app_store_gift_card', 'google_play_gift_card', 'paypal', 'u_card', 'other')),
ADD CONSTRAINT ck_carpool_listings_custom_payment_method
  CHECK (
    payment_method_code IS NULL
    OR (payment_method_code = 'other' AND custom_payment_method IS NOT NULL AND btrim(custom_payment_method) <> '')
    OR (payment_method_code <> 'other' AND custom_payment_method IS NULL)
  );
