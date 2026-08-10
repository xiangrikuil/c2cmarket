package apiorder

import (
	"context"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/openaiapi"
)

type DeliveryCredentialVerifier interface {
	Verify(ctx context.Context, baseURL, apiKey string, allowInsecureHTTP bool) openaiapi.Result
}

type OpenAIDeliveryCredentialVerifier struct {
	timeout time.Duration
}

func NewOpenAIDeliveryCredentialVerifier(timeout time.Duration) *OpenAIDeliveryCredentialVerifier {
	return &OpenAIDeliveryCredentialVerifier{timeout: timeout}
}

func (verifier *OpenAIDeliveryCredentialVerifier) Verify(ctx context.Context, baseURL, apiKey string, allowInsecureHTTP bool) openaiapi.Result {
	if verifier == nil {
		return openaiapi.Result{ErrorCode: openaiapi.ErrorInternal}
	}
	client, err := openaiapi.NewClient(baseURL, apiKey, openaiapi.Options{
		AllowInsecureHTTP: allowInsecureHTTP,
		Timeout:           verifier.timeout,
	})
	if err != nil {
		return openaiapi.Result{ErrorCode: openaiapi.ErrorBlockedTarget}
	}
	_, result := client.DiscoverModels(ctx)
	return result
}

func (s *Service) SetDeliveryCredentialVerifier(verifier DeliveryCredentialVerifier) {
	if s == nil {
		return
	}
	s.deliveryVerifier = verifier
}

func (s *Service) verifyAPIKeyDelivery(ctx context.Context, sellerUserID string, input ActionInput) *domain.AppError {
	order, appErr := s.SellerOrder(ctx, auth.User{ID: sellerUserID}, input.OrderID)
	if appErr != nil {
		return appErr
	}
	if input.ExpectedVersion > 0 && order.Version != input.ExpectedVersion {
		return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canTransition(order, "submit_delivery", s.now()) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单状态不能提交交付。")
	}
	if strings.TrimSpace(order.APIBaseURLSnapshot) == "" || strings.TrimSpace(order.NormalizedAPIBaseURLSnapshot) == "" {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "API target snapshot unavailable", "订单未冻结 API 目标，不能提交 API Key 交付。")
	}
	allowInsecureHTTP := usesInsecureHTTP(input.DeliveryCredential.APIBaseURL)
	target, err := openaiapi.NormalizeBaseURL(input.DeliveryCredential.APIBaseURL, allowInsecureHTTP)
	if err != nil {
		return deliveryFieldError("apiBaseUrl", "invalid", "API Base URL 格式不正确。")
	}
	if target.Canonical != order.NormalizedAPIBaseURLSnapshot {
		return deliveryFieldError("apiBaseUrl", "target_mismatch", "API Base URL 必须与订单冻结的探针连接地址一致。")
	}
	if s.deliveryVerifier == nil {
		if s.repo != nil {
			return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Delivery verifier unavailable", "交付鉴权服务暂时不可用。")
		}
		return nil
	}
	result := s.deliveryVerifier.Verify(ctx, order.APIBaseURLSnapshot, input.DeliveryCredential.APIKey, usesInsecureHTTP(order.APIBaseURLSnapshot))
	if result.Succeeded() {
		return nil
	}
	message := "平台无法使用该 API Key 完成模型列表鉴权，请检查后重试。"
	field := "apiKey"
	if result.ErrorCode == openaiapi.ErrorBlockedTarget || result.ErrorCode == openaiapi.ErrorDNS || result.ErrorCode == openaiapi.ErrorConnect || result.ErrorCode == openaiapi.ErrorTLS {
		field = "apiBaseUrl"
		message = "平台无法安全连接订单冻结的 API 地址，请检查目标配置后重试。"
	}
	return deliveryFieldError(field, string(result.ErrorCode), message)
}

func usesInsecureHTTP(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "http://")
}
