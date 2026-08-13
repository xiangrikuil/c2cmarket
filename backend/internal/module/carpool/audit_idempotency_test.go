package carpool

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestListingReviewIdempotencyAppendsOneMemoryAuditEvent(t *testing.T) {
	now := time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, func() time.Time { return now })
	listingID := uuid.NewString()
	service.listings[listingID] = Listing{
		ID: listingID, OwnerUserID: uuid.NewString(), Status: ListingStatusActive,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	service.listingOrder = append(service.listingOrder, listingID)
	admin := auth.User{ID: uuid.NewString(), IsAdmin: true}
	input := ReviewInput{
		ListingID: listingID, Action: "pause", Status: ListingStatusPaused,
		Reason: "例行治理暂停。", ExpectedVersion: 1, RequestID: "pause-request",
	}
	buildCompletion := func(listing Listing) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 200, ContentType: "application/json", Body: []byte(`{"status":"paused"}`),
			ResourceType: "carpool_listing", ResourceID: listing.ID,
		}, nil
	}

	listing, _, changed, appErr := service.UpdateListingReviewStatusWithIdempotency(
		context.Background(), admin, "pause-route", "pause-key", "pause-hash", input, buildCompletion,
	)
	if appErr != nil || !changed || listing.Status != ListingStatusPaused {
		t.Fatalf("pause listing: listing=%+v changed=%t error=%v", listing, changed, appErr)
	}
	_, _, changed, appErr = service.UpdateListingReviewStatusWithIdempotency(
		context.Background(), admin, "pause-route", "pause-key", "pause-hash", input, buildCompletion,
	)
	if appErr != nil || changed {
		t.Fatalf("replay pause: changed=%t error=%v", changed, appErr)
	}
	events := service.ListingAuditEvents()
	if len(events) != 1 || events[0].EventType != "carpool_listing.paused" ||
		events[0].ActorKind != "admin" || events[0].RequestID != "pause-request" || events[0].AggregateVersion != 2 {
		t.Fatalf("unexpected listing audit events: %+v", events)
	}
}
