package apimodeltest

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/openaiapi"

	"github.com/google/uuid"
)

const (
	defaultTimeout  = 15 * time.Second
	maxBaseURLRunes = 1000
	maxAPIKeyRunes  = 4096
	maxModelIDRunes = 512
)

type openAIClient interface {
	DiscoverModels(ctx context.Context) ([]string, openaiapi.Result)
	TestResponses(ctx context.Context, model string) openaiapi.Result
	TestChatCompletions(ctx context.Context, model string) openaiapi.Result
}

type clientFactory func(baseURL, apiKey string, options openaiapi.Options) (openAIClient, error)

type Service struct {
	repo      Repository
	timeout   time.Duration
	now       func() time.Time
	newClient clientFactory
}

func NewService(repo Repository, timeout time.Duration, now func() time.Time) *Service {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if now == nil {
		now = time.Now
	}
	return &Service{
		repo:    repo,
		timeout: timeout,
		now:     now,
		newClient: func(baseURL, apiKey string, options openaiapi.Options) (openAIClient, error) {
			return openaiapi.NewClient(baseURL, apiKey, options)
		},
	}
}

func (s *Service) OrderSources(ctx context.Context, user auth.User) ([]OrderSource, *domain.AppError) {
	if s == nil || strings.TrimSpace(user.ID) == "" {
		return nil, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if s.repo == nil {
		return []OrderSource{}, nil
	}
	return s.repo.ListAPIModelTestOrderSources(ctx, user.ID)
}

func (s *Service) Discover(ctx context.Context, user auth.User, source CredentialSource) (Discovery, *domain.AppError) {
	credential, appErr := s.resolveCredential(ctx, user, source)
	if appErr != nil {
		return Discovery{}, appErr
	}
	client, appErr := s.client(credential)
	if appErr != nil {
		return Discovery{}, appErr
	}
	models, result := client.DiscoverModels(ctx)
	if !result.Succeeded() {
		return Discovery{}, modelDiscoveryError(result)
	}
	return Discovery{
		BaseURL:      credential.BaseURL,
		Models:       models,
		DiscoveredAt: s.now().UTC(),
	}, nil
}

func (s *Service) Test(ctx context.Context, user auth.User, source CredentialSource, model string) (ModelTest, *domain.AppError) {
	model = strings.TrimSpace(model)
	if model == "" {
		return ModelTest{}, fieldError("model", "required", "必须选择要测试的模型。")
	}
	if utf8.RuneCountInString(model) > maxModelIDRunes || containsControl(model) {
		return ModelTest{}, fieldError("model", "invalid", "模型标识格式不正确。")
	}
	credential, appErr := s.resolveCredential(ctx, user, source)
	if appErr != nil {
		return ModelTest{}, appErr
	}
	client, appErr := s.client(credential)
	if appErr != nil {
		return ModelTest{}, appErr
	}
	responses := client.TestResponses(ctx, model)
	chatCompletions := client.TestChatCompletions(ctx, model)
	return ModelTest{
		Model:           model,
		Responses:       protocolResult(responses),
		ChatCompletions: protocolResult(chatCompletions),
		TestedAt:        s.now().UTC(),
	}, nil
}

func (s *Service) resolveCredential(ctx context.Context, user auth.User, source CredentialSource) (OrderCredential, *domain.AppError) {
	if s == nil || strings.TrimSpace(user.ID) == "" {
		return OrderCredential{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	source.Kind = strings.TrimSpace(source.Kind)
	source.BaseURL = strings.TrimSpace(source.BaseURL)
	source.APIKey = strings.TrimSpace(source.APIKey)
	source.OrderID = strings.TrimSpace(source.OrderID)
	switch source.Kind {
	case CredentialSourceManual:
		if source.OrderID != "" {
			return OrderCredential{}, fieldError("credentialSource.orderId", "not_allowed", "手动填写凭据时不能同时选择订单。")
		}
		if source.BaseURL == "" {
			return OrderCredential{}, fieldError("credentialSource.baseUrl", "required", "必须填写 API Base URL。")
		}
		if source.APIKey == "" {
			return OrderCredential{}, fieldError("credentialSource.apiKey", "required", "必须填写 Bearer API Key。")
		}
		if utf8.RuneCountInString(source.BaseURL) > maxBaseURLRunes || containsControl(source.BaseURL) {
			return OrderCredential{}, fieldError("credentialSource.baseUrl", "invalid", "API Base URL 格式不正确。")
		}
		if utf8.RuneCountInString(source.APIKey) > maxAPIKeyRunes || containsControl(source.APIKey) {
			return OrderCredential{}, fieldError("credentialSource.apiKey", "invalid", "API Key 格式不正确。")
		}
		return OrderCredential{
			BaseURL:                 source.BaseURL,
			APIKey:                  source.APIKey,
			AcknowledgeInsecureHTTP: source.AcknowledgeInsecureHTTP,
		}, nil
	case CredentialSourceOrder:
		if source.BaseURL != "" || source.APIKey != "" {
			return OrderCredential{}, fieldError("credentialSource.kind", "mixed_fields", "从订单导入时不能同时提交手动凭据。")
		}
		if _, err := uuid.Parse(source.OrderID); err != nil {
			return OrderCredential{}, fieldError("credentialSource.orderId", "invalid", "请选择有效的 API 订单。")
		}
		if s.repo == nil {
			return OrderCredential{}, orderNotFound()
		}
		credential, appErr := s.repo.GetAPIModelTestOrderCredential(ctx, user.ID, source.OrderID)
		if appErr != nil {
			return OrderCredential{}, appErr
		}
		credential.AcknowledgeInsecureHTTP = source.AcknowledgeInsecureHTTP
		return credential, nil
	default:
		return OrderCredential{}, fieldError("credentialSource.kind", "invalid", "请选择手动填写或订单导入。")
	}
}

func (s *Service) client(credential OrderCredential) (openAIClient, *domain.AppError) {
	if s == nil || s.newClient == nil {
		return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型测试服务暂时不可用。")
	}
	allowInsecureHTTP := credential.AcknowledgeInsecureHTTP
	if usesInsecureHTTP(credential.BaseURL) && !allowInsecureHTTP {
		return nil, fieldError("credentialSource.acknowledgeInsecureHttp", "required", "使用 HTTP 地址前必须确认 API Key 将通过未加密连接发送。")
	}
	target, err := openaiapi.NormalizeBaseURL(credential.BaseURL, allowInsecureHTTP)
	if err != nil {
		return nil, fieldError("credentialSource.baseUrl", string(openaiapi.ErrorBlockedTarget), "API Base URL 格式不正确或目标不可访问。")
	}
	client, err := s.newClient(target.Raw, credential.APIKey, openaiapi.Options{
		AllowInsecureHTTP: allowInsecureHTTP,
		Timeout:           s.timeout,
	})
	if err != nil {
		return nil, fieldError("credentialSource.baseUrl", string(openaiapi.ErrorBlockedTarget), "API Base URL 格式不正确或目标不可访问。")
	}
	return client, nil
}

func usesInsecureHTTP(baseURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(baseURL)), "http://")
}

func modelDiscoveryError(result openaiapi.Result) *domain.AppError {
	field := "credentialSource.apiKey"
	detail := "无法使用当前凭据获取模型列表。"
	status := http.StatusUnprocessableEntity
	switch result.ErrorCode {
	case openaiapi.ErrorBlockedTarget, openaiapi.ErrorDNS, openaiapi.ErrorConnect, openaiapi.ErrorTLS:
		field = "credentialSource.baseUrl"
		detail = "平台无法连接该 API Base URL。"
	case openaiapi.ErrorRateLimited:
		status = http.StatusTooManyRequests
		detail = "目标 API 当前触发额度或频率限制。"
	case openaiapi.ErrorTimeout:
		status = http.StatusGatewayTimeout
		detail = "目标 API 请求超时。"
	case openaiapi.ErrorUpstream:
		status = http.StatusBadGateway
		detail = "目标 API 上游暂时异常。"
	case openaiapi.ErrorInvalidResponse, openaiapi.ErrorResponseTooLarge:
		field = "credentialSource.baseUrl"
		detail = "目标 API 未返回有效的 OpenAI 兼容模型列表。"
	}
	return domain.NewFieldError(status, domain.CodeAPIModelTestRequestFailed, "API model discovery failed", detail, field, string(result.ErrorCode), detail)
}

func fieldError(field, code, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API model tester input invalid", message, field, code, message)
}

func orderNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API order not found", "API 订单不存在。")
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}
