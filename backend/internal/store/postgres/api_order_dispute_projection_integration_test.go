package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAPIOrderDisputeProjectionListQueriesIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	store, err := Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer store.Close()

	const absentUserID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	buyerUserID := absentUserID
	if err := store.pool.QueryRow(context.Background(), `
		SELECT COALESCE((SELECT buyer_user_id::text FROM api_orders ORDER BY created_at DESC LIMIT 1), $1)
	`, absentUserID).Scan(&buyerUserID); err != nil {
		t.Fatalf("select buyer projection fixture: %v", err)
	}
	sellerUserID := absentUserID
	if err := store.pool.QueryRow(context.Background(), `
		SELECT COALESCE((SELECT seller_user_id::text FROM api_orders ORDER BY created_at DESC LIMIT 1), $1)
	`, absentUserID).Scan(&sellerUserID); err != nil {
		t.Fatalf("select seller projection fixture: %v", err)
	}
	if _, appErr := store.ListAPIOrdersByBuyer(context.Background(), buyerUserID, time.Now()); appErr != nil {
		t.Fatalf("execute buyer list projection query: %v", appErr)
	}
	if _, appErr := store.ListAPIOrdersBySeller(context.Background(), sellerUserID, time.Now()); appErr != nil {
		t.Fatalf("execute seller list projection query: %v", appErr)
	}
}
