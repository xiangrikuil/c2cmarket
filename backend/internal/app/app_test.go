package app

import (
	"context"
	"testing"

	"c2c-market/backend/internal/config"
)

func TestNewRejectsInvalidModelAuditAllowlist(t *testing.T) {
	_, err := New(context.Background(), config.Config{
		ModelAuditAllowedHosts: []string{"*.example.com"},
	})
	if err == nil {
		t.Fatal("New() accepted a wildcard model audit allowlist entry")
	}
}
