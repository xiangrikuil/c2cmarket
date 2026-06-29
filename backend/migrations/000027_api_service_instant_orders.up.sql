-- API service instant order layer.
-- 日期：2026-06-24
-- 执行者：Codex

ALTER TABLE api_services
ADD COLUMN accepting_orders boolean NOT NULL DEFAULT false,
ADD COLUMN payment_window_minutes integer NOT NULL DEFAULT 10,
ADD CONSTRAINT chk_api_services_payment_window
  CHECK (payment_window_minutes BETWEEN 3 AND 15);

CREATE TABLE api_service_payment_options (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_service_id uuid NOT NULL REFERENCES api_services(id) ON DELETE CASCADE,
  payment_method text NOT NULL CHECK (payment_method IN ('wechat', 'alipay', 'usdt')),
  enabled boolean NOT NULL DEFAULT true,
  payment_instructions text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (api_service_id, payment_method),
  CHECK (trim(payment_instructions) <> '')
);

CREATE INDEX ix_api_service_payment_options_service
ON api_service_payment_options(api_service_id, enabled, payment_method);

ALTER TABLE api_purchase_intents
ADD COLUMN seller_quote_status text NOT NULL DEFAULT 'none'
  CHECK (seller_quote_status IN ('none', 'quoted', 'accepted', 'expired', 'revoked')),
ADD COLUMN seller_quoted_amount numeric(18,6),
ADD COLUMN seller_quoted_currency text CHECK (seller_quoted_currency IN ('CNY', 'USDT')),
ADD COLUMN seller_quote_note text,
ADD COLUMN seller_quoted_at timestamptz,
ADD COLUMN seller_quote_expires_at timestamptz,
ADD COLUMN seller_quote_version bigint NOT NULL DEFAULT 0,
ADD CONSTRAINT chk_api_purchase_intents_seller_quote
  CHECK (
    (
      seller_quote_status = 'none'
      AND seller_quoted_amount IS NULL
      AND seller_quoted_currency IS NULL
      AND seller_quoted_at IS NULL
    )
    OR (
      seller_quote_status <> 'none'
      AND seller_quoted_amount IS NOT NULL
      AND seller_quoted_amount > 0
      AND seller_quoted_currency IS NOT NULL
      AND seller_quoted_at IS NOT NULL
      AND seller_quote_version > 0
    )
  );

CREATE TABLE api_orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_purchase_intent_id uuid NOT NULL REFERENCES api_purchase_intents(id),
  api_service_id uuid NOT NULL REFERENCES api_services(id),
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  seller_user_id uuid NOT NULL REFERENCES users(id),
  status text NOT NULL CHECK (status IN (
    'pending_payment',
    'payment_submitted',
    'paid_confirmed',
    'delivery_submitted',
    'completed',
    'cancelled'
  )),
  dispute_status text NOT NULL DEFAULT 'none' CHECK (dispute_status IN ('none', 'open', 'closed')),
  dispute_case_id uuid REFERENCES dispute_cases(id),
  service_title_snapshot text NOT NULL,
  service_version_snapshot bigint NOT NULL,
  billing_mode_snapshot text NOT NULL,
  selected_package_id uuid,
  selected_package_snapshot jsonb,
  quote_version_snapshot bigint,
  amount numeric(18,6) NOT NULL CHECK (amount > 0),
  currency text NOT NULL CHECK (currency IN ('CNY', 'USDT')),
  selected_payment_method text NOT NULL CHECK (selected_payment_method IN ('wechat', 'alipay', 'usdt')),
  payment_window_minutes_snapshot integer NOT NULL CHECK (payment_window_minutes_snapshot BETWEEN 3 AND 15),
  payment_expires_at timestamptz NOT NULL,
  payment_instructions_snapshot text NOT NULL,
  payment_summary text,
  payment_submitted_at timestamptz,
  paid_confirmed_at timestamptz,
  delivery_note text,
  delivery_submitted_at timestamptz,
  completed_at timestamptz,
  cancelled_at timestamptz,
  cancel_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (id, buyer_user_id),
  UNIQUE (id, seller_user_id),
  CHECK (buyer_user_id <> seller_user_id),
  CHECK (trim(payment_instructions_snapshot) <> ''),
  CHECK (
    (
      status = 'pending_payment'
      AND payment_summary IS NULL
      AND payment_submitted_at IS NULL
      AND paid_confirmed_at IS NULL
      AND delivery_note IS NULL
      AND delivery_submitted_at IS NULL
      AND completed_at IS NULL
      AND cancelled_at IS NULL
      AND cancel_reason IS NULL
    )
    OR (
      status = 'payment_submitted'
      AND payment_summary IS NOT NULL
      AND payment_submitted_at IS NOT NULL
      AND paid_confirmed_at IS NULL
      AND delivery_note IS NULL
      AND delivery_submitted_at IS NULL
      AND completed_at IS NULL
      AND cancelled_at IS NULL
      AND cancel_reason IS NULL
    )
    OR (
      status = 'paid_confirmed'
      AND payment_summary IS NOT NULL
      AND payment_submitted_at IS NOT NULL
      AND paid_confirmed_at IS NOT NULL
      AND delivery_note IS NULL
      AND delivery_submitted_at IS NULL
      AND completed_at IS NULL
      AND cancelled_at IS NULL
      AND cancel_reason IS NULL
    )
    OR (
      status = 'delivery_submitted'
      AND payment_summary IS NOT NULL
      AND payment_submitted_at IS NOT NULL
      AND paid_confirmed_at IS NOT NULL
      AND delivery_note IS NOT NULL
      AND delivery_submitted_at IS NOT NULL
      AND completed_at IS NULL
      AND cancelled_at IS NULL
      AND cancel_reason IS NULL
    )
    OR (
      status = 'completed'
      AND payment_summary IS NOT NULL
      AND payment_submitted_at IS NOT NULL
      AND paid_confirmed_at IS NOT NULL
      AND delivery_note IS NOT NULL
      AND delivery_submitted_at IS NOT NULL
      AND completed_at IS NOT NULL
      AND cancelled_at IS NULL
      AND cancel_reason IS NULL
    )
    OR (
      status = 'cancelled'
      AND cancelled_at IS NOT NULL
      AND cancel_reason IS NOT NULL
      AND completed_at IS NULL
    )
  )
);

CREATE UNIQUE INDEX ux_api_orders_intent
ON api_orders(api_purchase_intent_id);

CREATE INDEX ix_api_orders_buyer
ON api_orders(buyer_user_id, updated_at DESC);

CREATE INDEX ix_api_orders_seller
ON api_orders(seller_user_id, updated_at DESC);

CREATE INDEX ix_api_orders_pending_expiry
ON api_orders(payment_expires_at)
WHERE status = 'pending_payment';

CREATE TABLE api_order_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_order_id uuid NOT NULL REFERENCES api_orders(id) ON DELETE CASCADE,
  actor_user_id uuid REFERENCES users(id),
  event_type text NOT NULL,
  from_status text,
  to_status text,
  note text,
  request_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_api_order_events_request
ON api_order_events(api_order_id, event_type, request_id);

CREATE TABLE api_order_payment_instruction_access_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_order_id uuid NOT NULL REFERENCES api_orders(id) ON DELETE CASCADE,
  buyer_user_id uuid NOT NULL REFERENCES users(id),
  request_id text NOT NULL,
  accessed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_api_order_payment_instruction_logs_order
ON api_order_payment_instruction_access_logs(api_order_id, accessed_at DESC);
