package core

import (
	"context"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
)

func (s *Service) GetAPIAccountPaymentSettings(ctx context.Context, user auth.User) (apimarket.AccountPaymentSettings, *domain.AppError) {
	return s.apiMarket.GetAccountPaymentSettings(ctx, user)
}

func (s *Service) UpdateAPIAccountPaymentSettings(ctx context.Context, user auth.User, input apimarket.UpdateAccountPaymentSettingsInput) (apimarket.AccountPaymentSettings, *domain.AppError) {
	return s.apiMarket.UpdateAccountPaymentSettings(ctx, user, input)
}
