package report

import (
	"testing"

	"c2c-market/backend/internal/domain"
)

func TestValidateCreateAccountGovernanceAppealUsesSensitiveContentSafeStatementBoundary(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		input CreateAccountGovernanceAppealInput
		code  string
	}{
		{name: "valid", input: CreateAccountGovernanceAppealInput{AppellantUserID: "user-1", Statement: "请复核账号限制所依据的事实。"}},
		{name: "missing user", input: CreateAccountGovernanceAppealInput{Statement: "请复核账号限制所依据的事实。"}, code: domain.CodeSessionExpired},
		{name: "short statement", input: CreateAccountGovernanceAppealInput{AppellantUserID: "user-1", Statement: "复核"}, code: domain.CodeValidationFailed},
		{name: "contact statement", input: CreateAccountGovernanceAppealInput{AppellantUserID: "user-1", Statement: "请通过 review.user@example.com 联系我复核。"}, code: domain.CodeContactContentDetected},
		{name: "secret statement", input: CreateAccountGovernanceAppealInput{AppellantUserID: "user-1", Statement: "请使用 api_key=sk-proj-abcdefghijklmnopqrstuvwxyz123456 复核。"}, code: domain.CodeSecretContentDetected},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			appErr := validateCreateAccountGovernanceAppeal(testCase.input)
			if testCase.code == "" {
				if appErr != nil {
					t.Fatalf("valid account-governance appeal rejected: %v", appErr)
				}
				return
			}
			if appErr == nil || appErr.Code != testCase.code {
				t.Fatalf("validation error = %#v, want code %s", appErr, testCase.code)
			}
		})
	}
}

func TestValidateNoSubmittedAccountGovernanceAppeal(t *testing.T) {
	if appErr := ValidateNoSubmittedAccountGovernanceAppeal(false); appErr != nil {
		t.Fatalf("missing submitted appeal must be allowed: %v", appErr)
	}
	if appErr := ValidateNoSubmittedAccountGovernanceAppeal(true); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("duplicate submitted appeal error = %#v", appErr)
	}
}
