package operationaudit

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestOperationAuditCursorRoundTripAndVersionValidation(t *testing.T) {
	want := CursorPosition{
		OccurredAt: time.Date(2026, 8, 12, 12, 34, 56, 123456789, time.UTC),
		SourceKind: SourceAPIOrder,
		EventID:    "10000000-0000-4000-8000-000000000001",
	}
	got, appErr := DecodeCursor(EncodeCursor(want))
	if appErr != nil || got == nil || !got.OccurredAt.Equal(want.OccurredAt) || got.SourceKind != want.SourceKind || got.EventID != want.EventID {
		t.Fatalf("cursor roundtrip failed: got=%+v err=%v", got, appErr)
	}
	badVersion := base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"t":"2026-08-12T12:34:56Z","s":"admin","id":"10000000-0000-4000-8000-000000000001"}`))
	if _, appErr := DecodeCursor(badVersion); appErr == nil {
		t.Fatal("unknown cursor version must fail")
	}
	badSource := base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"t":"2026-08-12T12:34:56Z","s":"request","id":"10000000-0000-4000-8000-000000000001"}`))
	if _, appErr := DecodeCursor(badSource); appErr == nil {
		t.Fatal("unknown cursor source must fail")
	}
}
