package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func (s *Store) CreateCarpoolListing(ctx context.Context, listing carpool.Listing, ack *carpool.RiskAcknowledgement) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	if appErr := insertCarpoolListingInTx(ctx, tx, listing, ack); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) PublishCarpoolListing(ctx context.Context, listing carpool.Listing, ack *carpool.RiskAcknowledgement, now time.Time) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	if _, _, appErr := lockContactVersionForOwner(ctx, tx, listing.OwnerContactMethodID, listing.OwnerUserID, "车主联系方式不可用或不属于当前用户。"); appErr != nil {
		return carpool.Listing{}, appErr
	}
	if appErr := ensureCarpoolPlanAllowedForPublish(ctx, tx, listing.ProductPlanID); appErr != nil {
		return carpool.Listing{}, appErr
	}
	listing.Status = carpool.ListingStatusActive
	listing.CreatedAt = now
	listing.UpdatedAt = now
	if listing.CycleTerm != nil {
		listing.CycleTerm.CreatedAt = now
		listing.CycleTerm.UpdatedAt = now
	}
	if appErr := insertCarpoolListingInTx(ctx, tx, listing, ack); appErr != nil {
		return carpool.Listing{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func insertCarpoolListingInTx(ctx context.Context, tx pgx.Tx, listing carpool.Listing, ack *carpool.RiskAcknowledgement) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO carpool_listings (
			id, owner_user_id, product_plan_id, owner_contact_method_id, title, summary, access_arrangement,
			source_url, price_monthly_cny, service_multiplier, monthly_quota_amount, quota_label, quota_unit, quota_period,
			buyer_seat_capacity, active_buyer_members,
			status, policy_version, risk_notice_code, risk_ack_required,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14,
			$15, $16,
			$17, $18, $19, $20,
			$21, $22, $23
		)
	`, listing.ID, listing.OwnerUserID, listing.ProductPlanID, listing.OwnerContactMethodID, listing.Title, listing.Summary, listing.AccessArrangement,
		nullText(listing.SourceURL), listing.PriceMonthlyCNY, listing.ServiceMultiplier, listing.MonthlyQuotaAmount, listing.QuotaLabel, listing.QuotaUnit, listing.QuotaPeriod,
		listing.BuyerSeatCapacity, listing.ActiveBuyerMembers,
		listing.Status, listing.PolicyVersion, nullText(listing.RiskNoticeCode), listing.RiskAckRequired,
		listing.CreatedAt, listing.UpdatedAt, listing.Version)
	if err != nil {
		return internalStoreError()
	}
	if listing.CycleTerm != nil {
		listing.CycleTerm.CarpoolListingID = listing.ID
		listing.CycleTerm.OwnerUserID = listing.OwnerUserID
		if appErr := upsertCarpoolCycleTermInTx(ctx, tx, *listing.CycleTerm, listing.UpdatedAt); appErr != nil {
			return appErr
		}
	}
	if ack != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO carpool_listing_policy_acknowledgements (
				carpool_listing_id, user_id, risk_notice_code, policy_version, risk_notice_version_id, acknowledged_at
			)
			SELECT $1, $2, $3, $4::bigint, version.id, $5
			FROM risk_notices notice
			JOIN risk_notice_versions version ON version.risk_notice_id = notice.id
			WHERE notice.code = $3 AND version.version::bigint = $4::bigint
		`, listing.ID, listing.OwnerUserID, ack.RiskNoticeCode, ack.PolicyVersion, ack.AcknowledgedAt)
		if err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func (s *Store) ListPublicCarpoolListings(ctx context.Context, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT `+carpoolListingColumns+`
			FROM `+carpoolListingViewSource+`
			WHERE status = 'active'
			ORDER BY updated_at DESC, id DESC
			LIMIT $1
		`, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT `+carpoolListingColumns+`
			FROM `+carpoolListingViewSource+`
			WHERE status = 'active'
			  AND (updated_at, id) < ($1, $2::uuid)
			ORDER BY updated_at DESC, id DESC
			LIMIT $3
		`, position.Time, position.ID, limit)
	}
	if err != nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	defer rows.Close()
	listings, appErr := scanCarpoolListings(rows)
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	return pageFromItems(listings, page, func(item carpool.Listing) (time.Time, string) { return item.UpdatedAt, item.ID }), nil
}

func (s *Store) GetPublicCarpoolListing(ctx context.Context, listingID string) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	listing, err := s.getCarpoolListing(ctx, s.pool, listingID, false, false)
	if errors.Is(err, pgx.ErrNoRows) || listing.Status != carpool.ListingStatusActive {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) ListCarpoolListingsByOwner(ctx context.Context, ownerUserID string) ([]carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+carpoolListingColumns+`
		FROM `+carpoolListingViewSource+`
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolListings(rows)
}

func (s *Store) ListAdminCarpoolListings(ctx context.Context, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		rows, err = s.pool.Query(ctx, `
			SELECT `+carpoolListingColumns+`
			FROM `+carpoolListingViewSource+`
			ORDER BY updated_at DESC, id DESC
			LIMIT $1
		`, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT `+carpoolListingColumns+`
			FROM `+carpoolListingViewSource+`
			WHERE (updated_at, id) < ($1, $2::uuid)
			ORDER BY updated_at DESC, id DESC
			LIMIT $3
		`, position.Time, position.ID, limit)
	}
	if err != nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	defer rows.Close()
	listings, appErr := scanCarpoolListings(rows)
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	return pageFromItems(listings, page, func(item carpool.Listing) (time.Time, string) { return item.UpdatedAt, item.ID }), nil
}

func (s *Store) GetAdminCarpoolListing(ctx context.Context, listingID string) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	listing, err := s.getCarpoolListing(ctx, s.pool, listingID, false, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) UpdateCarpoolListing(ctx context.Context, input carpool.UpdateListingInput, ack *carpool.RiskAcknowledgement, now time.Time) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	listing, err := s.getCarpoolListing(ctx, tx, input.ListingID, true, true)
	if errors.Is(err, pgx.ErrNoRows) || listing.OwnerUserID != input.OwnerUserID {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return carpool.Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if listing.Status != carpool.ListingStatusDraft && listing.Status != carpool.ListingStatusChangesRequested {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能修改。")
	}
	if _, _, appErr := lockContactVersionForOwner(ctx, tx, input.OwnerContactMethodID, input.OwnerUserID, "车主联系方式不可用或不属于当前用户。"); appErr != nil {
		return carpool.Listing{}, appErr
	}
	var planPolicyVersion int64
	var planRiskNoticeCode string
	var planRiskAckRequired bool
	var planQuotaLabel string
	var planQuotaUnit string
	var planQuotaPeriod string
	err = tx.QueryRow(ctx, `
		SELECT policy_version, COALESCE(risk_notice_code, ''), risk_ack_required, quota_label, quota_unit, quota_period
		FROM product_plans
		WHERE id = $1 AND active = true
	`, input.ProductPlanID).Scan(&planPolicyVersion, &planRiskNoticeCode, &planRiskAckRequired, &planQuotaLabel, &planQuotaUnit, &planQuotaPeriod)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}

	listing.ProductPlanID = strings.TrimSpace(input.ProductPlanID)
	listing.OwnerContactMethodID = strings.TrimSpace(input.OwnerContactMethodID)
	if listing.CycleTerm == nil {
		listing.CycleTerm = &carpool.CycleTerm{
			ID:               uuid.NewString(),
			CarpoolListingID: listing.ID,
			OwnerUserID:      listing.OwnerUserID,
			Version:          1,
			CreatedAt:        now,
		}
	}
	listing.CycleTerm.CarpoolListingID = listing.ID
	listing.CycleTerm.OwnerUserID = listing.OwnerUserID
	listing.CycleTerm.BillingPeriod = strings.TrimSpace(input.CycleTerm.BillingPeriod)
	listing.CycleTerm.CycleStartDay = input.CycleTerm.CycleStartDay
	listing.CycleTerm.NoticeDays = input.CycleTerm.NoticeDays
	listing.CycleTerm.ExitPolicy = strings.TrimSpace(input.CycleTerm.ExitPolicy)
	listing.CycleTerm.UsageRules = strings.TrimSpace(input.CycleTerm.UsageRules)
	listing.CycleTerm.UpdatedAt = now
	listing.CycleTerm.Version++
	listing.Title = strings.TrimSpace(input.Title)
	listing.Summary = strings.TrimSpace(input.Summary)
	listing.AccessArrangement = strings.TrimSpace(input.AccessArrangement)
	listing.SourceURL = strings.TrimSpace(input.SourceURL)
	listing.PriceMonthlyCNY = strings.TrimSpace(input.PriceMonthlyCNY)
	listing.ServiceMultiplier = strings.TrimSpace(input.ServiceMultiplier)
	listing.MonthlyQuotaAmount = strings.TrimSpace(input.MonthlyQuotaAmount)
	listing.QuotaLabel = strings.TrimSpace(planQuotaLabel)
	listing.QuotaUnit = strings.TrimSpace(planQuotaUnit)
	listing.QuotaPeriod = strings.TrimSpace(planQuotaPeriod)
	listing.BuyerSeatCapacity = input.BuyerSeatCapacity
	listing.ActiveBuyerMembers = input.ActiveBuyerMembers
	listing.PolicyVersion = planPolicyVersion
	listing.RiskNoticeCode = planRiskNoticeCode
	listing.RiskAckRequired = planRiskAckRequired
	listing.UpdatedAt = now
	listing.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET product_plan_id = $2,
		    owner_contact_method_id = $3,
		    title = $4,
		    summary = $5,
		    access_arrangement = $6,
		    source_url = $7,
		    price_monthly_cny = $8,
		    service_multiplier = $9,
		    monthly_quota_amount = $10,
		    quota_label = $11,
		    quota_unit = $12,
		    quota_period = $13,
		    buyer_seat_capacity = $14,
		    active_buyer_members = $15,
		    policy_version = $16,
		    risk_notice_code = $17,
		    risk_ack_required = $18,
		    updated_at = $19,
		    version = $20
		WHERE id = $1
	`, listing.ID, listing.ProductPlanID, listing.OwnerContactMethodID, listing.Title, listing.Summary, listing.AccessArrangement,
		nullText(listing.SourceURL), listing.PriceMonthlyCNY, listing.ServiceMultiplier, listing.MonthlyQuotaAmount, listing.QuotaLabel, listing.QuotaUnit, listing.QuotaPeriod,
		listing.BuyerSeatCapacity, listing.ActiveBuyerMembers,
		listing.PolicyVersion, nullText(listing.RiskNoticeCode), listing.RiskAckRequired, listing.UpdatedAt, listing.Version)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if listing.CycleTerm != nil {
		if appErr := upsertCarpoolCycleTermInTx(ctx, tx, *listing.CycleTerm, now); appErr != nil {
			return carpool.Listing{}, appErr
		}
	}
	if ack != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO carpool_listing_policy_acknowledgements (
				carpool_listing_id, user_id, risk_notice_code, policy_version, risk_notice_version_id, acknowledged_at
			)
			SELECT $1, $2, $3, $4::bigint, version.id, $5
			FROM risk_notices notice
			JOIN risk_notice_versions version ON version.risk_notice_id = notice.id
			WHERE notice.code = $3 AND version.version::bigint = $4::bigint
			ON CONFLICT (carpool_listing_id, user_id, risk_notice_code, policy_version) DO NOTHING
		`, listing.ID, listing.OwnerUserID, ack.RiskNoticeCode, ack.PolicyVersion, ack.AcknowledgedAt)
		if err != nil {
			return carpool.Listing{}, internalStoreError()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) SubmitCarpoolListingForReview(ctx context.Context, user auth.User, input carpool.SubmitListingReviewInput, now time.Time) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	listing, err := s.getCarpoolListing(ctx, tx, input.ListingID, true, true)
	if errors.Is(err, pgx.ErrNoRows) || listing.OwnerUserID != input.OwnerUserID {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return carpool.Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if listing.Status != carpool.ListingStatusDraft && listing.Status != carpool.ListingStatusChangesRequested {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能发布。")
	}
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound {
		return carpool.Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "linux.do binding required", "发布拼车前需要完成 linux.do 身份绑定。", "linuxDoBinding", "required", "需要先完成 linux.do 身份绑定。")
	}
	if appErr := ensureCarpoolPlanAllowedForPublish(ctx, tx, listing.ProductPlanID); appErr != nil {
		return carpool.Listing{}, appErr
	}
	if _, _, appErr := lockContactVersionForOwner(ctx, tx, listing.OwnerContactMethodID, listing.OwnerUserID, "车主联系方式不可用或不属于当前用户。"); appErr != nil {
		return carpool.Listing{}, appErr
	}
	listing.Status = carpool.ListingStatusActive
	listing.ReviewedByAdminID = ""
	listing.ReviewedAt = nil
	listing.ReviewReason = ""
	listing.UpdatedAt = now
	listing.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET status = $2,
		    reviewed_by_admin_id = NULL,
		    reviewed_at = NULL,
		    review_reason = NULL,
		    updated_at = $3,
		    version = $4
		WHERE id = $1
	`, listing.ID, listing.Status, listing.UpdatedAt, listing.Version)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) UpdateCarpoolListingReviewStatus(ctx context.Context, user auth.User, input carpool.ReviewInput, now time.Time) (carpool.Listing, *domain.AppError) {
	if !user.IsAdmin {
		return carpool.Listing{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	listing, err := s.getCarpoolListing(ctx, tx, input.ListingID, true, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return carpool.Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateCarpoolListingStatus(listing.Status, input.Status, input.Action) {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能执行该审核动作。")
	}
	if input.Action == "approve" {
		if appErr := ensureCarpoolPlanAllowedForPublish(ctx, tx, listing.ProductPlanID); appErr != nil {
			return carpool.Listing{}, appErr
		}
	}
	listing.Status = input.Status
	listing.ReviewedByAdminID = user.ID
	listing.ReviewedAt = &now
	listing.ReviewReason = strings.TrimSpace(input.Reason)
	listing.UpdatedAt = now
	listing.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET status = $2,
		    reviewed_by_admin_id = $3,
		    reviewed_at = $4,
		    review_reason = $5,
		    updated_at = $6,
		    version = $7
		WHERE id = $1
	`, listing.ID, listing.Status, listing.ReviewedByAdminID, listing.ReviewedAt, listing.ReviewReason, listing.UpdatedAt, listing.Version)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) CreateCarpoolApplication(ctx context.Context, application carpool.Application, ack *carpool.RiskAcknowledgement) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	if _, _, appErr := lockContactVersionForOwner(ctx, tx, application.BuyerContactMethodID, application.BuyerUserID, "买家联系方式不可用或不属于当前用户。"); appErr != nil {
		return appErr
	}
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = 'expired',
		    updated_at = $3,
		    version = version + 1
		WHERE carpool_listing_id = $1
		  AND buyer_user_id = $2
		  AND status = 'accepted_reserved'
		  AND reservation_expires_at <= $3
	`, application.CarpoolListingID, application.BuyerUserID, application.CreatedAt)
	if err != nil {
		return internalStoreError()
	}
	var activeMembershipExists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM carpool_memberships
			WHERE carpool_listing_id = $1
			  AND buyer_user_id = $2
			  AND status = 'active'
		)
	`, application.CarpoolListingID, application.BuyerUserID).Scan(&activeMembershipExists)
	if err != nil {
		return internalStoreError()
	}
	if activeMembershipExists {
		return domain.NewError(http.StatusConflict, domain.CodeActiveMembershipExists, "Active membership exists", "你已是该车源的成员。")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_applications (
			id, carpool_listing_id, buyer_user_id, owner_user_id, product_plan_id,
			buyer_contact_method_id, status, seat_count, listing_title_snapshot,
			price_monthly_cny_snapshot, policy_version_snapshot, risk_notice_code_snapshot,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15
		)
	`, application.ID, application.CarpoolListingID, application.BuyerUserID, application.OwnerUserID, application.ProductPlanID,
		application.BuyerContactMethodID, application.Status, application.SeatCount, application.ListingTitleSnapshot,
		application.PriceMonthlyCNY, application.PolicyVersionSnapshot, nullText(application.RiskNoticeCode),
		application.CreatedAt, application.UpdatedAt, application.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.NewError(http.StatusConflict, domain.CodeActiveApplicationExists, "Active application exists", "你已提交过该车源的进行中申请。")
		}
		return internalStoreError()
	}
	if ack != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO carpool_application_policy_acknowledgements (
				carpool_application_id, user_id, risk_notice_code, policy_version, risk_notice_version_id, acknowledged_at
			)
			SELECT $1, $2, $3, $4::bigint, version.id, $5
			FROM risk_notices notice
			JOIN risk_notice_versions version ON version.risk_notice_id = notice.id
			WHERE notice.code = $3 AND version.version::bigint = $4::bigint
		`, application.ID, application.BuyerUserID, ack.RiskNoticeCode, ack.PolicyVersion, ack.AcknowledgedAt)
		if err != nil {
			return internalStoreError()
		}
	}
	if appErr := insertCarpoolApplicationEventAndOwnerNotification(ctx, tx, application, application.BuyerUserID, "carpool_application.created", "收到新的上车申请", "你的车源收到新的上车申请，请查看申请详情。", "", application.CreatedAt); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListCarpoolApplicationsByBuyer(ctx context.Context, buyerUserID string) ([]carpool.Application, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+carpoolApplicationColumns+`
		FROM carpool_applications
		WHERE buyer_user_id = $1
		ORDER BY updated_at DESC
	`, buyerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolApplications(rows)
}

func (s *Store) GetCarpoolApplicationForBuyer(ctx context.Context, buyerUserID, applicationID string) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, s.pool, applicationID, false)
	if errors.Is(err, pgx.ErrNoRows) || application.BuyerUserID != buyerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	return application, nil
}

func (s *Store) ListCarpoolApplicationsByOwner(ctx context.Context, ownerUserID string) ([]carpool.Application, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+carpoolApplicationColumns+`
		FROM carpool_applications
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolApplications(rows)
}

func (s *Store) GetCarpoolApplicationForOwner(ctx context.Context, ownerUserID, applicationID string) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, s.pool, applicationID, false)
	if errors.Is(err, pgx.ErrNoRows) || application.OwnerUserID != ownerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	return application, nil
}

func (s *Store) AcceptCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.AcceptApplicationInput, now time.Time, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}

	application, appErr := s.acceptCarpoolApplicationInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	return application, completion, nil
}

func (s *Store) RejectCarpoolApplication(ctx context.Context, input carpool.RejectApplicationInput, now time.Time) (carpool.Application, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	application, err := s.getCarpoolApplication(ctx, tx, input.ApplicationID, true)
	if errors.Is(err, pgx.ErrNoRows) || application.OwnerUserID != input.OwnerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return carpool.Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != carpool.ApplicationStatusPendingOwner {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能拒绝。")
	}
	application.Status = carpool.ApplicationStatusRejected
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2,
		    decision_reason = $3,
		    decided_at = $4,
		    updated_at = $5,
		    version = $6
		WHERE id = $1
	`, application.ID, application.Status, application.DecisionReason, application.DecidedAt, application.UpdatedAt, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := insertCarpoolApplicationEventAndNotification(ctx, tx, application, input.OwnerUserID, "carpool_application.rejected", "上车申请已被车主拒绝", "车主已拒绝你的上车申请，请查看申请详情。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, internalStoreError()
	}
	return application, nil
}

func (s *Store) CancelCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.CancelApplicationInput, now time.Time, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}

	application, appErr := s.cancelCarpoolApplicationInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	return application, completion, nil
}

func (s *Store) WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.WithdrawAcceptanceInput, now time.Time, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}

	application, appErr := s.withdrawCarpoolAcceptanceInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	return application, completion, nil
}

func (s *Store) ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.ConfirmApplicationJoinInput, now time.Time, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}

	application, appErr := s.confirmCarpoolApplicationJoinInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	return application, completion, nil
}

func (s *Store) ListCarpoolMembershipsByBuyer(ctx context.Context, buyerUserID string) ([]carpool.Membership, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+carpoolMembershipColumns+`
		FROM carpool_memberships
		WHERE buyer_user_id = $1
		ORDER BY updated_at DESC
	`, buyerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolMemberships(rows)
}

func (s *Store) ListCarpoolMembershipsByOwner(ctx context.Context, ownerUserID string) ([]carpool.Membership, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+carpoolMembershipColumns+`
		FROM carpool_memberships
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC
	`, ownerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolMemberships(rows)
}

func (s *Store) ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.ConfirmMembershipCompleteInput, now time.Time, buildCompletion carpool.MembershipCompletionBuilder) (carpool.Membership, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}

	membership, appErr := s.confirmCarpoolMembershipCompleteInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(membership)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	return membership, completion, nil
}

func (s *Store) EndCarpoolMembershipWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.EndMembershipInput, now time.Time, buildCompletion carpool.MembershipCompletionBuilder) (carpool.Membership, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}

	membership, appErr := s.endCarpoolMembershipInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(membership)
	if appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Membership{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	return membership, completion, nil
}

const carpoolListingColumns = `
	id::text, owner_user_id::text, product_plan_id::text, owner_contact_method_id::text, title, summary, access_arrangement,
	COALESCE(cycle_term_id::text, ''), COALESCE(cycle_billing_period, ''), cycle_start_day, COALESCE(cycle_notice_days, 0),
	COALESCE(cycle_exit_policy, ''), COALESCE(cycle_usage_rules, ''), COALESCE(cycle_version, 0),
	COALESCE(cycle_created_at, created_at), COALESCE(cycle_updated_at, updated_at),
	COALESCE(source_url, ''), price_monthly_cny::text, service_multiplier::text,
	monthly_quota_amount::text, quota_label, quota_unit, quota_period, buyer_seat_capacity, active_buyer_members,
	status, COALESCE(reviewed_by_admin_id::text, ''), reviewed_at, COALESCE(review_reason, ''),
	policy_version, COALESCE(risk_notice_code, ''), risk_ack_required, reserved_seats::int,
	GREATEST(buyer_seat_capacity - active_buyer_members - reserved_seats, 0)::int AS available_seats,
	created_at, updated_at, version
`

const carpoolListingViewSource = `(
	SELECT l.*,
	       t.id AS cycle_term_id,
	       t.billing_period AS cycle_billing_period,
	       t.cycle_start_day,
	       t.notice_days AS cycle_notice_days,
	       t.exit_policy AS cycle_exit_policy,
	       t.usage_rules AS cycle_usage_rules,
	       t.version AS cycle_version,
	       t.created_at AS cycle_created_at,
	       t.updated_at AS cycle_updated_at,
	       COALESCE(SUM(a.seat_count) FILTER (
	         WHERE a.status = 'accepted_reserved'
	           AND a.reservation_expires_at > now()
	       ), 0) AS reserved_seats
	FROM carpool_listings l
	LEFT JOIN carpool_cycle_terms t ON t.carpool_listing_id = l.id
	LEFT JOIN carpool_applications a ON a.carpool_listing_id = l.id
	GROUP BY l.id, t.id
) listing_view`

const carpoolApplicationColumns = `
	id::text, carpool_listing_id::text, buyer_user_id::text, owner_user_id::text,
	product_plan_id::text, buyer_contact_method_id::text,
	CASE
	  WHEN status = 'accepted_reserved' AND reservation_expires_at <= now() THEN 'expired'
	  ELSE status
	END AS status,
	seat_count,
	listing_title_snapshot, price_monthly_cny_snapshot::text, policy_version_snapshot,
	COALESCE(risk_notice_code_snapshot, ''), COALESCE(contact_session_id::text, ''),
	reservation_expires_at, join_confirmation_deadline,
	(SELECT confirmed_at FROM carpool_join_confirmations WHERE carpool_application_id = carpool_applications.id AND actor_role = 'buyer') AS buyer_confirmed_at,
	(SELECT confirmed_at FROM carpool_join_confirmations WHERE carpool_application_id = carpool_applications.id AND actor_role = 'owner') AS owner_confirmed_at,
	joined_at, COALESCE(decision_reason, ''), decided_at, created_at, updated_at, version
`

const carpoolMembershipColumns = `
	id::text, carpool_listing_id::text, carpool_application_id::text, COALESCE(cycle_term_id::text, ''), buyer_user_id::text,
	owner_user_id::text, product_plan_id::text, status, seat_count,
	price_monthly_cny_snapshot::text, policy_version_snapshot, COALESCE(risk_notice_code_snapshot, ''),
	joined_at,
	(SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = carpool_memberships.id AND actor_role = 'buyer') AS buyer_completed_at,
	(SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = carpool_memberships.id AND actor_role = 'owner') AS owner_completed_at,
	CASE WHEN status = 'completed' THEN ended_at ELSE NULL END AS completed_at,
	ended_at, ended_reason, COALESCE(ended_by_user_id::text, ''),
	created_at, updated_at, version
`

func scanCarpoolListings(rows pgx.Rows) ([]carpool.Listing, *domain.AppError) {
	listings := []carpool.Listing{}
	for rows.Next() {
		var listing carpool.Listing
		if err := scanCarpoolListing(rows, &listing); err != nil {
			return nil, internalStoreError()
		}
		listings = append(listings, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return listings, nil
}

func scanCarpoolListing(row scanner, listing *carpool.Listing) error {
	var cycleTermID string
	var cycleTerm carpool.CycleTerm
	if err := row.Scan(
		&listing.ID,
		&listing.OwnerUserID,
		&listing.ProductPlanID,
		&listing.OwnerContactMethodID,
		&listing.Title,
		&listing.Summary,
		&listing.AccessArrangement,
		&cycleTermID,
		&cycleTerm.BillingPeriod,
		&cycleTerm.CycleStartDay,
		&cycleTerm.NoticeDays,
		&cycleTerm.ExitPolicy,
		&cycleTerm.UsageRules,
		&cycleTerm.Version,
		&cycleTerm.CreatedAt,
		&cycleTerm.UpdatedAt,
		&listing.SourceURL,
		&listing.PriceMonthlyCNY,
		&listing.ServiceMultiplier,
		&listing.MonthlyQuotaAmount,
		&listing.QuotaLabel,
		&listing.QuotaUnit,
		&listing.QuotaPeriod,
		&listing.BuyerSeatCapacity,
		&listing.ActiveBuyerMembers,
		&listing.Status,
		&listing.ReviewedByAdminID,
		&listing.ReviewedAt,
		&listing.ReviewReason,
		&listing.PolicyVersion,
		&listing.RiskNoticeCode,
		&listing.RiskAckRequired,
		&listing.ReservedSeats,
		&listing.AvailableSeats,
		&listing.CreatedAt,
		&listing.UpdatedAt,
		&listing.Version,
	); err != nil {
		return err
	}
	if cycleTermID != "" {
		cycleTerm.ID = cycleTermID
		cycleTerm.CarpoolListingID = listing.ID
		cycleTerm.OwnerUserID = listing.OwnerUserID
		listing.CycleTerm = &cycleTerm
	}
	return nil
}

func scanCarpoolApplications(rows pgx.Rows) ([]carpool.Application, *domain.AppError) {
	applications := []carpool.Application{}
	for rows.Next() {
		var application carpool.Application
		if err := scanCarpoolApplication(rows, &application); err != nil {
			return nil, internalStoreError()
		}
		applications = append(applications, application)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return applications, nil
}

func scanCarpoolApplication(row scanner, application *carpool.Application) error {
	return row.Scan(
		&application.ID,
		&application.CarpoolListingID,
		&application.BuyerUserID,
		&application.OwnerUserID,
		&application.ProductPlanID,
		&application.BuyerContactMethodID,
		&application.Status,
		&application.SeatCount,
		&application.ListingTitleSnapshot,
		&application.PriceMonthlyCNY,
		&application.PolicyVersionSnapshot,
		&application.RiskNoticeCode,
		&application.ContactSessionID,
		&application.ReservationExpiresAt,
		&application.JoinConfirmationDeadline,
		&application.BuyerConfirmedAt,
		&application.OwnerConfirmedAt,
		&application.JoinedAt,
		&application.DecisionReason,
		&application.DecidedAt,
		&application.CreatedAt,
		&application.UpdatedAt,
		&application.Version,
	)
}

func scanCarpoolMemberships(rows pgx.Rows) ([]carpool.Membership, *domain.AppError) {
	memberships := []carpool.Membership{}
	for rows.Next() {
		var membership carpool.Membership
		if err := scanCarpoolMembership(rows, &membership); err != nil {
			return nil, internalStoreError()
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return memberships, nil
}

func scanCarpoolMembership(row scanner, membership *carpool.Membership) error {
	return row.Scan(
		&membership.ID,
		&membership.CarpoolListingID,
		&membership.CarpoolApplicationID,
		&membership.CycleTermID,
		&membership.BuyerUserID,
		&membership.OwnerUserID,
		&membership.ProductPlanID,
		&membership.Status,
		&membership.SeatCount,
		&membership.PriceMonthlyCNY,
		&membership.PolicyVersionSnapshot,
		&membership.RiskNoticeCode,
		&membership.JoinedAt,
		&membership.BuyerCompletedAt,
		&membership.OwnerCompletedAt,
		&membership.CompletedAt,
		&membership.EndedAt,
		&membership.EndedReason,
		&membership.EndedByUserID,
		&membership.CreatedAt,
		&membership.UpdatedAt,
		&membership.Version,
	)
}

func (s *Store) getCarpoolListing(ctx context.Context, q queryer, listingID string, forUpdate bool, baseTable bool) (carpool.Listing, error) {
	source := carpoolListingViewSource
	if baseTable || forUpdate {
		if forUpdate {
			var id string
			if err := q.QueryRow(ctx, `SELECT id::text FROM carpool_listings WHERE id = $1 FOR UPDATE`, listingID).Scan(&id); err != nil {
				return carpool.Listing{}, err
			}
		}
		source = `(
			SELECT l.*,
			       t.id AS cycle_term_id,
			       t.billing_period AS cycle_billing_period,
			       t.cycle_start_day,
			       t.notice_days AS cycle_notice_days,
			       t.exit_policy AS cycle_exit_policy,
			       t.usage_rules AS cycle_usage_rules,
			       t.version AS cycle_version,
			       t.created_at AS cycle_created_at,
			       t.updated_at AS cycle_updated_at,
			       COALESCE(SUM(a.seat_count) FILTER (
			         WHERE a.status = 'accepted_reserved'
			           AND a.reservation_expires_at > now()
			       ), 0) AS reserved_seats
			FROM carpool_listings l
			LEFT JOIN carpool_cycle_terms t ON t.carpool_listing_id = l.id
			LEFT JOIN carpool_applications a ON a.carpool_listing_id = l.id
			WHERE l.id = $1
			GROUP BY l.id, t.id
		) listing_view`
		query := `SELECT ` + carpoolListingColumns + ` FROM ` + source
		var listing carpool.Listing
		err := scanCarpoolListing(q.QueryRow(ctx, query, listingID), &listing)
		return listing, err
	}
	query := `SELECT ` + carpoolListingColumns + ` FROM ` + source + ` WHERE id = $1`
	var listing carpool.Listing
	err := scanCarpoolListing(q.QueryRow(ctx, query, listingID), &listing)
	return listing, err
}
func (s *Store) getCarpoolApplication(ctx context.Context, q queryer, applicationID string, forUpdate bool) (carpool.Application, error) {
	query := `SELECT ` + carpoolApplicationColumns + ` FROM carpool_applications WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var application carpool.Application
	err := scanCarpoolApplication(q.QueryRow(ctx, query, applicationID), &application)
	return application, err
}

func (s *Store) getCarpoolMembership(ctx context.Context, q queryer, membershipID string, forUpdate bool) (carpool.Membership, error) {
	query := `SELECT ` + carpoolMembershipColumns + ` FROM carpool_memberships WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var membership carpool.Membership
	err := scanCarpoolMembership(q.QueryRow(ctx, query, membershipID), &membership)
	return membership, err
}

func upsertCarpoolCycleTermInTx(ctx context.Context, tx pgx.Tx, term carpool.CycleTerm, now time.Time) *domain.AppError {
	if strings.TrimSpace(term.ID) == "" {
		term.ID = uuid.NewString()
	}
	if term.CreatedAt.IsZero() {
		term.CreatedAt = now
	}
	if term.UpdatedAt.IsZero() {
		term.UpdatedAt = now
	}
	if term.Version <= 0 {
		term.Version = 1
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO carpool_cycle_terms (
			id, carpool_listing_id, owner_user_id, billing_period, cycle_start_day,
			notice_days, exit_policy, usage_rules, version, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (carpool_listing_id) DO UPDATE
		SET billing_period = EXCLUDED.billing_period,
		    cycle_start_day = EXCLUDED.cycle_start_day,
		    notice_days = EXCLUDED.notice_days,
		    exit_policy = EXCLUDED.exit_policy,
		    usage_rules = EXCLUDED.usage_rules,
		    version = carpool_cycle_terms.version + 1,
		    updated_at = EXCLUDED.updated_at
	`, term.ID, term.CarpoolListingID, term.OwnerUserID, term.BillingPeriod, term.CycleStartDay,
		term.NoticeDays, term.ExitPolicy, term.UsageRules, term.Version, term.CreatedAt, term.UpdatedAt)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) acceptCarpoolApplicationInTx(ctx context.Context, tx pgx.Tx, input carpool.AcceptApplicationInput, now time.Time) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, tx, input.ApplicationID, true)
	if errors.Is(err, pgx.ErrNoRows) || application.OwnerUserID != input.OwnerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return carpool.Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != carpool.ApplicationStatusPendingOwner {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能接受。")
	}

	listing, err := s.getCarpoolListing(ctx, tx, application.CarpoolListingID, true, true)
	if errors.Is(err, pgx.ErrNoRows) || listing.OwnerUserID != input.OwnerUserID {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if listing.Status != carpool.ListingStatusActive {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	if listing.AvailableSeats < application.SeatCount {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可预留名额。")
	}

	_, buyerVersion, appErr := lockContactVersionForOwner(ctx, tx, application.BuyerContactMethodID, application.BuyerUserID, "买家联系方式不可用或不属于当前用户。")
	if appErr != nil {
		return carpool.Application{}, appErr
	}
	_, ownerVersion, appErr := lockContactVersionForOwner(ctx, tx, listing.OwnerContactMethodID, input.OwnerUserID, "车主联系方式不可用或不属于当前用户。")
	if appErr != nil {
		return carpool.Application{}, appErr
	}

	sessionID := uuid.NewString()
	reservationExpiresAt := now.Add(carpool.JoinConfirmationDuration)
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_sessions (id, buyer_user_id, seller_user_id, opens_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $4)
	`, sessionID, application.BuyerUserID, application.OwnerUserID, now, reservationExpiresAt)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_session_items (contact_session_id, subject_user_id, side, contact_method_version_id, created_at)
		VALUES ($1, $2, 'buyer', $3, $5),
		       ($1, $4, 'seller', $6, $5)
	`, sessionID, application.BuyerUserID, buyerVersion.ID, application.OwnerUserID, now, ownerVersion.ID)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}

	application.Status = carpool.ApplicationStatusAcceptedReserved
	application.ContactSessionID = sessionID
	application.ReservationExpiresAt = &reservationExpiresAt
	application.JoinConfirmationDeadline = &reservationExpiresAt
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2,
		    contact_session_id = $3,
		    reservation_expires_at = $4,
		    join_confirmation_deadline = $5,
		    decided_at = $6,
		    updated_at = $7,
		    version = $8
		WHERE id = $1
	`, application.ID, application.Status, application.ContactSessionID, application.ReservationExpiresAt, application.JoinConfirmationDeadline, application.DecidedAt, application.UpdatedAt, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := insertCarpoolApplicationEventAndNotification(ctx, tx, application, input.OwnerUserID, "carpool_application.accepted", "上车申请已被车主接受", "车主已接受你的上车申请，并开启 30 分钟联系窗口。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	return application, nil
}

func (s *Store) cancelCarpoolApplicationInTx(ctx context.Context, tx pgx.Tx, input carpool.CancelApplicationInput, now time.Time) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, tx, input.ApplicationID, true)
	if errors.Is(err, pgx.ErrNoRows) || application.BuyerUserID != input.BuyerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return carpool.Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != carpool.ApplicationStatusPendingOwner && application.Status != carpool.ApplicationStatusAcceptedReserved {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能取消；已加入后请退出拼车。")
	}
	shouldRevokeContact := application.Status == carpool.ApplicationStatusAcceptedReserved
	application.Status = carpool.ApplicationStatusCancelledByBuyer
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2,
		    decision_reason = $3,
		    decided_at = $4,
		    updated_at = $5,
		    version = $6
		WHERE id = $1
	`, application.ID, application.Status, application.DecisionReason, application.DecidedAt, application.UpdatedAt, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if shouldRevokeContact {
		if appErr := updateCarpoolContactSessionStatus(ctx, tx, application.ContactSessionID, "revoked", now); appErr != nil {
			return carpool.Application{}, appErr
		}
	}
	if appErr := insertCarpoolApplicationEventAndOwnerNotification(ctx, tx, application, input.BuyerUserID, "carpool_application.cancelled_by_buyer", "上车申请已取消", "买家已取消上车申请或预留，请查看申请详情。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	return application, nil
}

func (s *Store) withdrawCarpoolAcceptanceInTx(ctx context.Context, tx pgx.Tx, input carpool.WithdrawAcceptanceInput, now time.Time) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, tx, input.ApplicationID, true)
	if errors.Is(err, pgx.ErrNoRows) || application.OwnerUserID != input.OwnerUserID {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return carpool.Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != carpool.ApplicationStatusAcceptedReserved {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能撤回接受。")
	}
	application.Status = carpool.ApplicationStatusCancelledByOwner
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2,
		    decision_reason = $3,
		    decided_at = $4,
		    updated_at = $5,
		    version = $6
		WHERE id = $1
	`, application.ID, application.Status, application.DecisionReason, application.DecidedAt, application.UpdatedAt, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := updateCarpoolContactSessionStatus(ctx, tx, application.ContactSessionID, "revoked", now); appErr != nil {
		return carpool.Application{}, appErr
	}
	if appErr := insertCarpoolApplicationEventAndNotification(ctx, tx, application, input.OwnerUserID, "carpool_application.cancelled_by_owner", "预留已被车主取消", "车主已撤回接受并取消预留，请查看申请详情。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	return application, nil
}

func (s *Store) confirmCarpoolApplicationJoinInTx(ctx context.Context, tx pgx.Tx, input carpool.ConfirmApplicationJoinInput, now time.Time) (carpool.Application, *domain.AppError) {
	application, err := s.getCarpoolApplication(ctx, tx, input.ApplicationID, true)
	if errors.Is(err, pgx.ErrNoRows) || !canActorConfirmCarpoolJoin(application, input.ActorUserID, input.ActorRole) {
		return carpool.Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return carpool.Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != carpool.ApplicationStatusAcceptedReserved {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能确认加入。")
	}
	if application.JoinConfirmationDeadline == nil || !now.Before(*application.JoinConfirmationDeadline) {
		_, _ = tx.Exec(ctx, `
			UPDATE carpool_applications
			SET status = 'expired',
			    updated_at = $2,
			    version = version + 1
			WHERE id = $1 AND status = 'accepted_reserved'
		`, application.ID, now)
		if appErr := updateCarpoolContactSessionStatus(ctx, tx, application.ContactSessionID, "expired", now); appErr != nil {
			return carpool.Application{}, appErr
		}
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeJoinConfirmationExpired, "Join confirmation expired", "确认加入期限已过。")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_join_confirmations (
			carpool_application_id, actor_user_id, actor_role, confirmed_at, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $4)
	`, application.ID, input.ActorUserID, input.ActorRole, now, strings.TrimSpace(input.RequestID))
	if err != nil {
		if isUniqueViolation(err) {
			return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "该角色已确认加入。")
		}
		return carpool.Application{}, internalStoreError()
	}

	switch input.ActorRole {
	case carpool.JoinActorBuyer:
		application.BuyerConfirmedAt = &now
	case carpool.JoinActorOwner:
		application.OwnerConfirmedAt = &now
	}

	buyerConfirmedAt, ownerConfirmedAt, appErr := loadCarpoolJoinConfirmationTimes(ctx, tx, application.ID)
	if appErr != nil {
		return carpool.Application{}, appErr
	}
	application.BuyerConfirmedAt = buyerConfirmedAt
	application.OwnerConfirmedAt = ownerConfirmedAt
	joined := buyerConfirmedAt != nil && ownerConfirmedAt != nil
	if joined {
		listing, err := s.getCarpoolListing(ctx, tx, application.CarpoolListingID, true, true)
		if errors.Is(err, pgx.ErrNoRows) || listing.OwnerUserID != application.OwnerUserID {
			return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可确认加入。")
		}
		if err != nil {
			return carpool.Application{}, internalStoreError()
		}
		if listing.Status != carpool.ListingStatusActive {
			return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可确认加入。")
		}
		availableForJoin, appErr := availableSeatsForJoiningApplication(ctx, tx, application, listing, now)
		if appErr != nil {
			return carpool.Application{}, appErr
		}
		if availableForJoin < application.SeatCount {
			return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可确认名额。")
		}
		application.Status = carpool.ApplicationStatusJoined
		application.JoinedAt = &now
		application.ReservationExpiresAt = nil
		application.UpdatedAt = now
		application.Version++
		_, err = tx.Exec(ctx, `
			UPDATE carpool_applications
			SET status = $2,
			    reservation_expires_at = $3,
			    joined_at = $4,
			    updated_at = $5,
			    version = $6
			WHERE id = $1
		`, application.ID, application.Status, application.ReservationExpiresAt, application.JoinedAt, application.UpdatedAt, application.Version)
		if err != nil {
			return carpool.Application{}, internalStoreError()
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO carpool_memberships (
				carpool_listing_id, carpool_application_id, buyer_user_id, owner_user_id,
				product_plan_id, status, seat_count, price_monthly_cny_snapshot,
				policy_version_snapshot, risk_notice_code_snapshot, joined_at,
				created_at, updated_at, version
			)
			VALUES (
				$1, $2, $3, $4,
				$5, 'active', $6, $7,
				$8, $9, $10,
				$10, $10, 1
			)
		`, application.CarpoolListingID, application.ID, application.BuyerUserID, application.OwnerUserID,
			application.ProductPlanID, application.SeatCount, application.PriceMonthlyCNY,
			application.PolicyVersionSnapshot, nullText(application.RiskNoticeCode), now)
		if err != nil {
			if isUniqueViolation(err) {
				return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeActiveMembershipExists, "Active membership exists", "你已是该车源的成员。")
			}
			return carpool.Application{}, internalStoreError()
		}
		_, err = tx.Exec(ctx, `
			UPDATE carpool_listings
			SET active_buyer_members = active_buyer_members + $2,
			    updated_at = $3,
			    version = version + 1
			WHERE id = $1
		`, application.CarpoolListingID, application.SeatCount, now)
		if err != nil {
			return carpool.Application{}, internalStoreError()
		}
	} else {
		application.UpdatedAt = now
		application.Version++
		_, err = tx.Exec(ctx, `
			UPDATE carpool_applications
			SET updated_at = $2,
			    version = $3
			WHERE id = $1
		`, application.ID, application.UpdatedAt, application.Version)
		if err != nil {
			return carpool.Application{}, internalStoreError()
		}
	}

	eventType := "carpool_application.join_confirmed"
	title := "上车确认已记录"
	body := "对方已确认加入，请查看申请详情。"
	if joined {
		eventType = "carpool_application.joined"
		title = "上车已确认"
		body = "双方已确认加入，成员关系已生效。"
	}
	notifyUserID := application.OwnerUserID
	if input.ActorRole == carpool.JoinActorOwner {
		notifyUserID = application.BuyerUserID
	}
	if appErr := insertCarpoolApplicationEventAndTargetNotification(ctx, tx, application, input.ActorUserID, notifyUserID, eventType, title, body, input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	return application, nil
}

func (s *Store) confirmCarpoolMembershipCompleteInTx(ctx context.Context, tx pgx.Tx, input carpool.ConfirmMembershipCompleteInput, now time.Time) (carpool.Membership, *domain.AppError) {
	membership, err := s.getCarpoolMembership(ctx, tx, input.MembershipID, true)
	if errors.Is(err, pgx.ErrNoRows) || !canActorConfirmCarpoolMembership(membership, input.ActorUserID, input.ActorRole) {
		return carpool.Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return carpool.Membership{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if membership.Status != carpool.MembershipStatusActive {
		return carpool.Membership{}, domain.NewError(http.StatusConflict, domain.CodeMembershipNotActive, "Membership not active", "当前成员关系不是可操作状态。")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_completion_confirmations (
			carpool_membership_id, actor_user_id, actor_role, confirmed_at, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $4)
	`, membership.ID, input.ActorUserID, input.ActorRole, now, strings.TrimSpace(input.RequestID))
	if err != nil {
		if isUniqueViolation(err) {
			return carpool.Membership{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "该角色已确认完成。")
		}
		return carpool.Membership{}, internalStoreError()
	}

	buyerCompletedAt, ownerCompletedAt, appErr := loadCarpoolCompletionConfirmationTimes(ctx, tx, membership.ID)
	if appErr != nil {
		return carpool.Membership{}, appErr
	}
	membership.BuyerCompletedAt = buyerCompletedAt
	membership.OwnerCompletedAt = ownerCompletedAt
	completed := buyerCompletedAt != nil && ownerCompletedAt != nil
	if completed {
		listing, err := s.getCarpoolListing(ctx, tx, membership.CarpoolListingID, true, true)
		if errors.Is(err, pgx.ErrNoRows) || listing.OwnerUserID != membership.OwnerUserID {
			return carpool.Membership{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可确认完成。")
		}
		if err != nil {
			return carpool.Membership{}, internalStoreError()
		}
		membership.Status = carpool.MembershipStatusCompleted
		membership.CompletedAt = &now
		membership.EndedAt = &now
		membership.EndedReason = "双方确认周期完成。"
		membership.EndedByUserID = input.ActorUserID
		membership.UpdatedAt = now
		membership.Version++
		_, err = tx.Exec(ctx, `
			UPDATE carpool_memberships
			SET status = $2,
			    ended_at = $3,
			    ended_reason = $4,
			    ended_by_user_id = $5,
			    updated_at = $6,
			    version = $7
			WHERE id = $1
		`, membership.ID, membership.Status, membership.EndedAt, membership.EndedReason, membership.EndedByUserID, membership.UpdatedAt, membership.Version)
		if err != nil {
			return carpool.Membership{}, internalStoreError()
		}
		if appErr := decrementCarpoolActiveMembers(ctx, tx, membership.CarpoolListingID, membership.SeatCount, now); appErr != nil {
			return carpool.Membership{}, appErr
		}
	} else {
		membership.UpdatedAt = now
		membership.Version++
		_, err = tx.Exec(ctx, `
			UPDATE carpool_memberships
			SET updated_at = $2,
			    version = $3
			WHERE id = $1
		`, membership.ID, membership.UpdatedAt, membership.Version)
		if err != nil {
			return carpool.Membership{}, internalStoreError()
		}
	}

	eventType := "carpool_membership.completion_confirmed"
	title := "完成确认已记录"
	body := "对方已确认周期完成，请查看成员详情。"
	if completed {
		eventType = "carpool_membership.completed"
		title = "成员周期已完成"
		body = "双方已确认周期完成，成员关系已结束。"
	}
	notifyUserID := membership.OwnerUserID
	if input.ActorRole == carpool.JoinActorOwner {
		notifyUserID = membership.BuyerUserID
	}
	if appErr := insertCarpoolMembershipEventAndTargetNotification(ctx, tx, membership, input.ActorUserID, notifyUserID, eventType, title, body, input.RequestID, now); appErr != nil {
		return carpool.Membership{}, appErr
	}
	return membership, nil
}

func (s *Store) endCarpoolMembershipInTx(ctx context.Context, tx pgx.Tx, input carpool.EndMembershipInput, now time.Time) (carpool.Membership, *domain.AppError) {
	membership, err := s.getCarpoolMembership(ctx, tx, input.MembershipID, true)
	if errors.Is(err, pgx.ErrNoRows) || !canActorEndCarpoolMembership(membership, input.ActorUserID, input.ActorRole, input.TargetStatus) {
		return carpool.Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return carpool.Membership{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if membership.Status != carpool.MembershipStatusActive {
		return carpool.Membership{}, domain.NewError(http.StatusConflict, domain.CodeMembershipNotActive, "Membership not active", "当前成员关系不是可操作状态。")
	}
	if _, err := s.getCarpoolListing(ctx, tx, membership.CarpoolListingID, true, true); errors.Is(err, pgx.ErrNoRows) {
		return carpool.Membership{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可结束成员关系。")
	} else if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	membership.Status = input.TargetStatus
	membership.EndedAt = &now
	membership.EndedReason = strings.TrimSpace(input.Reason)
	membership.EndedByUserID = input.ActorUserID
	membership.UpdatedAt = now
	membership.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_memberships
		SET status = $2,
		    ended_at = $3,
		    ended_reason = $4,
		    ended_by_user_id = $5,
		    updated_at = $6,
		    version = $7
		WHERE id = $1
	`, membership.ID, membership.Status, membership.EndedAt, membership.EndedReason, membership.EndedByUserID, membership.UpdatedAt, membership.Version)
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	if appErr := decrementCarpoolActiveMembers(ctx, tx, membership.CarpoolListingID, membership.SeatCount, now); appErr != nil {
		return carpool.Membership{}, appErr
	}
	if appErr := revokeCarpoolMembershipContactSession(ctx, tx, membership, now); appErr != nil {
		return carpool.Membership{}, appErr
	}

	eventType := "carpool_membership.left"
	title := "成员已退出"
	body := "买家已退出成员关系，请查看详情。"
	notifyUserID := membership.OwnerUserID
	if input.TargetStatus == carpool.MembershipStatusRemoved {
		eventType = "carpool_membership.removed"
		title = "成员已被移除"
		body = "车主已移除成员关系，请查看详情。"
		notifyUserID = membership.BuyerUserID
	}
	if appErr := insertCarpoolMembershipEventAndTargetNotification(ctx, tx, membership, input.ActorUserID, notifyUserID, eventType, title, body, input.RequestID, now); appErr != nil {
		return carpool.Membership{}, appErr
	}
	return membership, nil
}

func insertCarpoolApplicationEventAndNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	return insertCarpoolApplicationEventAndTargetNotification(ctx, tx, application, actorUserID, application.BuyerUserID, eventType, title, body, requestID, now)
}

func insertCarpoolApplicationEventAndTargetNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, notifyUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	return insertCarpoolApplicationEventAndTargetNotificationURL(ctx, tx, application, actorUserID, notifyUserID, eventType, title, body, requestID, now, "/my/rides/"+application.ID)
}

func insertCarpoolApplicationEventAndOwnerNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	return insertCarpoolApplicationEventAndTargetNotificationURL(ctx, tx, application, actorUserID, application.OwnerUserID, eventType, title, body, requestID, now, "/merchant/carpool-applications/"+application.ID)
}

func insertCarpoolApplicationEventAndTargetNotificationURL(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, notifyUserID, eventType, title, body, requestID string, now time.Time, targetURL string) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		targetURL = "/my/rides/" + application.ID
	}
	eventID := uuid.NewString()
	metadata, err := json.Marshal(map[string]string{
		"carpoolListingId": application.CarpoolListingID,
		"status":           application.Status,
	})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'carpool_application', $2, $3, $4, 'user', $5, $6, $7, $8)
	`, eventID, application.ID, eventType, actorUserID, application.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, $2, $3, $4, 'carpool_application', $5, $6, $2, $7, $8, $9)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, notifyUserID, eventType, title, body, application.ID, targetURL, eventID, "carpool_application:"+application.ID+":"+application.Status+":"+notifyUserID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertCarpoolMembershipEventAndTargetNotification(ctx context.Context, tx pgx.Tx, membership carpool.Membership, actorUserID, notifyUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	eventID := uuid.NewString()
	metadata, err := json.Marshal(map[string]string{
		"carpoolListingId":     membership.CarpoolListingID,
		"carpoolApplicationId": membership.CarpoolApplicationID,
		"status":               membership.Status,
	})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'carpool_membership', $2, $3, $4, 'user', $5, $6, $7, $8)
	`, eventID, membership.ID, eventType, actorUserID, membership.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, $2, $3, $4, 'carpool_membership', $5, $6, $2, $7, $8, $9)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, notifyUserID, eventType, title, body, membership.ID, "/my/memberships/"+membership.ID, eventID, "carpool_membership:"+membership.ID+":"+membership.Status+":"+notifyUserID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func loadCarpoolJoinConfirmationTimes(ctx context.Context, q queryer, applicationID string) (*time.Time, *time.Time, *domain.AppError) {
	row := q.QueryRow(ctx, `
		SELECT
		  (SELECT confirmed_at FROM carpool_join_confirmations WHERE carpool_application_id = $1 AND actor_role = 'buyer') AS buyer_confirmed_at,
		  (SELECT confirmed_at FROM carpool_join_confirmations WHERE carpool_application_id = $1 AND actor_role = 'owner') AS owner_confirmed_at
	`, applicationID)
	var buyerConfirmedAt *time.Time
	var ownerConfirmedAt *time.Time
	if err := row.Scan(&buyerConfirmedAt, &ownerConfirmedAt); err != nil {
		return nil, nil, internalStoreError()
	}
	return buyerConfirmedAt, ownerConfirmedAt, nil
}

func loadCarpoolCompletionConfirmationTimes(ctx context.Context, q queryer, membershipID string) (*time.Time, *time.Time, *domain.AppError) {
	row := q.QueryRow(ctx, `
		SELECT
		  (SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = $1 AND actor_role = 'buyer') AS buyer_completed_at,
		  (SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = $1 AND actor_role = 'owner') AS owner_completed_at
	`, membershipID)
	var buyerCompletedAt *time.Time
	var ownerCompletedAt *time.Time
	if err := row.Scan(&buyerCompletedAt, &ownerCompletedAt); err != nil {
		return nil, nil, internalStoreError()
	}
	return buyerCompletedAt, ownerCompletedAt, nil
}

func availableSeatsForJoiningApplication(ctx context.Context, q queryer, application carpool.Application, listing carpool.Listing, now time.Time) (int, *domain.AppError) {
	var otherReservedSeats int
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(seat_count), 0)::int
		FROM carpool_applications
		WHERE carpool_listing_id = $1
		  AND id <> $2
		  AND status = 'accepted_reserved'
		  AND reservation_expires_at > $3
	`, application.CarpoolListingID, application.ID, now).Scan(&otherReservedSeats); err != nil {
		return 0, internalStoreError()
	}
	available := listing.BuyerSeatCapacity - listing.ActiveBuyerMembers - otherReservedSeats
	if available < 0 {
		return 0, nil
	}
	return available, nil
}

func decrementCarpoolActiveMembers(ctx context.Context, tx pgx.Tx, listingID string, seatCount int, now time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		UPDATE carpool_listings
		SET active_buyer_members = GREATEST(active_buyer_members - $2, 0),
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
	`, listingID, seatCount, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func revokeCarpoolMembershipContactSession(ctx context.Context, tx pgx.Tx, membership carpool.Membership, now time.Time) *domain.AppError {
	var contactSessionID string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(contact_session_id::text, '')
		FROM carpool_applications
		WHERE id = $1
	`, membership.CarpoolApplicationID).Scan(&contactSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	return updateCarpoolContactSessionStatus(ctx, tx, contactSessionID, "revoked", now)
}

func updateCarpoolContactSessionStatus(ctx context.Context, tx pgx.Tx, sessionID, status string, now time.Time) *domain.AppError {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE contact_sessions
		SET status = $2,
		    ends_at = LEAST(ends_at, $3)
		WHERE id = $1
		  AND status = 'open'
	`, sessionID, status, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func canActorConfirmCarpoolJoin(application carpool.Application, userID, actorRole string) bool {
	switch actorRole {
	case carpool.JoinActorBuyer:
		return application.BuyerUserID == userID
	case carpool.JoinActorOwner:
		return application.OwnerUserID == userID
	default:
		return false
	}
}

func canActorConfirmCarpoolMembership(membership carpool.Membership, userID, actorRole string) bool {
	switch actorRole {
	case carpool.JoinActorBuyer:
		return membership.BuyerUserID == userID
	case carpool.JoinActorOwner:
		return membership.OwnerUserID == userID
	default:
		return false
	}
}

func canActorEndCarpoolMembership(membership carpool.Membership, userID, actorRole, targetStatus string) bool {
	switch actorRole {
	case carpool.JoinActorBuyer:
		return targetStatus == carpool.MembershipStatusLeft && membership.BuyerUserID == userID
	case carpool.JoinActorOwner:
		return targetStatus == carpool.MembershipStatusRemoved && membership.OwnerUserID == userID
	default:
		return false
	}
}
func canUpdateCarpoolListingStatus(currentStatus, nextStatus, action string) bool {
	switch action {
	case "approve":
		return nextStatus == carpool.ListingStatusActive && currentStatus == carpool.ListingStatusPendingReview
	case "reject":
		return nextStatus == carpool.ListingStatusRejected && currentStatus == carpool.ListingStatusPendingReview
	case "request_changes":
		return nextStatus == carpool.ListingStatusChangesRequested && currentStatus == carpool.ListingStatusPendingReview
	case "pause":
		return nextStatus == carpool.ListingStatusPaused && currentStatus == carpool.ListingStatusActive
	case "restore":
		return nextStatus == carpool.ListingStatusActive && currentStatus == carpool.ListingStatusPaused
	}
	switch nextStatus {
	case carpool.ListingStatusActive:
		return currentStatus == carpool.ListingStatusPendingReview
	case carpool.ListingStatusRejected:
		return currentStatus == carpool.ListingStatusPendingReview
	case carpool.ListingStatusChangesRequested:
		return currentStatus == carpool.ListingStatusPendingReview
	case carpool.ListingStatusPaused:
		return currentStatus == carpool.ListingStatusActive
	default:
		return false
	}
}

func ensureCarpoolPlanAllowedForPublish(ctx context.Context, q queryer, productPlanID string) *domain.AppError {
	var publishPolicy string
	err := q.QueryRow(ctx, `SELECT publish_policy FROM product_plans WHERE id = $1 AND active = true`, productPlanID).Scan(&publishPolicy)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
	}
	if err != nil {
		return internalStoreError()
	}
	switch publishPolicy {
	case "blocked":
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeInvalidStateTransition, "Product plan blocked", "该产品当前不允许发布车源。", "productPlanId", "blocked", "该产品当前不允许发布车源。")
	case "info_only":
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeInvalidStateTransition, "Product plan info only", "该产品当前仅开放行情信息，不开放拼车发布。", "productPlanId", "info_only", "该产品当前仅开放行情信息。")
	default:
		return nil
	}
}
