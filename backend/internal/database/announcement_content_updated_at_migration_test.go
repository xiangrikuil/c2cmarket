package database

import (
	"strings"
	"testing"
)

func TestAnnouncementContentUpdatedAtMigrationUsesConservativeBackfill(t *testing.T) {
	upSQL := readMigrationForTest(t, "000086_announcement_content_updated_at.up.sql")
	for _, required := range []string{
		"ADD COLUMN content_updated_at timestamptz",
		"SET content_updated_at = publish_at",
		"ALTER COLUMN content_updated_at SET NOT NULL",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("announcement content timestamp migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000086_announcement_content_updated_at.down.sql")
	if !strings.Contains(downSQL, "DROP COLUMN content_updated_at") {
		t.Fatal("announcement content timestamp rollback must drop content_updated_at")
	}
}
