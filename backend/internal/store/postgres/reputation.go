package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"

	"github.com/jackc/pgx/v5"
)

const carpoolApplicationConfirmationEvidenceSQL = `
  CROSS JOIN LATERAL (
    SELECT
      EXISTS (
        SELECT 1
        FROM carpool_join_confirmations confirmation
        WHERE confirmation.carpool_application_id = application.id
          AND confirmation.actor_role = 'buyer'
      ) AS buyer_confirmed,
      EXISTS (
        SELECT 1
        FROM carpool_join_confirmations confirmation
        WHERE confirmation.carpool_application_id = application.id
          AND confirmation.actor_role = 'owner'
      ) AS owner_confirmed
  ) AS confirmations
`

const carpoolApplicationResponsibleCancellationPredicate = `(
  (participants.role = 'buyer' AND application.status = 'cancelled_by_buyer')
  OR (participants.role = 'seller' AND application.status = 'cancelled_by_owner')
  OR (
    application.status = 'expired'
    AND (
      (participants.role = 'buyer' AND NOT confirmations.buyer_confirmed)
      OR (participants.role = 'seller' AND NOT confirmations.owner_confirmed)
    )
  )
)`

const apiOrderCancellationEvidenceSQL = `
  CROSS JOIN LATERAL (
    SELECT
      EXISTS (
        SELECT 1
        FROM api_order_events event
        WHERE event.api_order_id = api_order.id
          AND event.event_type = 'api_order.cancelled'
          AND event.actor_user_id = participants.user_id
      ) AS actor_cancelled,
      EXISTS (
        SELECT 1
        FROM api_order_events event
        WHERE event.api_order_id = api_order.id
          AND event.event_type = 'api_order.cancelled'
          AND event.actor_user_id IN (api_order.buyer_user_id, api_order.seller_user_id)
      ) AS known_actor_cancelled,
      (
        SELECT MIN(event.created_at)
        FROM api_order_events event
        WHERE event.api_order_id = api_order.id
          AND event.event_type = 'api_order.cancelled'
          AND event.actor_user_id = participants.user_id
      ) AS actor_cancelled_at
  ) AS cancellation
`

const apiOrderResponsibleCancellationPredicate = `(
  cancellation.actor_cancelled
  OR (
    participants.role = 'buyer'
    AND api_order.cancel_reason = 'payment_timeout'
  )
)`

const aggregateReputationFactsSQL = `
WITH requested AS (
  SELECT DISTINCT unnest($1::uuid[]) AS user_id
),
base AS (
  SELECT requested.user_id, roles.role, scopes.scope
  FROM requested
  CROSS JOIN (VALUES ('buyer'), ('seller')) AS roles(role)
  CROSS JOIN (VALUES ('overall'), ('carpool'), ('api')) AS scopes(scope)
),
terminal_facts AS (
  SELECT
    requested.user_id,
    participants.role,
    'carpool'::text AS scope,
    CASE
      WHEN membership.status = 'completed'
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END AS completed_count,
    CASE
      WHEN membership.status = 'completed'
        AND membership.ended_at >= $2
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END AS completed_count_90d,
    CASE
      WHEN (
        (participants.role = 'buyer' AND membership.status = 'left' AND membership.ended_by_user_id = membership.buyer_user_id)
        OR (participants.role = 'seller' AND membership.status = 'removed' AND membership.ended_by_user_id = membership.owner_user_id)
      )
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END AS responsible_cancellation_count,
    CASE
      WHEN membership.status IN ('left', 'removed')
        AND NOT COALESCE(
          (membership.status = 'left' AND membership.ended_by_user_id = membership.buyer_user_id)
          OR (membership.status = 'removed' AND membership.ended_by_user_id = membership.owner_user_id),
          false
        )
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END AS unknown_cancellation_count,
    0 AS unresolved_dispute_count,
    GREATEST(membership.updated_at, COALESCE(exclusion.updated_at, membership.updated_at)) AS source_updated_at
  FROM requested
  JOIN carpool_memberships membership
    ON membership.buyer_user_id = requested.user_id
    OR membership.owner_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (membership.buyer_user_id, 'buyer'::text),
      (membership.owner_user_id, 'seller'::text)
  ) AS participants(user_id, role)
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_membership'
   AND exclusion.transaction_id = membership.id
  WHERE participants.user_id = requested.user_id
    AND membership.status IN ('completed', 'left', 'removed')

  UNION ALL

  SELECT
    requested.user_id,
    participants.role,
    'carpool'::text,
    0,
    0,
    CASE
      WHEN ` + carpoolApplicationResponsibleCancellationPredicate + `
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    CASE
      WHEN application.status = 'expired'
        AND confirmations.buyer_confirmed
        AND confirmations.owner_confirmed
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    0,
    GREATEST(application.updated_at, COALESCE(exclusion.updated_at, application.updated_at))
  FROM requested
  JOIN carpool_applications application
    ON application.buyer_user_id = requested.user_id
    OR application.owner_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (application.buyer_user_id, 'buyer'::text),
      (application.owner_user_id, 'seller'::text)
  ) AS participants(user_id, role)
` + carpoolApplicationConfirmationEvidenceSQL + `
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_application'
   AND exclusion.transaction_id = application.id
  WHERE participants.user_id = requested.user_id
    AND application.status IN ('cancelled_by_buyer', 'cancelled_by_owner', 'expired')

  UNION ALL

  SELECT
    requested.user_id,
    participants.role,
    'api'::text,
    CASE
      WHEN api_order.status = 'completed'
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    CASE
      WHEN api_order.status = 'completed'
        AND api_order.completed_at >= $2
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    CASE
      WHEN ` + apiOrderResponsibleCancellationPredicate + `
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    CASE
      WHEN api_order.status = 'cancelled'
        AND NOT cancellation.known_actor_cancelled
        AND api_order.cancel_reason <> 'payment_timeout'
        AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
      THEN 1 ELSE 0
    END,
    0,
    GREATEST(api_order.updated_at, COALESCE(exclusion.updated_at, api_order.updated_at))
  FROM requested
  JOIN api_orders api_order
    ON api_order.buyer_user_id = requested.user_id
    OR api_order.seller_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (api_order.buyer_user_id, 'buyer'::text),
      (api_order.seller_user_id, 'seller'::text)
  ) AS participants(user_id, role)
` + apiOrderCancellationEvidenceSQL + `
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'api_order'
   AND exclusion.transaction_id = api_order.id
  WHERE participants.user_id = requested.user_id
    AND api_order.status IN ('completed', 'cancelled')
),
dispute_facts AS (
  SELECT
    requested.user_id,
    CASE
      WHEN membership.buyer_user_id = requested.user_id THEN 'buyer'::text
      ELSE 'seller'::text
    END AS role,
    'carpool'::text AS scope,
    0 AS completed_count,
    0 AS completed_count_90d,
    0 AS responsible_cancellation_count,
    0 AS unknown_cancellation_count,
    CASE
      WHEN exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL
      THEN 1 ELSE 0
    END AS unresolved_dispute_count,
    GREATEST(dispute.updated_at, COALESCE(exclusion.updated_at, dispute.updated_at)) AS source_updated_at
  FROM requested
  JOIN dispute_cases dispute
    ON dispute.subject_user_id = requested.user_id
   AND dispute.target_type = 'carpool_membership'
	AND dispute.status IN ('negotiating', 'open', 'waiting_info')
  JOIN carpool_memberships membership
    ON dispute.target_id = membership.id::text
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_membership'
   AND exclusion.transaction_id = membership.id
  WHERE requested.user_id IN (membership.buyer_user_id, membership.owner_user_id)

  UNION ALL

  SELECT
    requested.user_id,
    CASE
      WHEN application.buyer_user_id = requested.user_id THEN 'buyer'::text
      ELSE 'seller'::text
    END,
    'carpool'::text,
    0, 0, 0, 0,
    CASE
      WHEN exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL
      THEN 1 ELSE 0
    END,
    GREATEST(dispute.updated_at, COALESCE(exclusion.updated_at, dispute.updated_at))
  FROM requested
  JOIN dispute_cases dispute
    ON dispute.subject_user_id = requested.user_id
   AND dispute.target_type = 'carpool_application'
	AND dispute.status IN ('negotiating', 'open', 'waiting_info')
  JOIN carpool_applications application
    ON dispute.target_id = application.id::text
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_application'
   AND exclusion.transaction_id = application.id
  WHERE requested.user_id IN (application.buyer_user_id, application.owner_user_id)

  UNION ALL

  SELECT
    requested.user_id,
    CASE
      WHEN api_order.buyer_user_id = requested.user_id THEN 'buyer'::text
      ELSE 'seller'::text
    END,
    'api'::text,
    0, 0, 0, 0,
    CASE
      WHEN exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL
      THEN 1 ELSE 0
    END,
    GREATEST(dispute.updated_at, COALESCE(exclusion.updated_at, dispute.updated_at))
  FROM requested
  JOIN dispute_cases dispute
    ON dispute.subject_user_id = requested.user_id
   AND dispute.target_type = 'api_order'
	AND dispute.status IN ('negotiating', 'open', 'waiting_info')
  JOIN api_orders api_order
    ON dispute.target_id = api_order.id::text
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'api_order'
   AND exclusion.transaction_id = api_order.id
  WHERE requested.user_id IN (api_order.buyer_user_id, api_order.seller_user_id)

  UNION ALL

  SELECT
    requested.user_id,
    CASE
      WHEN intent.buyer_user_id = requested.user_id THEN 'buyer'::text
      ELSE 'seller'::text
    END,
    'api'::text,
    0, 0, 0, 0,
    CASE
      WHEN exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL
      THEN 1 ELSE 0
    END,
    GREATEST(dispute.updated_at, COALESCE(exclusion.updated_at, dispute.updated_at))
  FROM requested
  JOIN dispute_cases dispute
    ON dispute.subject_user_id = requested.user_id
   AND dispute.target_type = 'api_purchase_intent'
	AND dispute.status IN ('negotiating', 'open', 'waiting_info')
  JOIN api_purchase_intents intent
    ON dispute.target_id = intent.id::text
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'api_purchase_intent'
   AND exclusion.transaction_id = intent.id
  WHERE requested.user_id IN (intent.buyer_user_id, intent.owner_user_id)
),
fact_events AS (
  SELECT * FROM terminal_facts
  UNION ALL
  SELECT * FROM dispute_facts
)
SELECT
  base.user_id::text,
  base.role,
  base.scope,
  COALESCE(SUM(fact_events.completed_count), 0)::bigint,
  COALESCE(SUM(fact_events.completed_count_90d), 0)::bigint,
  COALESCE(SUM(fact_events.responsible_cancellation_count), 0)::bigint,
  COALESCE(SUM(fact_events.unknown_cancellation_count), 0)::bigint,
  COALESCE(SUM(fact_events.unresolved_dispute_count), 0)::bigint,
  MAX(fact_events.source_updated_at)
FROM base
LEFT JOIN fact_events
  ON fact_events.user_id = base.user_id
 AND fact_events.role = base.role
 AND (fact_events.scope = base.scope OR base.scope = 'overall')
GROUP BY base.user_id, base.role, base.scope
ORDER BY base.user_id, base.role, base.scope
`

const aggregateReputationEngineFactsSQL = `
WITH requested AS (
  SELECT DISTINCT unnest($1::uuid[]) AS user_id
),
base AS (
  SELECT requested.user_id, roles.role, scopes.scope
  FROM requested
  CROSS JOIN (VALUES ('buyer'), ('seller')) AS roles(role)
  CROSS JOIN (VALUES ('overall'), ('carpool'), ('api')) AS scopes(scope)
),
review_candidates AS (
  SELECT
    review.reviewee_user_id AS user_id,
    review.reviewee_role AS role,
    CASE review.transaction_type
      WHEN 'carpool_membership' THEN 'carpool'::text
      WHEN 'api_order' THEN 'api'::text
    END AS source_scope,
    review.rating,
    review.tags,
    review.status,
    review.review_deadline_at,
    COALESCE(review.visible_at, review.review_deadline_at) AS effective_visible_at,
    GREATEST(review.updated_at, COALESCE(exclusion.updated_at, review.updated_at)) AS source_updated_at
  FROM transaction_reviews review
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = review.transaction_type
   AND exclusion.transaction_id = COALESCE(review.carpool_membership_id, review.api_order_id)
  WHERE review.status <> 'removed'
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
),
review_events AS (
  SELECT
    review_candidates.user_id,
    review_candidates.role,
    scopes.scope,
    review_candidates.rating,
    review_candidates.tags,
    review_candidates.effective_visible_at,
    review_candidates.source_updated_at
  FROM review_candidates
  CROSS JOIN LATERAL (
    VALUES (review_candidates.source_scope), ('overall'::text)
  ) AS scopes(scope)
  WHERE review_candidates.status = 'published'
     OR (
       review_candidates.status = 'sealed'
       AND review_candidates.review_deadline_at <= $2
     )
),
review_stats AS (
  SELECT
    review_events.user_id,
    review_events.role,
    review_events.scope,
    COUNT(*)::bigint AS review_count,
    COALESCE(SUM(review_events.rating), 0)::bigint AS rating_sum,
    COUNT(*) FILTER (WHERE review_events.rating = 1)::bigint AS rating_1,
    COUNT(*) FILTER (WHERE review_events.rating = 2)::bigint AS rating_2,
    COUNT(*) FILTER (WHERE review_events.rating = 3)::bigint AS rating_3,
    COUNT(*) FILTER (WHERE review_events.rating = 4)::bigint AS rating_4,
    COUNT(*) FILTER (WHERE review_events.rating = 5)::bigint AS rating_5,
    COUNT(*) FILTER (
      WHERE review_events.effective_visible_at >= $2 - interval '90 days'
    )::bigint AS recent_review_count,
    MAX(review_events.source_updated_at) AS source_updated_at,
    MIN(review_events.effective_visible_at + interval '90 days') FILTER (
      WHERE review_events.effective_visible_at >= $2 - interval '90 days'
    ) AS next_recalculation_at
  FROM review_events
  JOIN requested ON requested.user_id = review_events.user_id
  GROUP BY review_events.user_id, review_events.role, review_events.scope
),
review_tag_counts AS (
  SELECT
    review_events.user_id,
    review_events.role,
    review_events.scope,
    tag_values.tag,
    CASE
      WHEN tag_values.tag IN ('响应较慢', '与描述不符') THEN 'negative'
      ELSE 'positive'
    END AS polarity,
    COUNT(*)::bigint AS tag_count
  FROM review_events
  CROSS JOIN LATERAL unnest(review_events.tags) AS tag_values(tag)
  WHERE trim(tag_values.tag) <> ''
  GROUP BY
    review_events.user_id,
    review_events.role,
    review_events.scope,
    tag_values.tag,
    CASE
      WHEN tag_values.tag IN ('响应较慢', '与描述不符') THEN 'negative'
      ELSE 'positive'
    END
),
review_tag_ranked AS (
  SELECT
    review_tag_counts.*,
    row_number() OVER (
      PARTITION BY user_id, role, scope, polarity
      ORDER BY tag_count DESC, tag
    ) AS tag_rank
  FROM review_tag_counts
),
review_tag_stats AS (
  SELECT
    user_id,
    role,
    scope,
    COALESCE(
      jsonb_agg(
        jsonb_build_object('tag', tag, 'count', tag_count)
        ORDER BY tag_count DESC, tag
      ) FILTER (WHERE polarity = 'positive' AND tag_rank <= 5),
      '[]'::jsonb
    ) AS positive_tags,
    COALESCE(
      jsonb_agg(
        jsonb_build_object('tag', tag, 'count', tag_count)
        ORDER BY tag_count DESC, tag
      ) FILTER (WHERE polarity = 'negative' AND tag_rank <= 5),
      '[]'::jsonb
    ) AS negative_tags
  FROM review_tag_ranked
  GROUP BY user_id, role, scope
),
review_deadlines AS (
  SELECT
    review_candidates.user_id,
    review_candidates.role,
    scopes.scope,
    MIN(review_candidates.review_deadline_at) AS next_recalculation_at,
    MAX(review_candidates.source_updated_at) AS source_updated_at
  FROM review_candidates
  JOIN requested ON requested.user_id = review_candidates.user_id
  CROSS JOIN LATERAL (
    VALUES (review_candidates.source_scope), ('overall'::text)
  ) AS scopes(scope)
  WHERE review_candidates.status = 'sealed'
    AND review_candidates.review_deadline_at > $2
  GROUP BY review_candidates.user_id, review_candidates.role, scopes.scope
),
platform_review_stats AS (
  SELECT
    review_events.role,
    review_events.scope,
    COUNT(*)::bigint AS review_count,
    AVG(review_events.rating::numeric)::double precision AS average_rating,
    MAX(review_events.source_updated_at) AS source_updated_at
  FROM review_events
  GROUP BY review_events.role, review_events.scope
),
completion_candidates AS (
  SELECT
    participants.user_id,
    participants.role,
    'carpool'::text AS source_scope,
    COALESCE(membership.ended_at, membership.updated_at) AS completed_at,
    GREATEST(membership.updated_at, COALESCE(exclusion.updated_at, membership.updated_at)) AS source_updated_at
  FROM requested
  JOIN carpool_memberships membership
    ON membership.buyer_user_id = requested.user_id
    OR membership.owner_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (membership.buyer_user_id, 'buyer'::text),
      (membership.owner_user_id, 'seller'::text)
  ) AS participants(user_id, role)
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_membership'
   AND exclusion.transaction_id = membership.id
  WHERE participants.user_id = requested.user_id
    AND membership.status = 'completed'
    AND COALESCE(membership.ended_at, membership.updated_at) >= $2 - interval '90 days'
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)

  UNION ALL

  SELECT
    participants.user_id,
    participants.role,
    'api'::text,
    api_order.completed_at,
    GREATEST(api_order.updated_at, COALESCE(exclusion.updated_at, api_order.updated_at))
  FROM requested
  JOIN api_orders api_order
    ON api_order.buyer_user_id = requested.user_id
    OR api_order.seller_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (api_order.buyer_user_id, 'buyer'::text),
      (api_order.seller_user_id, 'seller'::text)
  ) AS participants(user_id, role)
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'api_order'
   AND exclusion.transaction_id = api_order.id
  WHERE participants.user_id = requested.user_id
    AND api_order.status = 'completed'
    AND api_order.completed_at >= $2 - interval '90 days'
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
),
completion_boundaries AS (
  SELECT
    completion_candidates.user_id,
    completion_candidates.role,
    scopes.scope,
    MIN(completion_candidates.completed_at + interval '90 days') AS next_recalculation_at,
    MAX(completion_candidates.source_updated_at) AS source_updated_at
  FROM completion_candidates
  CROSS JOIN LATERAL (
    VALUES (completion_candidates.source_scope), ('overall'::text)
  ) AS scopes(scope)
  GROUP BY completion_candidates.user_id, completion_candidates.role, scopes.scope
),
fault_candidates AS (
  SELECT
    participants.user_id,
    participants.role,
    'carpool'::text AS source_scope,
    COALESCE(membership.ended_at, membership.updated_at) AS fault_at,
    GREATEST(membership.updated_at, COALESCE(exclusion.updated_at, membership.updated_at)) AS source_updated_at
  FROM requested
  JOIN carpool_memberships membership
    ON membership.buyer_user_id = requested.user_id
    OR membership.owner_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (membership.buyer_user_id, 'buyer'::text),
      (membership.owner_user_id, 'seller'::text)
  ) AS participants(user_id, role)
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_membership'
   AND exclusion.transaction_id = membership.id
  WHERE participants.user_id = requested.user_id
    AND COALESCE(membership.ended_at, membership.updated_at) >= $2 - interval '90 days'
    AND (
      (participants.role = 'buyer'
        AND membership.status = 'left'
        AND membership.ended_by_user_id = membership.buyer_user_id)
      OR
      (participants.role = 'seller'
        AND membership.status = 'removed'
        AND membership.ended_by_user_id = membership.owner_user_id)
    )
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)

  UNION ALL

  SELECT
    participants.user_id,
    participants.role,
    'carpool'::text,
    application.updated_at,
    GREATEST(application.updated_at, COALESCE(exclusion.updated_at, application.updated_at))
  FROM requested
  JOIN carpool_applications application
    ON application.buyer_user_id = requested.user_id
    OR application.owner_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (application.buyer_user_id, 'buyer'::text),
      (application.owner_user_id, 'seller'::text)
  ) AS participants(user_id, role)
` + carpoolApplicationConfirmationEvidenceSQL + `
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'carpool_application'
   AND exclusion.transaction_id = application.id
  WHERE participants.user_id = requested.user_id
    AND application.updated_at >= $2 - interval '90 days'
    AND ` + carpoolApplicationResponsibleCancellationPredicate + `
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)

  UNION ALL

  SELECT DISTINCT
    participants.user_id,
    participants.role,
    'api'::text,
    COALESCE(cancellation.actor_cancelled_at, api_order.cancelled_at, api_order.updated_at),
    GREATEST(
      api_order.updated_at,
      cancellation.actor_cancelled_at,
      COALESCE(exclusion.updated_at, api_order.updated_at)
    )
  FROM requested
  JOIN api_orders api_order
    ON api_order.buyer_user_id = requested.user_id
    OR api_order.seller_user_id = requested.user_id
  CROSS JOIN LATERAL (
    VALUES
      (api_order.buyer_user_id, 'buyer'::text),
      (api_order.seller_user_id, 'seller'::text)
  ) AS participants(user_id, role)
` + apiOrderCancellationEvidenceSQL + `
  LEFT JOIN reputation_transaction_exclusions exclusion
    ON exclusion.transaction_type = 'api_order'
   AND exclusion.transaction_id = api_order.id
  WHERE participants.user_id = requested.user_id
    AND api_order.status = 'cancelled'
    AND ` + apiOrderResponsibleCancellationPredicate + `
    AND COALESCE(cancellation.actor_cancelled_at, api_order.cancelled_at, api_order.updated_at) >= $2 - interval '90 days'
    AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL)
),
fault_stats AS (
  SELECT
    fault_candidates.user_id,
    fault_candidates.role,
    scopes.scope,
    COUNT(*)::bigint AS recent_fault_count,
    MAX(fault_candidates.source_updated_at) AS source_updated_at,
    MIN(fault_candidates.fault_at + interval '90 days') AS next_recalculation_at
  FROM fault_candidates
  CROSS JOIN LATERAL (
    VALUES (fault_candidates.source_scope), ('overall'::text)
  ) AS scopes(scope)
  GROUP BY fault_candidates.user_id, fault_candidates.role, scopes.scope
),
outcome_candidates AS (
  SELECT
    outcome.subject_user_id AS user_id,
    roles.role,
    scopes.scope,
    outcome.severity,
    GREATEST(outcome.updated_at, dispute.updated_at) AS source_updated_at,
    outcome.decided_at + interval '365 days' AS next_recalculation_at
  FROM dispute_reputation_outcomes outcome
  JOIN requested ON requested.user_id = outcome.subject_user_id
  JOIN dispute_cases dispute ON dispute.id = outcome.dispute_case_id
  CROSS JOIN LATERAL (
    SELECT role
    FROM (VALUES ('buyer'::text), ('seller'::text)) AS role_values(role)
    WHERE outcome.role_scope IN (role_values.role, 'all')
  ) AS roles
  CROSS JOIN LATERAL (
    SELECT scope
    FROM unnest(
      CASE
        WHEN dispute.target_type IN ('carpool_application', 'carpool_membership')
          THEN ARRAY['carpool'::text, 'overall'::text]
        WHEN dispute.target_type IN ('api_purchase_intent', 'api_order')
          THEN ARRAY['api'::text, 'overall'::text]
        ELSE ARRAY['overall'::text]
      END
    ) AS scope_values(scope)
  ) AS scopes
  WHERE outcome.status = 'active'
    AND outcome.responsibility IN ('responsible', 'shared')
    AND (
      dispute.target_type <> 'api_order'
      OR (
        SELECT remedy.status
        FROM api_order_dispute_remedies remedy
        WHERE remedy.dispute_case_id = dispute.id
        ORDER BY remedy.created_at DESC, remedy.id DESC
        LIMIT 1
      ) = 'overdue'
    )
    AND outcome.decided_at >= $2 - interval '365 days'
),
outcome_stats AS (
  SELECT
    outcome_candidates.user_id,
    outcome_candidates.role,
    outcome_candidates.scope,
    COUNT(*)::bigint AS fault_count,
    COUNT(*) FILTER (
      WHERE outcome_candidates.severity IN ('high', 'critical')
    )::bigint AS major_fault_count,
    MAX(outcome_candidates.source_updated_at) AS source_updated_at,
    MIN(outcome_candidates.next_recalculation_at) AS next_recalculation_at
  FROM outcome_candidates
  GROUP BY outcome_candidates.user_id, outcome_candidates.role, outcome_candidates.scope
),
restriction_candidates AS (
  SELECT
    restriction.user_id,
    roles.role,
    scopes.scope,
    restriction.starts_at,
    restriction.ends_at,
    restriction.updated_at AS source_updated_at
  FROM user_restrictions restriction
  JOIN requested ON requested.user_id = restriction.user_id
  CROSS JOIN LATERAL (
    SELECT role
    FROM (VALUES ('buyer'::text), ('seller'::text)) AS role_values(role)
    WHERE restriction.role_scope IN (role_values.role, 'all')
  ) AS roles
  CROSS JOIN LATERAL (
    SELECT scope
    FROM unnest(
      CASE
        WHEN restriction.action_code IN ('carpool_publish', 'carpool_apply', 'carpool_accept')
          THEN ARRAY['carpool'::text, 'overall'::text]
        WHEN restriction.action_code IN ('api_service_publish', 'api_order_create')
          THEN ARRAY['api'::text, 'overall'::text]
        ELSE ARRAY['carpool'::text, 'api'::text, 'overall'::text]
      END
    ) AS scope_values(scope)
  ) AS scopes
  WHERE restriction.revoked_at IS NULL
    AND (restriction.ends_at IS NULL OR $2 < restriction.ends_at)
),
restriction_stats AS (
  SELECT
    restriction_candidates.user_id,
    restriction_candidates.role,
    restriction_candidates.scope,
    COUNT(*) FILTER (
      WHERE restriction_candidates.starts_at <= $2
    )::bigint AS active_count,
    MAX(restriction_candidates.source_updated_at) AS source_updated_at,
    MIN(
      CASE
        WHEN restriction_candidates.starts_at > $2 THEN restriction_candidates.starts_at
        ELSE restriction_candidates.ends_at
      END
    ) AS next_recalculation_at
  FROM restriction_candidates
  GROUP BY restriction_candidates.user_id, restriction_candidates.role, restriction_candidates.scope
)
SELECT
  base.user_id::text,
  base.role,
  base.scope,
  COALESCE(fault_stats.recent_fault_count, 0)::bigint,
  COALESCE(outcome_stats.fault_count, 0)::bigint,
  COALESCE(outcome_stats.major_fault_count, 0)::bigint,
  COALESCE(restriction_stats.active_count, 0)::bigint,
  COALESCE(review_stats.review_count, 0)::bigint,
  COALESCE(review_stats.rating_sum, 0)::bigint,
  COALESCE(review_stats.rating_1, 0)::bigint,
  COALESCE(review_stats.rating_2, 0)::bigint,
  COALESCE(review_stats.rating_3, 0)::bigint,
  COALESCE(review_stats.rating_4, 0)::bigint,
  COALESCE(review_stats.rating_5, 0)::bigint,
  COALESCE(review_stats.recent_review_count, 0)::bigint,
  COALESCE(review_tag_stats.positive_tags, '[]'::jsonb),
  COALESCE(review_tag_stats.negative_tags, '[]'::jsonb),
  COALESCE(platform_review_stats.review_count, 0)::bigint,
  COALESCE(platform_review_stats.average_rating, 4.0)::double precision,
  GREATEST(
    review_stats.source_updated_at,
    review_deadlines.source_updated_at,
    platform_review_stats.source_updated_at,
    completion_boundaries.source_updated_at,
    fault_stats.source_updated_at,
    outcome_stats.source_updated_at,
    restriction_stats.source_updated_at
  ) AS source_updated_at,
  LEAST(
    review_stats.next_recalculation_at,
    review_deadlines.next_recalculation_at,
    completion_boundaries.next_recalculation_at,
    fault_stats.next_recalculation_at,
    outcome_stats.next_recalculation_at,
    restriction_stats.next_recalculation_at
  ) AS next_recalculation_at
FROM base
LEFT JOIN review_stats
  ON review_stats.user_id = base.user_id
 AND review_stats.role = base.role
 AND review_stats.scope = base.scope
LEFT JOIN review_deadlines
  ON review_deadlines.user_id = base.user_id
 AND review_deadlines.role = base.role
 AND review_deadlines.scope = base.scope
LEFT JOIN review_tag_stats
  ON review_tag_stats.user_id = base.user_id
 AND review_tag_stats.role = base.role
 AND review_tag_stats.scope = base.scope
LEFT JOIN platform_review_stats
  ON platform_review_stats.role = base.role
 AND platform_review_stats.scope = base.scope
LEFT JOIN completion_boundaries
  ON completion_boundaries.user_id = base.user_id
 AND completion_boundaries.role = base.role
 AND completion_boundaries.scope = base.scope
LEFT JOIN fault_stats
  ON fault_stats.user_id = base.user_id
 AND fault_stats.role = base.role
 AND fault_stats.scope = base.scope
LEFT JOIN outcome_stats
  ON outcome_stats.user_id = base.user_id
 AND outcome_stats.role = base.role
 AND outcome_stats.scope = base.scope
LEFT JOIN restriction_stats
  ON restriction_stats.user_id = base.user_id
 AND restriction_stats.role = base.role
 AND restriction_stats.scope = base.scope
ORDER BY base.user_id, base.role, base.scope
`

func (s *Store) AggregateFacts(ctx context.Context, userIDs []string, now time.Time) (map[string]reputation.RawFacts, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	result := make(map[string]reputation.RawFacts, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	rows, err := s.pool.Query(ctx, aggregateReputationFactsSQL, userIDs, now.Add(-90*24*time.Hour))
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID                       string
			role                         string
			scope                        string
			completedCount               int64
			completedCount90d            int64
			responsibleCancellationCount int64
			unknownCancellationCount     int64
			unresolvedDisputeCount       int64
			sourceDataUpdatedAt          *time.Time
		)
		if err := rows.Scan(
			&userID,
			&role,
			&scope,
			&completedCount,
			&completedCount90d,
			&responsibleCancellationCount,
			&unknownCancellationCount,
			&unresolvedDisputeCount,
			&sourceDataUpdatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		value := result[userID]
		value.UserID = userID
		target := scopeFacts(&value, role, scope)
		if target == nil {
			return nil, internalStoreError()
		}
		*target = reputation.ScopeFacts{
			Aggregated:                             true,
			CompletedCount:                         int(completedCount),
			CompletedCountLast90Days:               int(completedCount90d),
			RoleResponsibilityCancellationCount:    int(responsibleCancellationCount),
			UnknownResponsibilityCancellationCount: int(unknownCancellationCount),
			UnresolvedDisputeCount:                 int(unresolvedDisputeCount),
			SourceDataUpdatedAt:                    sourceDataUpdatedAt,
		}
		result[userID] = value
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}

	engineRows, err := s.pool.Query(ctx, aggregateReputationEngineFactsSQL, userIDs, now)
	if err != nil {
		return nil, internalStoreError()
	}
	defer engineRows.Close()
	for engineRows.Next() {
		var (
			userID                 string
			role                   string
			scope                  string
			recentFaultCount       int64
			faultDisputeCount      int64
			majorFaultDisputeCount int64
			activeRestrictionCount int64
			reviewCount            int64
			ratingSum              int64
			ratingOne              int64
			ratingTwo              int64
			ratingThree            int64
			ratingFour             int64
			ratingFive             int64
			recentReviewCount      int64
			commonPositiveTagsJSON []byte
			commonNegativeTagsJSON []byte
			platformReviewCount    int64
			platformAverageRating  float64
			sourceDataUpdatedAt    *time.Time
			nextRecalculationAt    *time.Time
		)
		if err := engineRows.Scan(
			&userID,
			&role,
			&scope,
			&recentFaultCount,
			&faultDisputeCount,
			&majorFaultDisputeCount,
			&activeRestrictionCount,
			&reviewCount,
			&ratingSum,
			&ratingOne,
			&ratingTwo,
			&ratingThree,
			&ratingFour,
			&ratingFive,
			&recentReviewCount,
			&commonPositiveTagsJSON,
			&commonNegativeTagsJSON,
			&platformReviewCount,
			&platformAverageRating,
			&sourceDataUpdatedAt,
			&nextRecalculationAt,
		); err != nil {
			return nil, internalStoreError()
		}
		value := result[userID]
		value.UserID = userID
		target := scopeFacts(&value, role, scope)
		if target == nil {
			return nil, internalStoreError()
		}
		target.Aggregated = true
		target.RoleResponsibilityCancellationCount90d = int(recentFaultCount)
		target.ConfirmedFaultDisputeCount365d = int(faultDisputeCount)
		target.ConfirmedMajorFaultDisputeCount365d = int(majorFaultDisputeCount)
		target.ActiveRestrictionCount = int(activeRestrictionCount)
		target.VerifiedReviewCount = int(reviewCount)
		target.RatingSum = int(ratingSum)
		target.RatingDistribution = reputation.RatingDistribution{
			One:   int(ratingOne),
			Two:   int(ratingTwo),
			Three: int(ratingThree),
			Four:  int(ratingFour),
			Five:  int(ratingFive),
		}
		target.RecentReviewCount90d = int(recentReviewCount)
		if err := json.Unmarshal(commonPositiveTagsJSON, &target.CommonPositiveTags); err != nil {
			return nil, internalStoreError()
		}
		if err := json.Unmarshal(commonNegativeTagsJSON, &target.CommonNegativeTags); err != nil {
			return nil, internalStoreError()
		}
		target.PlatformReviewCount = int(platformReviewCount)
		target.PlatformAverageRating = platformAverageRating
		target.SourceDataUpdatedAt = latestTimestamp(target.SourceDataUpdatedAt, sourceDataUpdatedAt)
		target.NextRecalculationAt = nextRecalculationAt
		result[userID] = value
	}
	if engineRows.Err() != nil {
		return nil, internalStoreError()
	}
	engineRows.Close()
	if appErr := s.applySourceAuthorFacts(ctx, userIDs, now, result); appErr != nil {
		return nil, appErr
	}
	return result, nil
}

func (s *Store) ExcludeTransaction(ctx context.Context, input reputation.ExclusionMutation, now time.Time) (reputation.TransactionExclusion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	exists, appErr := reputationTransactionExists(ctx, tx, input.TransactionType, input.TransactionID)
	if appErr != nil {
		return reputation.TransactionExclusion{}, appErr
	}
	if !exists {
		return reputation.TransactionExclusion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Transaction not found", "待排除的交易不存在。")
	}

	current, found, appErr := lockTransactionExclusion(ctx, tx, input.TransactionType, input.TransactionID)
	if appErr != nil {
		return reputation.TransactionExclusion{}, appErr
	}
	var exclusion reputation.TransactionExclusion
	if found {
		if current.RestoredAt == nil {
			return reputation.TransactionExclusion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Transaction already excluded", "该交易已被排除，无需重复操作。")
		}
		exclusion, err = scanTransactionExclusion(tx.QueryRow(ctx, `
			UPDATE reputation_transaction_exclusions
			SET excluded_at = $2,
			    excluded_by_admin_id = $3,
			    reason_code = $4,
			    reason = $5,
			    restored_at = NULL,
			    restored_by_admin_id = NULL,
			    updated_at = $2
			WHERE id = $1
			RETURNING id::text, transaction_type, transaction_id::text, excluded_at,
			          excluded_by_admin_id::text, reason_code, reason, restored_at,
			          COALESCE(restored_by_admin_id::text, ''), created_at, updated_at
		`, current.ID, now, input.AdminUserID, input.ReasonCode, input.Reason))
	} else {
		exclusion, err = scanTransactionExclusion(tx.QueryRow(ctx, `
			INSERT INTO reputation_transaction_exclusions (
			  transaction_type, transaction_id, excluded_at, excluded_by_admin_id,
			  reason_code, reason, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $3, $3)
			RETURNING id::text, transaction_type, transaction_id::text, excluded_at,
			          excluded_by_admin_id::text, reason_code, reason, restored_at,
			          COALESCE(restored_by_admin_id::text, ''), created_at, updated_at
		`, input.TransactionType, input.TransactionID, now, input.AdminUserID, input.ReasonCode, input.Reason))
	}
	if isUniqueViolation(err) {
		return reputation.TransactionExclusion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Transaction already excluded", "该交易已被排除，无需重复操作。")
	}
	if err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	if appErr := insertTransactionExclusionEvent(ctx, tx, exclusion, "excluded", input, now); appErr != nil {
		return reputation.TransactionExclusion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	return exclusion, nil
}

func (s *Store) RestoreTransaction(ctx context.Context, input reputation.ExclusionMutation, now time.Time) (reputation.TransactionExclusion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	current, found, appErr := lockTransactionExclusion(ctx, tx, input.TransactionType, input.TransactionID)
	if appErr != nil {
		return reputation.TransactionExclusion{}, appErr
	}
	if !found {
		return reputation.TransactionExclusion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Exclusion not found", "该交易没有可恢复的信誉排除记录。")
	}
	if current.RestoredAt != nil {
		return reputation.TransactionExclusion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Transaction already restored", "该交易已恢复信誉统计，无需重复操作。")
	}

	exclusion, err := scanTransactionExclusion(tx.QueryRow(ctx, `
		UPDATE reputation_transaction_exclusions
		SET restored_at = $2,
		    restored_by_admin_id = $3,
		    updated_at = $2
		WHERE id = $1
		RETURNING id::text, transaction_type, transaction_id::text, excluded_at,
		          excluded_by_admin_id::text, reason_code, reason, restored_at,
		          COALESCE(restored_by_admin_id::text, ''), created_at, updated_at
	`, current.ID, now, input.AdminUserID))
	if err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	if appErr := insertTransactionExclusionEvent(ctx, tx, exclusion, "restored", input, now); appErr != nil {
		return reputation.TransactionExclusion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.TransactionExclusion{}, internalStoreError()
	}
	return exclusion, nil
}

func (s *Store) CreateDisputeOutcomeWithIdempotency(ctx context.Context, entry idempotency.Entry, input reputation.CreateOutcomeInput, now time.Time, buildCompletion reputation.GovernanceCompletionBuilder) (reputation.GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	var targetType, targetID, status string
	var version int64
	if err := tx.QueryRow(ctx, `
		SELECT target_type, target_id, status, version
		FROM dispute_cases
		WHERE id = $1
		FOR UPDATE
	`, input.DisputeCaseID).Scan(&targetType, &targetID, &status, &version); errors.Is(err, pgx.ErrNoRows) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Dispute not found", "纠纷记录不存在。")
	} else if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if version != input.ExpectedVersion {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, reputationVersionConflict()
	}
	if status != "resolved" && status != "closed" {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Dispute unresolved", "未解决纠纷只能形成提醒，不能创建责任裁定。")
	}
	if targetType == report.TargetAPIOrder && (input.Responsibility == reputation.ResponsibilityResponsible || input.Responsibility == reputation.ResponsibilityShared) {
		var remedyStatus string
		if err := tx.QueryRow(ctx, `
			SELECT status
			FROM api_order_dispute_remedies
			WHERE dispute_case_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT 1
			FOR UPDATE
		`, input.DisputeCaseID).Scan(&remedyStatus); errors.Is(err, pgx.ErrNoRows) {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderRemedyOutcomeUnavailable()
		} else if err != nil {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
		}
		if remedyStatus != report.RemedyStatusOverdue {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderRemedyOutcomeUnavailable()
		}
	}
	var appealBlocksOutcome bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM appeals
			WHERE dispute_case_id = $1
			  AND status IN ('submitted', 'approved')
		)
	`, input.DisputeCaseID).Scan(&appealBlocksOutcome); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appealBlocksOutcome {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Appeal blocks outcome", "该纠纷有待处理或已批准的申诉，不能创建信誉裁定。")
	}
	subjectRole, appErr := disputeSubjectRole(ctx, tx, targetType, targetID, input.SubjectUserID)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if input.RoleScope != reputation.RoleAll && input.RoleScope != subjectRole {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Role scope invalid", "裁定角色必须与纠纷中的主体角色一致。", "roleScope", "invalid", "裁定角色必须与纠纷中的主体角色一致。")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE dispute_cases
		SET subject_user_id = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
	`, input.DisputeCaseID, input.SubjectUserID, now); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}

	outcome, err := scanDisputeOutcome(tx.QueryRow(ctx, `
		INSERT INTO dispute_reputation_outcomes (
		  dispute_case_id, subject_user_id, responsibility, severity, role_scope,
		  status, reason_code, public_reason, internal_reason,
		  decided_by_admin_id, decided_at, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $9, $10, $10, $10, 1)
		RETURNING `+disputeOutcomeReturningColumns+`
	`, input.DisputeCaseID, input.SubjectUserID, input.Responsibility, input.Severity, input.RoleScope,
		input.ReasonCode, strings.TrimSpace(input.PublicReason), strings.TrimSpace(input.InternalReason),
		input.AdminUserID, now))
	if isUniqueViolation(err) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Outcome exists", "该纠纷已有信誉裁定。")
	}
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	outcome.DisputeVersion = version + 1
	if appErr := insertReputationGovernanceEvent(ctx, tx, "outcome", outcome.ID, "outcome_created", input.AdminUserID, nil, outcome, input.InternalReason, input.RequestID, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	result := reputation.GovernanceMutationResult{Outcome: &outcome}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) CreateUserRestrictionWithIdempotency(ctx context.Context, entry idempotency.Entry, input reputation.CreateRestrictionInput, now time.Time, buildCompletion reputation.GovernanceCompletionBuilder) (reputation.GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	var userVersion int64
	if err := tx.QueryRow(ctx, `SELECT version FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&userVersion); errors.Is(err, pgx.ErrNoRows) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "用户不存在。")
	} else if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if userVersion != input.ExpectedUserVersion {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, reputationVersionConflict()
	}
	if input.SourceDisputeOutcomeID != "" {
		var subjectUserID, roleScope, outcomeStatus, targetType string
		if err := tx.QueryRow(ctx, `
			SELECT outcome.subject_user_id::text,
			       outcome.role_scope,
			       outcome.status,
			       dispute.target_type
			FROM dispute_reputation_outcomes outcome
			JOIN dispute_cases dispute ON dispute.id = outcome.dispute_case_id
			WHERE outcome.id = $1
			FOR UPDATE OF outcome, dispute
		`, input.SourceDisputeOutcomeID).Scan(&subjectUserID, &roleScope, &outcomeStatus, &targetType); errors.Is(err, pgx.ErrNoRows) {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Outcome not found", "关联信誉裁定不存在。")
		} else if err != nil {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
		}
		if outcomeStatus != reputation.OutcomeStatusActive || subjectUserID != input.UserID {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Outcome unavailable", "关联信誉裁定已反转或主体不匹配。")
		}
		if targetType == report.TargetAPIOrder {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderRestrictionRequiresDedicatedSanction()
		}
		if roleScope != reputation.RoleAll && input.RoleScope != roleScope {
			return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Role scope invalid", "限制角色不能超出关联裁定角色。", "roleScope", "invalid", "限制角色不能超出关联裁定角色。")
		}
	}

	restriction, err := scanUserRestriction(tx.QueryRow(ctx, `
		INSERT INTO user_restrictions (
		  user_id, restriction_type, reason, starts_at, ends_at, created_by_admin_id, created_at,
		  role_scope, action_code, reason_code, public_reason, source_dispute_outcome_id,
		  updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $7, 1)
		RETURNING `+userRestrictionReturningColumns+`
	`, input.UserID, strings.TrimSpace(input.RestrictionType), strings.TrimSpace(input.InternalReason),
		input.StartsAt, input.EndsAt, input.AdminUserID, now, input.RoleScope, input.ActionCode,
		input.ReasonCode, strings.TrimSpace(input.PublicReason), nullUUID(input.SourceDisputeOutcomeID)))
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if err := tx.QueryRow(ctx, `
		UPDATE users
		SET version = version + 1,
		    updated_at = $2
		WHERE id = $1
		RETURNING version
	`, input.UserID, now).Scan(&restriction.UserVersion); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertReputationGovernanceEvent(ctx, tx, "restriction", restriction.ID, "restriction_created", input.AdminUserID, nil, restriction, input.InternalReason, input.RequestID, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	result := reputation.GovernanceMutationResult{Restriction: &restriction}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func apiOrderRemedyOutcomeUnavailable() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Overdue remedy required", "API 订单责任裁定只能基于管理员已确认的逾期未履行事实。")
}

func apiOrderRestrictionRequiresDedicatedSanction() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Dedicated sanction required", "API 订单逾期限制必须通过纠纷处罚流程创建。")
}

func (s *Store) RevokeUserRestrictionWithIdempotency(ctx context.Context, entry idempotency.Entry, input reputation.RevokeRestrictionInput, now time.Time, buildCompletion reputation.GovernanceCompletionBuilder) (reputation.GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	before, err := scanUserRestriction(tx.QueryRow(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE id = $1
		FOR UPDATE
	`, input.RestrictionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Restriction not found", "信誉限制不存在。")
	}
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if before.Version != input.ExpectedVersion {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, reputationVersionConflict()
	}
	if before.RevokedAt != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Restriction revoked", "信誉限制已经撤销。")
	}
	after, err := scanUserRestriction(tx.QueryRow(ctx, `
		UPDATE user_restrictions
		SET revoked_at = $2,
		    revoked_by_admin_id = $3,
		    revocation_reason = $4,
		    updated_at = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING `+userRestrictionReturningColumns+`
	`, input.RestrictionID, now, input.AdminUserID, strings.TrimSpace(input.Reason)))
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertReputationGovernanceEvent(ctx, tx, "restriction", after.ID, "restriction_revoked", input.AdminUserID, before, after, input.Reason, input.RequestID, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	result := reputation.GovernanceMutationResult{Restriction: &after}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) FindActiveRestriction(ctx context.Context, userID, role, action string, now time.Time) (*reputation.UserRestriction, *domain.AppError) {
	item, err := scanUserRestriction(s.pool.QueryRow(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND starts_at <= $2
		  AND (ends_at IS NULL OR $2 < ends_at)
		  AND role_scope IN ($3, 'all')
		  AND action_code IN ($4, 'all')
		ORDER BY
		  CASE WHEN role_scope = $3 THEN 0 ELSE 1 END,
		  CASE WHEN action_code = $4 THEN 0 ELSE 1 END,
		  starts_at DESC,
		  id DESC
		LIMIT 1
	`, userID, now, role, action))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internalStoreError()
	}
	return &item, nil
}

func ensureAPIServicePublishAllowedInTx(ctx context.Context, tx pgx.Tx, sellerUserID string, now time.Time) *domain.AppError {
	var userExists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM users WHERE id = $1 FOR SHARE`, sellerUserID).Scan(&userExists); err != nil {
		return internalStoreError()
	}
	var publicReason string
	err := tx.QueryRow(ctx, `
		SELECT public_reason
		FROM user_restrictions
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND starts_at <= $2
		  AND (ends_at IS NULL OR $2 < ends_at)
		  AND role_scope IN ('seller', 'all')
		  AND action_code IN ('api_service_publish', 'all')
		ORDER BY
		  CASE WHEN role_scope = 'seller' THEN 0 ELSE 1 END,
		  CASE WHEN action_code = 'api_service_publish' THEN 0 ELSE 1 END,
		  starts_at DESC,
		  id DESC
		LIMIT 1
		FOR SHARE
	`, sellerUserID, now).Scan(&publicReason)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalStoreError()
	}
	if err == nil {
		if strings.TrimSpace(publicReason) == "" {
			publicReason = "当前信誉限制不允许执行该操作。"
		}
		return domain.NewError(http.StatusForbidden, domain.CodeReputationActionRestricted, "Reputation action restricted", publicReason)
	}

	var activeDispute bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM api_orders
			WHERE seller_user_id = $1
			  AND dispute_status IN ('negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation')
		)
	`, sellerUserID).Scan(&activeDispute); err != nil {
		return internalStoreError()
	}
	if activeDispute {
		return domain.NewError(http.StatusConflict, domain.CodeActiveAPIOrderDispute, "Active API order dispute", "当前存在未解决的 API 订单纠纷，暂不能发布或恢复 API 服务与额度，也不会接收新订单。请先完成纠纷处理。")
	}
	return nil
}

func disputeSubjectRole(ctx context.Context, tx pgx.Tx, targetType, targetID, subjectUserID string) (string, *domain.AppError) {
	var role string
	var err error
	switch targetType {
	case "public_user":
		if targetID == subjectUserID {
			return reputation.RoleAll, nil
		}
		return "", invalidDisputeSubject()
	case "carpool_application":
		err = tx.QueryRow(ctx, `
			SELECT CASE
			  WHEN buyer_user_id = $2 THEN 'buyer'
			  WHEN owner_user_id = $2 THEN 'seller'
			  ELSE ''
			END
			FROM carpool_applications
			WHERE id = $1
		`, targetID, subjectUserID).Scan(&role)
	case "carpool_membership":
		err = tx.QueryRow(ctx, `
			SELECT CASE
			  WHEN buyer_user_id = $2 THEN 'buyer'
			  WHEN owner_user_id = $2 THEN 'seller'
			  ELSE ''
			END
			FROM carpool_memberships
			WHERE id = $1
		`, targetID, subjectUserID).Scan(&role)
	case "api_purchase_intent":
		err = tx.QueryRow(ctx, `
			SELECT CASE
			  WHEN buyer_user_id = $2 THEN 'buyer'
			  WHEN owner_user_id = $2 THEN 'seller'
			  ELSE ''
			END
			FROM api_purchase_intents
			WHERE id = $1
		`, targetID, subjectUserID).Scan(&role)
	case "api_order":
		err = tx.QueryRow(ctx, `
			SELECT CASE
			  WHEN buyer_user_id = $2 THEN 'buyer'
			  WHEN seller_user_id = $2 THEN 'seller'
			  ELSE ''
			END
			FROM api_orders
			WHERE id = $1
		`, targetID, subjectUserID).Scan(&role)
	default:
		return "", invalidDisputeSubject()
	}
	if errors.Is(err, pgx.ErrNoRows) || role == "" {
		return "", invalidDisputeSubject()
	}
	if err != nil {
		return "", internalStoreError()
	}
	return role, nil
}

func invalidDisputeSubject() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Subject invalid", "纠纷主体必须是该目标中的实际交易参与方。", "subjectUserId", "invalid", "纠纷主体必须是该目标中的实际交易参与方。")
}

func reputationVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func scanDisputeOutcome(row pgx.Row) (reputation.DisputeOutcome, error) {
	var item reputation.DisputeOutcome
	err := row.Scan(
		&item.ID,
		&item.DisputeCaseID,
		&item.SubjectUserID,
		&item.Responsibility,
		&item.Severity,
		&item.RoleScope,
		&item.Status,
		&item.ReasonCode,
		&item.PublicReason,
		&item.InternalReason,
		&item.DecidedByAdminID,
		&item.DecidedAt,
		&item.ReversedAt,
		&item.ReversedByAdminID,
		&item.ReversalAppealID,
		&item.ReversalReason,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func scanUserRestriction(row pgx.Row) (reputation.UserRestriction, error) {
	var item reputation.UserRestriction
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.RestrictionType,
		&item.RoleScope,
		&item.ActionCode,
		&item.ReasonCode,
		&item.PublicReason,
		&item.InternalReason,
		&item.StartsAt,
		&item.EndsAt,
		&item.SourceDisputeOutcomeID,
		&item.SourceDisputeRemedyID,
		&item.CreatedByAdminID,
		&item.RevokedAt,
		&item.RevokedByAdminID,
		&item.RevocationReason,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func insertReputationGovernanceEvent(ctx context.Context, tx pgx.Tx, entityType, entityID, action, adminUserID string, before, after any, reason, requestID string, now time.Time) *domain.AppError {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	if before == nil {
		beforeJSON = []byte(`{}`)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO reputation_governance_events (
		  entity_type, entity_id, action, actor_admin_id,
		  before_json, after_json, reason, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9)
	`, entityType, entityID, action, adminUserID, string(beforeJSON), string(afterJSON),
		strings.TrimSpace(reason), strings.TrimSpace(requestID), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

const disputeOutcomeReturningColumns = `
	dispute_reputation_outcomes.id::text,
	dispute_reputation_outcomes.dispute_case_id::text,
	dispute_reputation_outcomes.subject_user_id::text,
	dispute_reputation_outcomes.responsibility,
	dispute_reputation_outcomes.severity,
	dispute_reputation_outcomes.role_scope,
	dispute_reputation_outcomes.status,
	dispute_reputation_outcomes.reason_code,
	dispute_reputation_outcomes.public_reason,
	dispute_reputation_outcomes.internal_reason,
	dispute_reputation_outcomes.decided_by_admin_id::text,
	dispute_reputation_outcomes.decided_at,
	dispute_reputation_outcomes.reversed_at,
	COALESCE(dispute_reputation_outcomes.reversed_by_admin_id::text, ''),
	COALESCE(dispute_reputation_outcomes.reversal_appeal_id::text, ''),
	dispute_reputation_outcomes.reversal_reason,
	dispute_reputation_outcomes.created_at,
	dispute_reputation_outcomes.updated_at,
	dispute_reputation_outcomes.version`

const userRestrictionColumns = `
	id::text,
	user_id::text,
	restriction_type,
	role_scope,
	action_code,
	reason_code,
	public_reason,
	reason,
	starts_at,
	ends_at,
	COALESCE(source_dispute_outcome_id::text, ''),
	COALESCE(source_dispute_remedy_id::text, ''),
	created_by_admin_id::text,
	revoked_at,
	COALESCE(revoked_by_admin_id::text, ''),
	revocation_reason,
	created_at,
	updated_at,
	version`

const userRestrictionReturningColumns = `
	user_restrictions.id::text,
	user_restrictions.user_id::text,
	user_restrictions.restriction_type,
	user_restrictions.role_scope,
	user_restrictions.action_code,
	user_restrictions.reason_code,
	user_restrictions.public_reason,
	user_restrictions.reason,
	user_restrictions.starts_at,
	user_restrictions.ends_at,
	COALESCE(user_restrictions.source_dispute_outcome_id::text, ''),
	COALESCE(user_restrictions.source_dispute_remedy_id::text, ''),
	user_restrictions.created_by_admin_id::text,
	user_restrictions.revoked_at,
	COALESCE(user_restrictions.revoked_by_admin_id::text, ''),
	user_restrictions.revocation_reason,
	user_restrictions.created_at,
	user_restrictions.updated_at,
	user_restrictions.version`

func scopeFacts(value *reputation.RawFacts, role, scope string) *reputation.ScopeFacts {
	var roleFacts *reputation.RoleFacts
	switch role {
	case reputation.RoleBuyer:
		roleFacts = &value.Buyer
	case reputation.RoleSeller:
		roleFacts = &value.Seller
	default:
		return nil
	}
	switch scope {
	case reputation.ScopeOverall:
		return &roleFacts.Overall
	case reputation.ScopeCarpool:
		return &roleFacts.Carpool
	case reputation.ScopeAPI:
		return &roleFacts.API
	default:
		return nil
	}
}

func latestTimestamp(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.After(*right) {
		return left
	}
	return right
}

func reputationTransactionExists(ctx context.Context, tx pgx.Tx, transactionType, transactionID string) (bool, *domain.AppError) {
	var query string
	switch transactionType {
	case reputation.TransactionCarpoolApplication:
		query = `SELECT EXISTS(SELECT 1 FROM carpool_applications WHERE id = $1)`
	case reputation.TransactionCarpoolMembership:
		query = `SELECT EXISTS(SELECT 1 FROM carpool_memberships WHERE id = $1)`
	case reputation.TransactionAPIPurchaseIntent:
		query = `SELECT EXISTS(SELECT 1 FROM api_purchase_intents WHERE id = $1)`
	case reputation.TransactionAPIOrder:
		query = `SELECT EXISTS(SELECT 1 FROM api_orders WHERE id = $1)`
	default:
		return false, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Transaction type invalid", "交易类型不支持信誉排除。", "transactionType", "invalid", "交易类型不支持信誉排除。")
	}
	var exists bool
	if err := tx.QueryRow(ctx, query, transactionID).Scan(&exists); err != nil {
		return false, internalStoreError()
	}
	return exists, nil
}

func lockTransactionExclusion(ctx context.Context, tx pgx.Tx, transactionType, transactionID string) (reputation.TransactionExclusion, bool, *domain.AppError) {
	value, err := scanTransactionExclusion(tx.QueryRow(ctx, `
		SELECT id::text, transaction_type, transaction_id::text, excluded_at,
		       excluded_by_admin_id::text, reason_code, reason, restored_at,
		       COALESCE(restored_by_admin_id::text, ''), created_at, updated_at
		FROM reputation_transaction_exclusions
		WHERE transaction_type = $1
		  AND transaction_id = $2
		FOR UPDATE
	`, transactionType, transactionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return reputation.TransactionExclusion{}, false, nil
	}
	if err != nil {
		return reputation.TransactionExclusion{}, false, internalStoreError()
	}
	return value, true, nil
}

func scanTransactionExclusion(row pgx.Row) (reputation.TransactionExclusion, error) {
	var value reputation.TransactionExclusion
	err := row.Scan(
		&value.ID,
		&value.TransactionType,
		&value.TransactionID,
		&value.ExcludedAt,
		&value.ExcludedByAdminID,
		&value.ReasonCode,
		&value.Reason,
		&value.RestoredAt,
		&value.RestoredByAdminID,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}

func insertTransactionExclusionEvent(ctx context.Context, tx pgx.Tx, exclusion reputation.TransactionExclusion, action string, input reputation.ExclusionMutation, now time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO reputation_transaction_exclusion_events (
		  exclusion_id, transaction_type, transaction_id, action,
		  actor_admin_id, reason_code, reason, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, exclusion.ID, exclusion.TransactionType, exclusion.TransactionID, action, input.AdminUserID, input.ReasonCode, input.Reason, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}
