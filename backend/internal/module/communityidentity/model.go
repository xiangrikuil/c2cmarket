package communityidentity

import "time"

// IdentityType 是固定的社区身份代码，与信誉徽章和信誉信号分开。
type IdentityType string

const (
	IdentityTypeFoundingUser    IdentityType = "FOUNDING_USER"
	IdentityTypeBetaContributor IdentityType = "BETA_CONTRIBUTOR"
)

type Source string

const (
	SourceAuto     Source = "AUTO"
	SourceAdmin    Source = "ADMIN"
	SourceBackfill Source = "BACKFILL"
)

const (
	DefaultFoundingCutoff = "2026-09-30T23:59:59+08:00"
	NotificationType      = "community_identity"
	NotificationEventType = "community_identity.granted"
)

type Identity struct {
	ID           string
	UserID       string
	Type         IdentityType
	Source       Source
	QualifiedAt  *time.Time
	GrantedAt    time.Time
	GrantedBy    string
	GrantReason  string
	RevokedAt    *time.Time
	RevokedBy    string
	RevokeReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PublicIdentity struct {
	Code        IdentityType
	Name        string
	Description string
	GrantedAt   time.Time
}

type GrantFoundingInput struct {
	UserID      string
	QualifiedAt time.Time
	Source      Source
}

type GrantAdminInput struct {
	TargetUserID string
	AdminUserID  string
	Type         IdentityType
	Reason       string
	RequestID    string
}

type RevokeInput struct {
	TargetUserID string
	AdminUserID  string
	Type         IdentityType
	Reason       string
	RequestID    string
}

func IsKnownType(value IdentityType) bool {
	return value == IdentityTypeFoundingUser || value == IdentityTypeBetaContributor
}

func Definition(value IdentityType) (PublicIdentity, bool) {
	switch value {
	case IdentityTypeFoundingUser:
		return PublicIdentity{Code: value, Name: "创始用户", Description: "在公开测试截止前完成账号验证的早期用户"}, true
	case IdentityTypeBetaContributor:
		return PublicIdentity{Code: value, Name: "内测共建者", Description: "帮助平台测试和改进产品的社区成员"}, true
	default:
		return PublicIdentity{}, false
	}
}

func ToPublic(value Identity) PublicIdentity {
	definition, _ := Definition(value.Type)
	definition.GrantedAt = value.GrantedAt
	return definition
}

func Compact(values []PublicIdentity) *PublicIdentity {
	var selected *PublicIdentity
	for _, value := range values {
		if selected == nil || value.Code == IdentityTypeBetaContributor {
			copy := value
			selected = &copy
		}
	}
	return selected
}
