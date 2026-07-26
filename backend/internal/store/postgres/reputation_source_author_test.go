package postgres

import (
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

func TestEffectiveSourceAuthorVerificationTracksExpiryAndResourceDrift(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	expiresAt := now
	stored := reputation.SourceAuthorVerification{
		ResourceType:           reputation.SourceResourceCarpool,
		ResourceID:             "11111111-1111-4111-8111-111111111111",
		SourceURL:              "https://linux.do/t/topic/1",
		ExpectedExternalUserID: "linux-user-1",
		ActualExternalUserID:   "linux-user-1",
		Status:                 reputation.SourceVerificationVerified,
		VerifiedAt:             timePointer(now.Add(-24 * time.Hour)),
		ExpiresAt:              &expiresAt,
		Version:                3,
	}
	resource := sourceAuthorResource{
		OwnerUserID:            "22222222-2222-4222-8222-222222222222",
		SourceURL:              stored.SourceURL,
		ExpectedExternalUserID: stored.ExpectedExternalUserID,
	}

	expired := effectiveSourceAuthorVerification(stored, true, resource, now)
	if expired.Status != reputation.SourceVerificationExpired {
		t.Fatalf("expected expired status, got %#v", expired)
	}

	resource.SourceURL = "https://linux.do/t/topic/2"
	pending := effectiveSourceAuthorVerification(stored, true, resource, now.Add(-time.Minute))
	if pending.Status != reputation.SourceVerificationPending ||
		pending.VerifiedAt != nil ||
		pending.ExpiresAt != nil ||
		pending.SourceURL != resource.SourceURL {
		t.Fatalf("resource drift must invalidate verification: %#v", pending)
	}

	missing := effectiveSourceAuthorVerification(
		reputation.SourceAuthorVerification{ResourceType: reputation.SourceResourceCarpool, ResourceID: stored.ResourceID},
		false,
		resource,
		now,
	)
	if missing.Status != reputation.SourceVerificationNotSubmitted || missing.Version != 0 {
		t.Fatalf("missing verification must expose synthetic version zero: %#v", missing)
	}
}

func TestValidateSourceAuthorIdentityDecision(t *testing.T) {
	t.Parallel()

	resource := sourceAuthorResource{ExpectedExternalUserID: "linux-user-1"}
	if appErr := validateSourceAuthorIdentityDecision(
		reputation.UpdateSourceAuthorVerificationInput{
			Status:               reputation.SourceVerificationVerified,
			ActualExternalUserID: "other-user",
		},
		resource,
	); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected verified identity mismatch rejection, got %#v", appErr)
	}
	if appErr := validateSourceAuthorIdentityDecision(
		reputation.UpdateSourceAuthorVerificationInput{
			Status:               reputation.SourceVerificationMismatch,
			ActualExternalUserID: resource.ExpectedExternalUserID,
		},
		resource,
	); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected false mismatch rejection, got %#v", appErr)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}
