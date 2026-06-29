package feedback

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	TypeFunctionIssue        = "function_issue"
	TypeDataCorrection       = "data_correction"
	TypeExperienceSuggestion = "experience_suggestion"
	TypePublishContactBlock  = "publish_contact_block"

	ImpactGeneral         = "general"
	ImpactBlocksOperation = "blocks_operation"
	ImpactCannotContinue  = "cannot_continue"

	StatusSubmitted     = "submitted"
	StatusRecorded      = "recorded"
	StatusFollowingUp   = "following_up"
	StatusResolved      = "resolved"
	StatusDeclined      = "declined"
	StatusNeedsUserInfo = "needs_user_info"
	StatusClosed        = "closed"

	EventSubmitted        = "submitted"
	EventAdminHandled     = "admin_handled"
	EventUserSupplemented = "user_supplemented"
	EventRead             = "read"
)

type Ticket struct {
	ID                  string
	SubmitterUserID     string
	SubmitterUsername   string
	SubmitterName       string
	Type                string
	Impact              string
	Status              string
	Title               string
	Description         string
	ContextPageLabel    string
	ContextTargetType   string
	ContextTargetID     string
	ContextTargetLabel  string
	ContextRoleLabel    string
	AdminResponse       string
	AdminInternalNote   string
	HandledByAdminID    string
	HandledByAdminName  string
	HandledAt           *time.Time
	LatestAdminUpdateAt *time.Time
	SubmitterReadAt     *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
	Events              []Event
}

type Event struct {
	ID            string
	TicketID      string
	ActorUserID   string
	ActorName     string
	ActorRole     string
	Action        string
	PublicMessage string
	InternalNote  string
	CreatedAt     time.Time
}

type CreateInput struct {
	SubmitterUserID    string
	SubmitterUsername  string
	SubmitterName      string
	Type               string
	Impact             string
	Title              string
	Description        string
	ContextPageLabel   string
	ContextTargetType  string
	ContextTargetID    string
	ContextTargetLabel string
	ContextRoleLabel   string
	RequestID          string
}

type SupplementInput struct {
	ID              string
	SubmitterUserID string
	Message         string
	RequestID       string
}

type AdminHandleInput struct {
	ID              string
	AdminUserID     string
	AdminName       string
	Status          string
	Response        string
	InternalNote    string
	ExpectedVersion int64
	RequestID       string
}

type CompletionBuilder func(Ticket) (idempotency.Completion, *domain.AppError)
