package server

import (
	"encoding/json"
	"testing"

	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"
)

func TestHistoricalPromptAuditFieldsRemainExplicitJSONNull(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		field string
		value any
	}{
		{name: "owner service", field: "promptAuditEnabled", value: toAPIServiceResponse(apimarket.Service{})},
		{name: "public service", field: "promptAuditEnabled", value: toPublicAPIServiceResponse(apimarket.Service{})},
		{name: "public quota offer", field: "promptAuditEnabled", value: toPublicAPIQuotaOfferResponse(apiquota.OfferCard{})},
		{name: "purchase intent", field: "promptAuditEnabledSnapshot", value: toAPIPurchaseIntentCoreResponse(apiintent.Intent{})},
		{name: "order", field: "promptAuditEnabledSnapshot", value: toAPIOrderResponse(apiorder.Order{}, false, false)},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			value, exists := payload[test.field]
			if !exists || string(value) != "null" {
				t.Fatalf("expected explicit null %s, got exists=%v value=%s", test.field, exists, value)
			}
		})
	}
}
