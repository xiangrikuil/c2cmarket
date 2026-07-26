package postgres

import (
	"context"
	"encoding/json"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

type reputationSnapshotKeyRecord struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Scope  string `json:"scope"`
}

type reputationSnapshotRecord struct {
	UserID              string          `json:"user_id"`
	Role                string          `json:"role"`
	Scope               string          `json:"scope"`
	Tier                string          `json:"tier"`
	State               string          `json:"state"`
	Confidence          string          `json:"confidence"`
	RuleVersion         string          `json:"rule_version"`
	Metrics             json.RawMessage `json:"metrics_json"`
	Warnings            json.RawMessage `json:"warnings_json"`
	Badges              json.RawMessage `json:"badges_json"`
	Progress            json.RawMessage `json:"progress_json"`
	TierEnteredAt       time.Time       `json:"tier_entered_at"`
	ReliableSince       *time.Time      `json:"reliable_since"`
	StateEnteredAt      time.Time       `json:"state_entered_at"`
	CalculatedAt        time.Time       `json:"calculated_at"`
	SourceDataUpdatedAt *time.Time      `json:"source_data_updated_at"`
	NextRecalculationAt *time.Time      `json:"next_recalculation_at"`
}

const reputationSnapshotRecordset = `
  SELECT *
  FROM jsonb_to_recordset($1::jsonb) AS incoming(
    user_id uuid,
    role text,
    scope text,
    tier text,
    state text,
    confidence text,
    rule_version text,
    metrics_json jsonb,
    warnings_json jsonb,
    badges_json jsonb,
    progress_json jsonb,
    tier_entered_at timestamptz,
    reliable_since timestamptz,
    state_entered_at timestamptz,
    calculated_at timestamptz,
    source_data_updated_at timestamptz,
    next_recalculation_at timestamptz
  )
`

func (s *Store) LoadReputationSnapshots(ctx context.Context, keys []reputation.SnapshotKey) (map[reputation.SnapshotKey]reputation.ReputationSnapshot, *domain.AppError) {
	result := make(map[reputation.SnapshotKey]reputation.ReputationSnapshot, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	records := make([]reputationSnapshotKeyRecord, 0, len(keys))
	for _, key := range keys {
		records = append(records, reputationSnapshotKeyRecord{
			UserID: key.UserID,
			Role:   key.Role,
			Scope:  key.Scope,
		})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		WITH requested AS (
		  SELECT *
		  FROM jsonb_to_recordset($1::jsonb) AS input(
		    user_id uuid,
		    role text,
		    scope text
		  )
		)
		SELECT
		  state.user_id::text,
		  state.role,
		  state.scope,
		  state.tier,
		  state.state,
		  state.confidence,
		  state.rule_version,
		  state.metrics_json,
		  state.warnings_json,
		  state.badges_json,
		  state.progress_json,
		  state.tier_entered_at,
		  state.reliable_since,
		  state.state_entered_at,
		  state.dirty_at,
		  state.calculated_at,
		  state.source_data_updated_at,
		  state.next_recalculation_at
		FROM user_reputation_states state
		JOIN requested
		  ON requested.user_id = state.user_id
		 AND requested.role = state.role
		 AND requested.scope = state.scope
	`, payload)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	for rows.Next() {
		var (
			snapshot     reputation.ReputationSnapshot
			metricsJSON  []byte
			warningsJSON []byte
			badgesJSON   []byte
			progressJSON []byte
		)
		if err := rows.Scan(
			&snapshot.UserID,
			&snapshot.Role,
			&snapshot.Scope,
			&snapshot.Tier,
			&snapshot.State,
			&snapshot.Confidence,
			&snapshot.RuleVersion,
			&metricsJSON,
			&warningsJSON,
			&badgesJSON,
			&progressJSON,
			&snapshot.TierEnteredAt,
			&snapshot.ReliableSince,
			&snapshot.StateEnteredAt,
			&snapshot.DirtyAt,
			&snapshot.CalculatedAt,
			&snapshot.SourceDataUpdatedAt,
			&snapshot.NextRecalculationAt,
		); err != nil {
			return nil, internalStoreError()
		}
		if err := json.Unmarshal(metricsJSON, &snapshot.Metrics); err != nil {
			return nil, internalStoreError()
		}
		if snapshot.Metrics.CommonPositiveTags == nil {
			snapshot.Metrics.CommonPositiveTags = []reputation.ReputationTagCount{}
		}
		if snapshot.Metrics.CommonNegativeTags == nil {
			snapshot.Metrics.CommonNegativeTags = []reputation.ReputationTagCount{}
		}
		if err := json.Unmarshal(warningsJSON, &snapshot.Warnings); err != nil {
			return nil, internalStoreError()
		}
		if err := json.Unmarshal(badgesJSON, &snapshot.Badges); err != nil {
			return nil, internalStoreError()
		}
		if err := json.Unmarshal(progressJSON, &snapshot.Progress); err != nil {
			return nil, internalStoreError()
		}
		result[snapshot.Key()] = snapshot
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (s *Store) SaveReputationSnapshots(ctx context.Context, snapshots []reputation.ReputationSnapshot) *domain.AppError {
	if len(snapshots) == 0 {
		return nil
	}
	records := make([]reputationSnapshotRecord, 0, len(snapshots))
	for _, snapshot := range snapshots {
		metricsJSON, err := json.Marshal(snapshot.Metrics)
		if err != nil {
			return internalStoreError()
		}
		warningsJSON, err := json.Marshal(nonNilStrings(snapshot.Warnings))
		if err != nil {
			return internalStoreError()
		}
		badgesJSON, err := json.Marshal(nonNilStrings(snapshot.Badges))
		if err != nil {
			return internalStoreError()
		}
		progressJSON, err := json.Marshal(nonNilProgress(snapshot.Progress))
		if err != nil {
			return internalStoreError()
		}
		records = append(records, reputationSnapshotRecord{
			UserID:              snapshot.UserID,
			Role:                snapshot.Role,
			Scope:               snapshot.Scope,
			Tier:                snapshot.Tier,
			State:               snapshot.State,
			Confidence:          snapshot.Confidence,
			RuleVersion:         snapshot.RuleVersion,
			Metrics:             metricsJSON,
			Warnings:            warningsJSON,
			Badges:              badgesJSON,
			Progress:            progressJSON,
			TierEnteredAt:       snapshot.TierEnteredAt,
			ReliableSince:       snapshot.ReliableSince,
			StateEnteredAt:      snapshot.StateEnteredAt,
			CalculatedAt:        snapshot.CalculatedAt,
			SourceDataUpdatedAt: snapshot.SourceDataUpdatedAt,
			NextRecalculationAt: snapshot.NextRecalculationAt,
		})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return internalStoreError()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	lockRows, err := tx.Query(ctx, `
		WITH incoming AS (`+reputationSnapshotRecordset+`)
		SELECT state.user_id
		FROM user_reputation_states state
		JOIN incoming
		  ON incoming.user_id = state.user_id
		 AND incoming.role = state.role
		 AND incoming.scope = state.scope
		ORDER BY state.user_id, state.role, state.scope
		FOR UPDATE
	`, payload)
	if err != nil {
		return internalStoreError()
	}
	lockRows.Close()
	if lockRows.Err() != nil {
		return internalStoreError()
	}

	if _, err := tx.Exec(ctx, `
		WITH incoming AS (`+reputationSnapshotRecordset+`)
		INSERT INTO user_reputation_history (
		  user_id,
		  role,
		  scope,
		  from_tier,
		  to_tier,
		  from_state,
		  to_state,
		  rule_version,
		  reason_snapshot,
		  created_at
		)
		SELECT
		  incoming.user_id,
		  incoming.role,
		  incoming.scope,
		  previous.tier,
		  incoming.tier,
		  previous.state,
		  incoming.state,
		  incoming.rule_version,
		  jsonb_build_object(
		    'metrics', incoming.metrics_json,
		    'warnings', incoming.warnings_json,
		    'badges', incoming.badges_json,
		    'progress', incoming.progress_json,
		    'calculatedAt', incoming.calculated_at
		  ),
		  incoming.calculated_at
		FROM incoming
		LEFT JOIN user_reputation_states previous
		  ON previous.user_id = incoming.user_id
		 AND previous.role = incoming.role
		 AND previous.scope = incoming.scope
		WHERE previous.user_id IS NULL
		   OR previous.tier IS DISTINCT FROM incoming.tier
		   OR previous.state IS DISTINCT FROM incoming.state
	`, payload); err != nil {
		return internalStoreError()
	}

	if _, err := tx.Exec(ctx, `
		WITH incoming AS (`+reputationSnapshotRecordset+`)
		INSERT INTO user_reputation_states (
		  user_id,
		  role,
		  scope,
		  tier,
		  state,
		  confidence,
		  rule_version,
		  metrics_json,
		  warnings_json,
		  badges_json,
		  progress_json,
		  tier_entered_at,
		  reliable_since,
		  state_entered_at,
		  dirty_at,
		  calculated_at,
		  source_data_updated_at,
		  next_recalculation_at
		)
		SELECT
		  user_id,
		  role,
		  scope,
		  tier,
		  state,
		  confidence,
		  rule_version,
		  metrics_json,
		  warnings_json,
		  badges_json,
		  progress_json,
		  tier_entered_at,
		  reliable_since,
		  state_entered_at,
		  NULL,
		  calculated_at,
		  source_data_updated_at,
		  next_recalculation_at
		FROM incoming
		ON CONFLICT (user_id, role, scope) DO UPDATE
		SET tier = EXCLUDED.tier,
		    state = EXCLUDED.state,
		    confidence = EXCLUDED.confidence,
		    rule_version = EXCLUDED.rule_version,
		    metrics_json = EXCLUDED.metrics_json,
		    warnings_json = EXCLUDED.warnings_json,
		    badges_json = EXCLUDED.badges_json,
		    progress_json = EXCLUDED.progress_json,
		    tier_entered_at = EXCLUDED.tier_entered_at,
		    reliable_since = EXCLUDED.reliable_since,
		    state_entered_at = EXCLUDED.state_entered_at,
		    dirty_at = NULL,
		    calculated_at = EXCLUDED.calculated_at,
		    source_data_updated_at = EXCLUDED.source_data_updated_at,
		    next_recalculation_at = EXCLUDED.next_recalculation_at
	`, payload); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListReputationUserIDs(ctx context.Context) ([]string, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM users ORDER BY id`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, internalStoreError()
		}
		result = append(result, userID)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (s *Store) ListReputationHistory(ctx context.Context, userID string, limit int) ([]reputation.ReputationHistory, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT
		  id::text,
		  user_id::text,
		  role,
		  scope,
		  from_tier,
		  to_tier,
		  from_state,
		  to_state,
		  rule_version,
		  reason_snapshot,
		  created_at
		FROM user_reputation_history
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	result := make([]reputation.ReputationHistory, 0)
	for rows.Next() {
		var (
			item       reputation.ReputationHistory
			reasonJSON []byte
		)
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Role,
			&item.Scope,
			&item.FromTier,
			&item.ToTier,
			&item.FromState,
			&item.ToState,
			&item.RuleVersion,
			&reasonJSON,
			&item.CreatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		var reason map[string]any
		if err := json.Unmarshal(reasonJSON, &reason); err != nil {
			return nil, internalStoreError()
		}
		item.ReasonSnapshot = reason
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilProgress(values []reputation.ReputationProgressItem) []reputation.ReputationProgressItem {
	if values == nil {
		return []reputation.ReputationProgressItem{}
	}
	return values
}
