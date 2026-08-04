package apimarket

import (
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
)

const (
	QuotaLimitModeLimited     = "limited"
	QuotaLimitModeUnlimited   = "unlimited"
	QuotaLimitModeUnspecified = "unspecified"

	QuotaLimitScopePerBuyerCredential  = "per_buyer_credential"
	QuotaDailyResetUTCPlus8CalendarDay = "utc_plus_8_calendar_day"
)

type QuotaUsageLimit struct {
	Mode      string
	AmountUSD string
}

type QuotaUsagePolicy struct {
	FiveHour QuotaUsageLimit
	Daily    QuotaUsageLimit
}

func UnspecifiedQuotaUsagePolicy() QuotaUsagePolicy {
	return QuotaUsagePolicy{
		FiveHour: QuotaUsageLimit{Mode: QuotaLimitModeUnspecified},
		Daily:    QuotaUsageLimit{Mode: QuotaLimitModeUnspecified},
	}
}

func NormalizeQuotaUsagePolicy(policy QuotaUsagePolicy) QuotaUsagePolicy {
	return QuotaUsagePolicy{
		FiveHour: normalizeQuotaUsageLimit(policy.FiveHour),
		Daily:    normalizeQuotaUsageLimit(policy.Daily),
	}
}

func ValidateQuotaUsagePolicy(policy QuotaUsagePolicy, fieldPrefix string, allowUnspecified bool) *domain.AppError {
	if appErr := validateQuotaUsageLimit(policy.FiveHour, fieldPrefix+".fiveHour", allowUnspecified); appErr != nil {
		return appErr
	}
	return validateQuotaUsageLimit(policy.Daily, fieldPrefix+".daily", allowUnspecified)
}

func normalizeQuotaUsageLimit(limit QuotaUsageLimit) QuotaUsageLimit {
	mode := strings.TrimSpace(limit.Mode)
	if mode == QuotaLimitModeLimited {
		return QuotaUsageLimit{Mode: mode, AmountUSD: normalizeDecimalText(limit.AmountUSD, 6)}
	}
	return QuotaUsageLimit{Mode: mode}
}

func validateQuotaUsageLimit(limit QuotaUsageLimit, field string, allowUnspecified bool) *domain.AppError {
	mode := strings.TrimSpace(limit.Mode)
	amount := strings.TrimSpace(limit.AmountUSD)
	switch mode {
	case QuotaLimitModeLimited:
		if _, ok := parsePositiveDecimal(amount); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota limit invalid", "额度限额必须填写正数美元金额。", field+".amountUsd", "invalid", "额度限额必须填写正数美元金额。")
		}
	case QuotaLimitModeUnlimited:
		if amount != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota limit invalid", "不限模式不能填写限额金额。", field+".amountUsd", "must_be_empty", "不限模式不能填写限额金额。")
		}
	case QuotaLimitModeUnspecified:
		if !allowUnspecified {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota limit required", "必须明确填写限额或选择不限。", field+".mode", "required", "必须明确填写限额或选择不限。")
		}
		if amount != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota limit invalid", "未说明模式不能填写限额金额。", field+".amountUsd", "must_be_empty", "未说明模式不能填写限额金额。")
		}
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota limit invalid", "额度限额模式不正确。", field+".mode", "invalid", "额度限额模式不正确。")
	}
	return nil
}
