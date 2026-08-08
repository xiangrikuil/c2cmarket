package apimodeltest

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/openaiapi"
)

func TestDiscoverUsesManualCredentialOnlyForCurrentCall(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	client := &fakeOpenAIClient{models: []string{"gpt-4.1-mini", "custom/model"}}
	service := NewService(nil, time.Second, func() time.Time { return now })
	var capturedBaseURL, capturedAPIKey string
	service.newClient = func(baseURL, apiKey string, _ openaiapi.Options) (openAIClient, error) {
		capturedBaseURL = baseURL
		capturedAPIKey = apiKey
		return client, nil
	}

	result, appErr := service.Discover(context.Background(), auth.User{ID: "user-1"}, CredentialSource{
		Kind: CredentialSourceManual, BaseURL: " https://API.example.com/v1/ ", APIKey: " sk-test ",
	})
	if appErr != nil {
		t.Fatalf("Discover() error: %v", appErr)
	}
	if capturedBaseURL != "https://API.example.com/v1/" || capturedAPIKey != "sk-test" {
		t.Fatalf("captured baseURL=%q apiKey=%q", capturedBaseURL, capturedAPIKey)
	}
	if result.BaseURL != capturedBaseURL || !reflect.DeepEqual(result.Models, client.models) || !result.DiscoveredAt.Equal(now) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOrderCredentialSourceIsResolvedForCurrentBuyer(t *testing.T) {
	orderID := "00000000-0000-0000-0000-000000000801"
	repository := &fakeRepository{credential: OrderCredential{BaseURL: "https://order.example.com/v1", APIKey: "order-key"}}
	service := NewService(repository, time.Second, time.Now)
	var capturedAPIKey string
	service.newClient = func(_ string, apiKey string, _ openaiapi.Options) (openAIClient, error) {
		capturedAPIKey = apiKey
		return &fakeOpenAIClient{}, nil
	}

	_, appErr := service.Discover(context.Background(), auth.User{ID: "buyer-1"}, CredentialSource{Kind: CredentialSourceOrder, OrderID: orderID})
	if appErr != nil {
		t.Fatalf("Discover() error: %v", appErr)
	}
	if repository.buyerUserID != "buyer-1" || repository.orderID != orderID || capturedAPIKey != "order-key" {
		t.Fatalf("repository buyer=%q order=%q key=%q", repository.buyerUserID, repository.orderID, capturedAPIKey)
	}
}

func TestHTTPSourceRequiresExplicitAcknowledgementForManualAndOrderCredentials(t *testing.T) {
	orderID := "00000000-0000-0000-0000-000000000801"
	repository := &fakeRepository{credential: OrderCredential{BaseURL: "http://api.example.com/v1", APIKey: "order-key"}}
	service := NewService(repository, time.Second, time.Now)
	factoryCalls := 0
	var capturedOptions openaiapi.Options
	service.newClient = func(_ string, _ string, options openaiapi.Options) (openAIClient, error) {
		factoryCalls++
		capturedOptions = options
		return &fakeOpenAIClient{}, nil
	}

	sources := []CredentialSource{
		{Kind: CredentialSourceManual, BaseURL: "http://api.example.com/v1", APIKey: "manual-key"},
		{Kind: CredentialSourceOrder, OrderID: orderID},
	}
	for _, source := range sources {
		_, appErr := service.Discover(context.Background(), auth.User{ID: "buyer-1"}, source)
		if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "credentialSource.acknowledgeInsecureHttp" {
			t.Fatalf("source=%+v error=%+v", source, appErr)
		}
	}
	if factoryCalls != 0 {
		t.Fatalf("client factory called %d times without acknowledgement", factoryCalls)
	}

	acknowledged := CredentialSource{
		Kind: CredentialSourceManual, BaseURL: "http://api.example.com/v1", APIKey: "manual-key", AcknowledgeInsecureHTTP: true,
	}
	if _, appErr := service.Discover(context.Background(), auth.User{ID: "buyer-1"}, acknowledged); appErr != nil {
		t.Fatalf("acknowledged Discover() error: %v", appErr)
	}
	if factoryCalls != 1 || !capturedOptions.AllowInsecureHTTP {
		t.Fatalf("factory calls=%d options=%+v", factoryCalls, capturedOptions)
	}
}

func TestModelTestReturnsIndependentProtocolResults(t *testing.T) {
	client := &fakeOpenAIClient{
		responses: openaiapi.Result{HTTPStatusClass: 2, DurationMS: 12},
		chat:      openaiapi.Result{HTTPStatusClass: 4, DurationMS: 7, ErrorCode: openaiapi.ErrorRateLimited},
	}
	service := NewService(nil, time.Second, time.Now)
	service.newClient = func(string, string, openaiapi.Options) (openAIClient, error) { return client, nil }

	result, appErr := service.Test(context.Background(), auth.User{ID: "user-1"}, CredentialSource{
		Kind: CredentialSourceManual, BaseURL: "https://api.example.com/v1", APIKey: "key",
	}, " gpt-4.1-mini ")
	if appErr != nil {
		t.Fatalf("Test() error: %v", appErr)
	}
	if result.Model != "gpt-4.1-mini" || !result.Responses.Succeeded || result.ChatCompletions.Succeeded || result.ChatCompletions.ErrorCode != openaiapi.ErrorRateLimited {
		t.Fatalf("unexpected result: %+v", result)
	}
	if client.responsesModel != result.Model || client.chatModel != result.Model {
		t.Fatalf("protocol models responses=%q chat=%q", client.responsesModel, client.chatModel)
	}
}

func TestDiscoverClassifiesAuthenticationWithoutReturningThirdPartyBody(t *testing.T) {
	service := NewService(nil, time.Second, time.Now)
	service.newClient = func(string, string, openaiapi.Options) (openAIClient, error) {
		return &fakeOpenAIClient{discoverResult: openaiapi.Result{HTTPStatusClass: 4, ErrorCode: openaiapi.ErrorAuthentication}}, nil
	}
	_, appErr := service.Discover(context.Background(), auth.User{ID: "user-1"}, CredentialSource{
		Kind: CredentialSourceManual, BaseURL: "https://api.example.com/v1", APIKey: "bad-key",
	})
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || appErr.Code != domain.CodeAPIModelTestRequestFailed {
		t.Fatalf("unexpected error: %+v", appErr)
	}
	if len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "credentialSource.apiKey" || appErr.FieldErrors[0].Code != string(openaiapi.ErrorAuthentication) {
		t.Fatalf("unexpected field error: %+v", appErr.FieldErrors)
	}
}

func TestCredentialSourceRejectsMixedAndInvalidInputs(t *testing.T) {
	service := NewService(nil, time.Second, time.Now)
	tests := []CredentialSource{
		{Kind: CredentialSourceManual, BaseURL: "https://api.example.com/v1", APIKey: "key", OrderID: "00000000-0000-0000-0000-000000000801"},
		{Kind: CredentialSourceOrder, OrderID: "not-a-uuid"},
		{Kind: "unknown"},
	}
	for _, source := range tests {
		if _, appErr := service.Discover(context.Background(), auth.User{ID: "user-1"}, source); appErr == nil || appErr.Code != domain.CodeValidationFailed {
			t.Fatalf("source=%+v error=%+v", source, appErr)
		}
	}
}

type fakeOpenAIClient struct {
	models         []string
	discoverResult openaiapi.Result
	responses      openaiapi.Result
	chat           openaiapi.Result
	responsesModel string
	chatModel      string
}

func (client *fakeOpenAIClient) DiscoverModels(context.Context) ([]string, openaiapi.Result) {
	return client.models, client.discoverResult
}

func (client *fakeOpenAIClient) TestResponses(_ context.Context, model string) openaiapi.Result {
	client.responsesModel = model
	return client.responses
}

func (client *fakeOpenAIClient) TestChatCompletions(_ context.Context, model string) openaiapi.Result {
	client.chatModel = model
	return client.chat
}

type fakeRepository struct {
	sources     []OrderSource
	credential  OrderCredential
	appErr      *domain.AppError
	buyerUserID string
	orderID     string
}

func (repository *fakeRepository) ListAPIModelTestOrderSources(_ context.Context, buyerUserID string) ([]OrderSource, *domain.AppError) {
	repository.buyerUserID = buyerUserID
	return repository.sources, repository.appErr
}

func (repository *fakeRepository) GetAPIModelTestOrderCredential(_ context.Context, buyerUserID, orderID string) (OrderCredential, *domain.AppError) {
	repository.buyerUserID = buyerUserID
	repository.orderID = orderID
	return repository.credential, repository.appErr
}
