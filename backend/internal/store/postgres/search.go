package postgres

import (
	"context"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/search"

	"github.com/jackc/pgx/v5"
)

func (s *Store) Search(ctx context.Context, keyword string, perTypeLimit int) ([]search.Result, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	if perTypeLimit <= 0 {
		perTypeLimit = search.DefaultPerType
	}
	pattern := "%" + escapeLike(strings.ToLower(keyword)) + "%"
	items := []search.Result{}

	queries := []struct {
		sql  string
		args []any
	}{
		{sql: searchOfficialPricesSQL, args: []any{pattern, perTypeLimit}},
		{sql: searchCarpoolsSQL, args: []any{pattern, perTypeLimit}},
		{sql: searchDemandsSQL, args: []any{pattern, perTypeLimit}},
		{sql: searchAPIServicesSQL, args: []any{pattern, perTypeLimit}},
		{sql: searchUsersSQL, args: []any{pattern, perTypeLimit}},
		{sql: searchMerchantsSQL, args: []any{pattern, perTypeLimit}},
	}
	for _, query := range queries {
		rows, err := s.pool.Query(ctx, query.sql, query.args...)
		if err != nil {
			return nil, internalStoreError()
		}
		scanned, appErr := scanSearchResults(rows)
		rows.Close()
		if appErr != nil {
			return nil, appErr
		}
		items = append(items, scanned...)
	}
	return items, nil
}

func scanSearchResults(rows pgx.Rows) ([]search.Result, *domain.AppError) {
	items := []search.Result{}
	for rows.Next() {
		var item search.Result
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Subtitle, &item.Badge, &item.To, &item.RankTime); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

const searchOfficialPricesSQL = `
SELECT
	'official-' || r.id::text AS id,
	'官方价格' AS type,
	'官方价格 ' || COALESCE(NULLIF(p.display_name, ''), r.product_plan_id::text) AS title,
	r.region_code || ' · ' || r.channel || ' · ¥' || r.normalized_monthly_cny::text || '/月' AS subtitle,
	r.status AS badge,
	'/official-prices/' || r.id::text AS to,
	r.created_at AS rank_time
FROM official_price_records r
LEFT JOIN product_plans p ON p.id = r.product_plan_id
WHERE r.status = 'active'
  AND (
	LOWER(COALESCE(p.display_name, '')) ILIKE $1 ESCAPE '\'
	OR LOWER(r.product_plan_id::text) ILIKE $1 ESCAPE '\'
	OR LOWER(r.region_code) ILIKE $1 ESCAPE '\'
	OR LOWER(r.channel) ILIKE $1 ESCAPE '\'
	OR LOWER(r.opening_method) ILIKE $1 ESCAPE '\'
	OR LOWER(r.original_amount::text) ILIKE $1 ESCAPE '\'
	OR LOWER(r.normalized_monthly_cny::text) ILIKE $1 ESCAPE '\'
  )
ORDER BY r.created_at DESC
LIMIT $2
`

const searchCarpoolsSQL = `
SELECT
	'carpool-' || l.id::text AS id,
	'车源' AS type,
	l.title AS title,
	'¥' || l.price_monthly_cny::text || '/月 · 可用席位 ' ||
	GREATEST(l.buyer_seat_capacity - l.active_buyer_members - COALESCE(reserved.reserved_seats, 0), 0)::text ||
	' · 每月' || l.quota_label || ' ' || l.monthly_quota_amount::text || ' ' || l.quota_unit AS subtitle,
	l.status AS badge,
	'/carpools/' || l.id::text AS to,
	l.updated_at AS rank_time
FROM carpool_listings l
LEFT JOIN LATERAL (
	SELECT COALESCE(SUM(a.seat_count), 0)::int AS reserved_seats
	FROM carpool_applications a
	WHERE a.carpool_listing_id = l.id
	  AND a.status = 'accepted_reserved'
	  AND a.reservation_expires_at > now()
) reserved ON true
WHERE l.status = 'active'
  AND (
	LOWER(l.title) ILIKE $1 ESCAPE '\'
	OR LOWER(l.summary) ILIKE $1 ESCAPE '\'
	OR LOWER(l.access_arrangement) ILIKE $1 ESCAPE '\'
	OR LOWER(COALESCE(l.source_url, '')) ILIKE $1 ESCAPE '\'
	OR LOWER(l.price_monthly_cny::text) ILIKE $1 ESCAPE '\'
	OR LOWER(l.monthly_quota_amount::text) ILIKE $1 ESCAPE '\'
	OR LOWER(l.quota_label) ILIKE $1 ESCAPE '\'
	OR LOWER(l.quota_unit) ILIKE $1 ESCAPE '\'
  )
ORDER BY l.updated_at DESC
LIMIT $2
`

const searchDemandsSQL = `
SELECT
	'demand-' || d.id::text AS id,
	'求车' AS type,
	d.title AS title,
	d.region_code || ' · 预算 ¥' || d.max_price_cny::text || '/月 · ' || u.display_name AS subtitle,
	d.status AS badge,
	'/demands/' || d.id::text AS to,
	d.updated_at AS rank_time
FROM demands d
JOIN users u ON u.id = d.publisher_user_id
WHERE d.status = 'active'
  AND (
	LOWER(d.title) ILIKE $1 ESCAPE '\'
	OR LOWER(d.region_code) ILIKE $1 ESCAPE '\'
	OR LOWER(d.owner_preference) ILIKE $1 ESCAPE '\'
	OR LOWER(COALESCE(d.note, '')) ILIKE $1 ESCAPE '\'
	OR LOWER(u.username) ILIKE $1 ESCAPE '\'
	OR LOWER(u.display_name) ILIKE $1 ESCAPE '\'
  )
ORDER BY d.updated_at DESC
LIMIT $2
`

var searchAPIServicesSQL = `
SELECT
	'api-' || s.id::text AS id,
	'API 服务' AS type,
	s.title AS title,
	COALESCE(NULLIF(mp.display_name, ''), 'API 商户') || ' · ' ||
	COALESCE(models.model_names, '模型待补充') AS subtitle,
	'在线' AS badge,
	'/api-market/' || s.id::text AS to,
	s.updated_at AS rank_time
FROM api_services s
LEFT JOIN merchant_profiles mp ON mp.id = s.merchant_profile_id AND mp.owner_user_id = s.owner_user_id
LEFT JOIN LATERAL (
	SELECT string_agg(m.model_name_snapshot, ' / ' ORDER BY m.model_name_snapshot) AS model_names,
	       string_agg(m.model_name_snapshot, ' ' ORDER BY m.model_name_snapshot) AS searchable_models
	FROM api_service_models m
	WHERE m.api_service_id = s.id AND m.enabled = true
) models ON true
WHERE ` + publicAPIServiceOrderablePredicate("s") + `
  AND (
	LOWER(s.title) ILIKE $1 ESCAPE '\'
	OR LOWER(s.short_description) ILIKE $1 ESCAPE '\'
	OR LOWER(COALESCE(mp.display_name, '')) ILIKE $1 ESCAPE '\'
	OR LOWER(COALESCE(models.searchable_models, '')) ILIKE $1 ESCAPE '\'
  )
ORDER BY s.updated_at DESC
LIMIT $2
`

const searchUsersSQL = `
SELECT
	'user-' || u.username AS id,
	'用户' AS type,
	u.display_name AS title,
	'公开个人主页 · @' || u.username || COALESCE(' · 信任等级' || l.trust_level::text, '') AS subtitle,
	CASE WHEN l.id IS NOT NULL THEN '已绑定 linux.do' ELSE '未绑定' END AS badge,
	'/u/' || u.username AS to,
	u.updated_at AS rank_time
FROM users u
LEFT JOIN linux_do_bindings l ON l.user_id = u.id
WHERE u.account_status = 'active'
  AND (
	LOWER(u.username) ILIKE $1 ESCAPE '\'
	OR LOWER(u.display_name) ILIKE $1 ESCAPE '\'
	OR LOWER(COALESCE(l.linux_do_username, '')) ILIKE $1 ESCAPE '\'
  )
ORDER BY u.updated_at DESC
LIMIT $2
`

var searchMerchantsSQL = `
SELECT DISTINCT ON (owner.username)
	'merchant-' || owner.username AS id,
	'商户' AS type,
	owner.display_name AS title,
	'@' || owner.username || ' · API 商户公开身份' AS subtitle,
	'公开个人身份' AS badge,
	'/u/' || owner.username AS to,
	MAX(s.updated_at) AS rank_time
FROM api_services s
JOIN users owner ON owner.id = s.owner_user_id
WHERE ` + publicAPIServiceOrderablePredicate("s") + `
  AND s.merchant_identity_mode = 'public_profile'
  AND owner.account_status = 'active'
  AND (
	LOWER(owner.username) ILIKE $1 ESCAPE '\'
	OR LOWER(owner.display_name) ILIKE $1 ESCAPE '\'
	OR LOWER(s.title) ILIKE $1 ESCAPE '\'
  )
GROUP BY owner.username, owner.display_name
ORDER BY owner.username ASC, rank_time DESC
LIMIT $2
`

var _ = time.Time{}
