package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apiorder"
)

func TestDestroyedAPIOrderCredentialResponseContainsOnlyAuditFacts(t *testing.T) {
	t.Parallel()

	destroyedAt := time.Date(2026, time.August, 3, 9, 0, 0, 0, time.UTC)
	response := toAPIOrderDeliveryCredentialResponse(apiorder.DeliveryCredential{
		DeliveryKind:  apiorder.DeliveryKindAPIKeyEndpoint,
		APIBaseURL:    "https://poisoned.example.com/v1",
		APIKey:        "sk-poisoned-secret",
		PanelLoginURL: "https://poisoned.example.com/login",
		Username:      "poisoned-user",
		Password:      "poisoned-password",
		Instructions:  "poisoned instructions",
		SubmittedAt:   destroyedAt.Add(-30 * 24 * time.Hour),
		DestroyedAt:   &destroyedAt,
		DestroyReason: "retention_expired",
	})
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal destroyed credential: %v", err)
	}
	encoded := string(body)
	for _, required := range []string{`"deliveryKind"`, `"submittedAt"`, `"destroyedAt"`, `"destroyReason":"retention_expired"`} {
		if !strings.Contains(encoded, required) {
			t.Fatalf("destroyed credential response missing %s: %s", required, encoded)
		}
	}
	for _, forbidden := range []string{`"apiBaseUrl"`, `"apiKey"`, `"panelLoginUrl"`, `"username"`, `"password"`, `"instructions"`} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("destroyed credential response leaked %s: %s", forbidden, encoded)
		}
	}
}
