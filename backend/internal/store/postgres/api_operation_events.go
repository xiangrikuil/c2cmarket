package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	apiOperationActorUser   = "user"
	apiOperationActorAdmin  = "admin"
	apiOperationActorSystem = "system"
)

// apiQuotaCredentialImportMetadata 只接受导入数量和交付类型，函数签名从源头
// 排除了 CSV、凭据、指纹及交付说明进入领域事件的可能。
func apiQuotaCredentialImportMetadata(importedCount int, deliveryKind string) map[string]any {
	return map[string]any{
		"importedCount": importedCount,
		"deliveryKind":  strings.TrimSpace(deliveryKind),
	}
}

func insertAPIOperationDomainEvent(
	ctx context.Context,
	tx pgx.Tx,
	aggregateType string,
	aggregateID string,
	eventType string,
	actorUserID string,
	actorKind string,
	aggregateVersion int64,
	requestID string,
	metadata any,
	createdAt time.Time,
) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, uuid.NewString(), aggregateType, aggregateID, eventType, nullUUID(actorUserID), actorKind,
		aggregateVersion, requestID, encoded, createdAt.UTC())
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertAPIServiceAdminAudit(
	ctx context.Context,
	tx pgx.Tx,
	adminUserID string,
	serviceID string,
	action string,
	requestID string,
	before map[string]string,
	after map[string]string,
	createdAt time.Time,
) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			id, admin_user_id, action, target_type, target_id,
			before_json, after_json, request_id, created_at
		) VALUES ($1, $2, $3, 'api_service', $4, $5, $6, $7, $8)
	`, uuid.NewString(), adminUserID, action, serviceID, beforeJSON, afterJSON, requestID, createdAt.UTC())
	if err != nil {
		return internalStoreError()
	}
	return nil
}
