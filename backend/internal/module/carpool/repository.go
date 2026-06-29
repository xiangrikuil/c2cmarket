package carpool

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateCarpoolListing(ctx context.Context, listing Listing, ack *RiskAcknowledgement) *domain.AppError
	PublishCarpoolListing(ctx context.Context, listing Listing, ack *RiskAcknowledgement, now time.Time) (Listing, *domain.AppError)
	ListPublicCarpoolListings(ctx context.Context) ([]Listing, *domain.AppError)
	GetPublicCarpoolListing(ctx context.Context, listingID string) (Listing, *domain.AppError)
	ListCarpoolListingsByOwner(ctx context.Context, ownerUserID string) ([]Listing, *domain.AppError)
	ListAdminCarpoolListings(ctx context.Context) ([]Listing, *domain.AppError)
	GetAdminCarpoolListing(ctx context.Context, listingID string) (Listing, *domain.AppError)
	UpdateCarpoolListing(ctx context.Context, input UpdateListingInput, ack *RiskAcknowledgement, now time.Time) (Listing, *domain.AppError)
	SubmitCarpoolListingForReview(ctx context.Context, user auth.User, input SubmitListingReviewInput, now time.Time) (Listing, *domain.AppError)
	UpdateCarpoolListingReviewStatus(ctx context.Context, user auth.User, input ReviewInput, now time.Time) (Listing, *domain.AppError)
	CreateCarpoolApplication(ctx context.Context, application Application, ack *RiskAcknowledgement) *domain.AppError
	ListCarpoolApplicationsByBuyer(ctx context.Context, buyerUserID string) ([]Application, *domain.AppError)
	GetCarpoolApplicationForBuyer(ctx context.Context, buyerUserID, applicationID string) (Application, *domain.AppError)
	ListCarpoolApplicationsByOwner(ctx context.Context, ownerUserID string) ([]Application, *domain.AppError)
	GetCarpoolApplicationForOwner(ctx context.Context, ownerUserID, applicationID string) (Application, *domain.AppError)
	AcceptCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input AcceptApplicationInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	RejectCarpoolApplication(ctx context.Context, input RejectApplicationInput, now time.Time) (Application, *domain.AppError)
	CancelCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input CancelApplicationInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, entry idempotency.Entry, input WithdrawAcceptanceInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, entry idempotency.Entry, input ConfirmApplicationJoinInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	ListCarpoolMembershipsByBuyer(ctx context.Context, buyerUserID string) ([]Membership, *domain.AppError)
	ListCarpoolMembershipsByOwner(ctx context.Context, ownerUserID string) ([]Membership, *domain.AppError)
	ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, entry idempotency.Entry, input ConfirmMembershipCompleteInput, now time.Time, buildCompletion MembershipCompletionBuilder) (Membership, idempotency.Completion, *domain.AppError)
	EndCarpoolMembershipWithIdempotency(ctx context.Context, entry idempotency.Entry, input EndMembershipInput, now time.Time, buildCompletion MembershipCompletionBuilder) (Membership, idempotency.Completion, *domain.AppError)
}
