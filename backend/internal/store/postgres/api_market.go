package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const apiServiceSupportedPaymentMethodsSQL = "'wechat', 'alipay'"

func (s *Store) CreateAPIService(ctx context.Context, service apimarket.Service) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, _, appErr := lockContactVersionForOwner(ctx, tx, service.OwnerContactMethodID, service.OwnerUserID, "商户联系方式不可用或不属于当前用户。"); appErr != nil {
		return appErr
	}
	if appErr := upsertAPIServiceInTx(ctx, tx, service); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListPublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter) ([]apimarket.Service, *domain.AppError) {
	where := `WHERE ` + publicAPIServiceOrderablePredicate("api_services")
	var args []any
	if strings.TrimSpace(filter.PaymentMethod) != "" {
		where += `
		  AND EXISTS (
		    SELECT 1
		    FROM api_service_payment_options po
		    WHERE po.api_service_id = api_services.id
		      AND po.enabled = true
		      AND po.payment_method = $1
		      AND po.payment_method IN (` + apiServiceSupportedPaymentMethodsSQL + `)
		  )
		`
		args = []any{strings.TrimSpace(filter.PaymentMethod)}
	}
	return s.listAPIServices(ctx, where, args)
}

func (s *Store) GetPublicAPIService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError) {
	service, err := s.getPublicAPIService(ctx, s.pool, serviceID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func publicAPIServiceOrderablePredicate(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "api_services"
	}
	return fmt.Sprintf(`%[1]s.review_status = 'approved'
		  AND %[1]s.publication_status = 'online'
		  AND %[1]s.moderation_status = 'clear'
		  AND %[1]s.accepting_orders = true
		  AND %[1]s.payment_window_minutes BETWEEN 3 AND 15
		  AND (%[1]s.billing_mode <> 'metered_usd_quota' OR %[1]s.quota_expires_at > now())
		  AND EXISTS (
		    SELECT 1
		    FROM api_service_payment_options po
		    WHERE po.api_service_id = %[1]s.id
		      AND po.enabled = true
		      AND po.payment_method IN (%[2]s)
		  )`, alias, apiServiceSupportedPaymentMethodsSQL)
}

func (s *Store) ListAPIServicesByOwner(ctx context.Context, ownerUserID string, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	return s.listAPIServicesPage(ctx, `WHERE owner_user_id = $1`, []any{ownerUserID}, page)
}

func (s *Store) GetAPIServiceForOwner(ctx context.Context, ownerUserID, serviceID string) (apimarket.Service, *domain.AppError) {
	service, err := s.getAPIService(ctx, s.pool, serviceID, false)
	if errors.Is(err, pgx.ErrNoRows) || service.OwnerUserID != ownerUserID {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) ListAdminAPIServices(ctx context.Context, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	return s.listAPIServicesPage(ctx, "", nil, page)
}

func (s *Store) GetAdminAPIService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError) {
	service, err := s.getAPIService(ctx, s.pool, serviceID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) UpdateAPIService(ctx context.Context, input apimarket.UpdateServiceInput, service apimarket.Service, now time.Time) (apimarket.Service, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apimarket.Service{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	current, err := s.getAPIService(ctx, tx, input.ServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) || current.OwnerUserID != input.OwnerUserID {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return apimarket.Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canEditAPIService(current) {
		return apimarket.Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能直接修改，请先开始修订。")
	}
	if _, _, appErr := lockContactVersionForOwner(ctx, tx, service.OwnerContactMethodID, service.OwnerUserID, "商户联系方式不可用或不属于当前用户。"); appErr != nil {
		return apimarket.Service{}, appErr
	}
	service.UpdatedAt = now
	service.Version = current.Version + 1
	service.AcceptingOrders = current.AcceptingOrders
	service.PaymentWindowMinutes = current.PaymentWindowMinutes
	service.PaymentOptions = current.PaymentOptions
	if appErr := upsertAPIServiceInTx(ctx, tx, service); appErr != nil {
		return apimarket.Service{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) UpdateAPIServiceOrderSettings(ctx context.Context, input apimarket.UpdateOrderSettingsInput, now time.Time) (apimarket.Service, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apimarket.Service{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	service, err := s.getAPIService(ctx, tx, input.ServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) || service.OwnerUserID != input.OwnerUserID {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return apimarket.Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if appErr := storeValidateAPIServiceOrderSettings(input); appErr != nil {
		return apimarket.Service{}, appErr
	}
	service.PaymentWindowMinutes = input.PaymentWindowMinutes
	service.PaymentOptions = storeBuildPaymentOptions(service.ID, service.PaymentOptions, input.PaymentOptions, now)
	service.AcceptingOrders = input.AcceptingOrders
	service.UpdatedAt = now
	service.Version++
	service = apimarket.WithOrderability(service)
	if input.AcceptingOrders && !service.IsOrderable {
		return apimarket.Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Service not orderable", "当前 API 服务不满足接单条件。", "acceptingOrders", "not_orderable", strings.Join(service.OrderableReasons, "；"))
	}
	if appErr := updateAPIServiceOrderSettingsInTx(ctx, tx, service); appErr != nil {
		return apimarket.Service{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) SubmitAPIServiceForReview(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput, now time.Time) (apimarket.Service, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	service, err := s.getAPIService(ctx, tx, input.ServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) || service.OwnerUserID != input.OwnerUserID {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return apimarket.Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if service.ReviewStatus != apimarket.ServiceReviewStatusDraft && service.ReviewStatus != apimarket.ServiceReviewStatusChangesRequested {
		return apimarket.Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能提交审核。")
	}
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound {
		return apimarket.Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "linux.do binding required", "提交 API 服务前需要完成 linux.do 身份绑定。", "linuxDoBinding", "required", "需要先完成 linux.do 身份绑定。")
	}
	if _, _, appErr := lockContactVersionForOwner(ctx, tx, service.OwnerContactMethodID, service.OwnerUserID, "商户联系方式不可用或不属于当前用户。"); appErr != nil {
		return apimarket.Service{}, appErr
	}
	service.ReviewStatus = apimarket.ServiceReviewStatusApproved
	service.PublicationStatus = apimarket.ServicePublicationStatusOffline
	service.ApprovedByAdminID = ""
	service.ApprovedAt = &now
	service.UpdatedAt = now
	service.Version++
	service = apimarket.WithOrderability(service)
	if appErr := updateAPIServiceStateInTx(ctx, tx, service); appErr != nil {
		return apimarket.Service{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) UpdateAPIServicePublication(ctx context.Context, input apimarket.ServiceOwnerActionInput, action string, now time.Time) (apimarket.Service, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	service, err := s.getAPIService(ctx, tx, input.ServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) || service.OwnerUserID != input.OwnerUserID {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return apimarket.Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateAPIServicePublication(service, action) {
		return apimarket.Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能执行该操作。")
	}
	if action == "publish" || action == "resume" {
		if strings.TrimSpace(service.OwnerContactMethodID) == "" {
			return apimarket.Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeMerchantContactRequired, "Merchant contact required", "上线 API 服务必须配置商户联系方式。")
		}
		if _, _, appErr := lockContactVersionForOwner(ctx, tx, service.OwnerContactMethodID, service.OwnerUserID, "商户联系方式当前不可用。"); appErr != nil {
			return apimarket.Service{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Merchant contact unavailable", "商户联系方式当前不可用。")
		}
	}
	service = applyAPIServicePublicationAction(service, action, now)
	if appErr := updateAPIServiceStateInTx(ctx, tx, service); appErr != nil {
		return apimarket.Service{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) UpdateAPIServiceModeration(ctx context.Context, user auth.User, input apimarket.ServiceAdminActionInput, now time.Time) (apimarket.Service, *domain.AppError) {
	if !user.IsAdmin {
		return apimarket.Service{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	service, err := s.getAPIService(ctx, tx, input.ServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apimarket.Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return apimarket.Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateAPIServiceAdminStatus(service, input.Action) {
		return apimarket.Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能执行该管理动作。")
	}
	service = applyAPIServiceAdminAction(service, input, now)
	if appErr := updateAPIServiceStateInTx(ctx, tx, service); appErr != nil {
		return apimarket.Service{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apimarket.Service{}, internalStoreError()
	}
	return service, nil
}

func (s *Store) CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiintent.CreateIntentInput, now time.Time, buildCompletion apiintent.CompletionBuilder) (apiintent.Intent, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	intent, appErr := s.createAPIPurchaseIntentInTx(ctx, tx, input, now)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(intent)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	return intent, completion, nil
}

func (s *Store) ListAPIPurchaseIntentsByBuyer(ctx context.Context, buyerUserID string, now time.Time) ([]apiintent.Intent, *domain.AppError) {
	return s.listAPIPurchaseIntents(ctx, `WHERE buyer_user_id = $1`, []any{buyerUserID})
}

func (s *Store) GetAPIPurchaseIntentForBuyer(ctx context.Context, buyerUserID, intentID string, now time.Time) (apiintent.Intent, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, s.pool, intentID, false)
	if errors.Is(err, pgx.ErrNoRows) || intent.BuyerUserID != buyerUserID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	return intent, nil
}

func (s *Store) GetAPIPurchaseIntentForBuyerWithMerchantContact(ctx context.Context, buyerUserID, intentID, requestID string, now time.Time) (apiintent.Intent, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	intent, err := s.getAPIPurchaseIntent(ctx, tx, intentID, false)
	if errors.Is(err, pgx.ErrNoRows) || intent.BuyerUserID != buyerUserID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	intent, appErr := s.withAPIPurchaseIntentMerchantContact(ctx, tx, intent)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if appErr := insertAPIPurchaseIntentContactAccessLogInTx(ctx, tx, intent.ID, buyerUserID, "merchant", requestID, now); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	return intent, nil
}

func (s *Store) ListAPIPurchaseIntentsByOwner(ctx context.Context, ownerUserID string, now time.Time) ([]apiintent.Intent, *domain.AppError) {
	return s.listAPIPurchaseIntents(ctx, `WHERE owner_user_id = $1`, []any{ownerUserID})
}

func (s *Store) GetAPIPurchaseIntentForOwner(ctx context.Context, ownerUserID, intentID string, now time.Time) (apiintent.Intent, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, s.pool, intentID, false)
	if errors.Is(err, pgx.ErrNoRows) || intent.OwnerUserID != ownerUserID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	return intent, nil
}

func (s *Store) GetAPIPurchaseIntentForOwnerWithBuyerContact(ctx context.Context, ownerUserID, intentID, requestID string, now time.Time) (apiintent.Intent, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	intent, err := s.getAPIPurchaseIntent(ctx, tx, intentID, false)
	if errors.Is(err, pgx.ErrNoRows) || intent.OwnerUserID != ownerUserID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	intent, appErr := s.withAPIPurchaseIntentBuyerContact(ctx, tx, intent)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if appErr := insertAPIPurchaseIntentContactAccessLogInTx(ctx, tx, intent.ID, ownerUserID, "buyer", requestID, now); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	return intent, nil
}

func (s *Store) ListAdminAPIPurchaseIntents(ctx context.Context, now time.Time) ([]apiintent.Intent, *domain.AppError) {
	return s.listAPIPurchaseIntents(ctx, "", nil)
}

func (s *Store) GetAdminAPIPurchaseIntent(ctx context.Context, intentID string, now time.Time) (apiintent.Intent, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, s.pool, intentID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	return intent, nil
}

func (s *Store) CancelAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiintent.ActionInput, now time.Time, buildCompletion apiintent.CompletionBuilder) (apiintent.Intent, idempotency.Completion, *domain.AppError) {
	return s.updateAPIPurchaseIntentWithIdempotency(ctx, entry, input, now, buildCompletion, "cancel")
}

func (s *Store) MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiintent.ActionInput, now time.Time, buildCompletion apiintent.CompletionBuilder) (apiintent.Intent, idempotency.Completion, *domain.AppError) {
	return s.updateAPIPurchaseIntentWithIdempotency(ctx, entry, input, now, buildCompletion, "mark_contacted")
}

func (s *Store) CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiintent.ActionInput, now time.Time, buildCompletion apiintent.CompletionBuilder) (apiintent.Intent, idempotency.Completion, *domain.AppError) {
	return s.updateAPIPurchaseIntentWithIdempotency(ctx, entry, input, now, buildCompletion, "close")
}

func (s *Store) updateAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiintent.ActionInput, now time.Time, buildCompletion apiintent.CompletionBuilder, action string) (apiintent.Intent, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	intent, appErr := s.updateAPIPurchaseIntentInTx(ctx, tx, input, now, action)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(intent)
	if appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiintent.Intent{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiintent.Intent{}, idempotency.Completion{}, internalStoreError()
	}
	return intent, completion, nil
}

const apiServiceColumns = `
	id::text, owner_user_id::text, COALESCE(merchant_profile_id::text, ''), merchant_identity_mode,
	COALESCE((SELECT mp.display_name FROM merchant_profiles mp WHERE mp.id = api_services.merchant_profile_id AND mp.owner_user_id = api_services.owner_user_id), ''),
	COALESCE((SELECT mp.slug FROM merchant_profiles mp WHERE mp.id = api_services.merchant_profile_id AND mp.owner_user_id = api_services.owner_user_id), ''),
	owner_contact_method_id::text, title, short_description, COALESCE(source_url, ''), distribution_system, billing_mode,
	COALESCE(declared_cny_per_usd_allowance::text, ''), COALESCE(declared_max_usd_allowance_per_intent::text, ''),
	quota_expires_at,
	minimum_intent_cny::text, COALESCE(maximum_intent_cny::text, ''), usage_visibility,
	COALESCE(public_access_note, ''), COALESCE(merchant_note, ''), COALESCE(merchant_support_note, ''),
	accepting_orders, payment_window_minutes,
	review_status, publication_status, moderation_status, COALESCE(approved_by_admin_id::text, ''),
	approved_at, COALESCE(moderation_reason, ''), created_at, updated_at, version
`

const apiPurchaseIntentColumns = `
	id::text, api_service_id::text, api_service_owner_user_id::text, buyer_user_id::text,
	owner_user_id::text, buyer_contact_method_id::text, buyer_contact_method_version_id::text,
	owner_contact_method_id::text, owner_contact_method_version_id::text, status,
	requested_cny_amount::text, COALESCE(requested_usd_allowance::text, ''),
	selected_access_mode, COALESCE(selected_package_id::text, ''), COALESCE(selected_package_snapshot::text, ''),
	service_version_snapshot, service_title_snapshot, distribution_system_snapshot, billing_mode_snapshot,
	buyer_contact_type_snapshot, buyer_contact_label_snapshot,
	owner_contact_type_snapshot, owner_contact_label_snapshot,
	COALESCE(declared_cny_per_usd_allowance_snapshot::text, ''),
	COALESCE(declared_max_usd_allowance_per_intent_snapshot::text, ''),
	minimum_intent_cny_snapshot::text, COALESCE(maximum_intent_cny_snapshot::text, ''),
	pricing_snapshot::text, COALESCE(buyer_note, ''),
	contacted_at, buyer_cancelled_at, COALESCE(buyer_cancel_reason, ''),
	owner_closed_at, COALESCE(owner_close_reason, ''),
	created_at, updated_at, version
`

func (s *Store) listAPIServices(ctx context.Context, whereClause string, args []any) ([]apimarket.Service, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	query := `SELECT ` + apiServiceColumns + ` FROM api_services `
	if strings.TrimSpace(whereClause) != "" {
		query += whereClause
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	services, appErr := scanAPIServices(rows)
	if appErr != nil {
		return nil, appErr
	}
	for i := range services {
		if appErr := s.loadAPIServiceChildren(ctx, s.pool, &services[i]); appErr != nil {
			return nil, appErr
		}
	}
	return services, nil
}

func (s *Store) listAPIServicesPage(ctx context.Context, whereClause string, args []any, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[apimarket.Service]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[apimarket.Service]{}, appErr
	}
	query := `SELECT ` + apiServiceColumns + ` FROM api_services `
	whereClause = strings.TrimSpace(whereClause)
	if whereClause != "" {
		query += whereClause
	}
	args = append([]any(nil), args...)
	if page.Cursor != "" {
		if whereClause == "" {
			query += `WHERE `
		} else {
			query += ` AND `
		}
		args = append(args, position.Time, position.ID)
		query += `(updated_at, id) < ($` + strconv.Itoa(len(args)-1) + `, $` + strconv.Itoa(len(args)) + `::uuid)`
	}
	args = append(args, page.Limit+1)
	query += ` ORDER BY updated_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[apimarket.Service]{}, internalStoreError()
	}
	defer rows.Close()
	services, appErr := scanAPIServices(rows)
	if appErr != nil {
		return domain.Page[apimarket.Service]{}, appErr
	}
	for i := range services {
		if appErr := s.loadAPIServiceChildren(ctx, s.pool, &services[i]); appErr != nil {
			return domain.Page[apimarket.Service]{}, appErr
		}
	}
	return pageFromItems(services, page, func(item apimarket.Service) (time.Time, string) { return item.UpdatedAt, item.ID }), nil
}

func (s *Store) getAPIService(ctx context.Context, q queryer, serviceID string, forUpdate bool) (apimarket.Service, error) {
	if forUpdate {
		var id string
		if err := q.QueryRow(ctx, `SELECT id::text FROM api_services WHERE id = $1 FOR UPDATE`, serviceID).Scan(&id); err != nil {
			return apimarket.Service{}, err
		}
	}
	var service apimarket.Service
	err := scanAPIService(q.QueryRow(ctx, `SELECT `+apiServiceColumns+` FROM api_services WHERE id = $1`, serviceID), &service)
	if err != nil {
		return apimarket.Service{}, err
	}
	if appErr := s.loadAPIServiceChildren(ctx, q, &service); appErr != nil {
		return apimarket.Service{}, errors.New(appErr.Error())
	}
	return service, nil
}

func (s *Store) getPublicAPIService(ctx context.Context, q queryer, serviceID string, forUpdate bool) (apimarket.Service, error) {
	if forUpdate {
		var id string
		if err := q.QueryRow(ctx, `SELECT id::text FROM api_services WHERE id = $1 AND `+publicAPIServiceOrderablePredicate("api_services")+` FOR UPDATE`, serviceID).Scan(&id); err != nil {
			return apimarket.Service{}, err
		}
	}
	var service apimarket.Service
	err := scanAPIService(q.QueryRow(ctx, `SELECT `+apiServiceColumns+` FROM api_services WHERE id = $1 AND `+publicAPIServiceOrderablePredicate("api_services"), serviceID), &service)
	if err != nil {
		return apimarket.Service{}, err
	}
	if appErr := s.loadAPIServiceChildren(ctx, q, &service); appErr != nil {
		return apimarket.Service{}, errors.New(appErr.Error())
	}
	return service, nil
}

func (s *Store) loadAPIServiceChildren(ctx context.Context, q queryer, service *apimarket.Service) *domain.AppError {
	accessRows, err := queryRows(ctx, q, `
		SELECT api_service_id::text, access_mode, COALESCE(public_note, '')
		FROM api_service_access_modes
		WHERE api_service_id = $1
		ORDER BY access_mode ASC
	`, service.ID)
	if err != nil {
		return internalStoreError()
	}
	defer accessRows.Close()
	service.AccessModes = nil
	for accessRows.Next() {
		var mode apimarket.ServiceAccessMode
		if err := accessRows.Scan(&mode.APIServiceID, &mode.AccessMode, &mode.PublicNote); err != nil {
			return internalStoreError()
		}
		service.AccessModes = append(service.AccessModes, mode)
	}
	if err := accessRows.Err(); err != nil {
		return internalStoreError()
	}

	modelRows, err := queryRows(ctx, q, `
		SELECT id::text, api_service_id::text, distribution_system, model_catalog_id::text,
		       COALESCE(model_price_version_id::text, ''), model_name_snapshot, provider_snapshot,
		       capabilities_snapshot, merchant_multiplier::text,
		       COALESCE(effective_input_price_per_million::text, ''),
		       COALESCE(effective_cached_input_price_per_million::text, ''),
		       COALESCE(effective_output_price_per_million::text, ''),
		       enabled, created_at, updated_at
		FROM api_service_models
		WHERE api_service_id = $1
		ORDER BY created_at ASC
	`, service.ID)
	if err != nil {
		return internalStoreError()
	}
	defer modelRows.Close()
	service.Models = nil
	for modelRows.Next() {
		var model apimarket.ServiceModel
		if err := modelRows.Scan(
			&model.ID,
			&model.APIServiceID,
			&model.DistributionSystem,
			&model.ModelCatalogID,
			&model.ModelPriceVersionID,
			&model.ModelNameSnapshot,
			&model.ProviderSnapshot,
			&model.CapabilitiesSnapshot,
			&model.MerchantMultiplier,
			&model.EffectiveInputPricePerMillion,
			&model.EffectiveCachedInputPricePerMillion,
			&model.EffectiveOutputPricePerMillion,
			&model.Enabled,
			&model.CreatedAt,
			&model.UpdatedAt,
		); err != nil {
			return internalStoreError()
		}
		service.Models = append(service.Models, model)
	}
	if err := modelRows.Err(); err != nil {
		return internalStoreError()
	}

	packageRows, err := queryRows(ctx, q, `
		SELECT id::text, api_service_id::text, name, price_cny::text, duration_days,
		       description, enabled, sort_order, created_at, updated_at
		FROM api_service_packages
		WHERE api_service_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, service.ID)
	if err != nil {
		return internalStoreError()
	}
	defer packageRows.Close()
	service.Packages = nil
	for packageRows.Next() {
		var pack apimarket.ServicePackage
		if err := packageRows.Scan(
			&pack.ID,
			&pack.APIServiceID,
			&pack.Name,
			&pack.PriceCNY,
			&pack.DurationDays,
			&pack.Description,
			&pack.Enabled,
			&pack.SortOrder,
			&pack.CreatedAt,
			&pack.UpdatedAt,
		); err != nil {
			return internalStoreError()
		}
		service.Packages = append(service.Packages, pack)
	}
	if err := packageRows.Err(); err != nil {
		return internalStoreError()
	}

	paymentRows, err := queryRows(ctx, q, `
		SELECT id::text, api_service_id::text, payment_method, enabled,
		       payment_instructions, COALESCE(payment_qr_code_data_url, ''),
		       created_at, updated_at, version
		FROM api_service_payment_options
		WHERE api_service_id = $1
		ORDER BY payment_method ASC
	`, service.ID)
	if err != nil {
		return internalStoreError()
	}
	defer paymentRows.Close()
	service.PaymentOptions = nil
	for paymentRows.Next() {
		var option apimarket.PaymentOption
		if err := paymentRows.Scan(
			&option.ID,
			&option.APIServiceID,
			&option.PaymentMethod,
			&option.Enabled,
			&option.PaymentInstructions,
			&option.PaymentQRCodeDataURL,
			&option.CreatedAt,
			&option.UpdatedAt,
			&option.Version,
		); err != nil {
			return internalStoreError()
		}
		service.PaymentOptions = append(service.PaymentOptions, option)
	}
	if err := paymentRows.Err(); err != nil {
		return internalStoreError()
	}
	*service = apimarket.WithOrderability(*service)
	return nil
}

func (s *Store) listAPIPurchaseIntents(ctx context.Context, whereClause string, args []any) ([]apiintent.Intent, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	query := `SELECT ` + apiPurchaseIntentColumns + ` FROM api_purchase_intents `
	if strings.TrimSpace(whereClause) != "" {
		query += whereClause
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAPIPurchaseIntents(rows)
}

func (s *Store) getAPIPurchaseIntent(ctx context.Context, q queryer, intentID string, forUpdate bool) (apiintent.Intent, error) {
	query := `SELECT ` + apiPurchaseIntentColumns + ` FROM api_purchase_intents WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var intent apiintent.Intent
	err := scanAPIPurchaseIntent(q.QueryRow(ctx, query, intentID), &intent)
	return intent, err
}

func (s *Store) createAPIPurchaseIntentInTx(ctx context.Context, tx pgx.Tx, input apiintent.CreateIntentInput, now time.Time) (apiintent.Intent, *domain.AppError) {
	service, err := s.getPublicAPIService(ctx, tx, input.APIServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	if validateErr := validateCreateAPIPurchaseIntentForStore(input, service); validateErr != nil {
		return apiintent.Intent{}, validateErr
	}

	buyerMethod, buyerVersion, appErr := lockContactVersionForOwner(ctx, tx, input.BuyerContactMethodID, input.BuyerUserID, "买家联系方式不可用或不属于当前用户。")
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	ownerMethod, ownerVersion, appErr := lockContactVersionForOwner(ctx, tx, service.OwnerContactMethodID, service.OwnerUserID, "商户联系方式不可用或归属不正确。")
	if appErr != nil {
		return apiintent.Intent{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Merchant contact unavailable", "商户联系方式当前不可用。")
	}

	intent, appErr := apiintent.NewIntent(input, service, buyerMethod, buyerVersion, ownerMethod, ownerVersion, now)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if appErr := insertAPIPurchaseIntentInTx(ctx, tx, intent); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if appErr := insertAPIPurchaseIntentEventAndTargetNotification(ctx, tx, intent, intent.BuyerUserID, intent.OwnerUserID, "api_purchase_intent.created", "收到新的购买意向", "买家已提交购买意向，请查看详情并站外联系。", input.RequestID, now); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	intent, appErr = s.withAPIPurchaseIntentMerchantContact(ctx, tx, intent)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	if appErr := insertAPIPurchaseIntentContactAccessLogInTx(ctx, tx, intent.ID, intent.BuyerUserID, "merchant", input.RequestID, now); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	return intent, nil
}

func (s *Store) updateAPIPurchaseIntentInTx(ctx context.Context, tx pgx.Tx, input apiintent.ActionInput, now time.Time, action string) (apiintent.Intent, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, tx, input.IntentID, true)
	if errors.Is(err, pgx.ErrNoRows) || !storeCanActorAccessAPIPurchaseIntent(intent, input.ActorUserID, action) {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && intent.Version != input.ExpectedVersion {
		return apiintent.Intent{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !storeCanUpdateAPIPurchaseIntentStatus(intent, action, now) {
		return apiintent.Intent{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能执行该操作。")
	}
	if action == "cancel" || action == "close" {
		if appErr := ensureNoAPIOrderForIntent(ctx, tx, intent.ID); appErr != nil {
			return apiintent.Intent{}, appErr
		}
	}
	intent = storeApplyAPIPurchaseIntentAction(intent, action, strings.TrimSpace(input.Reason), now)
	_, err = tx.Exec(ctx, `
		UPDATE api_purchase_intents
		SET status = $2,
		    contacted_at = $3,
		    buyer_cancelled_at = $4,
		    buyer_cancel_reason = $5,
		    owner_closed_at = $6,
		    owner_close_reason = $7,
		    updated_at = $8,
		    version = $9
		WHERE id = $1
	`, intent.ID, intent.Status, intent.ContactedAt, intent.BuyerCancelledAt, nullText(intent.BuyerCancelReason),
		intent.OwnerClosedAt, nullText(intent.OwnerCloseReason), intent.UpdatedAt, intent.Version)
	if err != nil {
		return apiintent.Intent{}, internalStoreError()
	}
	eventType := "api_purchase_intent.contacted"
	title := "购买意向已标记联系"
	body := "商户已标记完成联系，请查看购买意向。"
	notifyUserID := intent.BuyerUserID
	if action == "cancel" {
		eventType = "api_purchase_intent.buyer_cancelled"
		title = "购买意向已取消"
		body = "买家已取消购买意向。"
		notifyUserID = intent.OwnerUserID
	} else if action == "close" {
		eventType = "api_purchase_intent.owner_closed"
		title = "购买意向已关闭"
		body = "商户已关闭购买意向，请查看详情。"
	}
	if appErr := insertAPIPurchaseIntentEventAndTargetNotification(ctx, tx, intent, input.ActorUserID, notifyUserID, eventType, title, body, input.RequestID, now); appErr != nil {
		return apiintent.Intent{}, appErr
	}
	return intent, nil
}

func upsertAPIServiceInTx(ctx context.Context, tx pgx.Tx, service apimarket.Service) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_services (
			id, owner_user_id, merchant_profile_id, merchant_identity_mode, owner_contact_method_id,
			title, short_description, source_url, distribution_system, billing_mode,
			declared_cny_per_usd_allowance, declared_max_usd_allowance_per_intent,
			quota_expires_at,
			minimum_intent_cny, maximum_intent_cny, usage_visibility,
			public_access_note, merchant_note, merchant_support_note,
			review_status, publication_status, moderation_status,
			approved_by_admin_id, approved_at, moderation_reason,
			accepting_orders, payment_window_minutes,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12,
			$13,
			$14, $15, $16,
			$17, $18, $19,
			$20, $21, $22,
			$23, $24, $25,
			$26, $27,
			$28, $29, $30
		)
		ON CONFLICT (id) DO UPDATE
		SET merchant_profile_id = EXCLUDED.merchant_profile_id,
		    merchant_identity_mode = EXCLUDED.merchant_identity_mode,
		    owner_contact_method_id = EXCLUDED.owner_contact_method_id,
		    title = EXCLUDED.title,
		    short_description = EXCLUDED.short_description,
		    source_url = EXCLUDED.source_url,
		    distribution_system = EXCLUDED.distribution_system,
		    billing_mode = EXCLUDED.billing_mode,
		    declared_cny_per_usd_allowance = EXCLUDED.declared_cny_per_usd_allowance,
		    declared_max_usd_allowance_per_intent = EXCLUDED.declared_max_usd_allowance_per_intent,
		    quota_expires_at = EXCLUDED.quota_expires_at,
		    minimum_intent_cny = EXCLUDED.minimum_intent_cny,
		    maximum_intent_cny = EXCLUDED.maximum_intent_cny,
		    usage_visibility = EXCLUDED.usage_visibility,
		    public_access_note = EXCLUDED.public_access_note,
		    merchant_note = EXCLUDED.merchant_note,
		    merchant_support_note = EXCLUDED.merchant_support_note,
		    review_status = EXCLUDED.review_status,
		    publication_status = EXCLUDED.publication_status,
		    moderation_status = EXCLUDED.moderation_status,
		    approved_by_admin_id = EXCLUDED.approved_by_admin_id,
		    approved_at = EXCLUDED.approved_at,
		    moderation_reason = EXCLUDED.moderation_reason,
		    accepting_orders = EXCLUDED.accepting_orders,
		    payment_window_minutes = EXCLUDED.payment_window_minutes,
		    updated_at = EXCLUDED.updated_at,
		    version = EXCLUDED.version
		`, service.ID, service.OwnerUserID, nullUUID(service.MerchantProfileID), service.MerchantIdentityMode, service.OwnerContactMethodID,
		service.Title, service.ShortDescription, nullText(service.SourceURL), service.DistributionSystem, service.BillingMode,
		nullNumeric(service.DeclaredCNYPerUSDAllowance), nullNumeric(service.DeclaredMaxUSDAllowancePerIntent),
		service.QuotaExpiresAt,
		service.MinimumIntentCNY, nullNumeric(service.MaximumIntentCNY), service.UsageVisibility,
		nullText(service.PublicAccessNote), nullText(service.MerchantNote), nullText(service.MerchantSupportNote),
		service.ReviewStatus, service.PublicationStatus, service.ModerationStatus,
		nullUUID(service.ApprovedByAdminID), service.ApprovedAt, nullText(service.ModerationReason),
		service.AcceptingOrders, service.PaymentWindowMinutes,
		service.CreatedAt, service.UpdatedAt, service.Version)
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_access_modes WHERE api_service_id = $1`, service.ID); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_models WHERE api_service_id = $1`, service.ID); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_packages WHERE api_service_id = $1`, service.ID); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_payment_options WHERE api_service_id = $1`, service.ID); err != nil {
		return internalStoreError()
	}
	for _, mode := range service.AccessModes {
		_, err = tx.Exec(ctx, `
			INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note)
			VALUES ($1, $2, $3)
		`, service.ID, mode.AccessMode, nullText(mode.PublicNote))
		if err != nil {
			return internalStoreError()
		}
	}
	for _, model := range service.Models {
		_, err = tx.Exec(ctx, `
			INSERT INTO api_service_models (
				id, api_service_id, distribution_system, model_catalog_id, model_price_version_id,
				model_name_snapshot, provider_snapshot, capabilities_snapshot, merchant_multiplier,
				effective_input_price_per_million, effective_cached_input_price_per_million,
				effective_output_price_per_million, enabled, created_at, updated_at
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $11,
				$12, $13, $14, $15
			)
		`, model.ID, service.ID, service.DistributionSystem, model.ModelCatalogID, nullUUID(model.ModelPriceVersionID),
			model.ModelNameSnapshot, model.ProviderSnapshot, model.CapabilitiesSnapshot, model.MerchantMultiplier,
			nullNumeric(model.EffectiveInputPricePerMillion), nullNumeric(model.EffectiveCachedInputPricePerMillion),
			nullNumeric(model.EffectiveOutputPricePerMillion), model.Enabled, model.CreatedAt, model.UpdatedAt)
		if err != nil {
			return internalStoreError()
		}
	}
	for _, pack := range service.Packages {
		_, err = tx.Exec(ctx, `
			INSERT INTO api_service_packages (
				id, api_service_id, name, price_cny, duration_days, description,
				enabled, sort_order, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, pack.ID, service.ID, pack.Name, pack.PriceCNY, pack.DurationDays, pack.Description,
			pack.Enabled, pack.SortOrder, pack.CreatedAt, pack.UpdatedAt)
		if err != nil {
			return internalStoreError()
		}
	}
	for _, option := range service.PaymentOptions {
		_, err = tx.Exec(ctx, `
			INSERT INTO api_service_payment_options (
				id, api_service_id, payment_method, enabled, payment_instructions,
				payment_qr_code_data_url, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, option.ID, service.ID, option.PaymentMethod, option.Enabled, option.PaymentInstructions,
			nullText(option.PaymentQRCodeDataURL), option.CreatedAt, option.UpdatedAt, option.Version)
		if err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func updateAPIServiceStateInTx(ctx context.Context, tx pgx.Tx, service apimarket.Service) *domain.AppError {
	_, err := tx.Exec(ctx, `
		UPDATE api_services
		SET review_status = $2,
		    publication_status = $3,
		    moderation_status = $4,
		    approved_by_admin_id = $5,
		    approved_at = $6,
		    moderation_reason = $7,
		    accepting_orders = $8,
		    updated_at = $9,
		    version = $10
		WHERE id = $1
	`, service.ID, service.ReviewStatus, service.PublicationStatus, service.ModerationStatus,
		nullUUID(service.ApprovedByAdminID), service.ApprovedAt, nullText(service.ModerationReason),
		service.AcceptingOrders, service.UpdatedAt, service.Version)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func updateAPIServiceOrderSettingsInTx(ctx context.Context, tx pgx.Tx, service apimarket.Service) *domain.AppError {
	_, err := tx.Exec(ctx, `
		UPDATE api_services
		SET accepting_orders = $2,
		    payment_window_minutes = $3,
		    updated_at = $4,
		    version = $5
		WHERE id = $1
	`, service.ID, service.AcceptingOrders, service.PaymentWindowMinutes, service.UpdatedAt, service.Version)
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_payment_options WHERE api_service_id = $1`, service.ID); err != nil {
		return internalStoreError()
	}
	for _, option := range service.PaymentOptions {
		_, err = tx.Exec(ctx, `
			INSERT INTO api_service_payment_options (
				id, api_service_id, payment_method, enabled, payment_instructions,
				payment_qr_code_data_url, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, option.ID, service.ID, option.PaymentMethod, option.Enabled, option.PaymentInstructions,
			nullText(option.PaymentQRCodeDataURL), option.CreatedAt, option.UpdatedAt, option.Version)
		if err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func insertAPIPurchaseIntentInTx(ctx context.Context, tx pgx.Tx, intent apiintent.Intent) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_purchase_intents (
			id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
			buyer_contact_method_id, buyer_contact_method_version_id,
			owner_contact_method_id, owner_contact_method_version_id,
			status, requested_cny_amount, requested_usd_allowance, selected_access_mode, selected_package_id,
			selected_package_snapshot, service_version_snapshot, service_title_snapshot,
			distribution_system_snapshot, billing_mode_snapshot,
			buyer_contact_type_snapshot, buyer_contact_label_snapshot,
			owner_contact_type_snapshot, owner_contact_label_snapshot,
			declared_cny_per_usd_allowance_snapshot,
			declared_max_usd_allowance_per_intent_snapshot,
			minimum_intent_cny_snapshot, maximum_intent_cny_snapshot,
			pricing_snapshot, buyer_note, created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9,
			$10, $11, $12, $13, $14,
			$15, $16, $17,
			$18, $19,
			$20, $21,
			$22, $23,
			$24,
			$25,
			$26, $27,
			$28, $29, $30, $31, $32
		)
	`, intent.ID, intent.APIServiceID, intent.APIServiceOwnerUserID, intent.BuyerUserID, intent.OwnerUserID,
		intent.BuyerContactMethodID, intent.BuyerContactMethodVersionID,
		intent.OwnerContactMethodID, intent.OwnerContactMethodVersionID,
		intent.Status, intent.RequestedCNYAmount, nullNumeric(intent.RequestedUSDAllowance), intent.SelectedAccessMode, nullUUID(intent.SelectedPackageID),
		nullJSON(intent.SelectedPackageSnapshot), intent.ServiceVersionSnapshot, intent.ServiceTitleSnapshot,
		intent.DistributionSystemSnapshot, intent.BillingModeSnapshot,
		intent.BuyerContactTypeSnapshot, intent.BuyerContactLabelSnapshot,
		intent.OwnerContactTypeSnapshot, intent.OwnerContactLabelSnapshot,
		nullNumeric(intent.DeclaredCNYPerUSDAllowanceSnapshot),
		nullNumeric(intent.DeclaredMaxUSDAllowancePerIntentSnapshot),
		intent.MinimumIntentCNYSnapshot, nullNumeric(intent.MaximumIntentCNYSnapshot),
		json.RawMessage(intent.PricingSnapshot), nullText(intent.BuyerNote),
		intent.CreatedAt, intent.UpdatedAt, intent.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.NewError(http.StatusConflict, domain.CodeActiveAPIIntentExists, "Active API intent exists", "你已对该服务提交过进行中的购买意向。")
		}
		return internalStoreError()
	}
	return nil
}

func insertAPIPurchaseIntentEventAndTargetNotification(ctx context.Context, tx pgx.Tx, intent apiintent.Intent, actorUserID, notifyUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	eventID := uuid.NewString()
	metadata, err := json.Marshal(map[string]string{
		"apiServiceId": intent.APIServiceID,
		"status":       intent.Status,
	})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'api_purchase_intent', $2, $3, $4, 'user', $5, $6, $7, $8)
	`, eventID, intent.ID, eventType, actorUserID, intent.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, $2, $3, $4, 'api_purchase_intent', $5, $6, $2, $7, $8, $9)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, notifyUserID, eventType, title, body, intent.ID, apiPurchaseIntentNotificationTargetURL(intent, notifyUserID), eventID,
		"api_purchase_intent:"+intent.ID+":"+intent.Status+":"+notifyUserID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func apiPurchaseIntentNotificationTargetURL(intent apiintent.Intent, notifyUserID string) string {
	if notifyUserID == intent.OwnerUserID {
		return "/merchant/api-orders/" + intent.ID
	}
	return "/my/api-orders/" + intent.ID
}

func insertAPIPurchaseIntentContactAccessLogInTx(ctx context.Context, tx pgx.Tx, intentID, viewerUserID, viewedSide, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_purchase_intent_contact_access_logs (
			id, api_purchase_intent_id, viewer_user_id, viewed_contact_owner_side,
			request_id, accessed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.NewString(), intentID, viewerUserID, viewedSide, requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) withAPIPurchaseIntentMerchantContact(ctx context.Context, q queryer, intent apiintent.Intent) (apiintent.Intent, *domain.AppError) {
	item, appErr := s.readFrozenContactVersion(ctx, q, intent.OwnerContactMethodVersionID, intent.OwnerContactMethodID, intent.OwnerUserID, "merchant", intent.OwnerContactTypeSnapshot, intent.OwnerContactLabelSnapshot)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	intent.MerchantContact = &item
	return intent, nil
}

func (s *Store) withAPIPurchaseIntentBuyerContact(ctx context.Context, q queryer, intent apiintent.Intent) (apiintent.Intent, *domain.AppError) {
	item, appErr := s.readFrozenContactVersion(ctx, q, intent.BuyerContactMethodVersionID, intent.BuyerContactMethodID, intent.BuyerUserID, "buyer", intent.BuyerContactTypeSnapshot, intent.BuyerContactLabelSnapshot)
	if appErr != nil {
		return apiintent.Intent{}, appErr
	}
	intent.BuyerContact = &item
	return intent, nil
}

func (s *Store) readFrozenContactVersion(ctx context.Context, q queryer, versionID, methodID, ownerID, side, typeSnapshot, labelSnapshot string) (contact.ContactItemView, *domain.AppError) {
	if s == nil || s.contactCodec == nil {
		return contact.ContactItemView{}, internalStoreError()
	}
	var item contact.ContactItemView
	var ciphertext, nonce []byte
	err := q.QueryRow(ctx, `
		SELECT v.value_ciphertext, v.value_nonce, v.masked_value
		FROM contact_method_versions v
		WHERE v.id = $1 AND v.contact_method_id = $2 AND v.owner_user_id = $3
	`, versionID, methodID, ownerID).Scan(&ciphertext, &nonce, &item.MaskedValue)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactItemView{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Contact unavailable", "冻结联系方式不可用。")
	}
	if err != nil {
		return contact.ContactItemView{}, internalStoreError()
	}
	value, err := s.contactCodec.decode(ciphertext, nonce)
	if err != nil {
		return contact.ContactItemView{}, internalStoreError()
	}
	item.Side = side
	item.SubjectID = ownerID
	item.Type = typeSnapshot
	item.Label = labelSnapshot
	item.Value = value
	return item, nil
}
func canEditAPIService(service apimarket.Service) bool {
	return service.ReviewStatus == apimarket.ServiceReviewStatusDraft || service.ReviewStatus == apimarket.ServiceReviewStatusChangesRequested
}

func canUpdateAPIServicePublication(service apimarket.Service, action string) bool {
	switch action {
	case "publish":
		return service.ReviewStatus == apimarket.ServiceReviewStatusApproved &&
			service.PublicationStatus == apimarket.ServicePublicationStatusOffline &&
			service.ModerationStatus == apimarket.ServiceModerationStatusClear
	case "pause":
		return service.PublicationStatus == apimarket.ServicePublicationStatusOnline
	case "resume":
		return service.ReviewStatus == apimarket.ServiceReviewStatusApproved &&
			service.PublicationStatus == apimarket.ServicePublicationStatusOwnerPaused &&
			service.ModerationStatus == apimarket.ServiceModerationStatusClear
	case "start_revision":
		return service.PublicationStatus == apimarket.ServicePublicationStatusOnline ||
			service.PublicationStatus == apimarket.ServicePublicationStatusOwnerPaused
	default:
		return false
	}
}

func applyAPIServicePublicationAction(service apimarket.Service, action string, now time.Time) apimarket.Service {
	switch action {
	case "publish", "resume":
		service.PublicationStatus = apimarket.ServicePublicationStatusOnline
	case "pause":
		service.PublicationStatus = apimarket.ServicePublicationStatusOwnerPaused
	case "start_revision":
		service.PublicationStatus = apimarket.ServicePublicationStatusOffline
		service.ReviewStatus = apimarket.ServiceReviewStatusChangesRequested
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
	}
	service.UpdatedAt = now
	service.Version++
	return service
}

func canUpdateAPIServiceAdminStatus(service apimarket.Service, action string) bool {
	switch action {
	case "approve":
		return service.ReviewStatus == apimarket.ServiceReviewStatusPendingReview &&
			service.ModerationStatus == apimarket.ServiceModerationStatusClear
	case "request_changes":
		return service.ReviewStatus == apimarket.ServiceReviewStatusPendingReview
	case "reject":
		return service.ReviewStatus == apimarket.ServiceReviewStatusPendingReview
	case "suspend":
		return service.ModerationStatus == apimarket.ServiceModerationStatusClear
	case "restore":
		return service.ModerationStatus == apimarket.ServiceModerationStatusAdminSuspended
	case "remove":
		return service.ModerationStatus == apimarket.ServiceModerationStatusClear ||
			service.ModerationStatus == apimarket.ServiceModerationStatusAdminSuspended
	default:
		return false
	}
}

func applyAPIServiceAdminAction(service apimarket.Service, input apimarket.ServiceAdminActionInput, now time.Time) apimarket.Service {
	switch input.Action {
	case "approve":
		service.ReviewStatus = apimarket.ServiceReviewStatusApproved
		service.PublicationStatus = apimarket.ServicePublicationStatusOffline
		service.ApprovedByAdminID = input.AdminUserID
		service.ApprovedAt = &now
	case "request_changes":
		service.ReviewStatus = apimarket.ServiceReviewStatusChangesRequested
		service.PublicationStatus = apimarket.ServicePublicationStatusOffline
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
	case "reject":
		service.ReviewStatus = apimarket.ServiceReviewStatusRejected
		service.PublicationStatus = apimarket.ServicePublicationStatusOffline
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
	case "suspend":
		service.ModerationStatus = apimarket.ServiceModerationStatusAdminSuspended
		service.ModerationReason = strings.TrimSpace(input.Reason)
	case "restore":
		service.ModerationStatus = apimarket.ServiceModerationStatusClear
		service.ModerationReason = strings.TrimSpace(input.Reason)
	case "remove":
		service.ModerationStatus = apimarket.ServiceModerationStatusRemoved
		service.PublicationStatus = apimarket.ServicePublicationStatusArchived
		service.ModerationReason = strings.TrimSpace(input.Reason)
	}
	if input.Action == "approve" || input.Action == "request_changes" || input.Action == "reject" {
		service.ModerationReason = strings.TrimSpace(input.Reason)
	}
	service.UpdatedAt = now
	service.Version++
	return service
}

func validateCreateAPIPurchaseIntentForStore(input apiintent.CreateIntentInput, service apimarket.Service) *domain.AppError {
	if strings.TrimSpace(input.BuyerContactMethodID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "提交购买意向必须选择联系方式。", "buyerContactMethodId", "required", "必须选择联系方式。")
	}
	if input.BuyerUserID == service.OwnerUserID {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "不能向自己的 API 服务提交购买意向。")
	}
	amount, ok := storeParsePositiveDecimal(input.RequestedCNYAmount)
	if !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount invalid", "意向金额格式不正确。", "requestedCnyAmount", "invalid", "意向金额必须为正数。")
	}
	if minimum, ok := storeParsePositiveDecimal(service.MinimumIntentCNY); ok && amount.Cmp(minimum) < 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount too low", "意向金额不能低于服务最低金额。", "requestedCnyAmount", "too_low", "意向金额不能低于服务最低金额。")
	}
	if strings.TrimSpace(service.MaximumIntentCNY) != "" {
		maximum, ok := storeParsePositiveDecimal(service.MaximumIntentCNY)
		if !ok || amount.Cmp(maximum) > 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount too high", "意向金额不能高于服务最高金额。", "requestedCnyAmount", "too_high", "意向金额不能高于服务最高金额。")
		}
	}
	if err := storeValidateOptionalNonSecretText("buyerNote", input.BuyerNote); err != nil {
		return err
	}
	if strings.TrimSpace(input.SelectedAccessMode) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode required", "必须选择接入方式。", "selectedAccessMode", "required", "必须选择接入方式。")
	}
	if !storeAPIServiceHasAccessMode(service, input.SelectedAccessMode) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode invalid", "选择的接入方式不属于当前服务。", "selectedAccessMode", "invalid", "选择的接入方式不可用。")
	}
	switch service.BillingMode {
	case apimarket.ServiceBillingModeMetered:
		allowance, ok := storeParsePositiveDecimal(input.RequestedUSDAllowance)
		if !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance required", "美元额度服务必须填写意向美元额度。", "requestedUsdAllowance", "required", "必须填写意向美元额度。")
		}
		if strings.TrimSpace(service.DeclaredMaxUSDAllowancePerIntent) != "" {
			maxAllowance, ok := storeParsePositiveDecimal(service.DeclaredMaxUSDAllowancePerIntent)
			if !ok || allowance.Cmp(maxAllowance) > 0 {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance too high", "意向美元额度不能超过商户声明上限。", "requestedUsdAllowance", "too_high", "意向美元额度不能超过商户声明上限。")
			}
		}
		rate, ok := storeParsePositiveDecimal(service.DeclaredCNYPerUSDAllowance)
		if !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance price invalid", "美元额度售价不可用。", "requestedUsdAllowance", "invalid", "美元额度售价不可用。")
		}
		expectedAmount := new(big.Rat).Mul(allowance, rate)
		if storeDecimalString(expectedAmount, 2) != storeDecimalString(amount, 2) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount mismatch", "意向金额必须等于美元额度乘以商户声明单价。", "requestedCnyAmount", "mismatch", "意向金额必须匹配意向美元额度。")
		}
		if strings.TrimSpace(input.SelectedPackageID) != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package not allowed", "美元额度服务不能选择固定套餐。", "selectedPackageId", "not_allowed", "该服务不使用固定套餐。")
		}
	case apimarket.ServiceBillingModeFixedPackage:
		if strings.TrimSpace(input.SelectedPackageID) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package required", "固定套餐服务必须选择套餐。", "selectedPackageId", "required", "必须选择套餐。")
		}
		pack, ok := storeFindAPIServicePackage(service, input.SelectedPackageID)
		if !ok || !pack.Enabled {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "选择的套餐不可用。", "selectedPackageId", "invalid", "选择的套餐不可用。")
		}
		packPrice, ok := storeParsePositiveDecimal(pack.PriceCNY)
		if !ok || amount.Cmp(packPrice) != 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount mismatch", "意向金额必须等于所选套餐价格。", "requestedCnyAmount", "mismatch", "意向金额必须等于所选套餐价格。")
		}
		if strings.TrimSpace(input.RequestedUSDAllowance) != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance not allowed", "固定套餐服务不能填写美元额度。", "requestedUsdAllowance", "not_allowed", "该服务不使用美元额度。")
		}
	case apimarket.ServiceBillingModeManual:
		if strings.TrimSpace(input.RequestedUSDAllowance) != "" {
			if _, ok := storeParsePositiveDecimal(input.RequestedUSDAllowance); !ok {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance invalid", "意向美元额度格式不正确。", "requestedUsdAllowance", "invalid", "意向美元额度必须为正数。")
			}
		}
	default:
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务计费方式不可提交意向。")
	}
	return nil
}

func storeValidateAPIServiceOrderSettings(input apimarket.UpdateOrderSettingsInput) *domain.AppError {
	if input.PaymentWindowMinutes < 3 || input.PaymentWindowMinutes > 15 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment window invalid", "付款窗口必须在 3 到 15 分钟之间。", "paymentWindowMinutes", "range", "付款窗口必须在 3 到 15 分钟之间。")
	}
	if len(input.PaymentOptions) == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment option required", "至少配置一种收款方式。", "paymentOptions", "required", "至少配置一种收款方式。")
	}
	seen := map[string]bool{}
	enabledCount := 0
	for i, option := range input.PaymentOptions {
		field := fmt.Sprintf("paymentOptions.%d", i)
		method := strings.TrimSpace(option.PaymentMethod)
		if !storeIsSupportedPaymentMethod(method) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "付款方式不支持。", field+".paymentMethod", "invalid", "付款方式不支持。")
		}
		if seen[method] {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method duplicated", "付款方式不能重复。", field+".paymentMethod", "duplicate", "付款方式不能重复。")
		}
		seen[method] = true
		if option.Enabled {
			enabledCount++
			if storeRequiresPaymentQRCode(method) && strings.TrimSpace(option.PaymentQRCodeDataURL) == "" {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment QR code required", "启用微信或支付宝收款必须上传收款码。", field+".paymentQrCodeDataUrl", "required", "必须上传收款码。")
			}
			if !storeRequiresPaymentQRCode(method) && strings.TrimSpace(option.PaymentInstructions) == "" {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment instructions required", "启用收款方式必须填写收款说明。", field+".paymentInstructions", "required", "必须填写收款说明。")
			}
		}
		if err := storeValidateOptionalNonSecretText(field+".paymentInstructions", option.PaymentInstructions); err != nil {
			return err
		}
		if err := storeValidateOptionalPaymentQRCodeDataURL(field+".paymentQrCodeDataUrl", option.PaymentQRCodeDataURL); err != nil {
			return err
		}
	}
	if input.AcceptingOrders && enabledCount == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method required", "开启接单前至少启用一种收款方式。", "paymentOptions", "required", "至少启用一种收款方式。")
	}
	return nil
}

func storeIsSupportedPaymentMethod(method string) bool {
	return apimarket.IsSupportedPaymentMethod(method)
}

func storeBuildPaymentOptions(serviceID string, current []apimarket.PaymentOption, input []apimarket.PaymentOptionInput, now time.Time) []apimarket.PaymentOption {
	byMethod := map[string]apimarket.PaymentOption{}
	for _, option := range current {
		byMethod[option.PaymentMethod] = option
	}
	options := make([]apimarket.PaymentOption, 0, len(input))
	for _, item := range input {
		if !storeShouldPersistPaymentOption(item) {
			continue
		}
		method := strings.TrimSpace(item.PaymentMethod)
		option := byMethod[method]
		if option.ID == "" {
			option.ID = uuid.NewString()
			option.APIServiceID = serviceID
			option.PaymentMethod = method
			option.CreatedAt = now
			option.Version = 1
		} else {
			option.Version++
		}
		option.APIServiceID = serviceID
		option.PaymentMethod = method
		option.Enabled = item.Enabled
		option.PaymentInstructions = strings.TrimSpace(item.PaymentInstructions)
		option.PaymentQRCodeDataURL = strings.TrimSpace(item.PaymentQRCodeDataURL)
		option.UpdatedAt = now
		options = append(options, option)
	}
	return options
}

func storeShouldPersistPaymentOption(input apimarket.PaymentOptionInput) bool {
	return input.Enabled || strings.TrimSpace(input.PaymentInstructions) != "" || strings.TrimSpace(input.PaymentQRCodeDataURL) != ""
}

func storeRequiresPaymentQRCode(method string) bool {
	switch strings.TrimSpace(method) {
	case apimarket.PaymentMethodWechat, apimarket.PaymentMethodAlipay:
		return true
	default:
		return false
	}
}

func storeValidateOptionalPaymentQRCodeDataURL(field, value string) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 2*1024*1024 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code too large", "收款码图片过大。", field, "too_large", "收款码图片过大。")
	}
	if strings.ContainsAny(value, "\x00\r\n\t") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code invalid", "收款码数据格式不正确。", field, "invalid", "收款码数据格式不正确。")
	}
	if !strings.HasPrefix(value, "data:image/") || !strings.Contains(value, ";base64,") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code invalid", "收款码必须是图片 data URL。", field, "invalid", "收款码必须是图片 data URL。")
	}
	return nil
}

func storeFindAPIServicePackage(service apimarket.Service, packageID string) (apimarket.ServicePackage, bool) {
	packageID = strings.TrimSpace(packageID)
	for _, pack := range service.Packages {
		if pack.ID == packageID {
			return pack, true
		}
	}
	return apimarket.ServicePackage{}, false
}

func storeAPIServiceHasAccessMode(service apimarket.Service, accessMode string) bool {
	accessMode = strings.TrimSpace(accessMode)
	if accessMode == "" {
		return false
	}
	for _, mode := range service.AccessModes {
		if strings.TrimSpace(mode.AccessMode) == accessMode {
			return true
		}
	}
	return false
}

func storeCanActorAccessAPIPurchaseIntent(intent apiintent.Intent, actorUserID, action string) bool {
	switch action {
	case "cancel":
		return intent.BuyerUserID == actorUserID
	case "mark_contacted", "close":
		return intent.OwnerUserID == actorUserID
	default:
		return false
	}
}

func storeCanUpdateAPIPurchaseIntentStatus(intent apiintent.Intent, action string, now time.Time) bool {
	switch action {
	case "cancel":
		return intent.Status == apiintent.StatusOpen || intent.Status == apiintent.StatusContacted
	case "mark_contacted":
		return intent.Status == apiintent.StatusOpen
	case "close":
		return intent.Status == apiintent.StatusOpen || intent.Status == apiintent.StatusContacted
	default:
		return false
	}
}

func storeApplyAPIPurchaseIntentAction(intent apiintent.Intent, action, reason string, now time.Time) apiintent.Intent {
	switch action {
	case "cancel":
		intent.Status = apiintent.StatusBuyerCancelled
		intent.BuyerCancelledAt = &now
		intent.BuyerCancelReason = strings.TrimSpace(reason)
	case "mark_contacted":
		intent.Status = apiintent.StatusContacted
		intent.ContactedAt = &now
	case "close":
		intent.Status = apiintent.StatusOwnerClosed
		intent.OwnerClosedAt = &now
		intent.OwnerCloseReason = strings.TrimSpace(reason)
	}
	intent.UpdatedAt = now
	intent.Version++
	return intent
}

func scanAPIServices(rows pgx.Rows) ([]apimarket.Service, *domain.AppError) {
	services := []apimarket.Service{}
	for rows.Next() {
		var service apimarket.Service
		if err := scanAPIService(rows, &service); err != nil {
			return nil, internalStoreError()
		}
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return services, nil
}

func scanAPIService(row scanner, service *apimarket.Service) error {
	return row.Scan(
		&service.ID,
		&service.OwnerUserID,
		&service.MerchantProfileID,
		&service.MerchantIdentityMode,
		&service.MerchantDisplayName,
		&service.MerchantProfileSlug,
		&service.OwnerContactMethodID,
		&service.Title,
		&service.ShortDescription,
		&service.SourceURL,
		&service.DistributionSystem,
		&service.BillingMode,
		&service.DeclaredCNYPerUSDAllowance,
		&service.DeclaredMaxUSDAllowancePerIntent,
		&service.QuotaExpiresAt,
		&service.MinimumIntentCNY,
		&service.MaximumIntentCNY,
		&service.UsageVisibility,
		&service.PublicAccessNote,
		&service.MerchantNote,
		&service.MerchantSupportNote,
		&service.AcceptingOrders,
		&service.PaymentWindowMinutes,
		&service.ReviewStatus,
		&service.PublicationStatus,
		&service.ModerationStatus,
		&service.ApprovedByAdminID,
		&service.ApprovedAt,
		&service.ModerationReason,
		&service.CreatedAt,
		&service.UpdatedAt,
		&service.Version,
	)
}

func scanAPIPurchaseIntents(rows pgx.Rows) ([]apiintent.Intent, *domain.AppError) {
	intents := []apiintent.Intent{}
	for rows.Next() {
		var intent apiintent.Intent
		if err := scanAPIPurchaseIntent(rows, &intent); err != nil {
			return nil, internalStoreError()
		}
		intents = append(intents, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return intents, nil
}

func scanAPIPurchaseIntent(row scanner, intent *apiintent.Intent) error {
	return row.Scan(
		&intent.ID,
		&intent.APIServiceID,
		&intent.APIServiceOwnerUserID,
		&intent.BuyerUserID,
		&intent.OwnerUserID,
		&intent.BuyerContactMethodID,
		&intent.BuyerContactMethodVersionID,
		&intent.OwnerContactMethodID,
		&intent.OwnerContactMethodVersionID,
		&intent.Status,
		&intent.RequestedCNYAmount,
		&intent.RequestedUSDAllowance,
		&intent.SelectedAccessMode,
		&intent.SelectedPackageID,
		&intent.SelectedPackageSnapshot,
		&intent.ServiceVersionSnapshot,
		&intent.ServiceTitleSnapshot,
		&intent.DistributionSystemSnapshot,
		&intent.BillingModeSnapshot,
		&intent.BuyerContactTypeSnapshot,
		&intent.BuyerContactLabelSnapshot,
		&intent.OwnerContactTypeSnapshot,
		&intent.OwnerContactLabelSnapshot,
		&intent.DeclaredCNYPerUSDAllowanceSnapshot,
		&intent.DeclaredMaxUSDAllowancePerIntentSnapshot,
		&intent.MinimumIntentCNYSnapshot,
		&intent.MaximumIntentCNYSnapshot,
		&intent.PricingSnapshot,
		&intent.BuyerNote,
		&intent.ContactedAt,
		&intent.BuyerCancelledAt,
		&intent.BuyerCancelReason,
		&intent.OwnerClosedAt,
		&intent.OwnerCloseReason,
		&intent.CreatedAt,
		&intent.UpdatedAt,
		&intent.Version,
	)
}
