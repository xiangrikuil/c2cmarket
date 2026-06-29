package announcement

import "time"

const (
	CategoryPlatform    = "platform"
	CategoryRules       = "rules"
	CategoryMaintenance = "maintenance"
	CategoryFeature     = "feature"
	CategoryRisk        = "risk"
	CategoryOperation   = "operation"

	LevelNormal    = "normal"
	LevelImportant = "important"

	StatusDraft     = "draft"
	StatusScheduled = "scheduled"
	StatusPublished = "published"
	StatusOffline   = "offline"
	StatusExpired   = "expired"
	StatusArchived  = "archived"

	ChannelMessageCenter = "message_center"
	ChannelHomeBanner    = "home_banner"

	AuditCreated    = "announcement_created"
	AuditUpdated    = "announcement_updated"
	AuditPublished  = "announcement_published"
	AuditOfflined   = "announcement_offlined"
	AuditDuplicated = "announcement_duplicated"
)

type Audience struct {
	Type string `json:"type"`
}

type Receipt struct {
	AnnouncementID      string
	AnnouncementVersion int64
	FirstSeenAt         *time.Time
	ReadAt              *time.Time
	DismissedAt         *time.Time
}

type Announcement struct {
	ID              string
	Slug            string
	Title           string
	Summary         string
	ContentMarkdown string
	Category        string
	Level           string
	Status          string
	Channels        []string
	Audience        Audience
	IsPinned        bool
	IsDismissible   bool
	CTALabel        string
	CTAURL          string
	PublishAt       time.Time
	ExpireAt        *time.Time
	Version         int64
	CreatedBy       string
	UpdatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Receipt         *Receipt
}

type FormInput struct {
	Title           string
	Summary         string
	ContentMarkdown string
	Category        string
	Level           string
	Channels        []string
	IsPinned        bool
	IsDismissible   bool
	CTALabel        string
	CTAURL          string
	PublishAt       time.Time
	ExpireAt        *time.Time
}

type CreateInput struct {
	OperatorID   string
	OperatorName string
	Form         FormInput
}

type UpdateInput struct {
	ID           string
	OperatorID   string
	OperatorName string
	Form         FormInput
}

type ActionInput struct {
	ID           string
	OperatorID   string
	OperatorName string
	Reason       string
}

type ReceiptInput struct {
	AnnouncementID string
	UserID         string
	Action         string
}

type AuditLog struct {
	ID                string
	Action            string
	AnnouncementID    string
	AnnouncementTitle string
	OperatorID        string
	OperatorName      string
	Reason            string
	CreatedAt         time.Time
}
