package demand

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

func TestCreateDemandPublishesImmediately(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, func() time.Time { return now })
	user := auth.User{
		ID:          "user-1",
		Username:    "buyer",
		DisplayName: "买家",
	}

	created, appErr := service.Create(context.Background(), user, CreateInput{
		Title:           "ChatGPT Business",
		MaxPriceCNY:     "190",
		RegionCode:      "us",
		OwnerPreference: "personal",
		SourceURL:       "https://linux.do/t/topic/234567",
		Note:            "希望官方 workspace 成员席位。",
	})
	if appErr != nil {
		t.Fatalf("create demand: %v", appErr)
	}

	if created.Status != StatusActive {
		t.Fatalf("expected created demand to be active, got %q", created.Status)
	}

	publicRows, appErr := service.PublicDemands(context.Background())
	if appErr != nil {
		t.Fatalf("list public demands: %v", appErr)
	}
	if len(publicRows) != 1 || publicRows[0].ID != created.ID {
		t.Fatalf("expected created demand in public list, got %#v", publicRows)
	}

	publicDetail, appErr := service.PublicDemand(context.Background(), created.ID)
	if appErr != nil {
		t.Fatalf("get public demand: %v", appErr)
	}
	if publicDetail.ID != created.ID {
		t.Fatalf("expected public demand %q, got %q", created.ID, publicDetail.ID)
	}
}
