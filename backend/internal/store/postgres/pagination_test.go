package postgres

import (
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

type pageTestItem struct {
	id        string
	updatedAt time.Time
}

func TestKeysetCursorRoundTrip(t *testing.T) {
	sortTime := time.Date(2026, 7, 6, 12, 30, 5, 123456789, time.FixedZone("CST", 8*60*60))
	cursor := encodeKeysetCursor(sortTime, "00000000-0000-0000-0000-000000000001")

	position, appErr := decodeKeysetCursor(cursor)
	if appErr != nil {
		t.Fatalf("decode cursor: %v", appErr)
	}
	if !position.Time.Equal(sortTime) {
		t.Fatalf("sort time mismatch: got %s want %s", position.Time, sortTime)
	}
	if position.ID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("id mismatch: %s", position.ID)
	}
}

func TestDecodeKeysetCursorRejectsInvalidInput(t *testing.T) {
	if _, appErr := decodeKeysetCursor("not-base64"); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected validation error for invalid cursor, got %v", appErr)
	}
	badID := encodeKeysetCursor(time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC), "not-a-uuid")
	if _, appErr := decodeKeysetCursor(badID); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected validation error for cursor with invalid id, got %v", appErr)
	}
}

func TestPageFromItemsReturnsNextCursorAndLastPage(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	items := []pageTestItem{
		{id: "00000000-0000-0000-0000-000000000003", updatedAt: base.Add(3 * time.Minute)},
		{id: "00000000-0000-0000-0000-000000000002", updatedAt: base.Add(2 * time.Minute)},
		{id: "00000000-0000-0000-0000-000000000001", updatedAt: base.Add(time.Minute)},
	}
	cursorFor := func(item pageTestItem) (time.Time, string) {
		return item.updatedAt, item.id
	}

	first := pageFromItems(items, domain.PageRequest{Limit: 2}, cursorFor)
	if len(first.Items) != 2 {
		t.Fatalf("first page length = %d, want 2", len(first.Items))
	}
	if first.NextCursor == nil {
		t.Fatalf("expected next cursor")
	}
	position, appErr := decodeKeysetCursor(*first.NextCursor)
	if appErr != nil {
		t.Fatalf("decode next cursor: %v", appErr)
	}
	if position.ID != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("next cursor id = %s, want second item", position.ID)
	}

	last := pageFromItems(items[:1], domain.PageRequest{Limit: 2}, cursorFor)
	if len(last.Items) != 1 {
		t.Fatalf("last page length = %d, want 1", len(last.Items))
	}
	if last.NextCursor != nil {
		t.Fatalf("last page next cursor = %q, want nil", *last.NextCursor)
	}
}

func TestScalarKeysetCursorBindsSortAndValue(t *testing.T) {
	cursor := encodeScalarKeysetCursor("price_asc", "19.90", "00000000-0000-0000-0000-000000000001")

	position, appErr := decodeScalarKeysetCursor(cursor, "price_asc")
	if appErr != nil {
		t.Fatalf("decode scalar cursor: %v", appErr)
	}
	if position.Value != "19.90" || position.ID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected scalar cursor: %+v", position)
	}
	if _, appErr := decodeScalarKeysetCursor(cursor, "seats_desc"); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected sort mismatch validation error, got %v", appErr)
	}
}

func TestValidateNonNegativeDecimalCursorRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"not-a-number", "-0.01", "1/2", "NaN", "Infinity"} {
		if appErr := validateNonNegativeDecimalCursor(scalarKeysetPosition{Value: value}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
			t.Fatalf("expected validation error for decimal cursor %q, got %v", value, appErr)
		}
	}
	for _, value := range []string{"0", "19.90", "1e2"} {
		if appErr := validateNonNegativeDecimalCursor(scalarKeysetPosition{Value: value}); appErr != nil {
			t.Fatalf("expected decimal cursor %q to be valid, got %v", value, appErr)
		}
	}
}
