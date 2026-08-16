package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestPublishAnnouncementLocksValidatesAndAuditsInOneTransaction(t *testing.T) {
	source := readAnnouncementStoreSource(t)
	section := announcementStoreFunction(t, source, "func (s *Store) PublishAnnouncement", "func (s *Store) OfflineAnnouncement")

	begin := strings.Index(section, "s.pool.Begin(ctx)")
	lock := strings.Index(section, "FOR UPDATE OF a")
	resolve := strings.Index(section, "announcement.ResolvePublishTransition(current, now)")
	update := strings.Index(section, "UPDATE announcements")
	audit := strings.Index(section, "insertAnnouncementAudit(ctx, tx")
	commit := strings.Index(section, "tx.Commit(ctx)")
	positions := []int{begin, lock, resolve, update, audit, commit}
	for index, position := range positions {
		if position < 0 {
			t.Fatalf("publish transaction contract step %d is missing", index)
		}
		if index > 0 && positions[index-1] > position {
			t.Fatalf("publish transaction contract is out of order: %v", positions)
		}
	}
	if !strings.Contains(section, "SET status = $2, publish_at = $3") {
		t.Fatal("publish transaction must persist the resolved status and publication time")
	}
	if strings.Contains(section, "content_updated_at") {
		t.Fatal("publishing must preserve content_updated_at")
	}
}

func TestAnnouncementRepositoryUpdatesContentTimestampOnlyForVisibleContent(t *testing.T) {
	source := readAnnouncementStoreSource(t)
	updateSection := announcementStoreFunction(t, source, "func (s *Store) UpdateAnnouncement", "func (s *Store) PublishAnnouncement")
	for _, required := range []string{
		"contentUpdatedAt := before.ContentUpdatedAt",
		"announcement.UserVisibleContentChanged(before, input.Form)",
		"contentUpdatedAt = now",
		"content_updated_at = $18",
		"version = version + CASE WHEN $19 THEN 1 ELSE 0 END",
	} {
		if !strings.Contains(updateSection, required) {
			t.Fatalf("announcement content update contract is missing %q", required)
		}
	}

	offlineSection := announcementStoreFunction(t, source, "func (s *Store) OfflineAnnouncement", "func (s *Store) DuplicateAnnouncement")
	if strings.Contains(offlineSection, "content_updated_at") {
		t.Fatal("offlining must preserve content_updated_at")
	}

	insertSection := announcementStoreFunction(t, source, "func (s *Store) insertAnnouncement", "func (s *Store) queryAnnouncements")
	if !strings.Contains(insertSection, "created_at, updated_at, content_updated_at") ||
		!strings.Contains(insertSection, "$19, $19, $19") {
		t.Fatal("create and duplicate inserts must initialize content_updated_at from the action time")
	}
}

func TestAnnouncementRecipientAndReceiptContractsRemainTransactional(t *testing.T) {
	source := readAnnouncementStoreSource(t)
	publishSection := announcementStoreFunction(t, source, "func (s *Store) PublishAnnouncement", "func (s *Store) OfflineAnnouncement")
	for _, required := range []string{
		"FOR UPDATE OF a",
		"version = version + 1",
		"rebuildAnnouncementRecipients(ctx, tx, item, now)",
		"tx.Commit(ctx)",
	} {
		if !strings.Contains(publishSection, required) {
			t.Fatalf("publish recipient snapshot contract is missing %q", required)
		}
	}

	updateSection := announcementStoreFunction(t, source, "func (s *Store) UpdateAnnouncement", "func (s *Store) PublishAnnouncement")
	for _, required := range []string{
		"beforeStatus == announcement.StatusPublished || beforeStatus == announcement.StatusScheduled",
		"if deliveryChanged",
		"rebuildAnnouncementRecipients(ctx, tx, item, now)",
	} {
		if !strings.Contains(updateSection, required) {
			t.Fatalf("scheduled/published recipient rebuild contract is missing %q", required)
		}
	}

	receiptSection := announcementStoreFunction(t, source, "func (s *Store) UpsertReceipt", "func (s *Store) AdminAnnouncements")
	for _, required := range []string{
		"FOR UPDATE OF a",
		"nullableFirstSeenTime(input.Action, now)",
		"announcement_version = EXCLUDED.announcement_version",
		"acknowledged_at",
	} {
		if !strings.Contains(receiptSection, required) {
			t.Fatalf("versioned receipt contract is missing %q", required)
		}
	}

	recipientSection := announcementStoreFunction(t, source, "func (s *Store) rebuildAnnouncementRecipients", "func announcementAudienceFieldError")
	for _, required := range []string{
		"DELETE FROM announcement_recipients",
		"student_email_claims",
		"linux_do_bindings",
		"user_permissions",
		"announcement_version",
	} {
		if !strings.Contains(recipientSection, required) {
			t.Fatalf("recipient resolver contract is missing %q", required)
		}
	}
}

func readAnnouncementStoreSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("announcement.go")
	if err != nil {
		t.Fatalf("read announcement store: %v", err)
	}
	return string(data)
}

func announcementStoreFunction(t *testing.T, source, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("announcement store function %q is missing", startMarker)
	}
	end := strings.Index(source[start:], endMarker)
	if end < 0 {
		t.Fatalf("announcement store function boundary %q is missing", endMarker)
	}
	return source[start : start+end]
}
