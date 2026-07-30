package apimarket

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func (s *Manager) GetAccountPaymentSettings(ctx context.Context, user auth.User) (AccountPaymentSettings, *domain.AppError) {
	if s.repo != nil {
		settings, appErr := s.repo.GetAPIAccountPaymentSettings(ctx, user.ID)
		if appErr != nil {
			return AccountPaymentSettings{}, appErr
		}
		return normalizeAccountPaymentSettings(user.ID, settings), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return normalizeAccountPaymentSettings(user.ID, s.accountPaymentSettings[user.ID]), nil
}

func (s *Manager) UpdateAccountPaymentSettings(ctx context.Context, user auth.User, input UpdateAccountPaymentSettingsInput) (AccountPaymentSettings, *domain.AppError) {
	input.UserID = user.ID
	if appErr := validateAccountPaymentSettingsInput(input); appErr != nil {
		return AccountPaymentSettings{}, appErr
	}
	input.PaymentOptions = normalizePaymentOptionInputs(input.PaymentOptions)

	if s.repo != nil {
		settings, appErr := s.repo.UpdateAPIAccountPaymentSettings(ctx, input, s.now())
		if appErr != nil {
			return AccountPaymentSettings{}, appErr
		}
		return normalizeAccountPaymentSettings(user.ID, settings), nil
	}

	settings := normalizeAccountPaymentSettings(user.ID, AccountPaymentSettings{
		UserID:               user.ID,
		PaymentWindowMinutes: DefaultPaymentWindowMinutes,
		PaymentOptions:       input.PaymentOptions,
		UpdatedAt:            s.now(),
	})
	s.mu.Lock()
	s.accountPaymentSettings[user.ID] = settings
	s.mu.Unlock()
	return settings, nil
}

func validateAccountPaymentSettingsInput(input UpdateAccountPaymentSettingsInput) *domain.AppError {
	if input.PaymentWindowMinutes != DefaultPaymentWindowMinutes {
		return domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Payment window invalid",
			"买家确认付款窗口固定为 10 分钟。",
			"paymentWindowMinutes",
			"fixed",
			"付款窗口固定为 10 分钟。",
		)
	}
	enabledCount, appErr := validatePaymentOptionInputs(input.PaymentOptions)
	if appErr != nil {
		return appErr
	}
	if enabledCount != 1 {
		return domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Single payment method required",
			"微信支付和支付宝只能启用一种。",
			"paymentOptions",
			"single_enabled",
			"请选择一种收款方式。",
		)
	}
	return nil
}

func validatePaymentOptionInputs(options []PaymentOptionInput) (int, *domain.AppError) {
	if len(options) == 0 {
		return 0, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Payment option required",
			"至少配置一种收款方式。",
			"paymentOptions",
			"required",
			"至少配置一种收款方式。",
		)
	}

	seen := map[string]bool{}
	enabledCount := 0
	for i, option := range options {
		field := fmt.Sprintf("paymentOptions.%d", i)
		method := strings.TrimSpace(option.PaymentMethod)
		if !IsSupportedPaymentMethod(method) {
			return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "付款方式不支持。", field+".paymentMethod", "invalid", "付款方式不支持。")
		}
		if seen[method] {
			return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method duplicated", "付款方式不能重复。", field+".paymentMethod", "duplicate", "付款方式不能重复。")
		}
		seen[method] = true
		if option.Enabled {
			enabledCount++
			if requiresPaymentQRCode(method) && strings.TrimSpace(option.PaymentQRCodeDataURL) == "" {
				return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment QR code required", "启用微信或支付宝收款必须上传收款码。", field+".paymentQrCodeDataUrl", "required", "必须上传收款码。")
			}
			if !requiresPaymentQRCode(method) && strings.TrimSpace(option.PaymentInstructions) == "" {
				return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment instructions required", "启用收款方式必须填写收款说明。", field+".paymentInstructions", "required", "必须填写收款说明。")
			}
		}
		if err := validateOptionalNonSecretText(field+".paymentInstructions", option.PaymentInstructions); err != nil {
			return 0, err
		}
		if err := validateOptionalPaymentQRCodeDataURL(field+".paymentQrCodeDataUrl", option.PaymentQRCodeDataURL); err != nil {
			return 0, err
		}
	}
	if enabledCount > 1 {
		return 0, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Multiple payment methods enabled",
			"微信支付和支付宝只能启用一种。",
			"paymentOptions",
			"single_enabled",
			"只能启用一种收款方式。",
		)
	}
	return enabledCount, nil
}

func normalizeAccountPaymentSettings(userID string, settings AccountPaymentSettings) AccountPaymentSettings {
	settings.UserID = userID
	settings.PaymentWindowMinutes = DefaultPaymentWindowMinutes
	settings.PaymentOptions = normalizePaymentOptionInputs(settings.PaymentOptions)
	return settings
}

func normalizePaymentOptionInputs(options []PaymentOptionInput) []PaymentOptionInput {
	byMethod := make(map[string]PaymentOptionInput, len(options))
	for _, option := range options {
		method := strings.TrimSpace(option.PaymentMethod)
		if !IsSupportedPaymentMethod(method) {
			continue
		}
		option.PaymentMethod = method
		option.PaymentInstructions = strings.TrimSpace(option.PaymentInstructions)
		option.PaymentQRCodeDataURL = strings.TrimSpace(option.PaymentQRCodeDataURL)
		byMethod[method] = option
	}
	return []PaymentOptionInput{
		normalizedPaymentOptionInput(PaymentMethodWechat, byMethod[PaymentMethodWechat]),
		normalizedPaymentOptionInput(PaymentMethodAlipay, byMethod[PaymentMethodAlipay]),
	}
}

func normalizedPaymentOptionInput(method string, option PaymentOptionInput) PaymentOptionInput {
	option.PaymentMethod = method
	return option
}
