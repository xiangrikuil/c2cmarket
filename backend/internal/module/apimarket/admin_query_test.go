package apimarket

import (
	"context"
	"fmt"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func TestAdminServicesFiltersBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("online-%02d", index)
		manager.serviceOrder = append(manager.serviceOrder, id)
		manager.services[id] = Service{
			ID:                id,
			ReviewStatus:      ServiceReviewStatusApproved,
			PublicationStatus: ServicePublicationStatusOnline,
			ModerationStatus:  ServiceModerationStatusClear,
			UpdatedAt:         now.Add(-time.Duration(index) * time.Minute),
		}
	}
	manager.serviceOrder = append(manager.serviceOrder, "older-exception")
	manager.services["older-exception"] = Service{
		ID:                "older-exception",
		ReviewStatus:      ServiceReviewStatusApproved,
		PublicationStatus: ServicePublicationStatusOffline,
		ModerationStatus:  ServiceModerationStatusAdminSuspended,
		UpdatedAt:         now.Add(-time.Hour),
	}

	page, appErr := manager.AdminServices(context.Background(), auth.User{IsAdmin: true}, AdminServiceFilter{View: AdminServiceViewExceptions}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list admin API service exceptions: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "older-exception" || page.NextCursor != nil {
		t.Fatalf("expected older exception on first filtered page, got %+v", page)
	}
}
