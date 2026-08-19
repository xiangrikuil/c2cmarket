package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnnouncementCriticalDeliveryMigrationContract(t *testing.T) {
	base := filepath.Join("..", "..", "..", "migrations", "000109_announcement_critical_delivery")
	up, err := os.ReadFile(base + ".up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile(base + ".down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	for _, required := range []string{
		"'critical'", "requires_ack", "'global_bar'", "'modal'",
		"acknowledged_at", "CREATE TABLE announcement_recipients",
		"announcement_version", "ix_announcement_recipients_user_version",
	} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, required := range []string{
		"DROP TABLE IF EXISTS announcement_recipients",
		"DROP COLUMN IF EXISTS acknowledged_at",
		"WHEN level = 'critical' THEN 'important'",
		"CHECK (level IN ('normal', 'important'))",
	} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
