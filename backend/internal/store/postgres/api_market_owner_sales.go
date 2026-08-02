package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
)

func (s *Store) ListAPIServicesByOwner(
	ctx context.Context,
	ownerUserID string,
	filter apimarket.OwnerServiceFilter,
	page domain.PageRequest,
) (domain.Page[apimarket.Service], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[apimarket.Service]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[apimarket.Service]{}, appErr
	}

	args := []any{ownerUserID, filter.SalesView}
	query := `SELECT ` + apiServiceColumns + `,
	                 sales.overall_state,
	                 sales.channels
	          FROM api_services
	          JOIN LATERAL (` + ownerAPISalesAggregationSQL() + `) sales ON true
	          WHERE api_services.owner_user_id = $1
	            AND (
	              $2 = 'all'
	              OR ($2 = 'active' AND sales.overall_state IN ('selling', 'upcoming'))
	              OR ($2 = 'expired' AND sales.overall_state = 'expired')
	              OR ($2 = 'paused' AND sales.overall_state = 'paused')
	              OR ($2 = 'draft' AND sales.overall_state IN ('draft', 'offline'))
	            )`
	if page.Cursor != "" {
		args = append(args, position.Time, position.ID)
		query += ` AND (api_services.updated_at, api_services.id) < ($3, $4::uuid)`
	}
	args = append(args, page.Limit+1)
	query += ` ORDER BY api_services.updated_at DESC, api_services.id DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[apimarket.Service]{}, internalStoreError()
	}
	defer rows.Close()

	services := []apimarket.Service{}
	for rows.Next() {
		var service apimarket.Service
		var channelsJSON []byte
		destinations := append(
			apiServiceScanDestinations(&service),
			&service.SalesSummary.OverallState,
			&channelsJSON,
		)
		if err := rows.Scan(destinations...); err != nil {
			return domain.Page[apimarket.Service]{}, internalStoreError()
		}
		if err := json.Unmarshal(channelsJSON, &service.SalesSummary.Channels); err != nil {
			return domain.Page[apimarket.Service]{}, internalStoreError()
		}
		services = append(services, service)
	}
	if rows.Err() != nil {
		return domain.Page[apimarket.Service]{}, internalStoreError()
	}
	for index := range services {
		if appErr := s.loadAPIServiceChildren(ctx, s.pool, &services[index]); appErr != nil {
			return domain.Page[apimarket.Service]{}, appErr
		}
	}
	return pageFromItems(services, page, func(item apimarket.Service) (time.Time, string) {
		return item.UpdatedAt, item.ID
	}), nil
}

func ownerAPISalesAggregationSQL() string {
	serviceOrderable := publicAPIServiceOrderablePredicate("api_services")
	return `
		WITH flexible_channel AS (
		  SELECT
		    'flexible_quota'::text AS kind,
		    CASE
		      WHEN api_services.publication_status = 'archived'
		        OR api_services.moderation_status = 'removed' THEN 'archived'
		      WHEN api_services.publication_status = 'owner_paused'
		        OR api_services.moderation_status = 'admin_suspended' THEN 'paused'
		      WHEN api_services.review_status <> 'approved' THEN 'draft'
		      WHEN api_services.publication_status <> 'online' THEN 'offline'
		      WHEN api_services.quota_expires_at IS NULL THEN 'offline'
		      WHEN api_services.quota_expires_at <= now() THEN 'expired'
		      WHEN COALESCE(api_services.available_usd_allowance, api_services.declared_max_usd_allowance_per_intent, 0) <= 0 THEN 'sold_out'
		      WHEN (` + serviceOrderable + `) THEN 'selling'
		      ELSE 'offline'
		    END::text AS state,
		    COALESCE(api_services.available_usd_allowance, api_services.declared_max_usd_allowance_per_intent)::text AS available_usd_allowance,
		    NULL::integer AS available_copies,
		    NULL::timestamptz AS next_starts_at,
		    NULL::timestamptz AS sale_cutoff_at,
		    api_services.quota_expires_at AS expires_at,
		    api_services.updated_at AS relevant_at
		  WHERE api_services.billing_mode = 'metered_usd_quota'
		),
		limited_candidates AS (
		  SELECT
		    CASE
		      WHEN api_services.publication_status = 'archived'
		        OR api_services.moderation_status = 'removed' THEN 'archived'
		      WHEN api_services.publication_status = 'owner_paused'
		        OR api_services.moderation_status = 'admin_suspended' THEN 'paused'
		      WHEN batch.status = 'archived' OR offer.status = 'archived' THEN 'archived'
		      WHEN batch.status = 'paused' OR offer.status = 'paused' THEN 'paused'
		      WHEN api_services.review_status <> 'approved' THEN 'draft'
		      WHEN api_services.publication_status <> 'online'
		        OR api_services.moderation_status <> 'clear' THEN 'offline'
		      WHEN batch.status = 'draft' OR offer.id IS NULL OR offer.status = 'draft' THEN 'draft'
		      WHEN batch.sale_cutoff_at <= now() OR batch.expires_at <= now() THEN 'expired'
		      WHEN batch.status = 'published'
		        AND offer.status = 'published'
		        AND (` + serviceOrderable + `)
		        AND inventory.current_available > 0
		        AND (
		          offer.delivery_mode = 'manual'
		          OR credentials.available >= inventory.current_available
		        ) THEN 'selling'
		      WHEN batch.status = 'published'
		        AND offer.status = 'published'
		        AND (` + serviceOrderable + `)
		        AND inventory.next_starts_at IS NOT NULL
		        AND inventory.future_available > 0
		        AND (
		          offer.delivery_mode = 'manual'
		          OR credentials.available >= inventory.future_available
		        ) THEN 'upcoming'
		      WHEN batch.status = 'published' AND offer.status = 'published' THEN 'sold_out'
		      ELSE 'offline'
		    END::text AS state,
		    CASE
		      WHEN offer.sale_mode = 'scheduled' AND inventory.current_available = 0
		        THEN inventory.future_available
		      ELSE inventory.current_available
		    END::integer AS available_copies,
		    inventory.next_starts_at,
		    batch.sale_cutoff_at,
		    batch.expires_at,
		    GREATEST(batch.updated_at, COALESCE(offer.updated_at, batch.updated_at)) AS relevant_at
		  FROM api_quota_batches batch
		  LEFT JOIN api_quota_offers offer
		    ON offer.batch_id = batch.id
		    AND offer.api_service_id = batch.api_service_id
		    AND offer.owner_user_id = batch.owner_user_id
		  LEFT JOIN LATERAL (
		    SELECT
		      count(unit.id) FILTER (
		        WHERE unit.status = 'available'
		          AND allocation.status = 'active'
		          AND (
		            (offer.sale_mode = 'continuous' AND allocation.sale_round_id IS NULL)
		            OR (
		              offer.sale_mode = 'scheduled'
		              AND round.status = 'scheduled'
		              AND round.starts_at <= now()
		              AND round.ends_at > now()
		            )
		          )
		      )::integer AS current_available,
		      count(unit.id) FILTER (
		        WHERE unit.status = 'available'
		          AND allocation.status = 'active'
		          AND offer.sale_mode = 'scheduled'
		          AND round.status = 'scheduled'
		          AND round.starts_at > now()
		      )::integer AS future_available,
		      min(round.starts_at) FILTER (
		        WHERE allocation.status = 'active'
		          AND offer.sale_mode = 'scheduled'
		          AND round.status = 'scheduled'
		          AND round.starts_at > now()
		      ) AS next_starts_at
		    FROM api_quota_allocations allocation
		    LEFT JOIN api_quota_sale_rounds round
		      ON round.id = allocation.sale_round_id
		      AND round.batch_id = allocation.batch_id
		      AND round.api_service_id = allocation.api_service_id
		      AND round.owner_user_id = allocation.owner_user_id
		    LEFT JOIN api_quota_inventory_units unit
		      ON unit.allocation_id = allocation.id
		      AND unit.batch_id = allocation.batch_id
		      AND unit.offer_id = allocation.offer_id
		    WHERE allocation.offer_id = offer.id
		  ) inventory ON offer.id IS NOT NULL
		  LEFT JOIN LATERAL (
		    SELECT count(*)::integer AS available
		    FROM api_quota_credentials credential
		    WHERE credential.api_quota_offer_id = offer.id
		      AND credential.status = 'available'
		  ) credentials ON offer.id IS NOT NULL
		  WHERE batch.api_service_id = api_services.id
		    AND batch.owner_user_id = api_services.owner_user_id
		),
		limited_channel AS (
		  SELECT
		    'limited_quota'::text AS kind,
		    candidate.state,
		    NULL::text AS available_usd_allowance,
		    candidate.available_copies,
		    candidate.next_starts_at,
		    candidate.sale_cutoff_at,
		    candidate.expires_at,
		    candidate.relevant_at
		  FROM limited_candidates candidate
		  ORDER BY
		    CASE candidate.state
		      WHEN 'selling' THEN 0
		      WHEN 'upcoming' THEN 1
		      WHEN 'paused' THEN 2
		      WHEN 'sold_out' THEN 3
		      WHEN 'expired' THEN 4
		      WHEN 'draft' THEN 5
		      WHEN 'offline' THEN 6
		      WHEN 'archived' THEN 7
		      ELSE 8
		    END,
		    candidate.relevant_at DESC
		  LIMIT 1
		),
		channels AS (
		  SELECT * FROM flexible_channel
		  UNION ALL
		  SELECT * FROM limited_channel
		),
		fallback AS (
		  SELECT CASE
		    WHEN api_services.publication_status = 'archived'
		      OR api_services.moderation_status = 'removed' THEN 'archived'
		    WHEN api_services.publication_status = 'owner_paused'
		      OR api_services.moderation_status = 'admin_suspended' THEN 'paused'
		    WHEN api_services.review_status <> 'approved' THEN 'draft'
		    WHEN api_services.publication_status <> 'online' THEN 'offline'
		    WHEN api_services.billing_mode = 'metered_usd_quota'
		      AND api_services.quota_expires_at <= now() THEN 'expired'
		    WHEN api_services.billing_mode = 'metered_usd_quota'
		      AND COALESCE(api_services.available_usd_allowance, api_services.declared_max_usd_allowance_per_intent, 0) <= 0 THEN 'sold_out'
		    WHEN (` + serviceOrderable + `) THEN 'selling'
		    ELSE 'offline'
		  END::text AS state
		)
		SELECT
		  COALESCE(
		    (
		      array_agg(
		        channels.state
		        ORDER BY CASE channels.state
		          WHEN 'selling' THEN 0
		          WHEN 'upcoming' THEN 1
		          WHEN 'paused' THEN 2
		          WHEN 'sold_out' THEN 3
		          WHEN 'expired' THEN 4
		          WHEN 'draft' THEN 5
		          WHEN 'offline' THEN 6
		          WHEN 'archived' THEN 7
		          ELSE 8
		        END
		      ) FILTER (WHERE channels.kind IS NOT NULL)
		    )[1],
		    fallback.state
		  ) AS overall_state,
		  COALESCE(
		    jsonb_agg(
		      jsonb_strip_nulls(jsonb_build_object(
		        'kind', channels.kind,
		        'state', channels.state,
		        'availableUsdAllowance', channels.available_usd_allowance,
		        'availableCopies', channels.available_copies,
		        'nextStartsAt', channels.next_starts_at,
		        'saleCutoffAt', channels.sale_cutoff_at,
		        'expiresAt', channels.expires_at
		      ))
		      ORDER BY channels.kind
		    ) FILTER (WHERE channels.kind IS NOT NULL),
		    '[]'::jsonb
		  ) AS channels
		FROM fallback
		LEFT JOIN channels ON true
		GROUP BY fallback.state
	`
}
