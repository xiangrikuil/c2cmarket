package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/apimodeltest"
)

type apiModelTesterCredentialSourceRequest struct {
	Kind                    string `json:"kind"`
	BaseURL                 string `json:"baseUrl,omitempty"`
	APIKey                  string `json:"apiKey,omitempty"`
	OrderID                 string `json:"orderId,omitempty"`
	AcknowledgeInsecureHTTP bool   `json:"acknowledgeInsecureHttp"`
}

type apiModelTesterDiscoverRequest struct {
	CredentialSource apiModelTesterCredentialSourceRequest `json:"credentialSource"`
}

type apiModelTesterTestRequest struct {
	CredentialSource apiModelTesterCredentialSourceRequest `json:"credentialSource"`
	Model            string                                `json:"model"`
}

type apiModelTesterOrderSourcesResponse struct {
	Items []apiModelTesterOrderSourceResponse `json:"items"`
}

type apiModelTesterOrderSourceResponse struct {
	OrderID      string `json:"orderId"`
	OrderNo      string `json:"orderNo"`
	ServiceTitle string `json:"serviceTitle"`
	BaseURL      string `json:"baseUrl"`
	DeliveredAt  string `json:"deliveredAt"`
}

type apiModelTesterDiscoveryResponse struct {
	BaseURL      string   `json:"baseUrl"`
	Models       []string `json:"models"`
	DiscoveredAt string   `json:"discoveredAt"`
}

type apiModelTesterProtocolResultResponse struct {
	Succeeded       bool   `json:"succeeded"`
	HTTPStatusClass int    `json:"httpStatusClass"`
	DurationMS      int    `json:"durationMs"`
	ErrorCode       string `json:"errorCode"`
}

type apiModelTesterTestResponse struct {
	Model           string                               `json:"model"`
	Responses       apiModelTesterProtocolResultResponse `json:"responsesResult"`
	ChatCompletions apiModelTesterProtocolResultResponse `json:"chatCompletionsResult"`
	TestedAt        string                               `json:"testedAt"`
}

func (s *Server) handleAPIModelTesterOrderSources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.apiModelTester.OrderSources(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	response := apiModelTesterOrderSourcesResponse{Items: make([]apiModelTesterOrderSourceResponse, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, apiModelTesterOrderSourceResponse{
			OrderID:      item.OrderID,
			OrderNo:      item.OrderNo,
			ServiceTitle: item.ServiceTitle,
			BaseURL:      item.BaseURL,
			DeliveredAt:  item.DeliveredAt.UTC().Format(time.RFC3339Nano),
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAPIModelTesterDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	request, appErr := decodeStrictJSONOnly[apiModelTesterDiscoverRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.apiModelTester.Discover(r.Context(), user, toAPIModelTesterCredentialSource(request.CredentialSource))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, apiModelTesterDiscoveryResponse{
		BaseURL:      result.BaseURL,
		Models:       result.Models,
		DiscoveredAt: result.DiscoveredAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) handleAPIModelTesterTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	request, appErr := decodeStrictJSONOnly[apiModelTesterTestRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.apiModelTester.Test(r.Context(), user, toAPIModelTesterCredentialSource(request.CredentialSource), request.Model)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, apiModelTesterTestResponse{
		Model:           result.Model,
		Responses:       toAPIModelTesterProtocolResult(result.Responses),
		ChatCompletions: toAPIModelTesterProtocolResult(result.ChatCompletions),
		TestedAt:        result.TestedAt.UTC().Format(time.RFC3339Nano),
	})
}

func toAPIModelTesterCredentialSource(request apiModelTesterCredentialSourceRequest) apimodeltest.CredentialSource {
	return apimodeltest.CredentialSource{
		Kind:                    request.Kind,
		BaseURL:                 request.BaseURL,
		APIKey:                  request.APIKey,
		OrderID:                 request.OrderID,
		AcknowledgeInsecureHTTP: request.AcknowledgeInsecureHTTP,
	}
}

func toAPIModelTesterProtocolResult(result apimodeltest.ProtocolResult) apiModelTesterProtocolResultResponse {
	return apiModelTesterProtocolResultResponse{
		Succeeded:       result.Succeeded,
		HTTPStatusClass: result.HTTPStatusClass,
		DurationMS:      result.DurationMS,
		ErrorCode:       string(result.ErrorCode),
	}
}
