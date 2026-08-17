package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDynamicCatalogTransactionGuardsCoverEveryCreationBoundary(t *testing.T) {
	t.Parallel()

	assertCallCount := func(file, functionName, call string, minimum int) {
		t.Helper()
		data, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		source := string(data)
		start := strings.Index(source, "func "+functionName)
		if start < 0 {
			t.Fatalf("%s missing function %s", file, functionName)
		}
		remaining := source[start:]
		next := strings.Index(remaining[5:], "\nfunc ")
		if next >= 0 {
			remaining = remaining[:next+5]
		}
		if count := strings.Count(remaining, call); count < minimum {
			t.Fatalf("%s must call %s at least %d time(s), got %d", functionName, call, minimum, count)
		}
	}

	assertCallCount("api_market.go", "(s *Store) updateAPIServicePublicationInTx", "ensureAPIServiceCatalogActiveInTx", 1)
	assertCallCount("api_market.go", "(s *Store) createAPIPurchaseIntentInTx", "ensureAPIServiceCatalogActiveInTx", 1)
	assertCallCount("api_order.go", "(s *Store) createAPIOrderInTx", "ensureAPIServiceCatalogActiveInTx", 1)
	assertCallCount("api_quota.go", "(s *Store) CreateAPIQuotaOrderWithIdempotency", "ensureAPIServiceCatalogActiveInTx", 1)
	assertCallCount("api_quota.go", "publishAPIQuotaBatchInTx", "ensureAPIServiceCatalogActiveInTx", 1)
	assertCallCount("api_quota.go", "(s *Store) updateAPIQuotaBatchStatusInTx", "ensureAPIServiceCatalogActiveInTx", 1)
}

func TestCatalogRiskHoldPausesActionsAndAutomaticMaterialization(t *testing.T) {
	t.Parallel()

	orderSource, err := os.ReadFile("api_order.go")
	if err != nil {
		t.Fatalf("read api_order.go: %v", err)
	}
	maintenanceSource, err := os.ReadFile("maintenance.go")
	if err != nil {
		t.Fatalf("read maintenance.go: %v", err)
	}
	orderSQL := string(orderSource)
	for _, required := range []string{
		`action != "open_dispute"`,
		"rejectActiveCatalogRiskHoldInTx",
		"api_order_catalog_risk_holds hold",
		"activeCatalogHold",
	} {
		if !strings.Contains(orderSQL, required) {
			t.Fatalf("API order risk-hold contract missing %q", required)
		}
	}
	if !strings.Contains(string(maintenanceSource), "api_order_catalog_risk_holds hold") {
		t.Fatal("maintenance timeout scan must exclude orders with an active catalog risk hold")
	}
}

func TestCarpoolCatalogGovernanceRemovesListingWithoutRewritingApplications(t *testing.T) {
	t.Parallel()

	carpoolSource, err := os.ReadFile("carpool.go")
	if err != nil {
		t.Fatalf("read carpool.go: %v", err)
	}
	lifecycleSource, err := os.ReadFile("catalog_lifecycle.go")
	if err != nil {
		t.Fatalf("read catalog_lifecycle.go: %v", err)
	}
	if strings.Contains(string(carpoolSource), "WHERE a.status = 'accepted_reserved'") {
		t.Fatal("seat availability must not depend on obsolete reservations")
	}
	if !strings.Contains(string(lifecycleSource), "SET governance_status = 'removed'") ||
		strings.Contains(string(lifecycleSource), "SET status = 'cancelled_by_owner'") {
		t.Fatal("catalog governance must remove the listing without rewriting pending applications")
	}
}
