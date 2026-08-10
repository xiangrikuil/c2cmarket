package apiorder

import (
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestAdminOrderCreatedAfterUsesShanghaiCalendarDay(t *testing.T) {
	now := time.Date(2026, 8, 9, 16, 30, 0, 0, time.UTC)

	createdAfter, ok := (AdminOrderFilter{DateRange: AdminOrderDateRangeToday}).CreatedAfter(now)
	if !ok {
		t.Fatal("today filter must have a lower bound")
	}
	want := time.Date(2026, 8, 10, 0, 0, 0, 0, shanghaiTime)
	if !createdAfter.Equal(want) || createdAfter.Location() != shanghaiTime {
		t.Fatalf("today lower bound = %v, want %v in %v", createdAfter, want, shanghaiTime)
	}

	orders := []Order{
		{ID: "same-day", CreatedAt: want.Add(time.Minute), UpdatedAt: now},
		{ID: "previous-day", CreatedAt: want.Add(-time.Minute), UpdatedAt: now},
	}
	filtered := FilterAdminOrders(orders, AdminOrderFilter{DateRange: AdminOrderDateRangeToday}, now)
	if len(filtered) != 1 || filtered[0].ID != "same-day" {
		t.Fatalf("today filter returned %+v", filtered)
	}
}

func TestPageAdminOrdersUsesSortBoundStableKeysetCursor(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	orders := []Order{
		{ID: "00000000-0000-0000-0000-000000000003", OrderNo: "API-20260810-C", Amount: "20.00", CreatedAt: now, UpdatedAt: now},
		{ID: "00000000-0000-0000-0000-000000000002", OrderNo: "API-20260810-B", Amount: "10.00", CreatedAt: now, UpdatedAt: now},
		{ID: "00000000-0000-0000-0000-000000000001", OrderNo: "API-20260810-A", Amount: "10.00", CreatedAt: now, UpdatedAt: now},
	}
	filter := AdminOrderFilter{Sort: AdminOrderSortAmountAsc}

	first, appErr := PageAdminOrders(orders, filter, domain.PageRequest{Limit: 2}, now)
	if appErr != nil {
		t.Fatalf("first page: %v", appErr)
	}
	if len(first.Items) != 2 || first.Items[0].OrderNo != "API-20260810-A" || first.Items[1].OrderNo != "API-20260810-B" || first.NextCursor == nil {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second, appErr := PageAdminOrders(orders, filter, domain.PageRequest{Limit: 2, Cursor: *first.NextCursor}, now)
	if appErr != nil {
		t.Fatalf("second page: %v", appErr)
	}
	if len(second.Items) != 1 || second.Items[0].OrderNo != "API-20260810-C" || second.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v", second)
	}
	if _, appErr := PageAdminOrders(orders, AdminOrderFilter{Sort: AdminOrderSortAmountDesc}, domain.PageRequest{Limit: 2, Cursor: *first.NextCursor}, now); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("sort-mismatched cursor error = %+v", appErr)
	}
	invalidIDCursor := encodeAdminOrderCursor(AdminOrderSortAmountAsc, "10.00", "not-a-uuid")
	if _, appErr := PageAdminOrders(orders, filter, domain.PageRequest{Limit: 2, Cursor: invalidIDCursor}, now); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("invalid-ID cursor error = %+v", appErr)
	}
}

func TestFilterAdminOrdersMatchesNormalizedPublicOrderNumber(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	orders := []Order{
		{ID: "matching", OrderNo: "API-20260810-K7M4-P9Q2XZ", Amount: "10.00", CreatedAt: now, UpdatedAt: now},
		{ID: "other", OrderNo: "API-20260810-OTHER", Amount: "20.00", CreatedAt: now, UpdatedAt: now},
	}
	filtered := FilterAdminOrders(orders, AdminOrderFilter{Query: "k7m4p9q2xz"}, now)
	if len(filtered) != 1 || filtered[0].ID != "matching" {
		t.Fatalf("normalized order-number search returned %+v", filtered)
	}
}
