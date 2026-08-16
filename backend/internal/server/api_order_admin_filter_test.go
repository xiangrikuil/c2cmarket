package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"c2c-market/backend/internal/module/apiorder"
)

func TestParseAdminAPIOrderFilterAcceptsCompleteQuery(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-orders?q=API-20260810&statuses=pending_payment,completed&dateRange=7d&buyerId=00000000-0000-0000-0000-000000000001&sellerId=00000000-0000-0000-0000-000000000002&serviceId=00000000-0000-0000-0000-000000000003&dispute=active&minAmount=10.50&maxAmount=20&sort=amount_asc", nil)

	filter, appErr := parseAdminAPIOrderFilter(request)
	if appErr != nil {
		t.Fatalf("parse complete admin order query: %v", appErr)
	}
	if filter.Query != "API-20260810" || len(filter.Statuses) != 2 || filter.Statuses[0] != apiorder.StatusPendingPayment || filter.Statuses[1] != apiorder.StatusCompleted ||
		filter.DateRange != apiorder.AdminOrderDateRange7Days || filter.BuyerUserID != "00000000-0000-0000-0000-000000000001" ||
		filter.SellerUserID != "00000000-0000-0000-0000-000000000002" || filter.APIServiceID != "00000000-0000-0000-0000-000000000003" ||
		filter.Dispute != apiorder.AdminOrderDisputeActive || filter.MinAmount != "10.50" || filter.MaxAmount != "20" || filter.Sort != apiorder.AdminOrderSortAmountAsc {
		t.Fatalf("unexpected filter: %+v", filter)
	}
}

func TestParseAdminAPIOrderFilterRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string
	}{
		{name: "status", query: "statuses=unknown", field: "statuses"},
		{name: "date range", query: "dateRange=quarter", field: "dateRange"},
		{name: "buyer id", query: "buyerId=buyer", field: "buyerId"},
		{name: "seller id", query: "sellerId=seller", field: "sellerId"},
		{name: "service id", query: "serviceId=service", field: "serviceId"},
		{name: "dispute", query: "dispute=open", field: "dispute"},
		{name: "minimum amount", query: "minAmount=-1", field: "minAmount"},
		{name: "maximum amount", query: "maxAmount=1e2", field: "maxAmount"},
		{name: "amount range", query: "minAmount=20&maxAmount=10", field: "maxAmount"},
		{name: "sort", query: "sort=amount", field: "sort"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-orders?"+test.query, nil)
			_, appErr := parseAdminAPIOrderFilter(request)
			if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != test.field {
				t.Fatalf("query %q error = %+v", test.query, appErr)
			}
		})
	}
}

func TestParseAdminAPIOrderFilterTreatsEmptyDefaultsAsUnfiltered(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-orders?q=+&statuses=&dateRange=&buyerId=&sellerId=&serviceId=&dispute=&minAmount=&maxAmount=&sort=", nil)
	filter, appErr := parseAdminAPIOrderFilter(request)
	if appErr != nil {
		t.Fatalf("parse empty defaults: %v", appErr)
	}
	if filter.Query != "" || len(filter.Statuses) != 0 || filter.DateRange != "" || filter.Dispute != "" || filter.MinAmount != "" || filter.MaxAmount != "" || filter.Sort != "" {
		t.Fatalf("empty defaults produced filters: %+v", filter)
	}
}

func TestValidateAPIOrderListQueryRejectsUnknownDisputeFilter(t *testing.T) {
	for _, value := range []string{"", "all", "active", "none"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-orders?dispute="+value, nil)
		if appErr := validateAPIOrderListQuery(request); appErr != nil {
			t.Fatalf("dispute filter %q rejected: %+v", value, appErr)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-orders?dispute=open", nil)
	appErr := validateAPIOrderListQuery(request)
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || appErr.Code != "VALIDATION_FAILED" || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "dispute" {
		t.Fatalf("unexpected invalid dispute filter error: %+v", appErr)
	}
}
