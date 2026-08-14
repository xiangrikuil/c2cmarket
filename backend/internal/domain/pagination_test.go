package domain

import "testing"

func TestPageItemsRejectsInvalidCursor(t *testing.T) {
	for _, cursor := range []string{"not-a-cursor", "e30"} {
		if _, appErr := PageItems([]string{"first", "second"}, PageRequest{Limit: 1, Cursor: cursor}); appErr == nil || appErr.Code != CodeValidationFailed {
			t.Fatalf("expected invalid cursor validation error for %q, got %v", cursor, appErr)
		}
	}
}

func TestPageItemsAdvancesWithOpaqueCursor(t *testing.T) {
	first, appErr := PageItems([]string{"first", "second"}, PageRequest{Limit: 1})
	if appErr != nil || len(first.Items) != 1 || first.Items[0] != "first" || first.NextCursor == nil {
		t.Fatalf("unexpected first page: page=%+v error=%v", first, appErr)
	}

	second, appErr := PageItems([]string{"first", "second"}, PageRequest{Limit: 1, Cursor: *first.NextCursor})
	if appErr != nil || len(second.Items) != 1 || second.Items[0] != "second" || second.NextCursor != nil {
		t.Fatalf("unexpected second page: page=%+v error=%v", second, appErr)
	}
}
