package postgres

import (
	"errors"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apiorder"
)

func TestInsertAPIOrderWithNumberRetryRetriesOnlyNumberCollision(t *testing.T) {
	order := apiorder.Order{CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)}
	numbers := []string{"API-20260802-ABCDEFGHJK", "API-20260802-K7M4P9Q2XZ"}
	generateCalls := 0
	insertCalls := 0
	err := insertAPIOrderWithNumberRetry(&order, func(time.Time) (string, error) {
		value := numbers[generateCalls]
		generateCalls++
		return value, nil
	}, func() error {
		insertCalls++
		if insertCalls == 1 {
			return errAPIOrderNumberCollision
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry order number insertion: %v", err)
	}
	if generateCalls != 2 || insertCalls != 2 || order.OrderNo != numbers[1] {
		t.Fatalf("unexpected retry state: generate=%d insert=%d order=%+v", generateCalls, insertCalls, order)
	}
}

func TestInsertAPIOrderWithNumberRetryDoesNotHideOtherErrors(t *testing.T) {
	wantErr := errors.New("intent conflict")
	order := apiorder.Order{CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)}
	generateCalls := 0
	err := insertAPIOrderWithNumberRetry(&order, func(time.Time) (string, error) {
		generateCalls++
		return "API-20260802-ABCDEFGHJK", nil
	}, func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected non-number error to be returned, got %v", err)
	}
	if generateCalls != 1 {
		t.Fatalf("expected no retry for a non-number error, got %d generations", generateCalls)
	}
}
