package promotionreward

import (
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const referralCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const (
	CampaignCodeAPIServiceReferralV1 = "api_service_referral_v1"
	BusinessTimezone                 = "Asia/Shanghai"

	ReferralCodeStatusActive   = "active"
	ReferralCodeStatusDisabled = "disabled"

	ReferralStatusBound     = "bound"
	ReferralStatusQualified = "qualified"
	ReferralStatusRewarded  = "rewarded"
	ReferralStatusRejected  = "rejected"
	ReferralStatusRevoked   = "revoked"

	CouponSourceWelcome          = "welcome_first_api_service"
	CouponSourceReferralInviter  = "referral_inviter"
	CouponSourceReferralInvitee  = "referral_invitee"
	CouponSourceAdminGrant       = "admin_grant"
	CouponStatusPending          = "pending"
	CouponStatusAvailable        = "available"
	CouponStatusUsed             = "used"
	CouponStatusExpired          = "expired"
	CouponStatusRevoked          = "revoked"
	CouponStatusAll              = "all"
	DefaultPage                  = 1
	DefaultPageLimit             = 20
	MaximumPageLimit             = 100
	DefaultPromotionDurationHour = 24
)

type Campaign struct {
	ID                     string
	Code                   string
	ProgramEnabled         bool
	WelcomeEnabled         bool
	ReferralEnabled        bool
	StartsAt               time.Time
	EndsAt                 *time.Time
	PromotionDurationHours int
	CouponValidDays        int
	RewardDelayHours       int
	InviterMonthlyLimit    int
	RulesText              string
	CreatedByAdminID       string
	UpdatedByAdminID       string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

func (campaign Campaign) ActiveAt(now time.Time) bool {
	return campaign.ProgramEnabled && !now.Before(campaign.StartsAt) && (campaign.EndsAt == nil || now.Before(*campaign.EndsAt))
}

type PublicConfig struct {
	ProgramEnabled         bool
	WelcomeEnabled         bool
	ReferralEnabled        bool
	PromotionDurationHours int
	CouponValidDays        int
	RewardDelayHours       int
	InviterMonthlyLimit    int
	RulesText              string
	StartsAt               time.Time
	EndsAt                 *time.Time
}

type ReferralRecord struct {
	ID                    string
	InviterUserID         string
	InviteeUserID         string
	InviterDisplayName    string
	InviteeDisplayName    string
	Status                string
	BoundAt               time.Time
	QualifiedAt           *time.Time
	RewardedAt            *time.Time
	QualifiedAPIServiceID string
	RejectedAt            *time.Time
	RejectedReason        string
	RevokedAt             *time.Time
	RevokedReason         string
	RiskFlags             []string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int64
}

type ReferralStatistics struct {
	InvitedCount            int
	QualifiedCount          int
	RewardedCount           int
	PendingCount            int
	InviterRewardsThisMonth int
	InviterRewardsRemaining int
}

type ReferralSummary struct {
	Code       string
	Statistics ReferralStatistics
	Records    []ReferralRecord
	Campaign   PublicConfig
}

type Coupon struct {
	ID                  string
	CampaignID          string
	UserID              string
	UserDisplayName     string
	SourceType          string
	SourceID            string
	StoredStatus        string
	Status              string
	AvailableAt         time.Time
	ExpiresAt           time.Time
	DurationHours       int
	UsedAPIServiceID    string
	UsedAPIServiceTitle string
	ActivationID        string
	PromotionStartsAt   *time.Time
	PromotionEndsAt     *time.Time
	UsedAt              *time.Time
	RevokedAt           *time.Time
	RevokedReason       string
	RevokedByAdminID    string
	CreatedByAdminID    string
	GrantReason         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
}

func EffectiveCouponStatus(coupon Coupon, now time.Time) string {
	switch coupon.StoredStatus {
	case CouponStatusUsed, CouponStatusRevoked:
		return coupon.StoredStatus
	}
	if !now.Before(coupon.ExpiresAt) {
		return CouponStatusExpired
	}
	if now.Before(coupon.AvailableAt) {
		return CouponStatusPending
	}
	return CouponStatusAvailable
}

func CanonicalReferralCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 8 {
		return ""
	}
	for _, char := range value {
		if !strings.ContainsRune(referralCodeAlphabet, char) {
			return ""
		}
	}
	return value
}

type Pagination struct {
	Page       int
	Limit      int
	TotalItems int
	TotalPages int
}

type CouponPage struct {
	Items      []Coupon
	Pagination Pagination
}

type ReferralPage struct {
	Items      []ReferralRecord
	Pagination Pagination
}

type CouponQuery struct {
	Page       int
	Limit      int
	Status     string
	SourceType string
	Search     string
}

type ReferralQuery struct {
	Page   int
	Limit  int
	Status string
	Search string
}

type ApplyCouponInput struct {
	CouponID     string
	UserID       string
	APIServiceID string
	RequestID    string
}

type UpdateCampaignInput struct {
	AdminUserID            string
	ProgramEnabled         bool
	WelcomeEnabled         bool
	ReferralEnabled        bool
	StartsAt               time.Time
	EndsAt                 *time.Time
	PromotionDurationHours int
	CouponValidDays        int
	RewardDelayHours       int
	InviterMonthlyLimit    int
	RulesText              string
	ExpectedVersion        int64
	Reason                 string
	RequestID              string
}

type GrantCouponInput struct {
	AdminUserID   string
	UserID        string
	DurationHours int
	ValidDays     int
	Reason        string
	RequestID     string
}

type RevokeReferralInput struct {
	AdminUserID     string
	ReferralID      string
	ExpectedVersion int64
	Reason          string
	RequestID       string
}

type RevokeCouponInput struct {
	AdminUserID     string
	CouponID        string
	ExpectedVersion int64
	Reason          string
	RequestID       string
}

type CampaignCompletionBuilder func(Campaign) (idempotency.Completion, *domain.AppError)
type ReferralCompletionBuilder func(ReferralRecord) (idempotency.Completion, *domain.AppError)
type CouponCompletionBuilder func(Coupon) (idempotency.Completion, *domain.AppError)
