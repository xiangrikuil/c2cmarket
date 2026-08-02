-- Immutable public business numbers for API orders.
-- Date: 2026-08-02
-- Executor: Codex

ALTER TABLE api_orders
ADD COLUMN order_no text;

DO $$
DECLARE
  order_row record;
  alphabet constant text := 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';
  attempt integer;
  suffix text;
  candidate text;
BEGIN
  FOR order_row IN
    SELECT id, created_at
    FROM api_orders
    WHERE order_no IS NULL
    ORDER BY created_at, id
  LOOP
    attempt := 0;
    LOOP
      SELECT string_agg(
        substr(
          alphabet,
          (get_byte(decode(md5(order_row.id::text || ':' || attempt::text), 'hex'), byte_index) % char_length(alphabet)) + 1,
          1
        ),
        '' ORDER BY byte_index
      )
      INTO suffix
      FROM generate_series(0, 9) AS generated(byte_index);

      candidate := 'API-'
        || to_char(order_row.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYYMMDD')
        || '-'
        || suffix;

      EXIT WHEN NOT EXISTS (
        SELECT 1
        FROM api_orders existing
        WHERE existing.order_no = candidate
      );

      attempt := attempt + 1;
      IF attempt >= 1024 THEN
        RAISE EXCEPTION 'unable to backfill a unique API order number for %', order_row.id;
      END IF;
    END LOOP;

    UPDATE api_orders
    SET order_no = candidate
    WHERE id = order_row.id;
  END LOOP;
END;
$$;

ALTER TABLE api_orders
ALTER COLUMN order_no SET NOT NULL;

ALTER TABLE api_orders
ADD CONSTRAINT ck_api_orders_order_no_format
CHECK (order_no ~ '^API-[0-9]{8}-[ABCDEFGHJKMNPQRSTUVWXYZ23456789]{10}$'),
ADD CONSTRAINT ux_api_orders_order_no UNIQUE (order_no);

CREATE OR REPLACE FUNCTION preserve_api_order_no()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.order_no IS DISTINCT FROM OLD.order_no THEN
    RAISE EXCEPTION USING
      ERRCODE = '23514',
      CONSTRAINT = 'ck_api_orders_order_no_immutable',
      MESSAGE = 'api order number is immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_api_orders_order_no_immutable
BEFORE UPDATE OF order_no ON api_orders
FOR EACH ROW
EXECUTE FUNCTION preserve_api_order_no();
