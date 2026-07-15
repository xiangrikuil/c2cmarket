package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type apiOrderNotificationSpec struct {
	RecipientUserID string
	Title           string
	Body            string
	TargetURL       string
}

func apiOrderNotificationFor(order apiorder.Order, actorUserID, eventType string) (apiOrderNotificationSpec, bool) {
	buyerTarget := "/my/api-orders/" + order.ID
	sellerTarget := "/merchant/api-orders/" + order.ID
	switch eventType {
	case apiorder.EventPaymentSubmitted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "买家已标记付款",
			Body:            "买家已标记完成站外付款，请核对收款记录后确认。",
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventCancelled:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "买家已取消订单",
			Body:            "买家在付款前取消了订单，请查看记录。",
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventPaymentConfirmed:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "卖家已确认收款",
			Body:            "卖家已确认收到站外付款，接下来将准备交付。",
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventPaymentIssueReported:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "付款信息需要补充",
			Body:            "商户核对后标记为“" + apiorder.PaymentIssueLabel(order.PaymentIssueReason) + "”，请补充付款说明并重新提交。",
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDeliverySubmitted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "卖家已提交交付凭证",
			Body:            "卖家已提交买家专属接入信息，请进入订单详情核对并确认完成。",
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventCompleted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "买家已确认订单完成",
			Body:            "买家已确认交付可用，该订单已完成。",
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventPaymentTimeoutCancelled:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "订单因付款超时已取消",
			Body:            "付款窗口已结束，该订单已自动取消。",
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDisputeOpened:
		spec := apiOrderNotificationSpec{
			Title: "订单已申请人工介入",
			Body:  "对方已申请人工介入，请查看订单状态。",
		}
		switch actorUserID {
		case order.BuyerUserID:
			spec.RecipientUserID = order.SellerUserID
			spec.TargetURL = sellerTarget
		case order.SellerUserID:
			spec.RecipientUserID = order.BuyerUserID
			spec.TargetURL = buyerTarget
		default:
			return apiOrderNotificationSpec{}, false
		}
		return spec, true
	default:
		return apiOrderNotificationSpec{}, false
	}
}

func insertAPIOrderDomainEventAndNotificationInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, actorUserID, eventType, requestID string, now time.Time) *domain.AppError {
	spec, ok := apiOrderNotificationFor(order, actorUserID, eventType)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	actorKind := "user"
	if strings.TrimSpace(actorUserID) == "" {
		actorKind = "system"
	}
	metadata, err := json.Marshal(map[string]string{
		"status":        order.Status,
		"disputeStatus": order.DisputeStatus,
	})
	if err != nil {
		return internalStoreError()
	}
	eventID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'api_order', $2, $3, $4, $5, $6, $7, $8, $9)
	`, eventID, order.ID, eventType, nullUUID(actorUserID), actorKind, order.Version, requestID, metadata, now); err != nil {
		return internalStoreError()
	}
	if !ok || strings.TrimSpace(spec.RecipientUserID) == "" {
		return nil
	}
	dedupeKey := "api_order:" + order.ID + ":v" + strconv.FormatInt(order.Version, 10) + ":" + spec.RecipientUserID
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, 'api_order', $2, $3, 'api_order', $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, spec.RecipientUserID, spec.Title, spec.Body, order.ID, spec.TargetURL, eventType, eventID, dedupeKey, now); err != nil {
		return internalStoreError()
	}
	return nil
}
