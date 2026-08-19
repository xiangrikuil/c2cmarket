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
	CreateCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, listing Listing, ack *RiskAcknowledgement, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, *domain.AppError)
	PublishCarpoolListing(ctx context.Context, listing Listing, ack *RiskAcknowledgement, now time.Time) (Listing, *domain.AppError)
	PublishCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, listing Listing, ack *RiskAcknowledgement, now time.Time, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, *domain.AppError)
	ListPublicCarpoolListings(ctx context.Context, filter ListingFilter, page domain.PageRequest) (domain.Page[Listing], *domain.AppError)
	GetPublicCarpoolListing(ctx context.Context, listingID string) (Listing, *domain.AppError)
	ListCarpoolListingsByOwner(ctx context.Context, ownerUserID, view string, page domain.PageRequest) (domain.Page[Listing], *domain.AppError)
	GetCarpoolListingByOwner(ctx context.Context, ownerUserID, listingID string) (Listing, *domain.AppError)
	ListAdminCarpoolListings(ctx context.Context, filter ListingFilter, page domain.PageRequest) (domain.Page[Listing], *domain.AppError)
	GetAdminCarpoolListing(ctx context.Context, listingID string) (Listing, *domain.AppError)
	UpdateCarpoolListing(ctx context.Context, input UpdateListingInput, ack *RiskAcknowledgement, now time.Time) (Listing, *domain.AppError)
	UpdateCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateListingInput, ack *RiskAcknowledgement, now time.Time, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, *domain.AppError)
	UpdateCarpoolRecruitment(ctx context.Context, input RecruitmentInput, targetStatus string, now time.Time) (Listing, *domain.AppError)
	SubmitCarpoolListingForReview(ctx context.Context, user auth.User, input SubmitListingReviewInput, now time.Time) (Listing, *domain.AppError)
	SubmitCarpoolListingForReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input SubmitListingReviewInput, now time.Time, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, *domain.AppError)
	UpdateCarpoolListingReviewStatus(ctx context.Context, user auth.User, input ReviewInput, now time.Time) (Listing, *domain.AppError)
	UpdateCarpoolListingReviewStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input ReviewInput, now time.Time, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, *domain.AppError)
	CreateCarpoolApplication(ctx context.Context, application Application, ack *RiskAcknowledgement) *domain.AppError
	CreateCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, application Application, ack *RiskAcknowledgement, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	ListCarpoolApplicationsByBuyer(ctx context.Context, buyerUserID string) ([]Application, *domain.AppError)
	ListCarpoolApplicationsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]Application, *domain.AppError)
	GetCarpoolApplicationForBuyer(ctx context.Context, buyerUserID, applicationID string) (Application, *domain.AppError)
	GetCarpoolApplicationForActor(ctx context.Context, actor auth.BusinessActor, applicationID, participantRole string) (Application, *domain.AppError)
	ListCarpoolApplicationsByOwner(ctx context.Context, ownerUserID string) ([]Application, *domain.AppError)
	GetCarpoolApplicationForOwner(ctx context.Context, ownerUserID, applicationID string) (Application, *domain.AppError)
	AcceptCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input AcceptApplicationInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	ConfirmCarpoolApplicationConditions(ctx context.Context, input ConfirmApplicationConditionsInput, now time.Time) (Application, *domain.AppError)
	RejectCarpoolApplication(ctx context.Context, input RejectApplicationInput, now time.Time) (Application, *domain.AppError)
	RejectCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input RejectApplicationInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	CancelCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input CancelApplicationInput, now time.Time, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, *domain.AppError)
	ListCarpoolMembershipsByBuyer(ctx context.Context, buyerUserID string) ([]Membership, *domain.AppError)
	ListCarpoolMembershipsByOwner(ctx context.Context, ownerUserID string) ([]Membership, *domain.AppError)
	ListCarpoolMembershipsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]Membership, *domain.AppError)
	GetCarpoolMembershipForActor(ctx context.Context, actor auth.BusinessActor, membershipID, participantRole string) (Membership, *domain.AppError)
	EndCarpoolMembershipWithIdempotency(ctx context.Context, entry idempotency.Entry, input EndMembershipInput, now time.Time, buildCompletion MembershipCompletionBuilder) (Membership, idempotency.Completion, *domain.AppError)
	UpdateCarpoolMembershipOwnerNoteWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateMembershipOwnerNoteInput, now time.Time, buildCompletion MembershipCompletionBuilder) (Membership, idempotency.Completion, *domain.AppError)
}
