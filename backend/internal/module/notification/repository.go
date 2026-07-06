package notification

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	ListNotifications(ctx context.Context, userID string, page domain.PageRequest) (domain.Page[Notification], *domain.AppError)
	UnreadNotificationCount(ctx context.Context, userID string) (int, *domain.AppError)
	MarkNotificationRead(ctx context.Context, userID, notificationID string, now time.Time) (Notification, *domain.AppError)
	MarkAllNotificationsRead(ctx context.Context, userID string, now time.Time) (ReadAllResult, *domain.AppError)
}
