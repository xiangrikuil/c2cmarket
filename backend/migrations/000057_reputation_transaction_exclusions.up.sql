-- 增加可恢复、可审计的信誉交易排除记录。
-- 日期：2026-07-24
-- 执行者：Codex

CREATE TABLE reputation_transaction_exclusions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_type text NOT NULL CHECK (transaction_type IN (
    'carpool_application',
    'carpool_membership',
    'api_purchase_intent',
    'api_order'
  )),
  transaction_id uuid NOT NULL,
  excluded_at timestamptz NOT NULL,
  excluded_by_admin_id uuid NOT NULL REFERENCES users(id),
  reason_code text NOT NULL CHECK (reason_code ~ '^[a-z][a-z0-9_]{1,63}$'),
  reason text NOT NULL CHECK (trim(reason) <> ''),
  restored_at timestamptz,
  restored_by_admin_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (transaction_type, transaction_id),
  UNIQUE (id, transaction_type, transaction_id),
  CHECK (
    (restored_at IS NULL AND restored_by_admin_id IS NULL)
    OR (
      restored_at IS NOT NULL
      AND restored_by_admin_id IS NOT NULL
      AND restored_at >= excluded_at
    )
  )
);

CREATE INDEX ix_reputation_transaction_exclusions_active
ON reputation_transaction_exclusions(transaction_type, transaction_id, updated_at DESC)
WHERE restored_at IS NULL;

CREATE TABLE reputation_transaction_exclusion_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  exclusion_id uuid NOT NULL,
  transaction_type text NOT NULL,
  transaction_id uuid NOT NULL,
  action text NOT NULL CHECK (action IN ('excluded', 'restored')),
  actor_admin_id uuid NOT NULL REFERENCES users(id),
  reason_code text NOT NULL CHECK (reason_code ~ '^[a-z][a-z0-9_]{1,63}$'),
  reason text NOT NULL CHECK (trim(reason) <> ''),
  created_at timestamptz NOT NULL,
  FOREIGN KEY (exclusion_id, transaction_type, transaction_id)
    REFERENCES reputation_transaction_exclusions(id, transaction_type, transaction_id)
);

CREATE INDEX ix_reputation_transaction_exclusion_events_transaction
ON reputation_transaction_exclusion_events(transaction_type, transaction_id, created_at DESC, id DESC);

CREATE INDEX ix_reputation_transaction_exclusion_events_actor
ON reputation_transaction_exclusion_events(actor_admin_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_reputation_exclusion_event_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'reputation transaction exclusion events are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_reputation_exclusion_events_append_only
BEFORE UPDATE OR DELETE ON reputation_transaction_exclusion_events
FOR EACH ROW
EXECUTE FUNCTION reject_reputation_exclusion_event_mutation();
