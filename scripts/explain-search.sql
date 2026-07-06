-- Verify trigram expression-index eligibility for global search predicates.
--
-- Usage:
--   psql "$DATABASE_URL" -f scripts/explain-search.sql
--
-- The sample pattern is intentionally constant so this script can be run
-- without seed setup. Change '%gpt%' below when checking a specific dataset.
-- `enable_seqscan = off` is for local verification only; do not change
-- production planner settings.

SET enable_seqscan = off;

-- Expected: ix_product_plans_search_trgm
EXPLAIN (COSTS OFF)
SELECT r.id
FROM official_price_records r
JOIN product_plans p ON p.id = r.product_plan_id
WHERE r.status = 'active'
  AND lower(p.display_name || ' ' || p.provider_code || ' ' || p.slug) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_carpool_listings_search_trgm
EXPLAIN (COSTS OFF)
SELECT l.id
FROM carpool_listings l
WHERE l.status = 'active'
  AND lower(l.title || ' ' || l.summary || ' ' || l.access_arrangement) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_demands_search_trgm
EXPLAIN (COSTS OFF)
SELECT d.id
FROM demands d
WHERE d.status = 'active'
  AND lower(d.title || ' ' || d.region_code || ' ' || d.owner_preference || ' ' || COALESCE(d.note, '')) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_api_services_search_trgm
EXPLAIN (COSTS OFF)
SELECT s.id
FROM api_services s
WHERE s.review_status = 'approved'
  AND s.publication_status = 'online'
  AND s.moderation_status = 'clear'
  AND s.accepting_orders = true
  AND s.payment_window_minutes BETWEEN 3 AND 15
  AND EXISTS (
    SELECT 1
    FROM api_service_payment_options po
    WHERE po.api_service_id = s.id
      AND po.enabled = true
  )
  AND lower(s.title || ' ' || s.short_description) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_api_service_models_search_trgm
EXPLAIN (COSTS OFF)
SELECT m.api_service_id
FROM api_service_models m
WHERE m.enabled = true
  AND lower(m.model_name_snapshot || ' ' || m.provider_snapshot) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_merchant_profiles_search_trgm
EXPLAIN (COSTS OFF)
SELECT mp.id
FROM merchant_profiles mp
WHERE mp.status = 'active'
  AND lower(mp.display_name) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_users_search_trgm
EXPLAIN (COSTS OFF)
SELECT u.id
FROM users u
WHERE u.account_status = 'active'
  AND lower(u.username || ' ' || u.display_name) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_linux_do_bindings_search_trgm
EXPLAIN (COSTS OFF)
SELECT l.user_id
FROM linux_do_bindings l
WHERE lower(l.linux_do_username) ILIKE '%gpt%' ESCAPE '\';

-- Catalog support indexes from migrations 000024/000034. These are not part
-- of the global `/api/v1/search` result path today, but the expressions remain
-- verifiable here so future catalog search work can reuse them deliberately.

-- Expected: ix_api_model_catalog_search_trgm
EXPLAIN (COSTS OFF)
SELECT catalog.id
FROM api_model_catalog catalog
WHERE catalog.active = true
  AND lower(catalog.display_name || ' ' || catalog.model_key) ILIKE '%gpt%' ESCAPE '\';

-- Expected: ix_api_model_providers_search_trgm
EXPLAIN (COSTS OFF)
SELECT provider.id
FROM api_model_providers provider
WHERE provider.active = true
  AND lower(provider.display_name || ' ' || provider.code) ILIKE '%gpt%' ESCAPE '\';

RESET enable_seqscan;
