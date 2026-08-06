package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSlowActiveQueryCountIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	store, err := Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer store.Close()

	queryCtx, cancelQuery := context.WithCancel(context.Background())
	defer cancelQuery()
	queryDone := make(chan error, 1)
	go func() {
		_, queryErr := store.pool.Exec(queryCtx, "SELECT pg_sleep(2)")
		queryDone <- queryErr
	}()

	deadline := time.Now().Add(time.Second)
	for {
		countCtx, cancelCount := context.WithTimeout(context.Background(), time.Second)
		count, countErr := store.SlowActiveQueryCount(countCtx, 100*time.Millisecond)
		cancelCount()
		if countErr != nil {
			t.Fatalf("count slow active queries: %v", countErr)
		}
		if count >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sleeping query was not reported as slow and active")
		}
		time.Sleep(25 * time.Millisecond)
	}

	cancelQuery()
	select {
	case <-queryDone:
	case <-time.After(time.Second):
		t.Fatal("sleeping query did not stop after cancellation")
	}
}
