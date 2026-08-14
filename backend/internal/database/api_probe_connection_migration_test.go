package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIProbeConnectionMigrationReplacesOnlyLegacyProbeData(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000081_api_probe_connections_and_model_keys.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"DROP TABLE api_service_probe_samples",
		"DROP TABLE api_service_probe_authorization_events",
		"DROP TABLE api_service_probe_configs",
		"CREATE TABLE api_probe_connections",
		"CREATE TABLE api_probe_connection_samples",
		"UNIQUE (connection_id, slot_started_at)",
		"FOREIGN KEY (probe_connection_id, owner_user_id)",
		"probe_connection_id_snapshot",
		"normalized_api_base_url_snapshot",
		"RENAME COLUMN model_name_snapshot TO model_key_snapshot",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"DROP TABLE api_services", "DROP TABLE api_orders", "DROP TABLE users", "DROP TABLE api_quota_offers"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("migration contains destructive unrelated operation %q", forbidden)
		}
	}
}

func TestAPIProbeConnectionDownMigrationRejectsDataLoss(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000081_api_probe_connections_and_model_keys.down.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"cannot roll back migration 81 while probe connection data, bindings, or order target snapshots exist",
		"CREATE TABLE api_service_probe_configs",
		"CREATE TABLE api_service_probe_samples",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
