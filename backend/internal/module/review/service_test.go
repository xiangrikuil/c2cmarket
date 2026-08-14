package review

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

func TestTransactionReviewsRemainSealedUntilBothParticipantsSubmit(t *testing.T) {
	now := time.Date(2026, 7, 24, 4, 0, 0, 0, time.UTC)
	transaction := testReviewTransaction(TransactionCarpoolMembership, now.Add(-time.Hour))
	service := NewService(nil, nil, staticTransactionResolver{transactions: []Transaction{transaction}}, nil, func() time.Time { return now })

	buyerCompletion, appErr := service.SubmitWithIdempotency(context.Background(), transaction.BuyerUserID, "buyer-create", "buyer-key", "buyer-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationCreate,
		Rating:          5,
		Tags:            []string{"沟通顺畅", "描述真实"},
		Note:            "沟通清楚，交易过程符合说明。",
	}, testReviewCompletion)
	if appErr != nil {
		t.Fatalf("buyer submit failed: %v", appErr)
	}
	if buyerCompletion.Status != http.StatusCreated {
		t.Fatalf("expected created completion, got %d", buyerCompletion.Status)
	}

	sellerRows, appErr := service.ListMine(context.Background(), transaction.SellerUserID)
	if appErr != nil {
		t.Fatalf("seller review center failed: %v", appErr)
	}
	pending := findReviewRow(t, sellerRows, DirectionPending)
	if !pending.CanCreate {
		t.Fatalf("seller row should remain reviewable: %+v", pending)
	}
	for _, row := range sellerRows {
		if row.Direction == DirectionReceived {
			t.Fatalf("sealed counterparty submission must not be observable: %+v", row)
		}
	}

	_, appErr = service.SubmitWithIdempotency(context.Background(), transaction.SellerUserID, "seller-create", "seller-key", "seller-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationCreate,
		Rating:          4,
		Tags:            []string{"付款及时"},
		Note:            "确认及时，合作过程顺畅。",
	}, testReviewCompletion)
	if appErr != nil {
		t.Fatalf("seller submit failed: %v", appErr)
	}

	buyerRows, appErr := service.ListMine(context.Background(), transaction.BuyerUserID)
	if appErr != nil {
		t.Fatalf("buyer review center failed: %v", appErr)
	}
	buyerSent := findReviewRow(t, buyerRows, DirectionSent)
	buyerReceived := findReviewRow(t, buyerRows, DirectionReceived)
	if buyerSent.Status != StatusPublished || buyerSent.FrozenAt == nil || buyerSent.CanEdit {
		t.Fatalf("buyer review was not published and frozen: %+v", buyerSent)
	}
	if buyerReceived.Status != StatusPublished || !buyerReceived.ContentVisible || buyerReceived.Rating != 4 {
		t.Fatalf("seller review was not revealed after both submitted: %+v", buyerReceived)
	}

	_, appErr = service.SubmitWithIdempotency(context.Background(), transaction.BuyerUserID, "buyer-edit", "buyer-edit-key", "buyer-edit-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationEdit,
		Rating:          1,
		Tags:            []string{"响应较慢"},
		Note:            "不应允许公开后修改。",
	}, testReviewCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected frozen review rejection, got %#v", appErr)
	}
}

func TestReviewDeadlinePublishesExistingReviewAndRejectsLateSubmission(t *testing.T) {
	now := time.Date(2026, 7, 24, 5, 0, 0, 0, time.UTC)
	current := now
	transaction := testReviewTransaction(TransactionAPIOrder, now.Add(-time.Hour))
	service := NewService(nil, nil, staticTransactionResolver{transactions: []Transaction{transaction}}, nil, func() time.Time { return current })

	_, appErr := service.SubmitWithIdempotency(context.Background(), transaction.BuyerUserID, "api-create", "api-key", "api-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationCreate,
		Rating:          5,
		Tags:            []string{"交付清晰"},
		Note:            "交付说明清晰，确认过程顺畅。",
	}, testReviewCompletion)
	if appErr != nil {
		t.Fatalf("initial API review failed: %v", appErr)
	}

	current = transaction.ReviewDeadlineAt
	rows, appErr := service.ListMine(context.Background(), transaction.SellerUserID)
	if appErr != nil {
		t.Fatalf("seller review center failed: %v", appErr)
	}
	received := findReviewRow(t, rows, DirectionReceived)
	if received.Status != StatusPublished || received.FrozenAt == nil || !received.ContentVisible {
		t.Fatalf("deadline did not publish and freeze review: %+v", received)
	}

	_, appErr = service.SubmitWithIdempotency(context.Background(), transaction.BuyerUserID, "api-create", "api-key", "api-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationCreate,
		Rating:          5,
		Tags:            []string{"交付清晰"},
		Note:            "交付说明清晰，确认过程顺畅。",
	}, testReviewCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected expired idempotency replay to respect review deadline, got %#v", appErr)
	}

	_, appErr = service.SubmitWithIdempotency(context.Background(), transaction.SellerUserID, "api-late", "api-late-key", "api-late-hash", SubmitReviewInput{
		TransactionType: transaction.Type,
		TransactionID:   transaction.ID,
		Operation:       OperationCreate,
		Rating:          5,
		Tags:            []string{"合作愉快"},
		Note:            "这条评价已经超过截止时间。",
	}, testReviewCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected late submission rejection, got %#v", appErr)
	}
}

func TestReviewValidationRequiresPresetTags(t *testing.T) {
	appErr := ValidateSubmitInput(SubmitReviewInput{
		TransactionType: TransactionAPIOrder,
		TransactionID:   "30000000-0000-0000-0000-000000000001",
		ReviewerUserID:  "30000000-0000-0000-0000-000000000002",
		Operation:       OperationCreate,
		Rating:          5,
		Tags:            []string{"自定义标签"},
		Note:            "普通评价内容。",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected preset tag validation error, got %#v", appErr)
	}
}

func TestReviewValidationAllowsTagOrNoteAndRejectsEmptyContent(t *testing.T) {
	base := SubmitReviewInput{
		TransactionType: TransactionAPIOrder,
		TransactionID:   "30000000-0000-0000-0000-000000000001",
		ReviewerUserID:  "30000000-0000-0000-0000-000000000002",
		Operation:       OperationCreate,
		Rating:          5,
	}
	withTag := base
	withTag.Tags = []string{"smooth_comm"}
	if appErr := ValidateSubmitInput(withTag); appErr != nil {
		t.Fatalf("tag-only review should be valid: %v", appErr)
	}
	withNote := base
	withNote.Note = "沟通过程说明清楚。"
	if appErr := ValidateSubmitInput(withNote); appErr != nil {
		t.Fatalf("note-only review should be valid: %v", appErr)
	}
	if appErr := ValidateSubmitInput(base); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("empty review content should fail, got %#v", appErr)
	}
}

func TestReviewTagsAreScopedByDirectionAndTransaction(t *testing.T) {
	buyerTags := AllowedTags(TransactionAPIOrder, RoleBuyer, RoleSeller)
	sellerTags := AllowedTags(TransactionAPIOrder, RoleSeller, RoleBuyer)
	if !containsTagCode(buyerTags, "clear_delivery") || containsTagCode(buyerTags, "quick_payment") {
		t.Fatalf("buyer-to-seller tags are incorrect: %+v", buyerTags)
	}
	if !containsTagCode(sellerTags, "quick_payment") || containsTagCode(sellerTags, "clear_delivery") {
		t.Fatalf("seller-to-buyer tags are incorrect: %+v", sellerTags)
	}
	if ValidateTagsForScenario([]string{"quick_payment"}, TransactionAPIOrder, RoleBuyer, RoleSeller) {
		t.Fatal("buyer must not submit a seller-to-buyer payment tag")
	}
	labels := DisplayTagLabels([]string{"desc_diff", "与描述不符"})
	if len(labels) != 2 || labels[0] != "实际体验与描述有差异" || labels[1] != labels[0] {
		t.Fatalf("legacy tag aliases were not projected consistently: %+v", labels)
	}
}

func containsTagCode(items []TagDefinition, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func testReviewTransaction(transactionType string, completedAt time.Time) Transaction {
	return Transaction{
		Type:              transactionType,
		ID:                "30000000-0000-0000-0000-000000000001",
		Target:            "测试交易",
		BuyerUserID:       "30000000-0000-0000-0000-000000000002",
		BuyerUsername:     "buyer",
		BuyerDisplayName:  "买家",
		SellerUserID:      "30000000-0000-0000-0000-000000000003",
		SellerUsername:    "seller",
		SellerDisplayName: "卖家",
		CompletedAt:       completedAt,
		ReviewDeadlineAt:  completedAt.Add(ReviewWindow),
	}
}

func findReviewRow(t *testing.T, rows []ReviewCenterRow, direction string) ReviewCenterRow {
	t.Helper()
	for _, row := range rows {
		if row.Direction == direction {
			return row
		}
	}
	t.Fatalf("review row direction %s not found in %+v", direction, rows)
	return ReviewCenterRow{}
}

func testReviewCompletion(result MutationResult) (idempotency.Completion, *domain.AppError) {
	status := http.StatusOK
	if result.Row.Version == 1 {
		status = http.StatusCreated
	}
	return idempotency.Completion{
		Status:       status,
		ContentType:  "application/json",
		Body:         []byte(`{"ok":true}`),
		ResourceType: "review",
		ResourceID:   result.Row.ID,
	}, nil
}

type staticTransactionResolver struct {
	transactions []Transaction
}

func (r staticTransactionResolver) ReviewTransactionsByUserID(_ context.Context, userID string) ([]Transaction, *domain.AppError) {
	items := make([]Transaction, 0, len(r.transactions))
	for _, item := range r.transactions {
		if item.BuyerUserID == userID || item.SellerUserID == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r staticTransactionResolver) ResolveReviewTransaction(_ context.Context, transactionType, transactionID, userID string) (Transaction, *domain.AppError) {
	for _, item := range r.transactions {
		if item.Type == transactionType && item.ID == transactionID && (item.BuyerUserID == userID || item.SellerUserID == userID) {
			return item, nil
		}
	}
	return Transaction{}, reviewTransactionNotFound()
}
