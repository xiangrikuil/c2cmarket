package operationaudit

import "time"

const (
	SourceAdmin                  = "admin"
	SourceModeration             = "moderation"
	SourceDomain                 = "domain"
	SourceAPIOrder               = "api_order"
	SourceContactSessionAccess   = "contact_session_access"
	SourceAPIIntentContactAccess = "api_intent_contact_access"
	SourceAPIOrderAccess         = "api_order_access"
	SourceProbe                  = "probe"

	ActorUser   = "user"
	ActorAdmin  = "admin"
	ActorSystem = "system"

	OutcomeSucceeded     = "succeeded"
	OutcomeStatusChanged = "status_changed"
	OutcomeAccessed      = "accessed"

	DomainIdentity    = "identity"
	DomainAccount     = "account"
	DomainInstitution = "institution"
	DomainContact     = "contact"
	DomainCarpool     = "carpool"
	DomainAPIService  = "api_service"
	DomainAPIQuota    = "api_quota"
	DomainAPIOrder    = "api_order"
	DomainModeration  = "moderation"
	DomainProbe       = "probe"
)

var SourceKinds = []string{
	SourceAdmin,
	SourceModeration,
	SourceDomain,
	SourceAPIOrder,
	SourceContactSessionAccess,
	SourceAPIIntentContactAccess,
	SourceAPIOrderAccess,
	SourceProbe,
}

var ActorKinds = []string{ActorUser, ActorAdmin, ActorSystem}

var Outcomes = []string{OutcomeSucceeded, OutcomeStatusChanged, OutcomeAccessed}

var Domains = []string{
	DomainIdentity,
	DomainAccount,
	DomainInstitution,
	DomainContact,
	DomainCarpool,
	DomainAPIService,
	DomainAPIQuota,
	DomainAPIOrder,
	DomainModeration,
	DomainProbe,
}

type Filter struct {
	SourceKind  string
	Domain      string
	Action      string
	ActorKind   string
	ActorUserID string
	TargetType  string
	TargetID    string
	Outcome     string
	From        string
	To          string
	Search      string
	Limit       int
	Cursor      string
}

type CursorPosition struct {
	OccurredAt time.Time
	SourceKind string
	EventID    string
}

type Query struct {
	SourceKind  string
	Domain      string
	Action      string
	ActorKind   string
	ActorUserID string
	TargetType  string
	TargetID    string
	Outcome     string
	From        time.Time
	To          time.Time
	Search      string
	Limit       int
	Cursor      *CursorPosition
}

type Entry struct {
	ID            string
	SourceEventID string
	SourceKind    string
	Domain        string
	ActorKind     string
	ActorUserID   string
	ActorUsername string
	Action        string
	ActionLabel   string
	TargetType    string
	TargetID      string
	TargetLabel   string
	Outcome       string
	Summary       string
	DetailPath    string
	RequestID     string
	CreatedAt     time.Time
}
