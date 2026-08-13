package postgres

import (
	"context"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/operationaudit"
)

func (s *Store) ListOperationAudit(ctx context.Context, query operationaudit.Query) ([]operationaudit.Entry, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	return listOperationAudit(ctx, s.pool, query)
}

func listOperationAudit(ctx context.Context, reader rowQueryer, query operationaudit.Query) ([]operationaudit.Entry, *domain.AppError) {
	if reader == nil {
		return nil, internalStoreError()
	}
	statement, args := buildOperationAuditQuery(query)
	rows, err := reader.Query(ctx, statement, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := make([]operationaudit.Entry, 0, query.Limit+1)
	for rows.Next() {
		var item operationaudit.Entry
		if err := rows.Scan(
			&item.SourceEventID,
			&item.SourceKind,
			&item.ActorKind,
			&item.ActorUserID,
			&item.ActorUsername,
			&item.Action,
			&item.TargetType,
			&item.TargetID,
			&item.RequestID,
			&item.CreatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

type operationAuditBranch struct {
	sourceKind     string
	fromClause     string
	eventID        string
	timeColumn     string
	action         string
	actorKind      string
	actorUserID    string
	actorUsername  string
	targetType     string
	targetID       string
	requestID      string
	extraCondition string
}

type operationAuditTuple struct {
	definition operationaudit.ActionDefinition
	actorKind  string
}

func buildOperationAuditQuery(query operationaudit.Query) (string, []any) {
	args := make([]any, 0, 64)
	addArgument := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}

	tuplesBySource := eligibleOperationAuditTuples(query)

	fromPlaceholder := addArgument(query.From)
	toPlaceholder := addArgument(query.To)
	var actionPlaceholder, actorKindPlaceholder, actorUserPlaceholder, targetTypePlaceholder, targetIDPlaceholder string
	if query.Action != "" {
		actionPlaceholder = addArgument(query.Action)
	}
	if query.ActorKind != "" {
		actorKindPlaceholder = addArgument(query.ActorKind)
	}
	if query.ActorUserID != "" {
		actorUserPlaceholder = addArgument(query.ActorUserID)
	}
	if query.TargetType != "" {
		targetTypePlaceholder = addArgument(query.TargetType)
	}
	if query.TargetID != "" {
		targetIDPlaceholder = addArgument(query.TargetID)
	}
	var cursorTimePlaceholder, cursorSourcePlaceholder, cursorIDPlaceholder string
	if query.Cursor != nil {
		cursorTimePlaceholder = addArgument(query.Cursor.OccurredAt)
		cursorSourcePlaceholder = addArgument(query.Cursor.SourceKind)
		cursorIDPlaceholder = addArgument(query.Cursor.EventID)
	}
	limitPlaceholder := addArgument(query.Limit + 1)

	branches := []operationAuditBranch{
		{
			sourceKind: operationaudit.SourceAdmin,
			fromClause: "admin_audit_logs event LEFT JOIN users actor ON actor.id = event.admin_user_id",
			eventID:    "event.id", timeColumn: "event.created_at", action: "event.action",
			actorKind: "'admin'", actorUserID: "event.admin_user_id", actorUsername: "actor.username",
			targetType: "event.target_type", targetID: "event.target_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceModeration,
			fromClause: "moderation_audit_logs event LEFT JOIN users actor ON actor.id = event.actor_admin_id",
			eventID:    "event.id", timeColumn: "event.created_at", action: "event.action",
			actorKind: "'admin'", actorUserID: "event.actor_admin_id", actorUsername: "actor.username",
			targetType: "event.object_type", targetID: "event.object_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceDomain,
			fromClause: "domain_events event LEFT JOIN users actor ON actor.id = event.actor_user_id",
			eventID:    "event.id", timeColumn: "event.created_at", action: "event.event_type",
			actorKind: "event.actor_kind", actorUserID: "event.actor_user_id", actorUsername: "actor.username",
			targetType: "event.aggregate_type", targetID: "event.aggregate_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceAPIOrder,
			fromClause: "api_order_events event LEFT JOIN users actor ON actor.id = event.actor_user_id",
			eventID:    "event.id", timeColumn: "event.created_at", action: "event.event_type",
			actorKind:   "CASE WHEN event.actor_user_id IS NULL THEN 'system' ELSE 'user' END",
			actorUserID: "event.actor_user_id", actorUsername: "actor.username",
			targetType: "'api_order'", targetID: "event.api_order_id", requestID: "event.request_id",
			extraCondition: `(
				(event.actor_user_id IS NULL AND event.event_type IN (
					'api_order.payment_timeout_cancelled',
					'api_order.delivery_review_reminder_sent',
					'api_order.auto_completed'
				)) OR
				(event.actor_user_id IS NOT NULL AND event.event_type NOT IN (
					'api_order.payment_timeout_cancelled',
					'api_order.delivery_review_reminder_sent',
					'api_order.auto_completed'
				))
			)`,
		},
		{
			sourceKind: operationaudit.SourceContactSessionAccess,
			fromClause: "contact_access_logs event LEFT JOIN users actor ON actor.id = event.viewer_user_id",
			eventID:    "event.id", timeColumn: "event.accessed_at", action: "'contact_session.accessed'",
			actorKind: "'user'", actorUserID: "event.viewer_user_id", actorUsername: "actor.username",
			targetType: "'contact_session'", targetID: "event.contact_session_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceAPIIntentContactAccess,
			fromClause: "api_purchase_intent_contact_access_logs event LEFT JOIN users actor ON actor.id = event.viewer_user_id",
			eventID:    "event.id", timeColumn: "event.accessed_at", action: "'api_purchase_intent.contact_accessed'",
			actorKind: "'user'", actorUserID: "event.viewer_user_id", actorUsername: "actor.username",
			targetType: "'api_purchase_intent'", targetID: "event.api_purchase_intent_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceAPIOrderAccess,
			fromClause: "api_order_payment_instruction_access_logs event LEFT JOIN users actor ON actor.id = event.buyer_user_id",
			eventID:    "event.id", timeColumn: "event.accessed_at", action: "'api_order.payment_instructions_accessed'",
			actorKind: "'user'", actorUserID: "event.buyer_user_id", actorUsername: "actor.username",
			targetType: "'api_order'", targetID: "event.api_order_id", requestID: "event.request_id",
		},
		{
			sourceKind: operationaudit.SourceProbe,
			fromClause: "api_probe_connection_events event LEFT JOIN users actor ON actor.id = event.actor_user_id",
			eventID:    "event.id", timeColumn: "event.occurred_at", action: "event.action",
			actorKind: "event.actor_kind", actorUserID: "event.actor_user_id", actorUsername: "actor.username",
			targetType: "'api_probe_connection'", targetID: "event.target_connection_id", requestID: "event.request_id",
		},
	}

	rawBranches := make([]string, 0, len(branches))
	for _, branch := range branches {
		tuples := tuplesBySource[branch.sourceKind]
		if len(tuples) == 0 {
			continue
		}
		conditions := []string{
			branch.timeColumn + " >= " + fromPlaceholder,
			branch.timeColumn + " <= " + toPlaceholder,
			operationAuditTuplePredicate(branch, tuples, addArgument),
		}
		if branch.extraCondition != "" {
			conditions = append(conditions, branch.extraCondition)
		}
		if actionPlaceholder != "" {
			conditions = append(conditions, branch.action+" = "+actionPlaceholder)
		}
		if actorKindPlaceholder != "" {
			conditions = append(conditions, branch.actorKind+" = "+actorKindPlaceholder)
		}
		if actorUserPlaceholder != "" {
			conditions = append(conditions, branch.actorUserID+" = "+actorUserPlaceholder+"::uuid")
		}
		if targetTypePlaceholder != "" {
			conditions = append(conditions, branch.targetType+" = "+targetTypePlaceholder)
		}
		if targetIDPlaceholder != "" {
			conditions = append(conditions, branch.targetID+" = "+targetIDPlaceholder+"::uuid")
		}
		if query.Cursor != nil {
			conditions = append(conditions,
				branch.timeColumn+" <= "+cursorTimePlaceholder+" AND ("+
					branch.timeColumn+", '"+branch.sourceKind+"'::text, "+branch.eventID+") < ("+
					cursorTimePlaceholder+", "+cursorSourcePlaceholder+", "+cursorIDPlaceholder+"::uuid)",
			)
		}
		if query.Search != "" {
			conditions = append(conditions, operationAuditSearchPredicate(branch, tuples, strings.ToLower(query.Search), addArgument))
		}
		rawBranches = append(rawBranches, `(
		SELECT `+branch.eventID+` AS source_event_id,
		       '`+branch.sourceKind+`'::text AS source_kind,
		       `+branch.actorKind+`::text AS actor_kind,
		       COALESCE(`+branch.actorUserID+`::text, '') AS actor_user_id,
		       COALESCE(`+branch.actorUsername+`, '') AS actor_username,
		       `+branch.action+`::text AS action,
		       `+branch.targetType+`::text AS target_type,
		       COALESCE(`+branch.targetID+`::text, '') AS target_id,
		       COALESCE(`+branch.requestID+`, '') AS request_id,
		       `+branch.timeColumn+` AS occurred_at
		FROM `+branch.fromClause+`
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY `+branch.timeColumn+` DESC, `+branch.eventID+` DESC
		LIMIT `+limitPlaceholder+`)`)
	}

	if len(rawBranches) == 0 {
		rawBranches = append(rawBranches, `(
		SELECT NULL::uuid AS source_event_id, NULL::text AS source_kind,
		       NULL::text AS actor_kind, NULL::text AS actor_user_id,
		       NULL::text AS actor_username, NULL::text AS action,
		       NULL::text AS target_type, NULL::text AS target_id,
		       NULL::text AS request_id, NULL::timestamptz AS occurred_at
		WHERE false)`)
	}

	statement := `WITH raw AS (` + strings.Join(rawBranches, "\nUNION ALL\n") + `
)
SELECT raw.source_event_id::text, raw.source_kind, raw.actor_kind,
	   raw.actor_user_id, raw.actor_username, raw.action,
	   raw.target_type, raw.target_id, raw.request_id,
	   raw.occurred_at
FROM raw`
	statement += "\nORDER BY raw.occurred_at DESC, raw.source_kind DESC, raw.source_event_id DESC"
	statement += "\nLIMIT " + limitPlaceholder
	return statement, args
}

func eligibleOperationAuditTuples(query operationaudit.Query) map[string][]operationAuditTuple {
	result := make(map[string][]operationAuditTuple, len(operationaudit.SourceKinds))
	for _, definition := range operationaudit.ActionRegistry() {
		if query.SourceKind != "" && definition.SourceKind != query.SourceKind ||
			query.Domain != "" && definition.Domain != query.Domain ||
			query.Action != "" && definition.Action != query.Action ||
			query.TargetType != "" && definition.TargetType != query.TargetType ||
			query.Outcome != "" && definition.Outcome != query.Outcome {
			continue
		}
		for _, actorKind := range operationaudit.AllowedActorKinds(definition) {
			if query.ActorKind != "" && actorKind != query.ActorKind {
				continue
			}
			result[definition.SourceKind] = append(result[definition.SourceKind], operationAuditTuple{
				definition: definition,
				actorKind:  actorKind,
			})
		}
	}
	return result
}

func operationAuditTuplePredicate(
	branch operationAuditBranch,
	tuples []operationAuditTuple,
	addArgument func(any) string,
) string {
	predicates := make([]string, 0, len(tuples))
	for _, tuple := range tuples {
		predicates = append(predicates, "("+strings.Join([]string{
			branch.action + " = " + addArgument(tuple.definition.Action),
			branch.targetType + " = " + addArgument(tuple.definition.TargetType),
			branch.actorKind + " = " + addArgument(tuple.actorKind),
		}, " AND ")+")")
	}
	return "(" + strings.Join(predicates, " OR ") + ")"
}

func operationAuditSearchPredicate(
	branch operationAuditBranch,
	tuples []operationAuditTuple,
	search string,
	addArgument func(any) string,
) string {
	placeholder := addArgument(search)
	dynamic := []string{
		"strpos(lower(" + branch.eventID + "::text), " + placeholder + ") > 0",
		"strpos(lower('" + branch.sourceKind + "'), " + placeholder + ") > 0",
		"strpos(lower(" + branch.action + "::text), " + placeholder + ") > 0",
		"strpos(lower(COALESCE(" + branch.actorUsername + ", '')), " + placeholder + ") > 0",
		"strpos(lower(COALESCE(" + branch.actorUserID + "::text, '')), " + placeholder + ") > 0",
		"strpos(lower(" + branch.targetType + "::text), " + placeholder + ") > 0",
		"strpos(lower(COALESCE(" + branch.targetID + "::text, '')), " + placeholder + ") > 0",
		"strpos(lower(COALESCE(" + branch.requestID + ", '')), " + placeholder + ") > 0",
	}
	fixedMatches := make([]operationAuditTuple, 0, len(tuples))
	for _, tuple := range tuples {
		definition := tuple.definition
		if strings.Contains(strings.ToLower(definition.Domain), search) ||
			strings.Contains(strings.ToLower(definition.ActionLabel), search) ||
			strings.Contains(strings.ToLower(definition.Summary), search) {
			fixedMatches = append(fixedMatches, tuple)
		}
	}
	if len(fixedMatches) > 0 {
		dynamic = append(dynamic, operationAuditTuplePredicate(branch, fixedMatches, addArgument))
	}
	return "(" + strings.Join(dynamic, " OR ") + ")"
}
