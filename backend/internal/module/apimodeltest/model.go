package apimodeltest

import (
	"time"

	"c2c-market/backend/internal/platform/openaiapi"
)

const (
	CredentialSourceManual = "manual"
	CredentialSourceOrder  = "order"
)

type CredentialSource struct {
	Kind                    string
	BaseURL                 string
	APIKey                  string
	OrderID                 string
	AcknowledgeInsecureHTTP bool
}

type OrderSource struct {
	OrderID      string
	OrderNo      string
	ServiceTitle string
	BaseURL      string
	DeliveredAt  time.Time
}

type OrderCredential struct {
	BaseURL                 string
	APIKey                  string
	AcknowledgeInsecureHTTP bool
}

type Discovery struct {
	BaseURL      string
	Models       []string
	DiscoveredAt time.Time
}

type ProtocolResult struct {
	Succeeded       bool
	HTTPStatusClass int
	DurationMS      int
	ErrorCode       openaiapi.ErrorCode
}

type ModelTest struct {
	Model           string
	Responses       ProtocolResult
	ChatCompletions ProtocolResult
	TestedAt        time.Time
}

func protocolResult(result openaiapi.Result) ProtocolResult {
	return ProtocolResult{
		Succeeded:       result.Succeeded(),
		HTTPStatusClass: result.HTTPStatusClass,
		DurationMS:      result.DurationMS,
		ErrorCode:       result.ErrorCode,
	}
}
