package apiquota

import (
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestSystemSaleSlotsGenerateFixedBeijingSchedule(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 0, 30, 0, 0, time.UTC)
	slots := SystemSaleSlots(now)
	if len(slots) != 7 {
		t.Fatalf("expected 7 slots, got %d", len(slots))
	}

	first := slots[0]
	if first.Key != "2026-07-24@20:00" {
		t.Fatalf("unexpected first slot key %q", first.Key)
	}
	if got := first.StartsAt.Format(time.RFC3339); got != "2026-07-24T12:00:00Z" {
		t.Fatalf("unexpected first slot start %q", got)
	}
	if first.EndsAt.Sub(first.StartsAt) != 30*time.Minute {
		t.Fatalf("unexpected slot duration %s", first.EndsAt.Sub(first.StartsAt))
	}
	if first.StartsAt.Sub(first.RegistrationClosesAt) != time.Hour {
		t.Fatalf("unexpected registration window %s", first.StartsAt.Sub(first.RegistrationClosesAt))
	}
	if slots[6].Key != "2026-07-30@20:00" {
		t.Fatalf("unexpected last slot key %q", slots[6].Key)
	}
}

func TestSystemSaleSlotStateBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "registration open", now: beijingTime(t, "2026-07-24T18:59:59+08:00"), want: SystemSlotStateRegistrationOpen},
		{name: "registration closed", now: beijingTime(t, "2026-07-24T19:00:00+08:00"), want: SystemSlotStateRegistrationClosed},
		{name: "active", now: beijingTime(t, "2026-07-24T20:00:00+08:00"), want: SystemSlotStateActive},
		{name: "ended", now: beijingTime(t, "2026-07-24T20:30:00+08:00"), want: SystemSlotStateEnded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slot := SystemSaleSlots(test.now)[0]
			if slot.Key != "2026-07-24@20:00" || slot.State != test.want {
				t.Fatalf("slot = %#v, want state %q", slot, test.want)
			}
		})
	}
}

func TestResolveOpenSystemSaleSlotRejectsClosedOrArbitrarySlots(t *testing.T) {
	t.Parallel()

	openNow := beijingTime(t, "2026-07-24T18:00:00+08:00")
	slot, appErr := ResolveOpenSystemSaleSlot("2026-07-24@20:00", openNow)
	if appErr != nil || slot.Key != "2026-07-24@20:00" {
		t.Fatalf("expected open slot, got slot=%#v error=%v", slot, appErr)
	}

	_, appErr = ResolveOpenSystemSaleSlot("2026-07-24@20:00", beijingTime(t, "2026-07-24T19:00:00+08:00"))
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected registration conflict, got %v", appErr)
	}

	for _, invalid := range []string{"2026-07-24@09:00", "2026-07-31@20:00", "2026-07-24T20:00:00+08:00"} {
		_, appErr = ResolveOpenSystemSaleSlot(invalid, openNow)
		if appErr == nil || appErr.Code != domain.CodeValidationFailed {
			t.Fatalf("expected validation failure for %q, got %v", invalid, appErr)
		}
	}
}

func TestIsSystemSaleSlotKey(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{"2026-07-24@20:00"} {
		if !IsSystemSaleSlotKey(valid) {
			t.Fatalf("expected %q to be valid", valid)
		}
	}
	for _, invalid := range []string{"2026-07-24@09:00", "2026-07-24@13:00", "2026-07-24@10:00", "2026-07-24@20:01", "invalid"} {
		if IsSystemSaleSlotKey(invalid) {
			t.Fatalf("expected %q to be invalid", invalid)
		}
	}
}

func beijingTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
