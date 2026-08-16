-- Add private, append-only image evidence for API-order disputes.
-- Migration: 000105
-- Date: 2026-08-13
-- Executor: Codex

CREATE TABLE api_order_evidence_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  api_order_id uuid NOT NULL REFERENCES api_orders(id) ON DELETE RESTRICT,
  uploader_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  kind text NOT NULL CHECK (kind IN (
    'payment_result',
    'refund_result',
    'api_error',
    'quota_insufficient',
    'expired_early',
    'description_mismatch',
    'other_redacted_fact'
  )),
  object_key text,
  output_mime text NOT NULL CHECK (output_mime IN ('image/jpeg', 'image/png')),
  byte_size bigint NOT NULL CHECK (byte_size > 0 AND byte_size <= 5242880),
  width integer NOT NULL CHECK (width BETWEEN 1 AND 4096),
  height integer NOT NULL CHECK (height BETWEEN 1 AND 4096),
  sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
  scan_status text NOT NULL CHECK (scan_status IN ('passed', 'rejected')),
  status text NOT NULL CHECK (status IN ('ready', 'quarantined', 'destroy_pending', 'destroyed')),
  ready_at timestamptz,
  unbound_expires_at timestamptz,
  quarantined_expires_at timestamptz,
  destroy_requested_at timestamptz,
  destroyed_at timestamptz,
  destroy_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CHECK (
    (status = 'ready' AND scan_status = 'passed' AND object_key IS NOT NULL
      AND ready_at IS NOT NULL AND unbound_expires_at IS NOT NULL
      AND quarantined_expires_at IS NULL AND destroy_requested_at IS NULL
      AND destroyed_at IS NULL AND destroy_reason = '')
    OR (status = 'quarantined' AND scan_status = 'rejected'
      AND ready_at IS NULL AND unbound_expires_at IS NULL
      AND quarantined_expires_at IS NOT NULL AND destroy_requested_at IS NULL
      AND destroyed_at IS NULL AND destroy_reason = '')
    OR (status = 'destroy_pending' AND destroy_requested_at IS NOT NULL
      AND destroyed_at IS NULL AND destroy_reason <> '')
    OR (status = 'destroyed' AND object_key IS NULL
      AND destroy_requested_at IS NOT NULL AND destroyed_at IS NOT NULL
      AND destroy_reason <> '')
  )
);

CREATE INDEX ix_api_order_evidence_assets_unbound_cleanup
ON api_order_evidence_assets(unbound_expires_at, id)
WHERE status = 'ready';

CREATE INDEX ix_api_order_evidence_assets_quarantine_cleanup
ON api_order_evidence_assets(quarantined_expires_at, id)
WHERE status = 'quarantined';

CREATE INDEX ix_api_order_evidence_assets_destroy_retry
ON api_order_evidence_assets(destroy_requested_at, id)
WHERE status = 'destroy_pending';

CREATE INDEX ix_api_order_evidence_assets_order_created
ON api_order_evidence_assets(api_order_id, created_at, id);

CREATE TABLE api_order_evidence_bindings (
  asset_id uuid PRIMARY KEY REFERENCES api_order_evidence_assets(id) ON DELETE RESTRICT,
  dispute_case_id uuid NOT NULL REFERENCES dispute_cases(id) ON DELETE RESTRICT,
  visibility text NOT NULL CHECK (visibility IN ('participants_admin', 'submitter_admin', 'appellant_admin')),
  usage text NOT NULL CHECK (usage IN (
    'dispute_initial',
    'platform_escalation',
    'message',
    'info_supplement',
    'remedy_claim',
    'remedy_contest',
    'appeal'
  )),
  source_type text NOT NULL CHECK (source_type IN (
    'dispute_case',
    'dispute_message',
    'info_supplement',
    'dispute_remedy',
    'appeal'
  )),
  source_id uuid NOT NULL,
  dispute_message_id uuid REFERENCES api_order_dispute_messages(id) ON DELETE RESTRICT,
  info_supplement_id uuid REFERENCES moderation_info_supplements(id) ON DELETE RESTRICT,
  dispute_remedy_id uuid REFERENCES api_order_dispute_remedies(id) ON DELETE RESTRICT,
  appeal_id uuid REFERENCES appeals(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (usage IN ('dispute_initial', 'platform_escalation') AND source_type = 'dispute_case'
      AND source_id = dispute_case_id
      AND dispute_message_id IS NULL AND info_supplement_id IS NULL
      AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
    OR (usage = 'message' AND source_type = 'dispute_message'
      AND source_id = dispute_message_id
      AND dispute_message_id IS NOT NULL AND info_supplement_id IS NULL
      AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
    OR (usage = 'info_supplement' AND source_type = 'info_supplement'
      AND source_id = info_supplement_id
      AND dispute_message_id IS NULL AND info_supplement_id IS NOT NULL
      AND dispute_remedy_id IS NULL AND appeal_id IS NULL)
    OR (usage IN ('remedy_claim', 'remedy_contest') AND source_type = 'dispute_remedy'
      AND source_id = dispute_remedy_id
      AND dispute_message_id IS NULL AND info_supplement_id IS NULL
      AND dispute_remedy_id IS NOT NULL AND appeal_id IS NULL)
    OR (usage = 'appeal' AND source_type = 'appeal'
      AND source_id = appeal_id
      AND dispute_message_id IS NULL AND info_supplement_id IS NULL
      AND dispute_remedy_id IS NULL AND appeal_id IS NOT NULL)
  ),
  CHECK (
    (usage = 'info_supplement' AND visibility = 'submitter_admin')
    OR (usage = 'appeal' AND visibility = 'appellant_admin')
    OR (usage NOT IN ('info_supplement', 'appeal') AND visibility = 'participants_admin')
  )
);

CREATE INDEX ix_api_order_evidence_bindings_case_created
ON api_order_evidence_bindings(dispute_case_id, created_at, asset_id);

CREATE INDEX ix_api_order_evidence_bindings_source
ON api_order_evidence_bindings(source_type, source_id, created_at, asset_id);

CREATE OR REPLACE FUNCTION validate_api_order_evidence_binding_source()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM api_order_evidence_assets asset
    JOIN dispute_cases dispute ON dispute.id = NEW.dispute_case_id
    JOIN api_orders order_row ON order_row.id = asset.api_order_id
    WHERE asset.id = NEW.asset_id
      AND dispute.target_type = 'api_order'
      AND dispute.api_order_id = asset.api_order_id
      AND asset.uploader_user_id IN (order_row.buyer_user_id, order_row.seller_user_id)
  ) THEN
    RAISE EXCEPTION 'API-order evidence asset does not belong to the dispute order or a participant'
      USING ERRCODE = '23514';
  END IF;

  IF NEW.source_type = 'dispute_case' THEN
    IF NEW.source_id <> NEW.dispute_case_id THEN
      RAISE EXCEPTION 'API-order evidence source does not belong to the dispute'
        USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.source_type = 'dispute_message' THEN
    IF NOT EXISTS (
      SELECT 1 FROM api_order_dispute_messages
      WHERE id = NEW.dispute_message_id AND dispute_case_id = NEW.dispute_case_id
    ) THEN
      RAISE EXCEPTION 'API-order evidence source does not belong to the dispute'
        USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.source_type = 'info_supplement' THEN
    IF NOT EXISTS (
      SELECT 1
      FROM moderation_info_supplements supplement
      JOIN moderation_info_requests request_row ON request_row.id = supplement.info_request_id
      WHERE supplement.id = NEW.info_supplement_id
        AND request_row.entity_type = 'dispute'
        AND request_row.dispute_case_id = NEW.dispute_case_id
    ) THEN
      RAISE EXCEPTION 'API-order evidence source does not belong to the dispute'
        USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.source_type = 'dispute_remedy' THEN
    IF NOT EXISTS (
      SELECT 1 FROM api_order_dispute_remedies
      WHERE id = NEW.dispute_remedy_id AND dispute_case_id = NEW.dispute_case_id
    ) THEN
      RAISE EXCEPTION 'API-order evidence source does not belong to the dispute'
        USING ERRCODE = '23514';
    END IF;
  ELSIF NEW.source_type = 'appeal' THEN
    IF NOT EXISTS (
      SELECT 1 FROM appeals
      WHERE id = NEW.appeal_id AND dispute_case_id = NEW.dispute_case_id
    ) THEN
      RAISE EXCEPTION 'API-order evidence source does not belong to the dispute'
        USING ERRCODE = '23514';
    END IF;
  ELSE
    RAISE EXCEPTION 'unsupported API-order evidence source type'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_api_order_evidence_bindings_validate_source
BEFORE INSERT ON api_order_evidence_bindings
FOR EACH ROW
EXECUTE FUNCTION validate_api_order_evidence_binding_source();

CREATE OR REPLACE FUNCTION reject_api_order_evidence_binding_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'API-order evidence bindings are append-only'
    USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER trg_api_order_evidence_bindings_append_only
BEFORE UPDATE OR DELETE ON api_order_evidence_bindings
FOR EACH ROW
EXECUTE FUNCTION reject_api_order_evidence_binding_mutation();
