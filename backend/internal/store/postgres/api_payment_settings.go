package postgres

import (
	"context"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
)

func (s *Store) GetAPIAccountPaymentSettings(ctx context.Context, userID string) (apimarket.AccountPaymentSettings, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT payment_method, enabled, payment_instructions,
		       COALESCE(payment_qr_code_data_url, ''), updated_at
		FROM api_payment_account_options
		WHERE user_id = $1
		ORDER BY payment_method
	`, userID)
	if err != nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	defer rows.Close()

	settings := apimarket.AccountPaymentSettings{
		UserID:               userID,
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
	}
	for rows.Next() {
		var option apimarket.PaymentOptionInput
		var updatedAt time.Time
		if err := rows.Scan(
			&option.PaymentMethod,
			&option.Enabled,
			&option.PaymentInstructions,
			&option.PaymentQRCodeDataURL,
			&updatedAt,
		); err != nil {
			return apimarket.AccountPaymentSettings{}, internalStoreError()
		}
		settings.PaymentOptions = append(settings.PaymentOptions, option)
		if updatedAt.After(settings.UpdatedAt) {
			settings.UpdatedAt = updatedAt
		}
	}
	if rows.Err() != nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	return settings, nil
}

func (s *Store) UpdateAPIAccountPaymentSettings(ctx context.Context, input apimarket.UpdateAccountPaymentSettingsInput, now time.Time) (apimarket.AccountPaymentSettings, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	if _, err := tx.Exec(ctx, `DELETE FROM api_payment_account_options WHERE user_id = $1`, input.UserID); err != nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	for _, option := range input.PaymentOptions {
		if !option.Enabled && strings.TrimSpace(option.PaymentInstructions) == "" && strings.TrimSpace(option.PaymentQRCodeDataURL) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_payment_account_options (
				user_id, payment_method, enabled, payment_instructions,
				payment_qr_code_data_url, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $6, 1)
		`, input.UserID, strings.TrimSpace(option.PaymentMethod), option.Enabled,
			strings.TrimSpace(option.PaymentInstructions), nullText(option.PaymentQRCodeDataURL), now); err != nil {
			return apimarket.AccountPaymentSettings{}, internalStoreError()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.AccountPaymentSettings{}, internalStoreError()
	}
	return apimarket.AccountPaymentSettings{
		UserID:               input.UserID,
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions:       append([]apimarket.PaymentOptionInput(nil), input.PaymentOptions...),
		UpdatedAt:            now,
	}, nil
}
