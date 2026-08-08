package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIProbeRealModelHealthMigrationDefinesImmutableMeasurementContracts(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000082_api_probe_real_model_health.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"DELETE FROM api_probe_connection_samples",
		"measurement_version = measurement_version + 1",
		"CREATE TABLE api_probe_connection_attempts",
		"CREATE TABLE api_probe_connection_model_changes",
		"CREATE TABLE api_probe_latency_rules",
		"UNIQUE (model, protocol, environment, version)",
		"ux_api_probe_latency_rules_active",
		"(probe_model, probe_protocol, probe_environment, slot_started_at)",
		"ON DELETE CASCADE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestAPIProbeRealModelHealthDownMigrationRejectsObservationLoss(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000082_api_probe_real_model_health.down.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"EXISTS (SELECT 1 FROM api_probe_connection_attempts)",
		"EXISTS (SELECT 1 FROM api_probe_connection_model_changes)",
		"EXISTS (SELECT 1 FROM api_probe_latency_rules)",
		"cannot roll back migration 82 while real probe attempts, model history, or latency rules exist",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
