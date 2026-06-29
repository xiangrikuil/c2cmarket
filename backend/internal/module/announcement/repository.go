package announcement

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	UserAnnouncements(ctx context.Context, userID string, now time.Time) ([]Announcement, *domain.AppError)
	ActiveAnnouncements(ctx context.Context, userID, channel string, now time.Time) ([]Announcement, *domain.AppError)
	HomeAnnouncement(ctx context.Context, userID string, now time.Time) (*Announcement, *domain.AppError)
	UserAnnouncementBySlug(ctx context.Context, userID, slug string, now time.Time) (Announcement, *domain.AppError)
	AnnouncementUnreadCount(ctx context.Context, userID string, importantOnly bool, now time.Time) (int, *domain.AppError)
	UpsertReceipt(ctx context.Context, input ReceiptInput, now time.Time) (Receipt, *domain.AppError)

	AdminAnnouncements(ctx context.Context, now time.Time) ([]Announcement, *domain.AppError)
	AdminAnnouncementByID(ctx context.Context, id string, now time.Time) (Announcement, *domain.AppError)
	CreateAnnouncement(ctx context.Context, input CreateInput, now time.Time) (Announcement, *domain.AppError)
	UpdateAnnouncement(ctx context.Context, input UpdateInput, now time.Time) (Announcement, *domain.AppError)
	PublishAnnouncement(ctx context.Context, input ActionInput, now time.Time) (Announcement, *domain.AppError)
	OfflineAnnouncement(ctx context.Context, input ActionInput, now time.Time) (Announcement, *domain.AppError)
	DuplicateAnnouncement(ctx context.Context, input ActionInput, now time.Time) (Announcement, *domain.AppError)
	AnnouncementAuditLogs(ctx context.Context, now time.Time) ([]AuditLog, *domain.AppError)
}
