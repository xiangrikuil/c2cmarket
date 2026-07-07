package domain

import "fmt"

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AppError struct {
	Status      int
	Code        string
	Title       string
	Detail      string
	FieldErrors []FieldError
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

func NewError(status int, code, title, detail string) *AppError {
	return &AppError{
		Status: status,
		Code:   code,
		Title:  title,
		Detail: detail,
	}
}

func NewFieldError(status int, code, title, detail, field, fieldCode, message string) *AppError {
	err := NewError(status, code, title, detail)
	err.FieldErrors = []FieldError{{
		Field:   field,
		Code:    fieldCode,
		Message: message,
	}}
	return err
}

const (
	CodeAccountRestricted               = "ACCOUNT_RESTRICTED"
	CodeActiveApplicationExists         = "ACTIVE_APPLICATION_EXISTS"
	CodeActiveAPIIntentExists           = "ACTIVE_API_INTENT_EXISTS"
	CodeActiveReportExists              = "ACTIVE_REPORT_EXISTS"
	CodeAPIPurchaseIntentHasOrder       = "API_PURCHASE_INTENT_HAS_ORDER"
	CodeActiveMembershipExists          = "ACTIVE_MEMBERSHIP_EXISTS"
	CodeContactAccessForbidden          = "CONTACT_ACCESS_FORBIDDEN"
	CodeContactMethodDisabled           = "CONTACT_METHOD_DISABLED"
	CodeContactMethodNotOwned           = "CONTACT_METHOD_NOT_OWNED"
	CodeContactMethodRequired           = "CONTACT_METHOD_REQUIRED"
	CodeContactWindowExpired            = "CONTACT_WINDOW_EXPIRED"
	CodeCSRFTokenInvalid                = "CSRF_TOKEN_INVALID"
	CodeEmailRegistrationDisabled       = "EMAIL_REGISTRATION_DISABLED"
	CodeFieldNotAllowed                 = "FIELD_NOT_ALLOWED"
	CodeIdempotencyInProgress           = "IDEMPOTENCY_IN_PROGRESS"
	CodeIdempotencyKeyReused            = "IDEMPOTENCY_KEY_REUSED"
	CodeInternalError                   = "INTERNAL_ERROR"
	CodeInvalidStateTransition          = "INVALID_STATE_TRANSITION"
	CodeLinuxDoBindingRequired          = "LINUX_DO_BINDING_REQUIRED"
	CodeInvalidCredentials              = "INVALID_CREDENTIALS"
	CodeJoinConfirmationExpired         = "JOIN_CONFIRMATION_EXPIRED"
	CodeMembershipNotActive             = "MEMBERSHIP_NOT_ACTIVE"
	CodeMerchantContactRequired         = "MERCHANT_CONTACT_REQUIRED"
	CodeMerchantContactUnavailable      = "MERCHANT_CONTACT_UNAVAILABLE"
	CodeObjectNotFound                  = "OBJECT_NOT_FOUND"
	CodeOfficialPriceUserSubmitDisabled = "OFFICIAL_PRICE_USER_SUBMIT_DISABLED"
	CodePermissionDenied                = "PERMISSION_DENIED"
	CodePriceNormalizationRequired      = "PRICE_NORMALIZATION_REQUIRED"
	CodePreconditionRequired            = "PRECONDITION_REQUIRED"
	CodeRateLimited                     = "RATE_LIMITED"
	CodeProductPlanResolutionRequired   = "PRODUCT_PLAN_RESOLUTION_REQUIRED"
	CodeRiskAckRequired                 = "RISK_ACK_REQUIRED"
	CodeSecretContentDetected           = "SECRET_CONTENT_DETECTED"
	CodeSeatUnavailable                 = "SEAT_UNAVAILABLE"
	CodeSessionExpired                  = "SESSION_EXPIRED"
	CodeSessionRevoked                  = "SESSION_REVOKED"
	CodeURLNotAllowed                   = "URL_NOT_ALLOWED"
	CodeValidationFailed                = "VALIDATION_FAILED"
	CodeVersionConflict                 = "VERSION_CONFLICT"
)
