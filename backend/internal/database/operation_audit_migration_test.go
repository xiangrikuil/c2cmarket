package database

import (
	"os"
	"strings"
	"testing"
)

func TestOperationAuditMigrationEvolvesProbeLedgerWithoutSecondAuthority(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000092_operation_audit_projection.up.sql")
	if err != nil {
		t.Fatalf("read operation audit migration: %v", err)
	}
	source := string(up)
	for _, fragment := range []string{
		"RENAME TO api_probe_connection_events",
		"ALTER COLUMN actor_user_id DROP NOT NULL",
		"UPDATE api_probe_connection_events",
		"action = 'model_changed'",
		"legacy-probe-",
		"trg_api_probe_connection_events_append_only",
		"api probe connection events are append-only",
		"ux_api_probe_connection_events_request",
		"CREATE VIEW api_probe_connection_model_changes",
		"ix_admin_audit_logs_operation_actor_cursor",
		"ix_admin_audit_logs_operation_target_cursor",
		"ix_domain_events_operation_actor_cursor",
		"ix_domain_events_operation_target_cursor",
		"ix_api_order_events_operation_actor_cursor",
		"ix_api_order_events_operation_target_cursor",
		"ix_contact_access_logs_operation_actor_cursor",
		"ix_api_order_events_operation_cursor",
		"ix_api_intent_contact_access_operation_cursor",
		"ix_api_intent_contact_access_operation_target_cursor",
		"ix_api_order_access_operation_actor_cursor",
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if strings.Contains(source, "CREATE TABLE operation_audit") || strings.Contains(source, "CREATE TABLE global_audit") {
		t.Fatal("migration must not create a duplicate universal audit authority")
	}
}

func TestOperationAuditDownMigrationRejectsLossyProbeRollback(t *testing.T) {
	down, err := os.ReadFile("../../migrations/000092_operation_audit_projection.down.sql")
	if err != nil {
		t.Fatalf("read operation audit down migration: %v", err)
	}
	source := string(down)
	for _, fragment := range []string{
		"WHERE action <> 'model_changed'",
		"preserved deleted targets",
		"RENAME TO api_probe_connection_model_changes",
		"ON DELETE SET NULL",
		"ALTER COLUMN actor_user_id SET NOT NULL",
		"api_probe_connection_model_changes_changed_by_user_id_fkey",
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("down migration missing %q", fragment)
		}
	}
}
