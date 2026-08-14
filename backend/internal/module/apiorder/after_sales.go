package apiorder

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
)

const (
	AfterSalesReportingGracePeriod = 24 * time.Hour

	DisputeEligibilityEligible                 = "eligible"
	DisputeEligibilityOrderCancelled           = "order_cancelled"
	DisputeEligibilityDisputeExists            = "dispute_exists"
	DisputeEligibilityAfterSalesExpired        = "after_sales_expired"
	DisputeEligibilityCompletedValidityUnknown = "completed_validity_unknown"
)

func ValidityExpiresAt(order Order) *time.Time {
	if order.PackageExpiresAt != nil {
		value := order.PackageExpiresAt.UTC()
		return &value
	}
	if order.QuotaExpiresAtSnapshot != nil {
		value := order.QuotaExpiresAtSnapshot.UTC()
		return &value
	}
	if strings.TrimSpace(order.PricingSnapshot) == "" {
		return nil
	}
	var snapshot struct {
		ServiceValidityExpiresAt *time.Time `json:"serviceValidityExpiresAt"`
	}
	if err := json.Unmarshal([]byte(order.PricingSnapshot), &snapshot); err != nil || snapshot.ServiceValidityExpiresAt == nil {
		return nil
	}
	value := snapshot.ServiceValidityExpiresAt.UTC()
	return &value
}

func WithAfterSalesProjection(order Order, now time.Time) Order {
	order.MerchantConfirmOverdue = order.Status == StatusPaymentSubmitted && order.MerchantConfirmDueAt != nil && !now.Before(*order.MerchantConfirmDueAt)
	order.DeliveryOverdue = order.Status == StatusPaidConfirmed && order.DeliveryDueAt != nil && !now.Before(*order.DeliveryDueAt)
	order.CanReportLatePayment = order.Status == StatusCancelled && order.CancelReason == "payment_timeout" &&
		order.CancelledAt != nil && order.LatePaymentStatus == "" && now.Before(order.CancelledAt.Add(LatePaymentWindow))
	validityExpiresAt := ValidityExpiresAt(order)
	if validityExpiresAt != nil {
		deadline := validityExpiresAt.Add(AfterSalesReportingGracePeriod)
		order.AfterSalesExpiresAt = &deadline
	}
	order.CanOpenDispute = false
	switch {
	case order.Status == StatusCancelled:
		order.DisputeEligibilityReason = DisputeEligibilityOrderCancelled
	case order.DisputeStatus != DisputeStatusNone:
		order.DisputeEligibilityReason = DisputeEligibilityDisputeExists
	case validityExpiresAt != nil && !now.Before(validityExpiresAt.Add(AfterSalesReportingGracePeriod)):
		order.DisputeEligibilityReason = DisputeEligibilityAfterSalesExpired
	case order.Status == StatusCompleted && validityExpiresAt == nil:
		order.DisputeEligibilityReason = DisputeEligibilityCompletedValidityUnknown
	default:
		order.CanOpenDispute = true
		order.DisputeEligibilityReason = DisputeEligibilityEligible
	}
	return order
}

func DeliveryWindow(order Order) time.Duration {
	if order.PurchaseKind == PurchaseKindLimitedQuotaOffer && order.QuotaDeliveryETAMinutes >= 1 {
		return time.Duration(order.QuotaDeliveryETAMinutes) * time.Minute
	}
	return DefaultDeliveryWindow
}

func FulfillmentExpiresAt(order Order) *time.Time {
	if order.QuotaExpiresAtSnapshot != nil {
		value := order.QuotaExpiresAtSnapshot.UTC()
		return &value
	}
	if strings.TrimSpace(order.PricingSnapshot) == "" {
		return nil
	}
	var snapshot struct {
		ServiceValidityExpiresAt *time.Time `json:"serviceValidityExpiresAt"`
	}
	if err := json.Unmarshal([]byte(order.PricingSnapshot), &snapshot); err != nil || snapshot.ServiceValidityExpiresAt == nil {
		return nil
	}
	value := snapshot.ServiceValidityExpiresAt.UTC()
	return &value
}

func ValidateDisputeOccurrence(order Order, raw string, now time.Time) (*time.Time, *domain.AppError) {
	projection := WithAfterSalesProjection(order, now)
	if !projection.CanOpenDispute {
		return nil, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Dispute unavailable", "当前订单已超过售后补报期或已有纠纷，不能再次发起。")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if order.Status == StatusCompleted {
			return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Issue occurrence required", "已完成订单补报纠纷必须填写问题发生时间。", "issueOccurredAt", "required", "请选择问题实际发生时间。")
		}
		return nil, nil
	}
	occurredAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Issue occurrence invalid", "问题发生时间格式不正确。", "issueOccurredAt", "invalid", "请提交 RFC 3339 时间。")
	}
	occurredAt = occurredAt.UTC()
	if occurredAt.After(now) {
		return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Issue occurrence in future", "问题发生时间不能晚于当前时间。", "issueOccurredAt", "future", "问题发生时间不能晚于当前时间。")
	}
	if validityExpiresAt := ValidityExpiresAt(order); validityExpiresAt != nil && occurredAt.After(*validityExpiresAt) {
		return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Issue occurrence after validity", "补报的问题必须发生在所购服务有效期内。", "issueOccurredAt", "after_validity", "问题发生时间不能晚于服务有效期。")
	}
	return &occurredAt, nil
}
