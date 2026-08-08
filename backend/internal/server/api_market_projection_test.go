package server

import (
	"encoding/json"
	"testing"

	"c2c-market/backend/internal/module/apimarket"
)

func TestAPIServiceProjectionUsesCanonicalModelKeySnapshots(t *testing.T) {
	payload, err := json.Marshal(toAPIServiceResponse(apimarket.Service{
		Models: []apimarket.ServiceModel{{ModelKey: "gpt-4.1-mini"}},
		Packages: []apimarket.ServicePackage{{
			Models: []apimarket.ServicePackageModel{{ModelKey: "gpt-4.1-mini"}},
		}},
	}))
	if err != nil {
		t.Fatalf("marshal API service response: %v", err)
	}

	var response struct {
		Models   []map[string]any `json:"models"`
		Packages []struct {
			Models []map[string]any `json:"models"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode API service response: %v", err)
	}

	assertModelKeySnapshot := func(label string, model map[string]any) {
		t.Helper()
		if got := model["modelKeySnapshot"]; got != "gpt-4.1-mini" {
			t.Fatalf("%s modelKeySnapshot = %v, want gpt-4.1-mini", label, got)
		}
		if _, exists := model["modelKey"]; exists {
			t.Fatalf("%s must not expose the ambiguous modelKey field: %v", label, model)
		}
		if _, exists := model["modelNameSnapshot"]; exists {
			t.Fatalf("%s must not expose the removed decorative modelNameSnapshot field: %v", label, model)
		}
	}

	if len(response.Models) != 1 || len(response.Packages) != 1 || len(response.Packages[0].Models) != 1 {
		t.Fatalf("unexpected model projection shape: %+v", response)
	}
	assertModelKeySnapshot("service", response.Models[0])
	assertModelKeySnapshot("package", response.Packages[0].Models[0])
}
