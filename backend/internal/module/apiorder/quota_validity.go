package apiorder

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
)

func HasMinimumDeliveryValidity(order Order, now time.Time) bool {
	expiresAt := FulfillmentExpiresAt(order)
	return expiresAt == nil || !expiresAt.Before(now.Add(MinimumDeliveryValidity))
}

func QuotaValidityIssueError() *domain.AppError {
	return domain.NewError(
		http.StatusConflict,
		domain.CodeAPIQuotaValidityInsufficient,
		"Quota validity insufficient",
		"额度首次交付时剩余有效期必须不少于 60 分钟；本次未提交凭据，也未替换额度或恢复库存。",
	)
}

func ValidateDisputeResolutionForOrder(order Order, resolution, amount string) *domain.AppError {
	if appErr := ValidateRequestedDisputeAmount(resolution, amount, order.Amount); appErr != nil {
		return appErr
	}
	if resolution == DisputeResolutionContinueFulfillment && order.DeliverySubmittedAt != nil {
		return domain.NewError(
			http.StatusConflict,
			domain.CodeInvalidStateTransition,
			"Continue fulfillment unavailable",
			"订单已提交首次交付凭证，不能通过整改再次交付或替换凭证。",
		)
	}
	return nil
}
