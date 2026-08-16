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
			Body:            apiOrderNotificationBody(order, "买家已标记完成站外付款，请核对收款记录后确认。"),
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventCancelled:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "API 销售订单已取消",
			Body:            apiOrderNotificationBody(order, "买家在付款前取消了订单，请查看记录。"),
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventPaymentConfirmed:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "卖家已确认收款",
			Body:            apiOrderNotificationBody(order, "卖家已确认收到站外付款，接下来将准备交付。"),
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDeliveryDueReminder:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "API 订单交付即将截止",
			Body:            apiOrderNotificationBody(order, "交付截止时间将在 3 分钟内到达，请尽快提交买家专属接入信息。"),
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventPaymentIssueReported:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "付款信息需要补充",
			Body:            apiOrderNotificationBody(order, "商户核对后标记为“"+apiorder.PaymentIssueLabel(order.PaymentIssueReason)+"”，请补充付款说明并重新提交。"),
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDeliverySubmitted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "卖家已提交交付凭证",
			Body:            apiOrderNotificationBody(order, "卖家已提交买家专属接入信息，请在核验期内确认可用或反馈问题。"),
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDeliveryReviewReminder:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "交付凭证核验即将截止",
			Body:            apiOrderNotificationBody(order, "请尽快核验交付凭证；如未反馈问题，订单将在核验期结束后自动完成。"),
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventCompleted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "买家已确认订单完成",
			Body:            apiOrderNotificationBody(order, "买家已确认交付可用，该订单已完成。"),
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventAutoCompleted:
		return apiOrderNotificationSpec{
			RecipientUserID: order.SellerUserID,
			Title:           "订单已自动完成",
			Body:            apiOrderNotificationBody(order, "买家核验期已结束且未反馈问题，该订单已由系统自动完成。"),
			TargetURL:       sellerTarget,
		}, true
	case apiorder.EventPaymentTimeoutCancelled:
		return apiOrderNotificationSpec{
			RecipientUserID: order.BuyerUserID,
			Title:           "订单因付款超时已取消",
			Body:            apiOrderNotificationBody(order, "付款窗口已结束，该订单已自动取消。"),
			TargetURL:       buyerTarget,
		}, true
	case apiorder.EventDisputeOpened:
		spec := apiOrderNotificationSpec{
			Title: "订单已申请人工介入",
			Body:  apiOrderNotificationBody(order, "对方已申请人工介入，请查看订单状态。"),
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

func apiOrderNotificationBody(order apiorder.Order, body string) string {
	if strings.TrimSpace(order.OrderNo) == "" {
		return body
	}
	return "订单 " + order.OrderNo + "：" + body
}

func insertAPIOrderCatalogRiskNotificationInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, holdID, eventType string, now time.Time) *domain.AppError {
	title := "订单目录风险暂停已处置"
	body := "管理员已记录目录风险暂停处置结果，请查看订单状态。"
	if eventType == apiorder.EventCatalogRiskHoldCreated {
		title = "订单因目录风险暂停"
		body = "关联模型目录被紧急阻断，付款、核款、交付、确认完成和自动超时已暂停；仍可查看证据或发起纠纷。"
	}
	recipients := []struct {
		userID string
		url    string
	}{
		{userID: order.BuyerUserID, url: "/my/api-orders/" + order.ID},
		{userID: order.SellerUserID, url: "/merchant/api-orders/" + order.ID},
	}
	for _, recipient := range recipients {
		if strings.TrimSpace(recipient.userID) == "" {
			continue
		}
		dedupeKey := "api_order_catalog_risk_hold:" + holdID + ":" + eventType + ":" + recipient.userID
		if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (
				user_id, type, title, body, target_type, target_id, target_url,
				source_event_type, source_event_id, dedupe_key, created_at
			) VALUES ($1, 'api_order', $2, $3, 'api_order', $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		`, recipient.userID, title, apiOrderNotificationBody(order, body), order.ID, recipient.url,
			eventType, holdID, dedupeKey, now); err != nil {
			return internalStoreError()
		}
	}
	return nil
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
		"status":           order.Status,
		"disputeStatus":    order.DisputeStatus,
		"completionSource": order.CompletionSource,
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
	dedupeKey := "api_order:" + order.ID + ":v" + strconv.FormatInt(order.Version, 10) + ":" + eventType + ":" + spec.RecipientUserID
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
