package server

import (
	"encoding/json"
	"testing"

	"c2c-market/backend/internal/module/apimarket"
)

func TestAPIServiceResponsesPreserveHistoricalNullAccountPool(t *testing.T) {
	responses := []any{
		toPublicAPIServiceResponse(apimarket.Service{}),
		toAPIServiceResponse(apimarket.Service{}),
	}
	for _, response := range responses {
		body, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("marshal API service response: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode API service response: %v", err)
		}
		for _, key := range []string{"accountPoolType", "accountPoolLabel"} {
			value, exists := payload[key]
			if !exists || value != nil {
				t.Fatalf("expected explicit null %s, got exists=%v value=%v", key, exists, value)
			}
		}
	}
}
