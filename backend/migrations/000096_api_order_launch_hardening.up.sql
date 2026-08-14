-- 上线前收口 API 订单履约时限、逾期付款事实与抢购场次确认。
-- 日期：2026-08-14
-- 执行者：Codex

ALTER TABLE api_orders
  ADD COLUMN merchant_confirm_due_at timestamptz,
  ADD COLUMN delivery_due_at timestamptz,
  ADD COLUMN late_payment_status text,
  ADD COLUMN late_payment_reported_at timestamptz,
  ADD COLUMN late_payment_note text,
  ADD COLUMN late_payment_resolved_at timestamptz,
  ADD CONSTRAINT ck_api_orders_merchant_confirm_due
    CHECK (
      merchant_confirm_due_at IS NULL
      OR (
        payment_submitted_at IS NOT NULL
        AND merchant_confirm_due_at > payment_submitted_at
      )
    ),
  ADD CONSTRAINT ck_api_orders_delivery_due
    CHECK (
      delivery_due_at IS NULL
      OR (
        paid_confirmed_at IS NOT NULL
        AND delivery_due_at > paid_confirmed_at
      )
    ),
  ADD CONSTRAINT ck_api_orders_late_payment
    CHECK (
      (
        late_payment_status IS NULL
        AND late_payment_reported_at IS NULL
        AND late_payment_note IS NULL
        AND late_payment_resolved_at IS NULL
      )
      OR (
        late_payment_status = 'reported'
        AND status = 'cancelled'
        AND cancel_reason = 'payment_timeout'
        AND late_payment_reported_at IS NOT NULL
        AND late_payment_resolved_at IS NULL
      )
      OR (
        late_payment_status IN ('not_received', 'received_refund_pending')
        AND status = 'cancelled'
        AND cancel_reason = 'payment_timeout'
        AND late_payment_reported_at IS NOT NULL
        AND late_payment_resolved_at IS NOT NULL
      )
    );

CREATE INDEX ix_api_orders_buyer_pending_capacity
ON api_orders(buyer_user_id, api_service_id, selected_package_id, api_quota_offer_id, id)
WHERE status = 'pending_payment';

CREATE INDEX ix_api_orders_seller_late_payment
ON api_orders(seller_user_id, late_payment_reported_at, id)
WHERE late_payment_status = 'reported';

ALTER TABLE api_quota_sale_rounds
  ADD COLUMN fulfillment_confirmed_at timestamptz,
  ADD CONSTRAINT ck_api_quota_sale_rounds_fulfillment_confirmation
    CHECK (
      fulfillment_confirmed_at IS NULL
      OR (
        system_slot_key IS NOT NULL
        AND fulfillment_confirmed_at >= starts_at - interval '30 minutes'
        AND fulfillment_confirmed_at < starts_at
      )
    );

CREATE INDEX ix_api_quota_sale_rounds_confirmation
ON api_quota_sale_rounds(owner_user_id, starts_at, id)
WHERE system_slot_key IS NOT NULL AND fulfillment_confirmed_at IS NOT NULL;
