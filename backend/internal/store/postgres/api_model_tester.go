package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimodeltest"
	"c2c-market/backend/internal/module/apiorder"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListAPIModelTestOrderSources(ctx context.Context, buyerUserID string) ([]apimodeltest.OrderSource, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT orders.id::text, orders.order_no, orders.service_title_snapshot,
		       credential.api_base_url, credential.submitted_at
		FROM api_orders orders
		JOIN api_order_delivery_credentials credential
		  ON credential.api_order_id = orders.id
		WHERE orders.buyer_user_id = $1
		  AND orders.status IN ('delivery_submitted', 'completed')
		  AND credential.delivery_kind = 'api_key_endpoint'
		  AND credential.destroyed_at IS NULL
		ORDER BY credential.submitted_at DESC, orders.id DESC
	`, buyerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []apimodeltest.OrderSource{}
	for rows.Next() {
		var item apimodeltest.OrderSource
		if err := rows.Scan(&item.OrderID, &item.OrderNo, &item.ServiceTitle, &item.BaseURL, &item.DeliveredAt); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) GetAPIModelTestOrderCredential(ctx context.Context, buyerUserID, orderID string) (apimodeltest.OrderCredential, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, tx, orderID); err != nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	var status string
	var credentialID, deliveryKind, baseURL string
	var ciphertext, nonce []byte
	var keyVersion, cipherFormat string
	var destroyedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT orders.status,
		       COALESCE(credential.id::text, ''), COALESCE(credential.delivery_kind, ''),
		       COALESCE(credential.api_base_url, ''), credential.api_key_ciphertext,
		       credential.api_key_nonce, COALESCE(credential.secret_encryption_key_version, ''),
		       COALESCE(credential.secret_encryption_format, ''), credential.destroyed_at
		FROM api_orders orders
		LEFT JOIN api_order_delivery_credentials credential
		  ON credential.api_order_id = orders.id
		WHERE orders.id = $1 AND orders.buyer_user_id = $2
	`, orderID, buyerUserID).Scan(
		&status,
		&credentialID,
		&deliveryKind,
		&baseURL,
		&ciphertext,
		&nonce,
		&keyVersion,
		&cipherFormat,
		&destroyedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apimodeltest.OrderCredential{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API order not found", "API 订单不存在。")
	}
	if err != nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	if (status != apiorder.StatusDeliverySubmitted && status != apiorder.StatusCompleted) ||
		strings.TrimSpace(credentialID) == "" || deliveryKind != apiorder.DeliveryKindAPIKeyEndpoint ||
		destroyedAt != nil || strings.TrimSpace(baseURL) == "" || len(ciphertext) == 0 {
		return apimodeltest.OrderCredential{}, apiModelTestCredentialUnavailable()
	}
	apiKey, err := s.contactCodec.decode(ciphertext, nonce, keyVersion, cipherFormat, credentialID, contactFieldOrderAPIKey)
	if err != nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	if strings.TrimSpace(apiKey) == "" {
		return apimodeltest.OrderCredential{}, apiModelTestCredentialUnavailable()
	}
	if err := tx.Commit(ctx); err != nil {
		return apimodeltest.OrderCredential{}, internalStoreError()
	}
	return apimodeltest.OrderCredential{BaseURL: baseURL, APIKey: apiKey}, nil
}

func apiModelTestCredentialUnavailable() *domain.AppError {
	return domain.NewError(
		http.StatusConflict,
		domain.CodeAPIModelTestCredentialUnavailable,
		"API order credential unavailable",
		"该订单当前没有可用于模型测试的未销毁 API Key 交付凭据。",
	)
}
