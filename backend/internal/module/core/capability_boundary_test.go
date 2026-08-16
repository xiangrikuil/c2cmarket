package core

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiquota"
	authmodule "c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type capabilityBoundaryIdempotencyRepository struct {
	idempotency.Repository
	beginCalls int
}

func (r *capabilityBoundaryIdempotencyRepository) BeginIdempotency(context.Context, idempotency.Entry) (*idempotency.Entry, *domain.AppError) {
	r.beginCalls++
	return nil, domain.NewError(500, domain.CodeInternalError, "Unexpected idempotency begin", "能力校验不得进入幂等执行。")
}

func TestCapabilityProtectedFacadeMethodsRejectBeforeIdempotency(t *testing.T) {
	repository := &capabilityBoundaryIdempotencyRepository{}
	service := newService(
		func() time.Time { return time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC) },
		Repositories{Idempotency: repository},
	)
	ctx := context.Background()
	staleBuyer := testUser("stale-buyer", "stale-buyer")
	staleBuyer.Capabilities = []string{authmodule.CapabilityAPIOrderCreate}
	studentOwner := testStudentUser("student-owner", "student-owner")
	studentOwner.Capabilities = append([]string(nil), authmodule.AllCapabilities...)

	tests := []struct {
		name string
		run  func() *domain.AppError
	}{
		{
			name: "quota order create",
			run: func() *domain.AppError {
				_, appErr := service.CreateAPIQuotaOrderWithIdempotency(ctx, staleBuyer, "quota-order", "key", "hash", apiquota.CreateOrderInput{}, nil)
				return appErr
			},
		},
		{
			name: "purchase intent create",
			run: func() *domain.AppError {
				_, appErr := service.CreateAPIPurchaseIntentWithIdempotency(ctx, staleBuyer, "intent-create", "key", "hash", CreateAPIPurchaseIntentInput{}, nil)
				return appErr
			},
		},
		{
			name: "order create",
			run: func() *domain.AppError {
				_, appErr := service.CreateAPIOrderWithIdempotency(ctx, staleBuyer, "order-create", "key", "hash", APIOrderActionInput{}, CreateAPIOrderInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner intent mark contacted",
			run: func() *domain.AppError {
				_, appErr := service.MarkAPIPurchaseIntentContactedWithIdempotency(ctx, studentOwner, "intent-contacted", "key", "hash", APIPurchaseIntentActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner intent close",
			run: func() *domain.AppError {
				_, appErr := service.CloseAPIPurchaseIntentWithIdempotency(ctx, studentOwner, "intent-close", "key", "hash", APIPurchaseIntentActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner order confirm payment",
			run: func() *domain.AppError {
				_, appErr := service.ConfirmAPIOrderPaymentWithIdempotency(ctx, studentOwner, "order-confirm-payment", "key", "hash", APIOrderActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner order report payment issue",
			run: func() *domain.AppError {
				_, appErr := service.ReportAPIOrderPaymentIssueWithIdempotency(ctx, studentOwner, "order-payment-issue", "key", "hash", APIOrderActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner order submit delivery",
			run: func() *domain.AppError {
				_, appErr := service.SubmitAPIOrderDeliveryWithIdempotency(ctx, studentOwner, "order-delivery", "key", "hash", APIOrderActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner order dispute",
			run: func() *domain.AppError {
				_, appErr := service.OpenOwnerAPIOrderDisputeWithIdempotency(ctx, studentOwner, "order-dispute", "key", "hash", APIOrderActionInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner carpool accept",
			run: func() *domain.AppError {
				_, appErr := service.AcceptCarpoolApplicationWithIdempotency(ctx, studentOwner, "carpool-accept", "key", "hash", AcceptCarpoolApplicationInput{}, nil)
				return appErr
			},
		},
		{
			name: "owner carpool reject",
			run: func() *domain.AppError {
				_, appErr := service.RejectCarpoolApplication(ctx, studentOwner, RejectCarpoolApplicationInput{})
				return appErr
			},
		},
		{
			name: "owner carpool remove membership",
			run: func() *domain.AppError {
				_, appErr := service.EndCarpoolMembershipWithIdempotency(ctx, studentOwner, "carpool-remove", "key", "hash", EndCarpoolMembershipInput{ActorRole: CarpoolJoinActorOwner}, nil)
				return appErr
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := repository.beginCalls
			appErr := test.run()
			if appErr == nil || appErr.Code != domain.CodeCapabilityRequired {
				t.Fatalf("expected capability rejection, got %#v", appErr)
			}
			if repository.beginCalls != before {
				t.Fatalf("capability rejection started idempotency: before=%d after=%d", before, repository.beginCalls)
			}
		})
	}
}

func TestStudentOwnerWorkspacesRejectAtServiceBoundary(t *testing.T) {
	service := NewServiceWithClock(func() time.Time { return time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC) })
	student := testStudentUser("workspace-student", "workspace-student")
	ctx := context.Background()

	tests := []struct {
		name string
		run  func() *domain.AppError
	}{
		{name: "API intent workspace", run: func() *domain.AppError { _, appErr := service.OwnerAPIPurchaseIntents(ctx, student); return appErr }},
		{name: "API order workspace", run: func() *domain.AppError { _, appErr := service.OwnerAPIOrders(ctx, student); return appErr }},
		{name: "carpool application workspace", run: func() *domain.AppError { _, appErr := service.OwnerCarpoolApplications(ctx, student); return appErr }},
		{name: "carpool membership workspace", run: func() *domain.AppError { _, appErr := service.OwnerCarpoolMemberships(ctx, student); return appErr }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appErr := test.run()
			if appErr == nil || appErr.Code != domain.CodeCapabilityRequired {
				t.Fatalf("expected capability rejection, got %#v", appErr)
			}
		})
	}
}
