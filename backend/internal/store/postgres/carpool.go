package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"reflect"
	"strconv"
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

	if appErr := createCarpoolListingMutationInTx(ctx, tx, listing, ack, "carpool_listing.created", listing.OwnerUserID, "user", listing.RequestID, false, listing.CreatedAt); appErr != nil {
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

	listing.Status = carpool.ListingStatusActive
	listing.CreatedAt = now
	listing.UpdatedAt = now
	if listing.CycleTerm != nil {
		listing.CycleTerm.CreatedAt = now
		listing.CycleTerm.UpdatedAt = now
	}
	if appErr := createCarpoolListingMutationInTx(ctx, tx, listing, ack, "carpool_listing.published", listing.OwnerUserID, "user", listing.RequestID, true, now); appErr != nil {
		return carpool.Listing{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func createCarpoolListingMutationInTx(ctx context.Context, tx pgx.Tx, listing carpool.Listing, ack *carpool.RiskAcknowledgement, eventType, actorUserID, actorKind, requestID string, validatePublish bool, now time.Time) *domain.AppError {
	if appErr := ensureActiveBusinessUsersInTx(ctx, tx, listing.OwnerUserID); appErr != nil {
		return appErr
	}
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, listing.OwnerContactMethodID, listing.OwnerUserID, contact.UsageScopeCarpoolOwner, "车主联系方式不可用、不属于当前用户或未允许拼车用途。"); appErr != nil {
		return appErr
	}
	if validatePublish {
		if appErr := ensureCarpoolPlanAllowedForPublish(ctx, tx, listing.ProductPlanID); appErr != nil {
			return appErr
		}
	}
	if appErr := insertCarpoolListingInTx(ctx, tx, listing, ack); appErr != nil {
		return appErr
	}
	return insertCarpoolListingEvent(ctx, tx, listing, actorUserID, actorKind, eventType, requestID, now)
}

func (s *Store) CreateCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, listing carpool.Listing, ack *carpool.RiskAcknowledgement, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	return s.createCarpoolListingWithIdempotency(ctx, entry, listing, ack, "carpool_listing.created", false, listing.CreatedAt, buildCompletion)
}

func (s *Store) PublishCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, listing carpool.Listing, ack *carpool.RiskAcknowledgement, now time.Time, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	listing.Status = carpool.ListingStatusActive
	listing.CreatedAt = now
	listing.UpdatedAt = now
	if listing.CycleTerm != nil {
		listing.CycleTerm.CreatedAt = now
		listing.CycleTerm.UpdatedAt = now
	}
	return s.createCarpoolListingWithIdempotency(ctx, entry, listing, ack, "carpool_listing.published", true, now, buildCompletion)
}

func (s *Store) createCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, listing carpool.Listing, ack *carpool.RiskAcknowledgement, eventType string, validatePublish bool, now time.Time, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if appErr := createCarpoolListingMutationInTx(ctx, tx, listing, ack, eventType, listing.OwnerUserID, "user", listing.RequestID, validatePublish, now); appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	return listing, completion, nil
}

func insertCarpoolListingInTx(ctx context.Context, tx pgx.Tx, listing carpool.Listing, ack *carpool.RiskAcknowledgement) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO carpool_listings (
			id, owner_user_id, product_plan_id, owner_contact_method_id, title, summary, access_arrangement,
			distribution_method, distribution_method_note, provides_admin_account,
			region_code, region_name, source_url, price_monthly_cny, service_multiplier,
			daily_spend_limit_usd, weekly_spend_limit_usd, follows_official_quota_reset, vps_region,
			supports_mainland_china_direct_connection, opening_channel_code, custom_opening_channel,
			payment_method_code, custom_payment_method, quota_label, quota_unit, quota_period,
			buyer_seat_capacity, offline_occupied_seats, active_buyer_members,
			status, policy_version, risk_notice_code, risk_ack_required,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27,
			$28, $29, $30,
			$31, $32, $33, $34,
			$35, $36, $37
		)
	`, listing.ID, listing.OwnerUserID, listing.ProductPlanID, listing.OwnerContactMethodID, listing.Title, listing.Summary, listing.AccessArrangement,
		listing.DistributionMethod, listing.DistributionMethodNote, listing.ProvidesAdminAccount,
		listing.RegionCode, listing.RegionName, nullText(listing.SourceURL), listing.PriceMonthlyCNY, listing.ServiceMultiplier,
		listing.DailyQuotaAmount, listing.WeeklyQuotaAmount, listing.FollowsOfficialQuotaReset, listing.VPSRegion,
		listing.SupportsMainlandChinaDirectConnection, listing.OpeningChannelCode, listing.CustomOpeningChannel,
		listing.PaymentMethodCode, listing.CustomPaymentMethod, listing.QuotaLabel, listing.QuotaUnit, listing.QuotaPeriod,
		listing.BuyerSeatCapacity, listing.OfflineOccupiedSeats, listing.ActiveBuyerMembers,
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
	conditionsSnapshot, err := json.Marshal(carpool.NewListingConditionsSnapshot(listing))
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_listing_condition_versions (
			carpool_listing_id, conditions_version, conditions_snapshot, changed_by_user_id, created_at
		) VALUES ($1, $2, $3::jsonb, $4, $5)
	`, listing.ID, listing.ConditionsVersion, conditionsSnapshot, listing.OwnerUserID, listing.CreatedAt)
	if err != nil {
		return internalStoreError()
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

func (s *Store) ListPublicCarpoolListings(ctx context.Context, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	return s.listCarpoolListingsPage(ctx, filter, page, true, "")
}

func (s *Store) GetPublicCarpoolListing(ctx context.Context, listingID string) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	var listing carpool.Listing
	err := scanCarpoolListing(s.pool.QueryRow(ctx, `
		SELECT `+carpoolListingColumns+`
		FROM `+carpoolListingViewSource+`
		WHERE id = $1 AND `+publicCarpoolListingPredicate("listing_view"), listingID), &listing)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func publicCarpoolListingPredicate(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "carpool_listings"
	}
	return alias + `.status = 'active'
	  AND EXISTS (
	    SELECT 1 FROM users owner
	    WHERE owner.id = ` + alias + `.owner_user_id
	      AND owner.account_status = 'active'
	  )`
}

func (s *Store) ListCarpoolListingsByOwner(ctx context.Context, ownerUserID, view string, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	return s.listCarpoolListingsPage(ctx, carpool.ListingFilter{View: view}, page, false, ownerUserID)
}

func (s *Store) GetCarpoolListingByOwner(ctx context.Context, ownerUserID, listingID string) (carpool.Listing, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, internalStoreError()
	}
	listing, err := s.getCarpoolListing(ctx, s.pool, listingID, false, false)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && listing.OwnerUserID != ownerUserID) {
		return carpool.Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) ListAdminCarpoolListings(ctx context.Context, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	return s.listCarpoolListingsPage(ctx, filter, page, false, "")
}

func (s *Store) listCarpoolListingsPage(ctx context.Context, filter carpool.ListingFilter, page domain.PageRequest, publicOnly bool, ownerUserID string) (domain.Page[carpool.Listing], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	sortMode := filter.NormalizedSort()
	var timePosition keysetPosition
	var scalarPosition scalarKeysetPosition
	var appErr *domain.AppError
	if sortMode == carpool.ListingSortUpdatedDesc {
		timePosition, appErr = decodeKeysetCursor(page.Cursor)
	} else {
		scalarPosition, appErr = decodeScalarKeysetCursor(page.Cursor, sortMode)
	}
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	if page.Cursor != "" && sortMode == carpool.ListingSortPriceAsc {
		if appErr := validateNonNegativeDecimalCursor(scalarPosition); appErr != nil {
			return domain.Page[carpool.Listing]{}, appErr
		}
	}

	conditions := make([]string, 0, 8)
	args := make([]any, 0, 10)
	addArgument := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if publicOnly {
		conditions = append(conditions, "("+publicCarpoolListingPredicate("listing_view")+")")
	}
	if strings.TrimSpace(ownerUserID) != "" {
		placeholder := addArgument(ownerUserID)
		conditions = append(conditions, "owner_user_id = "+placeholder+"::uuid")
	}
	if filter.None {
		conditions = append(conditions, "FALSE")
	}
	if len(filter.ProductPlanIDs) > 0 {
		placeholder := addArgument(filter.ProductPlanIDs)
		conditions = append(conditions, "product_plan_id::text = ANY("+placeholder+"::text[])")
	}
	if len(filter.Statuses) > 0 {
		placeholder := addArgument(filter.Statuses)
		conditions = append(conditions, "status = ANY("+placeholder+"::text[])")
	}
	if region := strings.TrimSpace(filter.Region); region != "" {
		placeholder := addArgument(region)
		conditions = append(conditions, "(region_code = "+placeholder+" OR region_name = "+placeholder+")")
	}
	switch strings.TrimSpace(filter.View) {
	case carpool.ListingViewPublic:
		conditions = append(conditions, "status = 'active'")
	case carpool.ListingViewExceptions:
		conditions = append(conditions, "status <> 'active'")
	case carpool.OwnerListingViewRecruiting:
		conditions = append(conditions, "status = 'active'")
	case carpool.OwnerListingViewServing:
		conditions = append(conditions, "status = 'stopped'", "governance_status = 'clear'")
	case carpool.OwnerListingViewHistory:
		conditions = append(conditions, "governance_status = 'removed'")
	case carpool.OwnerListingViewNeedsEdit:
		conditions = append(conditions, "status = 'draft'")
	}
	if strings.TrimSpace(filter.Risk) == carpool.ListingRiskHigh {
		conditions = append(conditions, "(risk_ack_required = true OR strpos(lower(COALESCE(review_reason, '')), '风险') > 0)")
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		placeholder := addArgument(strings.ToLower(query))
		conditions = append(conditions, `(
			strpos(lower(id::text), `+placeholder+`) > 0 OR
			strpos(lower(title), `+placeholder+`) > 0 OR
			strpos(lower(summary), `+placeholder+`) > 0 OR
			strpos(lower(region_name), `+placeholder+`) > 0 OR
			strpos(lower(owner_user_id::text), `+placeholder+`) > 0 OR
			strpos(lower(COALESCE(review_reason, '')), `+placeholder+`) > 0
		)`)
	}
	if page.Cursor != "" {
		switch sortMode {
		case carpool.ListingSortPriceAsc:
			valuePlaceholder := addArgument(scalarPosition.Value)
			idPlaceholder := addArgument(scalarPosition.ID)
			conditions = append(conditions, "(price_monthly_cny, id) > ("+valuePlaceholder+"::numeric, "+idPlaceholder+"::uuid)")
		case carpool.ListingSortSeatsDesc:
			availableSeats, err := strconv.Atoi(scalarPosition.Value)
			if err != nil || availableSeats < 0 {
				return domain.Page[carpool.Listing]{}, invalidPageCursorError()
			}
			valuePlaceholder := addArgument(availableSeats)
			idPlaceholder := addArgument(scalarPosition.ID)
			conditions = append(conditions, "("+carpoolAvailableSeatsExpression+", id) < ("+valuePlaceholder+", "+idPlaceholder+"::uuid)")
		default:
			timePlaceholder := addArgument(timePosition.Time)
			idPlaceholder := addArgument(timePosition.ID)
			conditions = append(conditions, "(updated_at, id) < ("+timePlaceholder+", "+idPlaceholder+"::uuid)")
		}
	}

	query := "SELECT " + carpoolListingColumns + " FROM " + carpoolListingViewSource
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	switch sortMode {
	case carpool.ListingSortPriceAsc:
		query += " ORDER BY price_monthly_cny ASC, id ASC"
	case carpool.ListingSortSeatsDesc:
		query += " ORDER BY " + carpoolAvailableSeatsExpression + " DESC, id DESC"
	default:
		query += " ORDER BY updated_at DESC, id DESC"
	}
	args = append(args, page.Limit+1)
	query += " LIMIT $" + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[carpool.Listing]{}, internalStoreError()
	}
	defer rows.Close()
	listings, appErr := scanCarpoolListings(rows)
	if appErr != nil {
		return domain.Page[carpool.Listing]{}, appErr
	}
	switch sortMode {
	case carpool.ListingSortPriceAsc:
		return pageFromScalarItems(listings, page, sortMode, func(item carpool.Listing) (string, string) {
			return item.PriceMonthlyCNY, item.ID
		}), nil
	case carpool.ListingSortSeatsDesc:
		return pageFromScalarItems(listings, page, sortMode, func(item carpool.Listing) (string, string) {
			return strconv.Itoa(item.AvailableSeats), item.ID
		}), nil
	default:
		return pageFromItems(listings, page, func(item carpool.Listing) (time.Time, string) { return item.UpdatedAt, item.ID }), nil
	}
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
	listing, appErr := s.updateCarpoolListingInTx(ctx, tx, input, ack, now)
	if appErr != nil {
		return carpool.Listing{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) UpdateCarpoolListingWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.UpdateListingInput, ack *carpool.RiskAcknowledgement, now time.Time, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	listing, appErr := s.updateCarpoolListingInTx(ctx, tx, input, ack, now)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	return listing, completion, nil
}

func (s *Store) updateCarpoolListingInTx(ctx context.Context, tx pgx.Tx, input carpool.UpdateListingInput, ack *carpool.RiskAcknowledgement, now time.Time) (carpool.Listing, *domain.AppError) {
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
	if listing.Status != carpool.ListingStatusDraft && listing.Status != carpool.ListingStatusActive && listing.Status != carpool.ListingStatusStopped {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能修改。")
	}
	if listing.Status != carpool.ListingStatusDraft && strings.TrimSpace(input.ProductPlanID) != listing.ProductPlanID {
		return carpool.Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Product plan immutable", "已发布车源不能更换产品或套餐，请新建车源。", "productPlanId", "immutable", "已发布车源不能更换产品或套餐。")
	}
	if input.BuyerSeatCapacity < listing.ActiveBuyerMembers+input.OfflineOccupiedSeats {
		return carpool.Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSeatUnavailable, "Seat unavailable", "买家总名额不能小于线下已占名额与平台有效成员数之和。", "buyerSeatCapacity", "below_occupied", "总名额不能小于已占名额。")
	}
	previousConditions := carpool.NewListingConditionsSnapshot(listing)
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, input.OwnerContactMethodID, input.OwnerUserID, contact.UsageScopeCarpoolOwner, "车主联系方式不可用、不属于当前用户或未允许拼车用途。"); appErr != nil {
		return carpool.Listing{}, appErr
	}
	var planPolicyVersion int64
	var planRiskNoticeCode string
	var planRiskAckRequired bool
	var planQuotaLabel string
	var planQuotaUnit string
	var planQuotaPeriod string
	err = tx.QueryRow(ctx, `
		SELECT plan.policy_version, COALESCE(plan.risk_notice_code, ''), plan.risk_ack_required,
		       plan.quota_label, plan.quota_unit, plan.quota_period
		FROM product_plans plan
		JOIN product_categories category ON category.id = plan.category_id
		WHERE plan.id = $1 AND plan.status = 'active' AND category.status = 'active'
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
	listing.DistributionMethod = strings.TrimSpace(input.DistributionMethod)
	listing.DistributionMethodNote = strings.TrimSpace(input.DistributionMethodNote)
	listing.ProvidesAdminAccount = input.ProvidesAdminAccount
	if listing.DistributionMethod == carpool.ListingDistributionMethodAccountLogin {
		listing.ProvidesAdminAccount = false
	}
	listing.RegionCode = strings.TrimSpace(input.RegionCode)
	listing.RegionName = strings.TrimSpace(input.RegionName)
	listing.SourceURL = strings.TrimSpace(input.SourceURL)
	listing.PriceMonthlyCNY = strings.TrimSpace(input.PriceMonthlyCNY)
	listing.ServiceMultiplier = strings.TrimSpace(input.ServiceMultiplier)
	listing.DailyQuotaAmount = nullStringPointer(input.DailyQuotaAmount)
	listing.WeeklyQuotaAmount = nullStringPointer(input.WeeklyQuotaAmount)
	listing.FollowsOfficialQuotaReset = input.FollowsOfficialQuotaReset
	listing.VPSRegion = nullStringPointer(input.VPSRegion)
	listing.SupportsMainlandChinaDirectConnection = input.SupportsMainlandChinaDirectConnection
	listing.OpeningChannelCode = nullStringPointer(input.OpeningChannelCode)
	listing.CustomOpeningChannel = nullStringPointer(input.CustomOpeningChannel)
	listing.PaymentMethodCode = nullStringPointer(input.PaymentMethodCode)
	listing.CustomPaymentMethod = nullStringPointer(input.CustomPaymentMethod)
	listing.QuotaLabel = strings.TrimSpace(planQuotaLabel)
	listing.QuotaUnit = strings.TrimSpace(planQuotaUnit)
	listing.QuotaPeriod = strings.TrimSpace(planQuotaPeriod)
	listing.BuyerSeatCapacity = input.BuyerSeatCapacity
	listing.OfflineOccupiedSeats = input.OfflineOccupiedSeats
	listing.PolicyVersion = planPolicyVersion
	listing.RiskNoticeCode = planRiskNoticeCode
	listing.RiskAckRequired = planRiskAckRequired
	conditionsChanged := !reflect.DeepEqual(previousConditions, carpool.NewListingConditionsSnapshot(listing))
	if conditionsChanged {
		listing.ConditionsVersion++
	}
	listing.UpdatedAt = now
	listing.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET product_plan_id = $2,
		    owner_contact_method_id = $3,
		    title = $4,
		    summary = $5,
		    access_arrangement = $6,
		    distribution_method = $7,
		    distribution_method_note = $8,
		    provides_admin_account = $9,
		    region_code = $10,
		    region_name = $11,
		    source_url = $12,
		    price_monthly_cny = $13,
		    service_multiplier = $14,
		    daily_spend_limit_usd = $15,
		    weekly_spend_limit_usd = $16,
		    follows_official_quota_reset = $17,
		    vps_region = $18,
		    supports_mainland_china_direct_connection = $19,
		    opening_channel_code = $20,
		    custom_opening_channel = $21,
		    payment_method_code = $22,
		    custom_payment_method = $23,
		    quota_label = $24,
		    quota_unit = $25,
		    quota_period = $26,
		    buyer_seat_capacity = $27,
		    offline_occupied_seats = $28,
		    policy_version = $29,
		    risk_notice_code = $30,
		    risk_ack_required = $31,
		    conditions_version = $32,
		    updated_at = $33,
		    version = $34
		WHERE id = $1
	`, listing.ID, listing.ProductPlanID, listing.OwnerContactMethodID, listing.Title, listing.Summary, listing.AccessArrangement,
		listing.DistributionMethod, listing.DistributionMethodNote, listing.ProvidesAdminAccount,
		listing.RegionCode, listing.RegionName,
		nullText(listing.SourceURL), listing.PriceMonthlyCNY, listing.ServiceMultiplier,
		listing.DailyQuotaAmount, listing.WeeklyQuotaAmount, listing.FollowsOfficialQuotaReset, listing.VPSRegion,
		listing.SupportsMainlandChinaDirectConnection, listing.OpeningChannelCode, listing.CustomOpeningChannel,
		listing.PaymentMethodCode, listing.CustomPaymentMethod, listing.QuotaLabel, listing.QuotaUnit, listing.QuotaPeriod,
		listing.BuyerSeatCapacity, listing.OfflineOccupiedSeats,
		listing.PolicyVersion, nullText(listing.RiskNoticeCode), listing.RiskAckRequired, listing.ConditionsVersion, listing.UpdatedAt, listing.Version)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if listing.CycleTerm != nil {
		if appErr := upsertCarpoolCycleTermInTx(ctx, tx, *listing.CycleTerm, now); appErr != nil {
			return carpool.Listing{}, appErr
		}
	}
	if conditionsChanged {
		conditionsSnapshot, marshalErr := json.Marshal(carpool.NewListingConditionsSnapshot(listing))
		if marshalErr != nil {
			return carpool.Listing{}, internalStoreError()
		}
		_, err = tx.Exec(ctx, `
		INSERT INTO carpool_listing_condition_versions (
			carpool_listing_id, conditions_version, conditions_snapshot, changed_by_user_id, created_at
		) VALUES ($1, $2, $3::jsonb, $4, $5)
		`, listing.ID, listing.ConditionsVersion, conditionsSnapshot, listing.OwnerUserID, now)
		if err != nil {
			return carpool.Listing{}, internalStoreError()
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
	if appErr := insertCarpoolListingEvent(ctx, tx, listing, listing.OwnerUserID, "user", "carpool_listing.updated", input.RequestID, now); appErr != nil {
		return carpool.Listing{}, appErr
	}
	return listing, nil
}

func (s *Store) UpdateCarpoolRecruitment(ctx context.Context, input carpool.RecruitmentInput, targetStatus string, now time.Time) (carpool.Listing, *domain.AppError) {
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
	if listing.GovernanceStatus != "clear" || (listing.Status != carpool.ListingStatusActive && listing.Status != carpool.ListingStatusStopped) {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不能修改招募状态。")
	}
	if targetStatus == carpool.ListingStatusActive && listing.AvailableSeats <= 0 {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前没有可用名额，不能继续招募。")
	}
	stopReason := ""
	if targetStatus == carpool.ListingStatusStopped {
		stopReason = "owner"
	}
	listing.Status = targetStatus
	listing.RecruitmentStopReason = stopReason
	listing.UpdatedAt = now
	listing.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET status = $2, recruitment_stop_reason = $3, updated_at = $4, version = $5
		WHERE id = $1
	`, listing.ID, listing.Status, listing.RecruitmentStopReason, listing.UpdatedAt, listing.Version)
	if err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if appErr := insertCarpoolListingEvent(ctx, tx, listing, input.OwnerUserID, "user", "carpool_listing.recruitment_updated", input.RequestID, now); appErr != nil {
		return carpool.Listing{}, appErr
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
	listing, appErr := s.submitCarpoolListingForReviewInTx(ctx, tx, user, input, now)
	if appErr != nil {
		return carpool.Listing{}, appErr
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
	listing, appErr := s.updateCarpoolListingReviewStatusInTx(ctx, tx, user, input, now)
	if appErr != nil {
		return carpool.Listing{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	return listing, nil
}

func (s *Store) SubmitCarpoolListingForReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input carpool.SubmitListingReviewInput, now time.Time, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	listing, appErr := s.submitCarpoolListingForReviewInTx(ctx, tx, user, input, now)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	return listing, completion, nil
}

func (s *Store) UpdateCarpoolListingReviewStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input carpool.ReviewInput, now time.Time, buildCompletion carpool.ListingCompletionBuilder) (carpool.Listing, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || !user.IsAdmin {
		if !user.IsAdmin {
			return carpool.Listing{}, idempotency.Completion{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
		}
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	listing, appErr := s.updateCarpoolListingReviewStatusInTx(ctx, tx, user, input, now)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return carpool.Listing{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Listing{}, idempotency.Completion{}, internalStoreError()
	}
	return listing, completion, nil
}

func (s *Store) submitCarpoolListingForReviewInTx(ctx context.Context, tx pgx.Tx, user auth.User, input carpool.SubmitListingReviewInput, now time.Time) (carpool.Listing, *domain.AppError) {
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
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, listing.OwnerContactMethodID, listing.OwnerUserID, contact.UsageScopeCarpoolOwner, "车主联系方式不可用、不属于当前用户或未允许拼车用途。"); appErr != nil {
		return carpool.Listing{}, appErr
	}
	listing.Status = carpool.ListingStatusActive
	listing.ReviewedByAdminID = ""
	listing.ReviewedAt = nil
	listing.ReviewReason = ""
	listing.UpdatedAt = now
	listing.Version++
	if _, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET status = $2, reviewed_by_admin_id = NULL, reviewed_at = NULL, review_reason = NULL,
		    updated_at = $3, version = $4
		WHERE id = $1
	`, listing.ID, listing.Status, listing.UpdatedAt, listing.Version); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	if appErr := insertCarpoolListingEvent(ctx, tx, listing, listing.OwnerUserID, "user", "carpool_listing.published", input.RequestID, now); appErr != nil {
		return carpool.Listing{}, appErr
	}
	return listing, nil
}

func (s *Store) updateCarpoolListingReviewStatusInTx(ctx context.Context, tx pgx.Tx, user auth.User, input carpool.ReviewInput, now time.Time) (carpool.Listing, *domain.AppError) {
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
	governanceAction := input.Action == "pause" || input.Action == "restore"
	if governanceAction {
		if !canUpdateCarpoolListingGovernance(listing, input.Action) {
			return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源治理状态不能执行该操作。")
		}
	} else if !canUpdateCarpoolListingStatus(listing.Status, input.Status, input.Action) {
		return carpool.Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能执行该审核动作。")
	}
	if input.Action == "approve" {
		if appErr := ensureCarpoolPlanAllowedForPublish(ctx, tx, listing.ProductPlanID); appErr != nil {
			return carpool.Listing{}, appErr
		}
	}
	if governanceAction {
		if input.Action == "pause" {
			listing.GovernanceStatus = "removed"
		} else {
			listing.GovernanceStatus = "clear"
		}
	} else {
		listing.Status = input.Status
	}
	listing.ReviewedByAdminID = user.ID
	listing.ReviewedAt = &now
	listing.ReviewReason = strings.TrimSpace(input.Reason)
	listing.UpdatedAt = now
	listing.Version++
	if _, err = tx.Exec(ctx, `
		UPDATE carpool_listings
			SET status = $2, governance_status = $3, reviewed_by_admin_id = $4,
			    reviewed_at = $5, review_reason = $6, updated_at = $7, version = $8
			WHERE id = $1
		`, listing.ID, listing.Status, listing.GovernanceStatus, listing.ReviewedByAdminID, listing.ReviewedAt, listing.ReviewReason, listing.UpdatedAt, listing.Version); err != nil {
		return carpool.Listing{}, internalStoreError()
	}
	eventType := carpoolListingReviewEventType(input.Action)
	if eventType == "" {
		return carpool.Listing{}, internalStoreError()
	}
	if appErr := insertCarpoolListingEvent(ctx, tx, listing, user.ID, "admin", eventType, input.RequestID, now); appErr != nil {
		return carpool.Listing{}, appErr
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
	if appErr := createCarpoolApplicationMutationInTx(ctx, tx, application, ack); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CreateCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, application carpool.Application, ack *carpool.RiskAcknowledgement, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
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
	if appErr := createCarpoolApplicationMutationInTx(ctx, tx, application, ack); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, application.CreatedAt); appErr != nil {
		return carpool.Application{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, idempotency.Completion{}, internalStoreError()
	}
	return application, completion, nil
}

func createCarpoolApplicationMutationInTx(ctx context.Context, tx pgx.Tx, application carpool.Application, ack *carpool.RiskAcknowledgement) *domain.AppError {
	if appErr := ensureActiveBusinessUsersInTx(ctx, tx, application.BuyerUserID, application.OwnerUserID); appErr != nil {
		return appErr
	}
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, application.BuyerContactMethodID, application.BuyerUserID, contact.UsageScopeBuyer, "买家联系方式不可用、不属于当前用户或未允许买家用途。"); appErr != nil {
		return appErr
	}
	var listingStatus, governanceStatus, lockedPlanID, planStatus, categoryStatus string
	var lockedOwnerID string
	var listingConditionsVersion int64
	var availableSeats int
	err := tx.QueryRow(ctx, `
		SELECT listing.status, listing.governance_status, listing.owner_user_id::text,
		       listing.conditions_version,
		       GREATEST(listing.buyer_seat_capacity - listing.offline_occupied_seats - listing.active_buyer_members, 0)::int,
		       plan.id::text, plan.status, category.status
		FROM carpool_listings listing
		JOIN product_plans plan ON plan.id = listing.product_plan_id
		JOIN product_categories category ON category.id = plan.category_id
		WHERE listing.id = $1
		FOR SHARE OF listing, plan, category
	`, application.CarpoolListingID).Scan(&listingStatus, &governanceStatus, &lockedOwnerID, &listingConditionsVersion, &availableSeats, &lockedPlanID, &planStatus, &categoryStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Carpool catalog unavailable", "车源或关联目录已不可用，请刷新后重试。")
	}
	if err != nil {
		return internalStoreError()
	}
	if listingStatus != carpool.ListingStatusActive || governanceStatus != "clear" || availableSeats < application.SeatCount || lockedOwnerID != application.OwnerUserID || lockedPlanID != application.ProductPlanID || planStatus != "active" || categoryStatus != "active" {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Carpool catalog unavailable", "车源或关联目录已退役、被阻断或发生变化，当前不能提交申请。")
	}
	if listingConditionsVersion != application.ConditionsVersionSnapshot {
		return domain.NewError(http.StatusConflict, domain.CodeVersionConflict, "Conditions changed", "车源条件已更新，请刷新后重新确认。")
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
	conditionsSnapshot, err := json.Marshal(application.ConditionsSnapshot)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_applications (
			id, carpool_listing_id, buyer_user_id, owner_user_id, product_plan_id,
			buyer_contact_method_id, status, seat_count, listing_title_snapshot,
			price_monthly_cny_snapshot, policy_version_snapshot, risk_notice_code_snapshot,
			conditions_version_snapshot, conditions_snapshot, accepted_conditions_version, conditions_accepted_at,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14::jsonb, $15, $16,
			$17, $18, $19
		)
	`, application.ID, application.CarpoolListingID, application.BuyerUserID, application.OwnerUserID, application.ProductPlanID,
		application.BuyerContactMethodID, application.Status, application.SeatCount, application.ListingTitleSnapshot,
		application.PriceMonthlyCNY, application.PolicyVersionSnapshot, nullText(application.RiskNoticeCode),
		application.ConditionsVersionSnapshot, conditionsSnapshot, application.AcceptedConditionsVersion, application.ConditionsAcceptedAt,
		application.CreatedAt, application.UpdatedAt, application.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.NewError(http.StatusConflict, domain.CodeActiveApplicationExists, "Active application exists", "你已提交过该车源的进行中申请。")
		}
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_application_condition_acceptances (
			carpool_application_id, conditions_version, conditions_snapshot,
			accepted_by_user_id, accepted_at, request_id
		) VALUES ($1, $2, $3::jsonb, $4, $5, $6)
	`, application.ID, application.AcceptedConditionsVersion, conditionsSnapshot, application.BuyerUserID, application.ConditionsAcceptedAt, application.RequestID)
	if err != nil {
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
	if appErr := insertCarpoolApplicationEventAndOwnerNotification(ctx, tx, application, application.BuyerUserID, "carpool_application.created", "收到新的上车申请", "你的车源收到新的上车申请，请查看申请详情。", application.RequestID, application.CreatedAt); appErr != nil {
		return appErr
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

func (s *Store) ListCarpoolApplicationsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]carpool.Application, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == carpool.JoinActorOwner {
			return s.ListCarpoolApplicationsByOwner(ctx, actor.UserID)
		}
		return s.ListCarpoolApplicationsByBuyer(ctx, actor.UserID)
	}
	where, args, ok := restrictedCarpoolWhere(actor, participantRole, "carpool_application", "carpool_applications", "")
	if !ok {
		return nil, carpoolRelationshipNotFound()
	}
	rows, err := s.pool.Query(ctx, `SELECT `+carpoolApplicationColumns+` FROM carpool_applications `+where+` ORDER BY updated_at DESC`, args...)
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

func (s *Store) GetCarpoolApplicationForActor(ctx context.Context, actor auth.BusinessActor, applicationID, participantRole string) (carpool.Application, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == carpool.JoinActorOwner {
			return s.GetCarpoolApplicationForOwner(ctx, actor.UserID, applicationID)
		}
		return s.GetCarpoolApplicationForBuyer(ctx, actor.UserID, applicationID)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := lockAccountGovernanceUser(ctx, tx, actor.UserID); appErr != nil {
		return carpool.Application{}, appErr
	}
	application, err := s.getCarpoolApplication(ctx, tx, applicationID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Application{}, carpoolRelationshipNotFound()
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := authorizeRestrictedCarpoolInTx(ctx, tx, actor, participantRole, "carpool_application", application.ID, application.BuyerUserID, application.OwnerUserID, application.CreatedAt); appErr != nil {
		return carpool.Application{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
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

func (s *Store) ConfirmCarpoolApplicationConditions(ctx context.Context, input carpool.ConfirmApplicationConditionsInput, now time.Time) (carpool.Application, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	defer rollback(ctx, tx)

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
	if application.Status != carpool.ApplicationStatusPendingOwner {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请不能更新条件确认。")
	}
	listing, err := s.getCarpoolListing(ctx, tx, application.CarpoolListingID, true, true)
	if errors.Is(err, pgx.ErrNoRows) || listing.GovernanceStatus != "clear" || (listing.Status != carpool.ListingStatusActive && listing.Status != carpool.ListingStatusStopped) {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Listing unavailable", "当前车源不可确认。")
	}
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	application.ConditionsVersionSnapshot = listing.ConditionsVersion
	application.ConditionsSnapshot = carpool.NewListingConditionsSnapshot(listing)
	application.AcceptedConditionsVersion = listing.ConditionsVersion
	application.ConditionsAcceptedAt = now
	application.ListingTitleSnapshot = listing.Title
	application.PriceMonthlyCNY = listing.PriceMonthlyCNY
	application.UpdatedAt = now
	application.Version++
	snapshot, err := json.Marshal(application.ConditionsSnapshot)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET listing_title_snapshot = $2,
		    price_monthly_cny_snapshot = $3,
		    conditions_version_snapshot = $4,
		    conditions_snapshot = $5::jsonb,
		    accepted_conditions_version = $4,
		    conditions_accepted_at = $6,
		    updated_at = $6,
		    version = $7
		WHERE id = $1
	`, application.ID, application.ListingTitleSnapshot, application.PriceMonthlyCNY, application.ConditionsVersionSnapshot, snapshot, now, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_application_condition_acceptances (
			carpool_application_id, conditions_version, conditions_snapshot,
			accepted_by_user_id, accepted_at, request_id
		) VALUES ($1, $2, $3::jsonb, $4, $5, $6)
		ON CONFLICT (carpool_application_id, conditions_version) DO NOTHING
	`, application.ID, application.AcceptedConditionsVersion, snapshot, application.BuyerUserID, now, input.RequestID)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := insertCarpoolApplicationEventAndOwnerNotification(ctx, tx, application, application.BuyerUserID, "carpool_application.conditions_confirmed", "买家已确认最新车源条件", "买家已确认最新版车源条件，可以继续处理上车申请。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, internalStoreError()
	}
	return application, nil
}

func (s *Store) RejectCarpoolApplication(ctx context.Context, input carpool.RejectApplicationInput, now time.Time) (carpool.Application, *domain.AppError) {
	if s == nil || s.pool == nil {
		return carpool.Application{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	application, appErr := s.rejectCarpoolApplicationInTx(ctx, tx, input, now)
	if appErr != nil {
		return carpool.Application{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Application{}, internalStoreError()
	}
	return application, nil
}

func (s *Store) RejectCarpoolApplicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.RejectApplicationInput, now time.Time, buildCompletion carpool.ApplicationCompletionBuilder) (carpool.Application, idempotency.Completion, *domain.AppError) {
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
	application, appErr := s.rejectCarpoolApplicationInTx(ctx, tx, input, now)
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

func (s *Store) rejectCarpoolApplicationInTx(ctx context.Context, tx pgx.Tx, input carpool.RejectApplicationInput, now time.Time) (carpool.Application, *domain.AppError) {
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

func (s *Store) ListCarpoolMembershipsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]carpool.Membership, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == carpool.JoinActorOwner {
			return s.ListCarpoolMembershipsByOwner(ctx, actor.UserID)
		}
		return s.ListCarpoolMembershipsByBuyer(ctx, actor.UserID)
	}
	where, args, ok := restrictedCarpoolWhere(actor, participantRole, "carpool_membership", "carpool_memberships", "")
	if !ok {
		return nil, carpoolRelationshipNotFound()
	}
	rows, err := s.pool.Query(ctx, `SELECT `+carpoolMembershipColumns+` FROM carpool_memberships `+where+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanCarpoolMemberships(rows)
}

func (s *Store) GetCarpoolMembershipForActor(ctx context.Context, actor auth.BusinessActor, membershipID, participantRole string) (carpool.Membership, *domain.AppError) {
	if actor.Audience != auth.SessionAudienceRestrictedBusiness {
		return carpool.Membership{}, carpoolRelationshipNotFound()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := lockAccountGovernanceUser(ctx, tx, actor.UserID); appErr != nil {
		return carpool.Membership{}, appErr
	}
	membership, err := s.getCarpoolMembership(ctx, tx, membershipID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Membership{}, carpoolRelationshipNotFound()
	}
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	if appErr := authorizeRestrictedCarpoolInTx(ctx, tx, actor, participantRole, "carpool_membership", membership.ID, membership.BuyerUserID, membership.OwnerUserID, membership.CreatedAt); appErr != nil {
		return carpool.Membership{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	return membership, nil
}

func restrictedCarpoolWhere(actor auth.BusinessActor, participantRole, resourceType, tableName, resourceID string) (string, []any, bool) {
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || actor.UserID == "" || actor.GovernanceActionID == "" || actor.GovernanceVersion < 1 || actor.RestrictionEffectiveAt.IsZero() {
		return "", nil, false
	}
	participantColumn := "buyer_user_id"
	if participantRole == carpool.JoinActorOwner {
		participantColumn = "owner_user_id"
	} else if participantRole != carpool.JoinActorBuyer {
		return "", nil, false
	}
	where := `WHERE ` + tableName + `.` + participantColumn + ` = $1
		AND ` + tableName + `.created_at <= $4
		AND EXISTS (
			SELECT 1
			FROM account_governance_resource_dispositions disposition
			JOIN account_governance_disposition_actions link ON link.disposition_id = disposition.id
			JOIN account_governance_actions action ON action.id = link.governance_action_id
			JOIN users user_account ON user_account.id = action.target_user_id
			WHERE disposition.resource_type = $5
			  AND disposition.resource_id = ` + tableName + `.id
			  AND disposition.result = 'preserved'
			  AND link.governance_action_id = $2
			  AND action.target_user_id = $1
			  AND action.governance_version = $3
			  AND action.effective_at = $4
			  AND action.status = 'effective'
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = action.id
			  AND user_account.governance_version = action.governance_version
		)`
	args := []any{actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt, resourceType}
	if strings.TrimSpace(resourceID) != "" {
		where += ` AND ` + tableName + `.id = $6`
		args = append(args, resourceID)
	}
	return where, args, true
}

func authorizeRestrictedCarpoolInTx(ctx context.Context, tx pgx.Tx, actor auth.BusinessActor, participantRole, resourceType, resourceID, buyerUserID, ownerUserID string, createdAt time.Time) *domain.AppError {
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || createdAt.After(actor.RestrictionEffectiveAt) {
		return carpoolRelationshipNotFound()
	}
	if participantRole == carpool.JoinActorBuyer && buyerUserID != actor.UserID || participantRole == carpool.JoinActorOwner && ownerUserID != actor.UserID {
		return carpoolRelationshipNotFound()
	}
	if participantRole != carpool.JoinActorBuyer && participantRole != carpool.JoinActorOwner {
		return carpoolRelationshipNotFound()
	}
	var authorized bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users user_account
			JOIN account_governance_actions action ON action.id = user_account.current_governance_action_id
			JOIN account_governance_disposition_actions link ON link.governance_action_id = action.id
			JOIN account_governance_resource_dispositions disposition ON disposition.id = link.disposition_id
			WHERE user_account.id = $1
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = $2
			  AND user_account.governance_version = $3
			  AND action.status = 'effective'
			  AND action.effective_at = $4
			  AND disposition.resource_type = $5
			  AND disposition.resource_id = $6
			  AND disposition.result = 'preserved'
		)
	`, actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt, resourceType, resourceID).Scan(&authorized); err != nil {
		return internalStoreError()
	}
	if !authorized {
		return carpoolRelationshipNotFound()
	}
	return nil
}

func carpoolRelationshipNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool relationship not found", "拼车关系不存在。")
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
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		if appErr := lockAccountGovernanceUser(ctx, tx, input.ActorUserID); appErr != nil {
			return carpool.Membership{}, idempotency.Completion{}, appErr
		}
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

func (s *Store) UpdateCarpoolMembershipOwnerNoteWithIdempotency(ctx context.Context, entry idempotency.Entry, input carpool.UpdateMembershipOwnerNoteInput, now time.Time, buildCompletion carpool.MembershipCompletionBuilder) (carpool.Membership, idempotency.Completion, *domain.AppError) {
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
	if input.OwnerAudience == auth.SessionAudienceRestrictedBusiness {
		if appErr := lockAccountGovernanceUser(ctx, tx, input.OwnerUserID); appErr != nil {
			return carpool.Membership{}, idempotency.Completion{}, appErr
		}
	}
	membership, err := s.getCarpoolMembership(ctx, tx, input.MembershipID, true)
	if errors.Is(err, pgx.ErrNoRows) || membership.OwnerUserID != input.OwnerUserID {
		return carpool.Membership{}, idempotency.Completion{}, carpoolRelationshipNotFound()
	}
	if err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
	}
	if input.OwnerAudience == auth.SessionAudienceRestrictedBusiness {
		actor := auth.BusinessActor{UserID: input.OwnerUserID, Audience: input.OwnerAudience, GovernanceActionID: input.GovernanceActionID, GovernanceVersion: input.GovernanceVersion, RestrictionEffectiveAt: input.RestrictionEffectiveAt}
		if appErr := authorizeRestrictedCarpoolInTx(ctx, tx, actor, carpool.JoinActorOwner, "carpool_membership", membership.ID, membership.BuyerUserID, membership.OwnerUserID, membership.CreatedAt); appErr != nil {
			return carpool.Membership{}, idempotency.Completion{}, appErr
		}
	} else if input.OwnerAudience != "" && input.OwnerAudience != auth.SessionAudienceNormal {
		return carpool.Membership{}, idempotency.Completion{}, carpoolRelationshipNotFound()
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return carpool.Membership{}, idempotency.Completion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	membership.OwnerNote = strings.TrimSpace(input.Note)
	membership.UpdatedAt = now
	membership.Version++
	if _, err := tx.Exec(ctx, `
		UPDATE carpool_memberships
		SET owner_note = $2, updated_at = $3, version = $4
		WHERE id = $1
	`, membership.ID, membership.OwnerNote, now, membership.Version); err != nil {
		return carpool.Membership{}, idempotency.Completion{}, internalStoreError()
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

const carpoolAvailableSeatsExpression = `GREATEST(buyer_seat_capacity - offline_occupied_seats - active_buyer_members, 0)`

const carpoolListingColumns = `
	id::text, owner_user_id::text, product_plan_id::text, owner_contact_method_id::text, title, summary, access_arrangement,
	distribution_method, distribution_method_note, provides_admin_account,
	region_code, region_name,
	COALESCE(cycle_term_id::text, ''), COALESCE(cycle_billing_period, ''), cycle_start_day, COALESCE(cycle_notice_days, 0),
	COALESCE(cycle_exit_policy, ''), COALESCE(cycle_usage_rules, ''), COALESCE(cycle_version, 0),
	COALESCE(cycle_created_at, created_at), COALESCE(cycle_updated_at, updated_at),
	COALESCE(source_url, ''),
	COALESCE((
	  SELECT CASE
	    WHEN verification.source_url IS DISTINCT FROM listing_view.source_url
	      OR verification.expected_external_user_id IS DISTINCT FROM COALESCE((
	        SELECT binding.linux_do_user_id
	        FROM linux_do_bindings binding
	        WHERE binding.user_id = listing_view.owner_user_id
	      ), '')
	    THEN 'pending'
	    WHEN verification.status = 'verified'
	      AND verification.expires_at IS NOT NULL
	      AND verification.expires_at <= now()
	    THEN 'expired'
	    ELSE verification.status
	  END
	  FROM source_author_verifications verification
	  WHERE verification.resource_type = 'carpool'
	    AND verification.resource_id = listing_view.id
	), 'not_submitted'),
	(
	  SELECT verification.verified_at
	  FROM source_author_verifications verification
	  WHERE verification.resource_type = 'carpool'
	    AND verification.resource_id = listing_view.id
	    AND verification.source_url = listing_view.source_url
	    AND verification.expected_external_user_id = COALESCE((
	      SELECT binding.linux_do_user_id
	      FROM linux_do_bindings binding
	      WHERE binding.user_id = listing_view.owner_user_id
	    ), '')
	    AND verification.status = 'verified'
	),
	(
	  SELECT verification.expires_at
	  FROM source_author_verifications verification
	  WHERE verification.resource_type = 'carpool'
	    AND verification.resource_id = listing_view.id
	    AND verification.source_url = listing_view.source_url
	    AND verification.expected_external_user_id = COALESCE((
	      SELECT binding.linux_do_user_id
	      FROM linux_do_bindings binding
	      WHERE binding.user_id = listing_view.owner_user_id
	    ), '')
	    AND verification.status = 'verified'
	),
	price_monthly_cny::text, service_multiplier::text,
	daily_spend_limit_usd::text, weekly_spend_limit_usd::text, follows_official_quota_reset, vps_region,
	supports_mainland_china_direct_connection, opening_channel_code, custom_opening_channel,
	payment_method_code, custom_payment_method,
	quota_label, quota_unit, quota_period, buyer_seat_capacity, offline_occupied_seats, active_buyer_members,
	status, governance_status, recruitment_stop_reason, conditions_version,
	COALESCE(reviewed_by_admin_id::text, ''), reviewed_at, COALESCE(review_reason, ''),
	policy_version, COALESCE(risk_notice_code, ''), risk_ack_required,
	` + carpoolAvailableSeatsExpression + `::int AS available_seats,
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
	       t.updated_at AS cycle_updated_at
	FROM carpool_listings l
	LEFT JOIN carpool_cycle_terms t ON t.carpool_listing_id = l.id
) listing_view`

const carpoolApplicationColumns = `
	id::text, carpool_listing_id::text, buyer_user_id::text, owner_user_id::text,
	product_plan_id::text, buyer_contact_method_id::text,
	status,
	seat_count,
	listing_title_snapshot, price_monthly_cny_snapshot::text, policy_version_snapshot,
	COALESCE(risk_notice_code_snapshot, ''), COALESCE(contact_session_id::text, ''),
	conditions_version_snapshot, conditions_snapshot, accepted_conditions_version, conditions_accepted_at,
	joined_at, COALESCE(decision_reason, ''), decided_at, created_at, updated_at, version
`

const carpoolMembershipColumns = `
	id::text, carpool_listing_id::text, carpool_application_id::text, COALESCE(cycle_term_id::text, ''), buyer_user_id::text,
	owner_user_id::text, product_plan_id::text, status, seat_count,
	price_monthly_cny_snapshot::text, policy_version_snapshot, COALESCE(risk_notice_code_snapshot, ''),
	conditions_version_snapshot, conditions_snapshot, joined_at,
	ended_at, ended_reason, COALESCE(ended_by_user_id::text, ''), COALESCE(owner_note, ''),
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
	var dailyQuotaAmount *string
	var weeklyQuotaAmount *string
	var followsOfficialQuotaReset *bool
	var vpsRegion *string
	var supportsMainlandChinaDirectConnection *bool
	var openingChannelCode *string
	var customOpeningChannel *string
	var paymentMethodCode *string
	var customPaymentMethod *string
	if err := row.Scan(
		&listing.ID,
		&listing.OwnerUserID,
		&listing.ProductPlanID,
		&listing.OwnerContactMethodID,
		&listing.Title,
		&listing.Summary,
		&listing.AccessArrangement,
		&listing.DistributionMethod,
		&listing.DistributionMethodNote,
		&listing.ProvidesAdminAccount,
		&listing.RegionCode,
		&listing.RegionName,
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
		&listing.SourceAuthorVerification.Status,
		&listing.SourceAuthorVerification.VerifiedAt,
		&listing.SourceAuthorVerification.ExpiresAt,
		&listing.PriceMonthlyCNY,
		&listing.ServiceMultiplier,
		&dailyQuotaAmount,
		&weeklyQuotaAmount,
		&followsOfficialQuotaReset,
		&vpsRegion,
		&supportsMainlandChinaDirectConnection,
		&openingChannelCode,
		&customOpeningChannel,
		&paymentMethodCode,
		&customPaymentMethod,
		&listing.QuotaLabel,
		&listing.QuotaUnit,
		&listing.QuotaPeriod,
		&listing.BuyerSeatCapacity,
		&listing.OfflineOccupiedSeats,
		&listing.ActiveBuyerMembers,
		&listing.Status,
		&listing.GovernanceStatus,
		&listing.RecruitmentStopReason,
		&listing.ConditionsVersion,
		&listing.ReviewedByAdminID,
		&listing.ReviewedAt,
		&listing.ReviewReason,
		&listing.PolicyVersion,
		&listing.RiskNoticeCode,
		&listing.RiskAckRequired,
		&listing.AvailableSeats,
		&listing.CreatedAt,
		&listing.UpdatedAt,
		&listing.Version,
	); err != nil {
		return err
	}
	listing.DailyQuotaAmount = dailyQuotaAmount
	listing.WeeklyQuotaAmount = weeklyQuotaAmount
	listing.FollowsOfficialQuotaReset = followsOfficialQuotaReset
	listing.VPSRegion = vpsRegion
	listing.SupportsMainlandChinaDirectConnection = supportsMainlandChinaDirectConnection
	listing.OpeningChannelCode = openingChannelCode
	listing.CustomOpeningChannel = customOpeningChannel
	listing.PaymentMethodCode = paymentMethodCode
	listing.CustomPaymentMethod = customPaymentMethod
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
	var conditionsSnapshot []byte
	if err := row.Scan(
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
		&application.ConditionsVersionSnapshot,
		&conditionsSnapshot,
		&application.AcceptedConditionsVersion,
		&application.ConditionsAcceptedAt,
		&application.JoinedAt,
		&application.DecisionReason,
		&application.DecidedAt,
		&application.CreatedAt,
		&application.UpdatedAt,
		&application.Version,
	); err != nil {
		return err
	}
	return json.Unmarshal(conditionsSnapshot, &application.ConditionsSnapshot)
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
	var conditionsSnapshot []byte
	if err := row.Scan(
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
		&membership.ConditionsVersionSnapshot,
		&conditionsSnapshot,
		&membership.JoinedAt,
		&membership.EndedAt,
		&membership.EndedReason,
		&membership.EndedByUserID,
		&membership.OwnerNote,
		&membership.CreatedAt,
		&membership.UpdatedAt,
		&membership.Version,
	); err != nil {
		return err
	}
	return json.Unmarshal(conditionsSnapshot, &membership.ConditionsSnapshot)
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
			       t.updated_at AS cycle_updated_at
			FROM carpool_listings l
			LEFT JOIN carpool_cycle_terms t ON t.carpool_listing_id = l.id
			WHERE l.id = $1
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
	if appErr := ensureActiveBusinessUsersInTx(ctx, tx, application.BuyerUserID, application.OwnerUserID); appErr != nil {
		return carpool.Application{}, appErr
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
	if listing.AvailableSeats < application.SeatCount {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可用名额。")
	}
	if listing.Status != carpool.ListingStatusActive || listing.GovernanceStatus != "clear" {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	if application.AcceptedConditionsVersion != listing.ConditionsVersion {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeVersionConflict, "Conditions changed", "车源条件已更新，需要买家先确认最新条件。")
	}

	_, buyerVersion, appErr := lockContactVersionForOwnerAndScope(ctx, tx, application.BuyerContactMethodID, application.BuyerUserID, contact.UsageScopeBuyer, "买家联系方式不可用、不属于当前用户或未允许买家用途。")
	if appErr != nil {
		return carpool.Application{}, appErr
	}
	_, ownerVersion, appErr := lockContactVersionForOwnerAndScope(ctx, tx, listing.OwnerContactMethodID, input.OwnerUserID, contact.UsageScopeCarpoolOwner, "车主联系方式不可用、不属于当前用户或未允许拼车用途。")
	if appErr != nil {
		return carpool.Application{}, appErr
	}

	sessionID := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_sessions (id, buyer_user_id, seller_user_id, opens_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, NULL, 'open', $4)
	`, sessionID, application.BuyerUserID, application.OwnerUserID, now)
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

	application.Status = carpool.ApplicationStatusJoined
	application.ContactSessionID = sessionID
	application.JoinedAt = &now
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	_, err = tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2,
		    contact_session_id = $3,
		    joined_at = $4,
		    decided_at = $5,
		    updated_at = $6,
		    version = $7
		WHERE id = $1
	`, application.ID, application.Status, application.ContactSessionID, application.JoinedAt, application.DecidedAt, application.UpdatedAt, application.Version)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	conditionsSnapshot, err := json.Marshal(application.ConditionsSnapshot)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO carpool_memberships (
			carpool_listing_id, carpool_application_id, cycle_term_id,
			buyer_user_id, owner_user_id, product_plan_id, status, seat_count,
			price_monthly_cny_snapshot, policy_version_snapshot, risk_notice_code_snapshot,
				conditions_version_snapshot, conditions_snapshot, joined_at,
				owner_note, created_at, updated_at, version
		) VALUES (
			$1, $2, NULLIF($3, '')::uuid,
			$4, $5, $6, 'active', $7,
			$8, $9, $10,
				$11, $12::jsonb, $13,
				'', $13, $13, 1
		)
	`, application.CarpoolListingID, application.ID, cycleTermID(listing),
		application.BuyerUserID, application.OwnerUserID, application.ProductPlanID, application.SeatCount,
		application.PriceMonthlyCNY, application.PolicyVersionSnapshot, nullText(application.RiskNoticeCode),
		application.ConditionsVersionSnapshot, conditionsSnapshot, now)
	if err != nil {
		if isUniqueViolation(err) {
			return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeActiveMembershipExists, "Active membership exists", "你已是该车源的成员。")
		}
		return carpool.Application{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		UPDATE carpool_listings
		SET active_buyer_members = active_buyer_members + $2,
		    status = CASE
		      WHEN buyer_seat_capacity - offline_occupied_seats - active_buyer_members - $2 <= 0 THEN 'stopped'
		      ELSE status
		    END,
		    recruitment_stop_reason = CASE
		      WHEN buyer_seat_capacity - offline_occupied_seats - active_buyer_members - $2 <= 0 THEN 'full'
		      ELSE recruitment_stop_reason
		    END,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
	`, application.CarpoolListingID, application.SeatCount, now)
	if err != nil {
		return carpool.Application{}, internalStoreError()
	}
	if appErr := insertCarpoolApplicationEventAndNotification(ctx, tx, application, input.OwnerUserID, "carpool_application.joined", "上车申请已确认", "车主已确认你上车，成员关系和双方联系方式现已开放。", input.RequestID, now); appErr != nil {
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
	if application.Status != carpool.ApplicationStatusPendingOwner {
		return carpool.Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能取消；已加入后请退出拼车。")
	}
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
	if appErr := insertCarpoolApplicationEventAndOwnerNotification(ctx, tx, application, input.BuyerUserID, "carpool_application.cancelled_by_buyer", "上车申请已取消", "买家已取消待处理的上车申请，请查看申请详情。", input.RequestID, now); appErr != nil {
		return carpool.Application{}, appErr
	}
	return application, nil
}

func (s *Store) endCarpoolMembershipInTx(ctx context.Context, tx pgx.Tx, input carpool.EndMembershipInput, now time.Time) (carpool.Membership, *domain.AppError) {
	membership, err := s.getCarpoolMembership(ctx, tx, input.MembershipID, true)
	if errors.Is(err, pgx.ErrNoRows) || !canActorEndCarpoolMembership(membership, input.ActorUserID, input.ActorRole, input.TargetStatus) {
		return carpool.Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if err != nil {
		return carpool.Membership{}, internalStoreError()
	}
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		actor := auth.BusinessActor{UserID: input.ActorUserID, Audience: input.ActorAudience, GovernanceActionID: input.GovernanceActionID, GovernanceVersion: input.GovernanceVersion, RestrictionEffectiveAt: input.RestrictionEffectiveAt}
		if appErr := authorizeRestrictedCarpoolInTx(ctx, tx, actor, input.ActorRole, "carpool_membership", membership.ID, membership.BuyerUserID, membership.OwnerUserID, membership.CreatedAt); appErr != nil {
			return carpool.Membership{}, appErr
		}
	} else if input.ActorAudience != "" && input.ActorAudience != auth.SessionAudienceNormal {
		return carpool.Membership{}, carpoolRelationshipNotFound()
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

func insertCarpoolListingEvent(ctx context.Context, tx pgx.Tx, listing carpool.Listing, actorUserID, actorKind, eventType, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	metadata, err := json.Marshal(map[string]string{
		"status":           listing.Status,
		"governanceStatus": listing.GovernanceStatus,
	})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'carpool_listing', $2, $3, $4, $5, $6, $7, $8, $9)
	`, uuid.NewString(), listing.ID, eventType, actorUserID, actorKind, listing.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func carpoolListingReviewEventType(action string) string {
	switch strings.TrimSpace(action) {
	case "approve":
		return "carpool_listing.published"
	case "reject":
		return "carpool_listing.rejected"
	case "request_changes":
		return "carpool_listing.changes_requested"
	case "pause":
		return "carpool_listing.paused"
	case "restore":
		return "carpool_listing.resumed"
	default:
		return ""
	}
}

func insertCarpoolApplicationEventAndTargetNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, notifyUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	return insertCarpoolApplicationEventAndTargetNotificationURL(ctx, tx, application, actorUserID, notifyUserID, eventType, title, body, requestID, now, "/my/rides/"+application.ID)
}

func insertCarpoolApplicationEventAndOwnerNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, actorUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	return insertCarpoolApplicationEventAndTargetNotificationURL(ctx, tx, application, actorUserID, application.OwnerUserID, eventType, title, body, requestID, now, "/merchant/carpool-applications/"+application.ID)
}

func insertSystemCarpoolApplicationEventAndTargetNotification(ctx context.Context, tx pgx.Tx, application carpool.Application, notifyUserID, eventType, title, body, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "system:carpool-reservation-expiry"
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
		VALUES ($1, 'carpool_application', $2, $3, NULL, 'system', $4, $5, $6, $7)
	`, eventID, application.ID, eventType, application.Version, requestID, metadata, now)
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
	`, notifyUserID, eventType, title, body, application.ID, "/merchant/carpool-applications/"+application.ID,
		eventID, "carpool_application:"+application.ID+":"+application.Status+":"+notifyUserID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
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
		    ends_at = $3
		WHERE id = $1
		  AND status = 'open'
	`, sessionID, status, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func cycleTermID(listing carpool.Listing) string {
	if listing.CycleTerm == nil {
		return ""
	}
	return listing.CycleTerm.ID
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
		return nextStatus == carpool.ListingStatusChangesRequested && (currentStatus == carpool.ListingStatusPendingReview || currentStatus == carpool.ListingStatusActive)
	}
	switch nextStatus {
	case carpool.ListingStatusActive:
		return currentStatus == carpool.ListingStatusPendingReview
	case carpool.ListingStatusRejected:
		return currentStatus == carpool.ListingStatusPendingReview
	case carpool.ListingStatusChangesRequested:
		return currentStatus == carpool.ListingStatusPendingReview || currentStatus == carpool.ListingStatusActive
	default:
		return false
	}
}

func canUpdateCarpoolListingGovernance(listing carpool.Listing, action string) bool {
	published := listing.Status == carpool.ListingStatusActive || listing.Status == carpool.ListingStatusStopped
	switch action {
	case "pause":
		return published && listing.GovernanceStatus == "clear"
	case "restore":
		return published && listing.GovernanceStatus == "removed"
	default:
		return false
	}
}

func ensureCarpoolPlanAllowedForPublish(ctx context.Context, q queryer, productPlanID string) *domain.AppError {
	var publishPolicy string
	err := q.QueryRow(ctx, `
		SELECT plan.publish_policy
		FROM product_plans plan
		JOIN product_categories category ON category.id = plan.category_id
		WHERE plan.id = $1 AND plan.status = 'active' AND category.status = 'active'
	`, productPlanID).Scan(&publishPolicy)
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
