package contact

import "time"

type ContactMethod struct {
	ID               string
	UserID           string
	Type             string
	Label            string
	MaskedValue      string
	DisplayValue     string
	Enabled          bool
	IsDefault        bool
	VerifiedAt       *time.Time
	CurrentVersionID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

type ContactMethodVersion struct {
	ID              string
	ContactMethodID string
	OwnerUserID     string
	Value           string
	MaskedValue     string
	Fingerprint     string
	CreatedAt       time.Time
}

type ContactSession struct {
	ID                  string
	BuyerUserID         string
	SellerUserID        string
	BuyerVersionID      string
	SellerVersionID     string
	OpensAt             time.Time
	EndsAt              time.Time
	Revoked             bool
	ContactAccessLogIDs []string
}

type ContactAccessLog struct {
	ID               string
	ContactSessionID string
	ViewerUserID     string
	AccessedAt       time.Time
	RequestID        string
}

type ContactMethodInput struct {
	UserID    string
	Type      string
	Label     string
	Value     string
	IsDefault bool
	Enabled   bool
}

type UpdateContactMethodInput struct {
	UserID    string
	MethodID  string
	Type      string
	Label     string
	Value     string
	IsDefault bool
	Enabled   bool
}

type CreateContactSessionInput struct {
	BuyerUserID           string
	SellerUserID          string
	BuyerContactMethodID  string
	SellerContactMethodID string
	Duration              time.Duration
}

type ContactSessionView struct {
	SessionID string
	EndsAt    time.Time
	Items     []ContactItemView
}

type ContactItemView struct {
	Side        string
	SubjectID   string
	Type        string
	Label       string
	Value       string
	MaskedValue string
}
