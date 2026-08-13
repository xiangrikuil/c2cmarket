package auth

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	EnsureUser(ctx context.Context, username string, isAdmin bool, now time.Time) (User, *domain.AppError)
	SetDevAdminPermission(ctx context.Context, userID string, isAdmin bool, now time.Time) *domain.AppError
	UserByID(ctx context.Context, userID string) (User, *domain.AppError)
	ListAdminUsers(ctx context.Context, query AdminUserDirectoryQuery) (AdminUserDirectory, *domain.AppError)
	ListAdminAuditLogs(ctx context.Context, filter AdminAuditLogFilter, page domain.PageRequest) (domain.Page[AdminAuditLog], *domain.AppError)
	AdminUserDetail(ctx context.Context, userID string) (AdminUserDetail, *domain.AppError)
	UpdateAdminUserStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminUserStatusInput, now time.Time, buildCompletion AdminUserCompletionBuilder) (AdminUserMutationResult, idempotency.Completion, *domain.AppError)
	UpdateAdminUserPermissionWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminUserPermissionInput, now time.Time, buildCompletion AdminUserCompletionBuilder) (AdminUserMutationResult, idempotency.Completion, *domain.AppError)
	ResolveExistingOAuthUser(ctx context.Context, provider, subject string) (User, bool, *domain.AppError)
	UpsertOAuthUser(ctx context.Context, profile OAuthProfile, now time.Time) (OAuthUserResult, *domain.AppError)
	BootstrapAdminPassword(ctx context.Context, credential PasswordCredential, now time.Time) (BootstrapAdminResult, *domain.AppError)
	PasswordCredential(ctx context.Context, username string) (PasswordCredential, *domain.AppError)
	PasswordCredentialByUserID(ctx context.Context, userID string) (PasswordCredential, *domain.AppError)
	UpsertPasswordCredential(ctx context.Context, credential PasswordCredential, now time.Time) *domain.AppError
	CreateSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, absoluteExpiresAt, now time.Time) *domain.AppError
	GetSession(ctx context.Context, sessionTokenHash string, now time.Time) (User, Session, *domain.AppError)
	GetSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, Session, *domain.AppError)
	RenewSession(ctx context.Context, sessionTokenHash string, now, targetExpiresAt, renewBefore time.Time) (time.Time, bool, *domain.AppError)
	RefreshSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) *domain.AppError
	RevokeSession(ctx context.Context, sessionTokenHash string, revokedAt time.Time) *domain.AppError
	CreateAccountAppealSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) (User, *domain.AppError)
	RotateAccountAppealSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, AccountAppealSession, *domain.AppError)
	GetAccountAppealSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, AccountAppealSession, *domain.AppError)
}

// StudentRegistrationRepository is a focused durable boundary so existing
// auth repository test doubles do not need to implement registration writes.
type StudentRegistrationRepository interface {
	StudentRegistrationConfig(ctx context.Context) (StudentRegistrationConfig, *domain.AppError)
	StartStudentEmailRegistration(ctx context.Context, input EmailRegistrationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError
	ConfirmStudentEmailRegistration(ctx context.Context, input EmailRegistrationConfirmInput, codeHash string, credential PasswordCredential, sessionTokenHash, csrfTokenHash string, sessionExpiresAt, sessionAbsoluteExpiresAt, now time.Time) (User, *domain.AppError)
}

// StudentRegistrationAdminRepository owns the audited, optimistic and
// idempotent administration boundary for the persistent registration policy.
type StudentRegistrationAdminRepository interface {
	AdminStudentRegistration(ctx context.Context) (StudentRegistrationConfig, *domain.AppError)
	UpdateAdminStudentRegistrationWithIdempotency(ctx context.Context, entry idempotency.Entry, input StudentRegistrationSettingUpdate, now time.Time, buildCompletion StudentRegistrationCompletionBuilder) (StudentRegistrationConfig, idempotency.Completion, *domain.AppError)
	AdminStudentInstitutionDomains(ctx context.Context) ([]StudentInstitutionDomain, *domain.AppError)
	CreateStudentInstitutionDomainWithIdempotency(ctx context.Context, entry idempotency.Entry, input StudentInstitutionDomainCreateInput, now time.Time, buildCompletion StudentInstitutionDomainCompletionBuilder) (StudentInstitutionDomain, idempotency.Completion, *domain.AppError)
	UpdateStudentInstitutionDomainWithIdempotency(ctx context.Context, entry idempotency.Entry, input StudentInstitutionDomainUpdateInput, now time.Time, buildCompletion StudentInstitutionDomainCompletionBuilder) (StudentInstitutionDomain, idempotency.Completion, *domain.AppError)
}

type OAuthLinkRepository interface {
	MarkSessionPasswordReauthenticated(ctx context.Context, sessionTokenHash string, reauthenticatedAt time.Time) *domain.AppError
	StartOAuthLink(ctx context.Context, sessionTokenHash, stateHash, purpose string, expiresAt, now time.Time) *domain.AppError
	CompleteOAuthLink(ctx context.Context, sessionTokenHash, stateHash string, profile OAuthProfile, replacementSessionTokenHash, replacementCSRFTokenHash string, replacementExpiresAt, replacementAbsoluteExpiresAt, now time.Time) (User, *domain.AppError)
}

// RestrictedBusinessSessionRepository keeps restricted credentials isolated
// from normal and account-appeal sessions.
type RestrictedBusinessSessionRepository interface {
	CreateRestrictedBusinessSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) (RestrictedBusinessSession, *domain.AppError)
	GetRestrictedBusinessSession(ctx context.Context, sessionTokenHash string, now time.Time) (User, RestrictedBusinessSession, *domain.AppError)
	GetRestrictedBusinessSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, RestrictedBusinessSession, *domain.AppError)
	RotateRestrictedBusinessSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, RestrictedBusinessSession, *domain.AppError)
	RevokeRestrictedBusinessSession(ctx context.Context, sessionTokenHash string, revokedAt time.Time) *domain.AppError
}

// GovernanceOAuthRepository keeps restricted-business and account-appeal
// authorization states in separate, one-time persistence boundaries.
type GovernanceOAuthRepository interface {
	StartRestrictedBusinessOAuth(ctx context.Context, stateHash string, expiresAt, now time.Time) *domain.AppError
	CompleteRestrictedBusinessOAuth(ctx context.Context, stateHash string, profile OAuthProfile, sessionTokenHash, csrfTokenHash string, sessionExpiresAt, now time.Time) (User, RestrictedBusinessSession, *domain.AppError)
	StartAccountAppealOAuth(ctx context.Context, stateHash string, expiresAt, now time.Time) *domain.AppError
	CompleteAccountAppealOAuth(ctx context.Context, stateHash string, profile OAuthProfile, sessionTokenHash, csrfTokenHash string, sessionExpiresAt, now time.Time) (User, AccountAppealSession, *domain.AppError)
}

// AdminReauthenticationRepository owns purpose-bound, session-bound grants.
type AdminReauthenticationRepository interface {
	CreateAdminReauthenticationGrant(ctx context.Context, sessionTokenHash, purpose, method string, verifiedAt, expiresAt time.Time) (AdminReauthenticationGrant, *domain.AppError)
	StartAdminReauthenticationOAuth(ctx context.Context, sessionTokenHash, stateHash, purpose string, expiresAt, now time.Time) *domain.AppError
	CompleteAdminReauthenticationOAuth(ctx context.Context, sessionTokenHash, stateHash string, profile OAuthProfile, verifiedAt, expiresAt time.Time) (AdminReauthenticationGrant, *domain.AppError)
}
