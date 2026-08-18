package postgres

import (
	"fmt"

	"c2c-market/backend/internal/module/apimarket"
)

// apiServiceSortExpression 返回公开 API 服务列表使用的非负升序排序值。
// 这些表达式同时用于 SELECT、ORDER BY 和 cursor 条件，避免分页时排序口径漂移。
func apiServiceSortExpression(alias, sortMode string) string {
	completed := apiServiceCompleted30dSortExpression(alias)
	response := apiServiceResponseSortExpression(alias)
	reputation := apiServiceReputationSortExpression(alias)
	switch sortMode {
	case apimarket.PublicServiceSortRecommended:
		disputes := apiServiceUnresolvedDisputesSortExpression(alias)
		updated := apiServiceUpdatedSortExpression(alias)
		return "(" + reputation + ") * 1000000000000000000000000::numeric + (" + response + ") * 1000000000000::numeric + (" + completed + ") * 1000000::numeric + (" + disputes + ") * 1000::numeric + (" + updated + ")"
	case apimarket.PublicServiceSortReputationDesc:
		return reputation
	case apimarket.PublicServiceSortCompletedDesc:
		return completed
	case apimarket.PublicServiceSortResponseFast:
		return response
	default:
		return ""
	}
}

func apiServiceCompleted30dSortExpression(alias string) string {
	return fmt.Sprintf(`GREATEST(0::numeric, 1000000000::numeric - COALESCE((
		SELECT COUNT(*)::numeric
		FROM api_orders completed_orders
		WHERE completed_orders.api_service_id = %s.id
		  AND completed_orders.status = 'completed'
		  AND completed_orders.completed_at >= now() - interval '30 days'
	), 0::numeric))`, alias)
}

func apiServiceResponseSortExpression(alias string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT percentile_cont(0.5) WITHIN GROUP (
			ORDER BY EXTRACT(EPOCH FROM (response_events.first_seller_response_at - response_events.created_at)) / 60.0
		)
		FROM (
			SELECT response_orders.id,
			       response_orders.created_at,
			       MIN(response_order_events.created_at) FILTER (
			         WHERE response_order_events.event_type IN (
			           'api_order.payment_confirmed',
			           'api_order.payment_issue_reported',
			           'api_order.delivery_submitted'
			         )
			       ) AS first_seller_response_at
			FROM api_orders response_orders
			LEFT JOIN api_order_events response_order_events ON response_order_events.api_order_id = response_orders.id
			WHERE response_orders.api_service_id = %s.id
			GROUP BY response_orders.id, response_orders.created_at
		) response_events
		WHERE response_events.first_seller_response_at IS NOT NULL
	), 1000000000::double precision)::numeric`, alias)
}

func apiServiceUnresolvedDisputesSortExpression(alias string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT COUNT(*)::numeric
		FROM api_orders unresolved_orders
		WHERE unresolved_orders.api_service_id = %s.id
		  AND unresolved_orders.dispute_status NOT IN ('none', 'closed')
	), 0::numeric)`, alias)
}

func apiServiceUpdatedSortExpression(alias string) string {
	return fmt.Sprintf(`GREATEST(0::numeric, EXTRACT(EPOCH FROM (timestamptz '2100-01-01 00:00:00+00' - %s.updated_at))::numeric)`, alias)
}

func apiServiceReputationSortExpression(alias string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT (
			4 - CASE seller_reputation.tier
				WHEN 'high_trust' THEN 4
				WHEN 'reliable' THEN 3
				WHEN 'normal' THEN 2
				WHEN 'insufficient' THEN 1
				ELSE 0
			END
			)::numeric * 1000000000000::numeric
			+ CASE WHEN NULLIF(seller_reputation.metrics_json->>'weightedRating', '') IS NULL THEN 1 ELSE 0 END * 100000000000::numeric
			+ (500::numeric - COALESCE(NULLIF(seller_reputation.metrics_json->>'weightedRating', '')::numeric * 100, 0::numeric)) * 1000000::numeric
			+ GREATEST(0::numeric, 1000000::numeric - COALESCE(NULLIF(seller_reputation.metrics_json->>'verifiedReviewCount', '')::numeric, 0::numeric))
			+ CASE WHEN seller_reputation.state = 'restricted' THEN 100000000000::numeric ELSE 0::numeric END
		FROM user_reputation_states seller_reputation
		WHERE seller_reputation.user_id = %s.owner_user_id
		  AND seller_reputation.role = 'seller'
		  AND seller_reputation.scope = 'api'
		LIMIT 1
	), 1000000000000000::numeric)`, alias)
}
