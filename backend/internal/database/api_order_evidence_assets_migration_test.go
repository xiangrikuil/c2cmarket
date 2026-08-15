package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderEvidenceAssetsMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000105_api_order_evidence_assets.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000105_api_order_evidence_assets.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	upSQL := string(up)
	for _, required := range []string{
		"CREATE TABLE api_order_evidence_assets",
		"CREATE TABLE api_order_evidence_bindings",
		"object_key text",
		"byte_size bigint",
		"sha256 bytea",
		"asset_id uuid PRIMARY KEY",
		"visibility text NOT NULL",
		"source_type text NOT NULL",
		"platform_escalation",
		"API-order evidence bindings are append-only",
		"unbound_expires_at timestamptz",
		"quarantined_expires_at timestamptz",
		"destroy_requested_at timestamptz",
		"destroyed_at timestamptz",
		"ix_api_order_evidence_assets_destroy_retry",
		"validate_api_order_evidence_binding_source",
		"dispute.api_order_id = asset.api_order_id",
		"request_row.dispute_case_id = NEW.dispute_case_id",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"base64", "data:image", "bytea NOT NULL DEFAULT"} {
		if strings.Contains(strings.ToLower(upSQL), strings.ToLower(forbidden)) {
			t.Fatalf("migration contains forbidden persistence %q", forbidden)
		}
	}
	if !strings.Contains(string(down), "cannot roll back API-order evidence assets while immutable bindings exist") {
		t.Fatal("down migration must reject destructive rollback")
	}
	if !strings.Contains(string(down), "DROP FUNCTION IF EXISTS validate_api_order_evidence_binding_source()") {
		t.Fatal("down migration must remove the evidence source validation function")
	}
}

func TestAPIOrderPlatformEscalationContextMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000106_api_order_platform_escalation_context.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000106_api_order_platform_escalation_context.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	upSQL := string(up)
	for _, required := range []string{
		"negotiation_channels text[]",
		"negotiation_ended_confirmed boolean",
		"negotiation_summary text",
		"requested_platform_action text",
		"escalated_by_user_id uuid",
		"escalated_at timestamptz",
		"superseded_reason text",
		"wechat",
		"linux_do",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, required := range []string{"DROP COLUMN IF EXISTS negotiation_channels", "DROP COLUMN IF EXISTS superseded_reason"} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
