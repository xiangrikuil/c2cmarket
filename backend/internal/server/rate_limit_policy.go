package server

import "net/http"

type rateLimitPolicy struct {
	Group       string
	IPLimit     int
	UserLimit   int
	TargetLimit int
}

var (
	passwordLoginRateLimit = rateLimitPolicy{
		Group: "password_login", IPLimit: 20, UserLimit: 10,
	}
	oauthStartRateLimit = rateLimitPolicy{
		Group: "oauth_start", IPLimit: 30, UserLimit: 15,
	}
	oauthCallbackRateLimit = rateLimitPolicy{
		Group: "oauth_callback", IPLimit: 20, UserLimit: 10,
	}
	accountAppealStartRateLimit = rateLimitPolicy{
		Group: "account_appeal_start", IPLimit: 20, UserLimit: 10,
	}
	accountAppealSessionRateLimit = rateLimitPolicy{
		Group: "account_appeal_session", IPLimit: 60, UserLimit: 30,
	}
	accountAppealCreateRateLimit = rateLimitPolicy{
		Group: "account_appeal_create", IPLimit: 10, UserLimit: 5,
	}
	emailRegistrationStartRateLimit = rateLimitPolicy{
		Group: "email_registration_start", IPLimit: 20, UserLimit: 5, TargetLimit: 3,
	}
	emailRegistrationConfirmRateLimit = rateLimitPolicy{
		Group: "email_registration_confirm", IPLimit: 30, UserLimit: 10, TargetLimit: 10,
	}
	emailVerificationStartRateLimit = rateLimitPolicy{
		Group: "email_verification_start", IPLimit: 20, UserLimit: 5, TargetLimit: 3,
	}
	emailVerificationConfirmRateLimit = rateLimitPolicy{
		Group: "email_verification_confirm", IPLimit: 30, UserLimit: 10, TargetLimit: 10,
	}
	carpoolApplicationRateLimit = rateLimitPolicy{
		Group: "carpool_application_create", IPLimit: 60, UserLimit: 12,
	}
	apiPurchaseIntentRateLimit = rateLimitPolicy{
		Group: "api_purchase_intent_create", IPLimit: 60, UserLimit: 12,
	}
	apiOrderRateLimit = rateLimitPolicy{
		Group: "api_order_create", IPLimit: 60, UserLimit: 12,
	}
	apiQuotaOrderRateLimit = rateLimitPolicy{
		Group: "api_quota_order_create", IPLimit: 3000, UserLimit: 12,
	}
	contactReadRateLimit = rateLimitPolicy{
		Group: "contact_read", IPLimit: 120, UserLimit: 60,
	}
	reportCreateRateLimit = rateLimitPolicy{
		Group: "report_create", IPLimit: 20, UserLimit: 5,
	}
	appealCreateRateLimit = rateLimitPolicy{
		Group: "appeal_create", IPLimit: 20, UserLimit: 5,
	}
	reportSupplementRateLimit = rateLimitPolicy{
		Group: "report_supplement", IPLimit: 30, UserLimit: 10,
	}
	modelAuditRunRateLimit = rateLimitPolicy{
		Group: "model_audit_run_create", IPLimit: 20, UserLimit: 5,
	}
)

func (s *Server) limitPolicy(policy rateLimitPolicy, next http.HandlerFunc) http.HandlerFunc {
	return s.limitHandlerByActor(policy.Group, policy.IPLimit, policy.UserLimit, next)
}

func (s *Server) allowTarget(w http.ResponseWriter, r *http.Request, policy rateLimitPolicy, targetType, target string) bool {
	retryAfter, appErr := s.checkTargetRateLimit(policy.Group, targetType, target, policy.TargetLimit)
	if appErr == nil {
		return true
	}
	writeRateLimitProblem(w, r, retryAfter, appErr)
	return false
}
