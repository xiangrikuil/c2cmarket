package postgres

import (
	"strings"
	"testing"
)

func TestPublicMarketPredicatesRequireActiveOwnerAccounts(t *testing.T) {
	for name, predicate := range map[string]string{
		"api service": publicAPIServiceOrderablePredicate("service"),
		"carpool":     publicCarpoolListingPredicate("listing"),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(predicate, "owner.account_status = 'active'") {
				t.Fatalf("public predicate must require an active owner account: %s", predicate)
			}
		})
	}
	if !strings.Contains(searchCarpoolsSQL, "owner.account_status = 'active'") {
		t.Fatalf("carpool search must reuse the active-owner predicate: %s", searchCarpoolsSQL)
	}
}
