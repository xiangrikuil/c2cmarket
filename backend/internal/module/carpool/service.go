package carpool

import (
	"context"
	"math/big"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type ProductPlanResolver interface {
	ProductPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError)
}

type ApplicationCreateGuard func(ctx context.Context, user auth.User) *domain.AppError

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	catalog     ProductPlanResolver
	contact     *contact.Service
	idempotency *idempotency.Service
	createGuard ApplicationCreateGuard

	listings               map[string]Listing
	listingOrder           []string
	applications           map[string]Application
	appOrder               []string
	memberships            map[string]Membership
	memberByApp            map[string]string
	memberOrder            []string
	listingAuditEvents     []ListingAuditEvent
	applicationAuditEvents []ApplicationAuditEvent
}

// SetApplicationCreateGuard 注入必须在幂等重放判定后执行的可变准入检查。
func (s *Service) SetApplicationCreateGuard(guard ApplicationCreateGuard) {
	s.createGuard = guard
}

func NewService(repo Repository, catalogResolver ProductPlanResolver, contactService *contact.Service, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if contactService == nil {
		contactService = contact.NewService(nil, now)
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:          now,
		repo:         repo,
		catalog:      catalogResolver,
		contact:      contactService,
		idempotency:  idempotencyService,
		listings:     make(map[string]Listing),
		applications: make(map[string]Application),
		memberships:  make(map[string]Membership),
		memberByApp:  make(map[string]string),
	}
}

func (s *Service) CreateListing(ctx context.Context, user auth.User, input CreateListingInput) (Listing, *domain.AppError) {
	input.OwnerUserID = user.ID
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return Listing{}, appErr
	}
	if err := validateCreateListingInput(input, plan); err != nil {
		return Listing{}, err
	}

	now := s.now()
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
	listing := newListing(user.ID, input, plan, ListingStatusDraft, now)

	if s.repo != nil {
		if appErr := s.repo.CreateCarpoolListing(ctx, listing, ack); appErr != nil {
			return Listing{}, appErr
		}
		return listing, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		return Listing{}, contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.created", input.RequestID)
	return listing, nil
}

func (s *Service) CreateListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateListingInput, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, bool, *domain.AppError) {
	if err := idempotency.ValidateKey(strings.TrimSpace(key)); err != nil {
		return Listing{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := validateCreateListingInput(input, plan); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	now := s.now()
	listing := newListing(user.ID, input, plan, ListingStatusDraft, now)
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Listing{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		listing, completion, appErr := s.repo.CreateCarpoolListingWithIdempotency(ctx, *entry, listing, ack, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Listing{}, idempotency.Completion{}, false, appErr
		}
		return listing, completion, true, nil
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	s.mu.Lock()
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.created", input.RequestID)
	s.mu.Unlock()
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	return listing, completion, true, nil
}

func newListing(ownerUserID string, input CreateListingInput, plan catalog.ProductPlan, status string, now time.Time) Listing {
	listing := Listing{
		ID:                   uuid.NewString(),
		OwnerUserID:          ownerUserID,
		ProductPlanID:        plan.ID,
		OwnerContactMethodID: strings.TrimSpace(input.OwnerContactMethodID),
		CycleTerm: &CycleTerm{
			ID:            uuid.NewString(),
			OwnerUserID:   ownerUserID,
			BillingPeriod: strings.TrimSpace(input.CycleTerm.BillingPeriod),
			CycleStartDay: input.CycleTerm.CycleStartDay,
			NoticeDays:    input.CycleTerm.NoticeDays,
			ExitPolicy:    strings.TrimSpace(input.CycleTerm.ExitPolicy),
			UsageRules:    strings.TrimSpace(input.CycleTerm.UsageRules),
			Version:       1,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		Title:                                 strings.TrimSpace(input.Title),
		Summary:                               strings.TrimSpace(input.Summary),
		AccessArrangement:                     strings.TrimSpace(input.AccessArrangement),
		DistributionMethod:                    strings.TrimSpace(input.DistributionMethod),
		DistributionMethodNote:                strings.TrimSpace(input.DistributionMethodNote),
		ProvidesAdminAccount:                  normalizedProvidesAdminAccount(input.DistributionMethod, input.ProvidesAdminAccount),
		RegionCode:                            strings.TrimSpace(input.RegionCode),
		RegionName:                            strings.TrimSpace(input.RegionName),
		SourceURL:                             strings.TrimSpace(input.SourceURL),
		PriceMonthlyCNY:                       strings.TrimSpace(input.PriceMonthlyCNY),
		ServiceMultiplier:                     strings.TrimSpace(input.ServiceMultiplier),
		DailyQuotaAmount:                      optionalString(input.DailyQuotaAmount),
		WeeklyQuotaAmount:                     optionalString(input.WeeklyQuotaAmount),
		FollowsOfficialQuotaReset:             input.FollowsOfficialQuotaReset,
		VPSRegion:                             optionalString(input.VPSRegion),
		SupportsMainlandChinaDirectConnection: input.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    optionalString(input.OpeningChannelCode),
		CustomOpeningChannel:                  optionalString(input.CustomOpeningChannel),
		PaymentMethodCode:                     optionalString(input.PaymentMethodCode),
		CustomPaymentMethod:                   optionalString(input.CustomPaymentMethod),
		QuotaLabel:                            strings.TrimSpace(plan.QuotaLabel),
		QuotaUnit:                             strings.TrimSpace(plan.QuotaUnit),
		QuotaPeriod:                           strings.TrimSpace(plan.QuotaPeriod),
		BuyerSeatCapacity:                     input.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  input.OfflineOccupiedSeats,
		Status:                                status,
		GovernanceStatus:                      "clear",
		ConditionsVersion:                     1,
		PolicyVersion:                         plan.PolicyVersion,
		RiskNoticeCode:                        plan.RiskNoticeCode,
		RiskAckRequired:                       plan.RiskAckRequired,
		RequestID:                             strings.TrimSpace(input.RequestID),
		CreatedAt:                             now,
		UpdatedAt:                             now,
		Version:                               1,
	}
	listing.AvailableSeats = listing.BuyerSeatCapacity - listing.OfflineOccupiedSeats
	return listing
}

func (s *Service) PublishListing(ctx context.Context, user auth.User, input PublishListingInput) (Listing, *domain.AppError) {
	input.OwnerUserID = user.ID
	if err := requireLinuxDoBindingForPublish(user); err != nil {
		return Listing{}, err
	}
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return Listing{}, appErr
	}
	if err := validateCreateListingInput(input, plan); err != nil {
		return Listing{}, err
	}
	if err := validatePlanPublishAllowed(plan); err != nil {
		return Listing{}, err
	}

	now := s.now()
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
	listing := newListing(user.ID, input, plan, ListingStatusActive, now)

	if s.repo != nil {
		return s.repo.PublishCarpoolListing(ctx, listing, ack, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		return Listing{}, contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.published", input.RequestID)
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) PublishListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input PublishListingInput, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, bool, *domain.AppError) {
	if err := idempotency.ValidateKey(strings.TrimSpace(key)); err != nil {
		return Listing{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	if appErr := requireLinuxDoBindingForPublish(user); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := validateCreateListingInput(input, plan); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := validatePlanPublishAllowed(plan); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	now := s.now()
	listing := newListing(user.ID, input, plan, ListingStatusActive, now)
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Listing{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		listing, completion, appErr := s.repo.PublishCarpoolListingWithIdempotency(ctx, *entry, listing, ack, now, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Listing{}, idempotency.Completion{}, false, appErr
		}
		return listing, completion, true, nil
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	s.mu.Lock()
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.published", input.RequestID)
	s.mu.Unlock()
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	return listing, completion, true, nil
}

func (s *Service) UpdateListing(ctx context.Context, user auth.User, input UpdateListingInput) (Listing, *domain.AppError) {
	input, plan, ack, now, appErr := s.prepareUpdateListing(ctx, user, input)
	if appErr != nil {
		return Listing{}, appErr
	}
	if s.repo != nil {
		return s.repo.UpdateCarpoolListing(ctx, input, ack, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[input.ListingID]
	if !ok || listing.OwnerUserID != user.ID {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if listing.Status != ListingStatusDraft && listing.Status != ListingStatusActive && listing.Status != ListingStatusStopped {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能修改。")
	}
	if listing.Status != ListingStatusDraft && plan.ID != listing.ProductPlanID {
		return Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Product plan immutable", "已发布车源不能更换产品或套餐，请新建车源。", "productPlanId", "immutable", "已发布车源不能更换产品或套餐。")
	}
	if input.BuyerSeatCapacity < listing.ActiveBuyerMembers+input.OfflineOccupiedSeats {
		return Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSeatUnavailable, "Seat unavailable", "买家总名额不能小于线下已占名额与平台有效成员数之和。", "buyerSeatCapacity", "below_occupied", "总名额不能小于已占名额。")
	}
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(input.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		return Listing{}, contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}

	previousConditions := NewListingConditionsSnapshot(listing)
	listing.ProductPlanID = plan.ID
	listing.OwnerContactMethodID = strings.TrimSpace(input.OwnerContactMethodID)
	if listing.CycleTerm == nil {
		listing.CycleTerm = &CycleTerm{
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
	listing.ProvidesAdminAccount = normalizedProvidesAdminAccount(input.DistributionMethod, input.ProvidesAdminAccount)
	listing.RegionCode = strings.TrimSpace(input.RegionCode)
	listing.RegionName = strings.TrimSpace(input.RegionName)
	listing.SourceURL = strings.TrimSpace(input.SourceURL)
	listing.PriceMonthlyCNY = strings.TrimSpace(input.PriceMonthlyCNY)
	listing.ServiceMultiplier = strings.TrimSpace(input.ServiceMultiplier)
	listing.DailyQuotaAmount = optionalString(input.DailyQuotaAmount)
	listing.WeeklyQuotaAmount = optionalString(input.WeeklyQuotaAmount)
	listing.FollowsOfficialQuotaReset = input.FollowsOfficialQuotaReset
	listing.VPSRegion = optionalString(input.VPSRegion)
	listing.SupportsMainlandChinaDirectConnection = input.SupportsMainlandChinaDirectConnection
	listing.OpeningChannelCode = optionalString(input.OpeningChannelCode)
	listing.CustomOpeningChannel = optionalString(input.CustomOpeningChannel)
	listing.PaymentMethodCode = optionalString(input.PaymentMethodCode)
	listing.CustomPaymentMethod = optionalString(input.CustomPaymentMethod)
	listing.QuotaLabel = strings.TrimSpace(plan.QuotaLabel)
	listing.QuotaUnit = strings.TrimSpace(plan.QuotaUnit)
	listing.QuotaPeriod = strings.TrimSpace(plan.QuotaPeriod)
	listing.BuyerSeatCapacity = input.BuyerSeatCapacity
	listing.OfflineOccupiedSeats = input.OfflineOccupiedSeats
	listing.PolicyVersion = plan.PolicyVersion
	listing.RiskNoticeCode = plan.RiskNoticeCode
	listing.RiskAckRequired = plan.RiskAckRequired
	if !reflect.DeepEqual(previousConditions, NewListingConditionsSnapshot(listing)) {
		listing.ConditionsVersion++
	}
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.updated", input.RequestID)
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) prepareUpdateListing(ctx context.Context, user auth.User, input UpdateListingInput) (UpdateListingInput, catalog.ProductPlan, *RiskAcknowledgement, time.Time, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ListingID) == "" {
		return UpdateListingInput{}, catalog.ProductPlan{}, nil, time.Time{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须提供车源。", "listingId", "required", "必须提供车源。")
	}
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return UpdateListingInput{}, catalog.ProductPlan{}, nil, time.Time{}, appErr
	}
	if err := validateCreateListingInput(CreateListingInput{
		OwnerUserID:                           user.ID,
		ProductPlanID:                         input.ProductPlanID,
		OwnerContactMethodID:                  input.OwnerContactMethodID,
		CycleTerm:                             input.CycleTerm,
		Title:                                 input.Title,
		Summary:                               input.Summary,
		AccessArrangement:                     input.AccessArrangement,
		DistributionMethod:                    input.DistributionMethod,
		DistributionMethodNote:                input.DistributionMethodNote,
		ProvidesAdminAccount:                  input.ProvidesAdminAccount,
		RegionCode:                            input.RegionCode,
		RegionName:                            input.RegionName,
		SourceURL:                             input.SourceURL,
		PriceMonthlyCNY:                       input.PriceMonthlyCNY,
		ServiceMultiplier:                     input.ServiceMultiplier,
		DailyQuotaAmount:                      input.DailyQuotaAmount,
		WeeklyQuotaAmount:                     input.WeeklyQuotaAmount,
		FollowsOfficialQuotaReset:             input.FollowsOfficialQuotaReset,
		VPSRegion:                             input.VPSRegion,
		SupportsMainlandChinaDirectConnection: input.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    input.OpeningChannelCode,
		CustomOpeningChannel:                  input.CustomOpeningChannel,
		PaymentMethodCode:                     input.PaymentMethodCode,
		CustomPaymentMethod:                   input.CustomPaymentMethod,
		BuyerSeatCapacity:                     input.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  input.OfflineOccupiedSeats,
		RiskAcknowledgement:                   input.RiskAcknowledgement,
	}, plan); err != nil {
		return UpdateListingInput{}, catalog.ProductPlan{}, nil, time.Time{}, err
	}

	now := s.now()
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
	return input, plan, ack, now, nil
}

func (s *Service) UpdateRecruitment(ctx context.Context, user auth.User, input RecruitmentInput, targetStatus string) (Listing, *domain.AppError) {
	if targetStatus != ListingStatusActive && targetStatus != ListingStatusStopped {
		return Listing{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid recruitment status", "招募状态不正确。")
	}
	input.OwnerUserID = user.ID
	if s.repo != nil {
		return s.repo.UpdateCarpoolRecruitment(ctx, input, targetStatus, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[input.ListingID]
	if !ok || listing.OwnerUserID != user.ID {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if listing.GovernanceStatus != "clear" || (listing.Status != ListingStatusActive && listing.Status != ListingStatusStopped) {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不能修改招募状态。")
	}
	listing = s.withSeatSummaryLocked(listing)
	if targetStatus == ListingStatusActive && listing.AvailableSeats <= 0 {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前没有可用名额，不能继续招募。")
	}
	listing.Status = targetStatus
	listing.RecruitmentStopReason = ""
	if targetStatus == ListingStatusStopped {
		listing.RecruitmentStopReason = "owner"
	}
	listing.UpdatedAt = s.now()
	listing.Version++
	s.listings[listing.ID] = listing
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.recruitment_updated", input.RequestID)
	return listing, nil
}

func (s *Service) UpdateListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input UpdateListingInput, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, bool, *domain.AppError) {
	if appErr := idempotency.ValidateKey(strings.TrimSpace(key)); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if buildCompletion == nil {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	preparedInput, _, ack, now, appErr := s.prepareUpdateListing(ctx, user, input)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Listing{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		listing, completion, appErr := s.repo.UpdateCarpoolListingWithIdempotency(ctx, *entry, preparedInput, ack, now, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Listing{}, idempotency.Completion{}, false, appErr
		}
		return listing, completion, true, nil
	}
	listing, appErr := s.UpdateListing(ctx, user, preparedInput)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	return listing, completion, true, nil
}

func (s *Service) SubmitListingForReview(ctx context.Context, user auth.User, input SubmitListingReviewInput) (Listing, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ListingID) == "" {
		return Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须提供车源。", "listingId", "required", "必须提供车源。")
	}
	if s.repo != nil {
		return s.repo.SubmitCarpoolListingForReview(ctx, user, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[input.ListingID]
	if !ok || listing.OwnerUserID != user.ID {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if listing.Status != ListingStatusDraft && listing.Status != ListingStatusChangesRequested {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能发布。")
	}
	if err := requireLinuxDoBindingForPublish(user); err != nil {
		return Listing{}, err
	}
	plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
	if appErr != nil {
		return Listing{}, appErr
	}
	if err := validatePlanPublishAllowed(plan); err != nil {
		return Listing{}, err
	}
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, user.ID, contact.UsageScopeCarpoolOwner); !ok {
		return Listing{}, contact.WechatRequiredError("ownerContactMethodId", "恢复或发布拼车前必须先配置微信联系方式。")
	}
	now := s.now()
	listing.Status = ListingStatusActive
	listing.ReviewedByAdminID = ""
	listing.ReviewedAt = nil
	listing.ReviewReason = ""
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	s.appendListingAuditEventLocked(listing, user.ID, "user", "carpool_listing.published", input.RequestID)
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) SubmitListingForReviewWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input SubmitListingReviewInput, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, bool, *domain.AppError) {
	if err := idempotency.ValidateKey(strings.TrimSpace(key)); err != nil {
		return Listing{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ListingID) == "" {
		return Listing{}, idempotency.Completion{}, false, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须提供车源。", "listingId", "required", "必须提供车源。")
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Listing{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		listing, completion, appErr := s.repo.SubmitCarpoolListingForReviewWithIdempotency(ctx, *entry, user, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Listing{}, idempotency.Completion{}, false, appErr
		}
		return listing, completion, true, nil
	}
	listing, appErr := s.SubmitListingForReview(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	return listing, completion, true, nil
}

func (s *Service) PublicListings(ctx context.Context, filter ListingFilter, page domain.PageRequest) (domain.Page[Listing], *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListPublicCarpoolListings(ctx, filter, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var listings []Listing
	for _, id := range s.listingOrder {
		listing := s.withSeatSummaryLocked(s.listings[id])
		if isPublicListing(listing) {
			listings = append(listings, listing)
		}
	}
	return domain.PageItems(filterListings(listings, filter), page)
}

func (s *Service) PublicListing(ctx context.Context, listingID string) (Listing, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicCarpoolListing(ctx, listingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[listingID]
	if !ok || !isPublicListing(listing) {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) MyListings(ctx context.Context, user auth.User, view string, page domain.PageRequest) (domain.Page[Listing], *domain.AppError) {
	if !isOwnerListingView(view) {
		return domain.Page[Listing]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid owner listing view", "车源视图无效。", "view", "invalid", "车源视图无效。")
	}
	if s.repo != nil {
		return s.repo.ListCarpoolListingsByOwner(ctx, user.ID, view, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var listings []Listing
	for _, id := range s.listingOrder {
		listing := s.withSeatSummaryLocked(s.listings[id])
		if listing.OwnerUserID == user.ID && matchesOwnerListingView(listing, view) {
			listings = append(listings, listing)
		}
	}
	sort.Slice(listings, func(i, j int) bool {
		if listings[i].UpdatedAt.Equal(listings[j].UpdatedAt) {
			return listings[i].ID > listings[j].ID
		}
		return listings[i].UpdatedAt.After(listings[j].UpdatedAt)
	})
	return domain.PageItems(listings, page)
}

func (s *Service) MyListing(ctx context.Context, user auth.User, listingID string) (Listing, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetCarpoolListingByOwner(ctx, user.ID, listingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[listingID]
	if !ok || listing.OwnerUserID != user.ID {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	return s.withSeatSummaryLocked(listing), nil
}

func isOwnerListingView(view string) bool {
	switch strings.TrimSpace(view) {
	case OwnerListingViewAll, OwnerListingViewRecruiting, OwnerListingViewServing, OwnerListingViewHistory, OwnerListingViewNeedsEdit:
		return true
	default:
		return false
	}
}

func matchesOwnerListingView(listing Listing, view string) bool {
	switch strings.TrimSpace(view) {
	case OwnerListingViewRecruiting:
		return listing.Status == ListingStatusActive
	case OwnerListingViewServing:
		return listing.Status == ListingStatusStopped && listing.GovernanceStatus == "clear"
	case OwnerListingViewHistory:
		return listing.GovernanceStatus == "removed"
	case OwnerListingViewNeedsEdit:
		return listing.Status == ListingStatusDraft
	default:
		return true
	}
}

func (s *Service) AdminListings(ctx context.Context, user auth.User, filter ListingFilter, page domain.PageRequest) (domain.Page[Listing], *domain.AppError) {
	if !user.IsAdmin {
		return domain.Page[Listing]{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListAdminCarpoolListings(ctx, filter, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listings := make([]Listing, 0, len(s.listingOrder))
	for _, id := range s.listingOrder {
		listings = append(listings, s.withSeatSummaryLocked(s.listings[id]))
	}
	return domain.PageItems(filterListings(listings, filter), page)
}

func (s *Service) AdminListing(ctx context.Context, user auth.User, listingID string) (Listing, *domain.AppError) {
	if !user.IsAdmin {
		return Listing{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.GetAdminCarpoolListing(ctx, listingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[listingID]
	if !ok {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) UpdateListingReviewStatus(ctx context.Context, user auth.User, input ReviewInput) (Listing, *domain.AppError) {
	if !user.IsAdmin {
		return Listing{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = user.ID
	if err := validateReviewInput(input); err != nil {
		return Listing{}, err
	}
	if s.repo != nil {
		return s.repo.UpdateCarpoolListingReviewStatus(ctx, user, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[input.ListingID]
	if !ok {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	governanceAction := input.Action == "pause" || input.Action == "restore"
	if governanceAction {
		if !canUpdateListingGovernance(listing, input.Action) {
			return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源治理状态不能执行该操作。")
		}
	} else if !canUpdateListingStatus(listing.Status, input.Status, input.Action) {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能执行该审核动作。")
	}
	if input.Action == "approve" {
		plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
		if appErr != nil {
			return Listing{}, appErr
		}
		if err := validatePlanPublishAllowed(plan); err != nil {
			return Listing{}, err
		}
	}
	now := s.now()
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
	s.listings[listing.ID] = listing
	s.appendListingAuditEventLocked(listing, user.ID, "admin", listingReviewEventType(input.Action), input.RequestID)
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) UpdateListingReviewStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input ReviewInput, buildCompletion ListingCompletionBuilder) (Listing, idempotency.Completion, bool, *domain.AppError) {
	if !user.IsAdmin {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if err := idempotency.ValidateKey(strings.TrimSpace(key)); err != nil {
		return Listing{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Listing{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.AdminUserID = user.ID
	if appErr := validateReviewInput(input); appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Listing{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		listing, completion, appErr := s.repo.UpdateCarpoolListingReviewStatusWithIdempotency(ctx, *entry, user, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Listing{}, idempotency.Completion{}, false, appErr
		}
		return listing, completion, true, nil
	}
	listing, appErr := s.UpdateListingReviewStatus(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	completion, appErr := buildCompletion(listing)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Listing{}, idempotency.Completion{}, false, appErr
	}
	return listing, completion, true, nil
}

func (s *Service) CreateApplication(ctx context.Context, user auth.User, input CreateApplicationInput) (Application, *domain.AppError) {
	input.BuyerUserID = user.ID
	if s.repo != nil {
		listing, appErr := s.repo.GetPublicCarpoolListing(ctx, input.ListingID)
		if appErr != nil {
			return Application{}, appErr
		}
		plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
		if appErr != nil {
			return Application{}, appErr
		}
		eligibility, appErr := s.applicationEligibilityWithListing(ctx, user, listing, plan)
		if appErr != nil {
			return Application{}, appErr
		}
		if !eligibility.CanApply {
			return Application{}, eligibilityError(eligibility)
		}
		if err := validateCreateApplicationInput(input, listing, plan); err != nil {
			return Application{}, err
		}
		now := s.now()
		application := newApplication(input, listing, now)
		if appErr := s.repo.CreateCarpoolApplication(ctx, application, normalizedRiskAck(input.RiskAcknowledgement, now)); appErr != nil {
			return Application{}, appErr
		}
		return application, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[input.ListingID]
	if !ok || !isPublicListing(listing) {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	listing = s.withSeatSummaryLocked(listing)
	plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
	if appErr != nil {
		return Application{}, appErr
	}
	eligibility := s.applicationEligibilityLocked(user, listing, plan)
	if !eligibility.CanApply {
		return Application{}, eligibilityError(eligibility)
	}
	if err := validateCreateApplicationInput(input, listing, plan); err != nil {
		return Application{}, err
	}
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(input.BuyerContactMethodID, user.ID, contact.UsageScopeBuyer); !ok {
		return Application{}, contact.WechatRequiredError("buyerContactMethodId", "申请拼车前必须先配置微信联系方式。")
	}
	now := s.now()
	application := newApplication(input, listing, now)
	s.applications[application.ID] = application
	s.appOrder = append(s.appOrder, application.ID)
	s.appendApplicationAuditEventLocked(application, user.ID, "user", "carpool_application.created", input.RequestID)
	return application, nil
}

func (s *Service) CreateApplicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateApplicationInput, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, bool, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return Application{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.BuyerUserID = user.ID
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Application{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.createGuard != nil {
		if appErr := s.createGuard(ctx, user); appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, appErr
		}
	}

	listing, appErr := s.publicListingForApplication(ctx, input.ListingID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if appErr := validateCreateApplicationInput(input, listing, plan); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if s.repo != nil {
		eligibility, appErr := s.applicationEligibilityWithListing(ctx, user, listing, plan)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, appErr
		}
		if !eligibility.CanApply {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, eligibilityError(eligibility)
		}
		now := s.now()
		application := newApplication(input, listing, now)
		ack := normalizedRiskAck(input.RiskAcknowledgement, now)
		application, completion, appErr := s.repo.CreateCarpoolApplicationWithIdempotency(ctx, *entry, application, ack, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, appErr
		}
		return application, completion, true, nil
	}
	now := s.now()
	s.mu.Lock()
	listing, ok := s.listings[input.ListingID]
	if !ok || !isPublicListing(listing) {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	listing = s.withSeatSummaryLocked(listing)
	eligibility := s.applicationEligibilityLocked(user, listing, plan)
	if !eligibility.CanApply {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, eligibilityError(eligibility)
	}
	if appErr := validateCreateApplicationInput(input, listing, plan); appErr != nil {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if _, _, ok := s.contact.WechatVersionForOwnerAndScope(input.BuyerContactMethodID, user.ID, contact.UsageScopeBuyer); !ok {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, contact.WechatRequiredError("buyerContactMethodId", "申请拼车前必须先配置微信联系方式。")
	}
	application := newApplication(input, listing, now)
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	s.applications[application.ID] = application
	s.appOrder = append(s.appOrder, application.ID)
	s.appendApplicationAuditEventLocked(application, user.ID, "user", "carpool_application.created", input.RequestID)
	s.mu.Unlock()
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	return application, completion, true, nil
}

func (s *Service) publicListingForApplication(ctx context.Context, listingID string) (Listing, *domain.AppError) {
	listingID = strings.TrimSpace(listingID)
	if s.repo != nil {
		return s.repo.GetPublicCarpoolListing(ctx, listingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[listingID]
	if !ok || !isPublicListing(listing) {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) ApplicationEligibility(ctx context.Context, user auth.User, listingID string) (ApplicationEligibility, *domain.AppError) {
	if s.repo != nil {
		listing, appErr := s.repo.GetPublicCarpoolListing(ctx, listingID)
		if appErr != nil {
			return ApplicationEligibility{}, appErr
		}
		plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
		if appErr != nil {
			return ApplicationEligibility{}, appErr
		}
		return s.applicationEligibilityWithListing(ctx, user, listing, plan)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	listing, ok := s.listings[strings.TrimSpace(listingID)]
	if !ok || !isPublicListing(listing) {
		return ApplicationEligibility{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	listing = s.withSeatSummaryLocked(listing)
	plan, appErr := s.productPlan(ctx, listing.ProductPlanID)
	if appErr != nil {
		return ApplicationEligibility{}, appErr
	}
	return s.applicationEligibilityLocked(user, listing, plan), nil
}

func (s *Service) applicationEligibilityWithListing(ctx context.Context, user auth.User, listing Listing, plan catalog.ProductPlan) (ApplicationEligibility, *domain.AppError) {
	applications, appErr := s.repo.ListCarpoolApplicationsByBuyer(ctx, user.ID)
	if appErr != nil {
		return ApplicationEligibility{}, appErr
	}
	memberships, appErr := s.repo.ListCarpoolMembershipsByBuyer(ctx, user.ID)
	if appErr != nil {
		return ApplicationEligibility{}, appErr
	}
	hasApplication := false
	for _, application := range applications {
		if application.CarpoolListingID == listing.ID && isOngoingApplicationStatus(application.Status) {
			hasApplication = true
			break
		}
	}
	hasMembership := false
	for _, membership := range memberships {
		if membership.CarpoolListingID == listing.ID && membership.Status == MembershipStatusActive {
			hasMembership = true
			break
		}
	}
	return EvaluateApplicationEligibility(EligibilityContext{
		Listing:                listing,
		Plan:                   plan,
		CurrentUserID:          user.ID,
		HasOngoingApplication:  hasApplication,
		HasActiveMembership:    hasMembership,
		ApplyCapabilityChecked: true,
		HasApplyCapability:     auth.HasCapability(user, auth.CapabilityCarpoolApply),
	}), nil
}

func (s *Service) applicationEligibilityLocked(user auth.User, listing Listing, plan catalog.ProductPlan) ApplicationEligibility {
	hasApplication := false
	for _, application := range s.applications {
		if application.CarpoolListingID == listing.ID && application.BuyerUserID == user.ID && isOngoingApplicationStatus(application.Status) {
			hasApplication = true
			break
		}
	}
	hasMembership := false
	for _, membership := range s.memberships {
		if membership.CarpoolListingID == listing.ID && membership.BuyerUserID == user.ID && membership.Status == MembershipStatusActive {
			hasMembership = true
			break
		}
	}
	return EvaluateApplicationEligibility(EligibilityContext{
		Listing:                listing,
		Plan:                   plan,
		CurrentUserID:          user.ID,
		HasOngoingApplication:  hasApplication,
		HasActiveMembership:    hasMembership,
		ApplyCapabilityChecked: true,
		HasApplyCapability:     auth.HasCapability(user, auth.CapabilityCarpoolApply),
	})
}

func (s *Service) MyApplications(ctx context.Context, user auth.User) ([]Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolApplicationsByBuyer(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var applications []Application
	for _, id := range s.appOrder {
		application := s.applications[id]
		if application.BuyerUserID == user.ID {
			applications = append(applications, application)
		}
	}
	return applications, nil
}

func (s *Service) MyApplication(ctx context.Context, user auth.User, applicationID string) (Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetCarpoolApplicationForBuyer(ctx, user.ID, applicationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	application, ok := s.applications[applicationID]
	if !ok || application.BuyerUserID != user.ID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	return application, nil
}

func (s *Service) ConfirmApplicationConditions(ctx context.Context, user auth.User, input ConfirmApplicationConditionsInput) (Application, *domain.AppError) {
	input.BuyerUserID = user.ID
	if strings.TrimSpace(input.ApplicationID) == "" {
		return Application{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供上车申请。", "id", "required", "必须提供上车申请。")
	}
	if s.repo != nil {
		return s.repo.ConfirmCarpoolApplicationConditions(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	application, ok := s.applications[input.ApplicationID]
	if !ok || application.BuyerUserID != user.ID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusPendingOwner {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请不能更新条件确认。")
	}
	listing, ok := s.listings[application.CarpoolListingID]
	if !ok || listing.GovernanceStatus != "clear" || (listing.Status != ListingStatusActive && listing.Status != ListingStatusStopped) {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Listing unavailable", "当前车源不可确认。")
	}
	now := s.now()
	application.ConditionsVersionSnapshot = listing.ConditionsVersion
	application.ConditionsSnapshot = NewListingConditionsSnapshot(listing)
	application.AcceptedConditionsVersion = listing.ConditionsVersion
	application.ConditionsAcceptedAt = now
	application.ListingTitleSnapshot = listing.Title
	application.PriceMonthlyCNY = listing.PriceMonthlyCNY
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	s.appendApplicationAuditEventLocked(application, user.ID, "user", "carpool_application.conditions_confirmed", input.RequestID)
	return application, nil
}

func (s *Service) ApplicationsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]Application, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == JoinActorOwner {
			return s.OwnerApplications(ctx, auth.User{ID: actor.UserID})
		}
		return s.MyApplications(ctx, auth.User{ID: actor.UserID})
	}
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || s.repo == nil {
		return nil, carpoolRelationshipNotFound()
	}
	return s.repo.ListCarpoolApplicationsForActor(ctx, actor, participantRole)
}

func carpoolRelationshipNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool relationship not found", "拼车关系不存在。")
}

func (s *Service) ApplicationForActor(ctx context.Context, actor auth.BusinessActor, applicationID, participantRole string) (Application, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == JoinActorOwner {
			return s.OwnerApplication(ctx, auth.User{ID: actor.UserID}, applicationID)
		}
		return s.MyApplication(ctx, auth.User{ID: actor.UserID}, applicationID)
	}
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || s.repo == nil {
		return Application{}, carpoolRelationshipNotFound()
	}
	return s.repo.GetCarpoolApplicationForActor(ctx, actor, applicationID, participantRole)
}

func (s *Service) OwnerApplications(ctx context.Context, user auth.User) ([]Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolApplicationsByOwner(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var applications []Application
	for _, id := range s.appOrder {
		application := s.applications[id]
		if application.OwnerUserID == user.ID {
			applications = append(applications, application)
		}
	}
	return applications, nil
}

func (s *Service) OwnerApplication(ctx context.Context, user auth.User, applicationID string) (Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetCarpoolApplicationForOwner(ctx, user.ID, applicationID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	application, ok := s.applications[applicationID]
	if !ok || application.OwnerUserID != user.ID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	return application, nil
}

func (s *Service) AcceptApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input AcceptApplicationInput, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, bool, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return Application{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = userID
	if err := validateAcceptApplicationInput(input); err != nil {
		return Application{}, idempotency.Completion{}, false, err
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Application{}, idempotency.CompletionFromEntry(entry), false, nil
	}

	if s.repo != nil {
		application, completion, appErr := s.repo.AcceptCarpoolApplicationWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, appErr
		}
		return application, completion, true, nil
	}

	application, appErr := s.acceptApplicationInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	return application, completion, true, nil
}

func (s *Service) RejectApplication(ctx context.Context, input RejectApplicationInput) (Application, *domain.AppError) {
	if err := validateRejectApplicationInput(input); err != nil {
		return Application{}, err
	}
	if s.repo != nil {
		return s.repo.RejectCarpoolApplication(ctx, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	application, ok := s.applications[input.ApplicationID]
	if !ok || application.OwnerUserID != input.OwnerUserID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusPendingOwner {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能拒绝。")
	}
	now := s.now()
	application.Status = ApplicationStatusRejected
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	s.appendApplicationAuditEventLocked(application, input.OwnerUserID, "user", "carpool_application.rejected", input.RequestID)
	return application, nil
}

func (s *Service) RejectApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input RejectApplicationInput, buildCompletion ApplicationCompletionBuilder) (Application, idempotency.Completion, bool, *domain.AppError) {
	key = strings.TrimSpace(key)
	if appErr := idempotency.ValidateKey(key); appErr != nil {
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if buildCompletion == nil {
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = userID
	if appErr := validateRejectApplicationInput(input); appErr != nil {
		return Application{}, idempotency.Completion{}, false, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return Application{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return Application{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		application, completion, appErr := s.repo.RejectCarpoolApplicationWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Application{}, idempotency.Completion{}, false, appErr
		}
		return application, completion, true, nil
	}

	now := s.now()
	s.mu.Lock()
	previous, ok := s.applications[input.ApplicationID]
	if !ok || previous.OwnerUserID != input.OwnerUserID {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && previous.Version != input.ExpectedVersion {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if previous.Status != ApplicationStatusPendingOwner {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能拒绝。")
	}
	application := previous
	application.Status = ApplicationStatusRejected
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	s.applications[application.ID] = application
	eventCount := len(s.applicationAuditEvents)
	s.appendApplicationAuditEventLocked(application, userID, "user", "carpool_application.rejected", input.RequestID)
	s.mu.Unlock()
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.mu.Lock()
		s.applications[previous.ID] = previous
		if len(s.applicationAuditEvents) > eventCount {
			s.applicationAuditEvents = s.applicationAuditEvents[:eventCount]
		}
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return Application{}, idempotency.Completion{}, false, appErr
	}
	return application, completion, true, nil
}

func (s *Service) CancelApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CancelApplicationInput, buildCompletion ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.BuyerUserID = userID
	if err := validateCancelApplicationInput(input); err != nil {
		return idempotency.Completion{}, err
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}

	if s.repo != nil {
		_, completion, appErr := s.repo.CancelCarpoolApplicationWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	application, appErr := s.cancelApplicationInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(application)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) MyMemberships(ctx context.Context, user auth.User) ([]Membership, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolMembershipsByBuyer(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var memberships []Membership
	for _, id := range s.memberOrder {
		membership := s.memberships[id]
		if membership.BuyerUserID == user.ID {
			memberships = append(memberships, membership)
		}
	}
	return memberships, nil
}

func (s *Service) OwnerMemberships(ctx context.Context, user auth.User) ([]Membership, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolMembershipsByOwner(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var memberships []Membership
	for _, id := range s.memberOrder {
		membership := s.memberships[id]
		if membership.OwnerUserID == user.ID {
			memberships = append(memberships, membership)
		}
	}
	return memberships, nil
}

func (s *Service) MembershipsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]Membership, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == JoinActorOwner {
			return s.OwnerMemberships(ctx, auth.User{ID: actor.UserID})
		}
		return s.MyMemberships(ctx, auth.User{ID: actor.UserID})
	}
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || s.repo == nil {
		return nil, carpoolRelationshipNotFound()
	}
	return s.repo.ListCarpoolMembershipsForActor(ctx, actor, participantRole)
}

func withEndCarpoolBusinessActor(input EndMembershipInput, actor auth.BusinessActor) EndMembershipInput {
	input.ActorUserID = actor.UserID
	input.ActorAudience = actor.Audience
	input.GovernanceActionID = actor.GovernanceActionID
	input.GovernanceVersion = actor.GovernanceVersion
	input.RestrictionEffectiveAt = actor.RestrictionEffectiveAt
	return input
}

func (s *Service) EndMembershipForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input EndMembershipInput, buildCompletion MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	return s.EndMembershipWithIdempotency(ctx, actor.UserID, routeKey, key, requestHash, withEndCarpoolBusinessActor(input, actor), buildCompletion)
}

func (s *Service) EndMembershipWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input EndMembershipInput, buildCompletion MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.ActorUserID = userID
	if err := validateEndMembershipInput(input); err != nil {
		return idempotency.Completion{}, err
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		if input.ActorAudience == auth.SessionAudienceRestrictedBusiness && s.repo != nil {
			actor := auth.BusinessActor{UserID: userID, Audience: input.ActorAudience, GovernanceActionID: input.GovernanceActionID, GovernanceVersion: input.GovernanceVersion, RestrictionEffectiveAt: input.RestrictionEffectiveAt}
			if _, appErr := s.repo.GetCarpoolMembershipForActor(ctx, actor, input.MembershipID, input.ActorRole); appErr != nil {
				return idempotency.Completion{}, appErr
			}
		}
		return idempotency.CompletionFromEntry(entry), nil
	}

	if s.repo != nil {
		_, completion, appErr := s.repo.EndCarpoolMembershipWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	membership, appErr := s.endMembershipInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(membership)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) UpdateMembershipOwnerNoteForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input UpdateMembershipOwnerNoteInput, buildCompletion MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.OwnerUserID = actor.UserID
	input.OwnerAudience = actor.Audience
	input.GovernanceActionID = actor.GovernanceActionID
	input.GovernanceVersion = actor.GovernanceVersion
	input.RestrictionEffectiveAt = actor.RestrictionEffectiveAt
	return s.UpdateMembershipOwnerNoteWithIdempotency(ctx, actor.UserID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) UpdateMembershipOwnerNoteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input UpdateMembershipOwnerNoteInput, buildCompletion MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = userID
	input.Note = strings.TrimSpace(input.Note)
	if err := validateMembershipOwnerNoteInput(input); err != nil {
		return idempotency.Completion{}, err
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}

	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateCarpoolMembershipOwnerNoteWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	membership, appErr := s.updateMembershipOwnerNoteInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(membership)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) productPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError) {
	if s.catalog == nil {
		return catalog.ProductPlan{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "产品目录服务不可用。")
	}
	return s.catalog.ProductPlan(ctx, planID)
}

func (s *Service) withSeatSummaryLocked(listing Listing) Listing {
	listing.AvailableSeats = listing.BuyerSeatCapacity - listing.OfflineOccupiedSeats - listing.ActiveBuyerMembers
	if listing.AvailableSeats < 0 {
		listing.AvailableSeats = 0
	}
	return listing
}

func isPublicListing(listing Listing) bool {
	return listing.Status == ListingStatusActive && listing.GovernanceStatus == "clear"
}

func (s *Service) appendListingAuditEventLocked(listing Listing, actorUserID, actorKind, eventType, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	s.listingAuditEvents = append(s.listingAuditEvents, ListingAuditEvent{
		ListingID: listing.ID, EventType: eventType, ActorUserID: actorUserID, ActorKind: actorKind,
		AggregateVersion: listing.Version, RequestID: requestID, Status: listing.Status,
		GovernanceStatus: listing.GovernanceStatus, CreatedAt: s.now(),
	})
}

func (s *Service) appendApplicationAuditEventLocked(application Application, actorUserID, actorKind, eventType, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	s.applicationAuditEvents = append(s.applicationAuditEvents, ApplicationAuditEvent{
		ApplicationID: application.ID, EventType: eventType, ActorUserID: actorUserID, ActorKind: actorKind,
		AggregateVersion: application.Version, RequestID: requestID, Status: application.Status, CreatedAt: s.now(),
	})
}

// ApplicationAuditEvents 返回内存模式的安全事件副本，仅用于本地测试与开发态一致性验证。
func (s *Service) ApplicationAuditEvents() []ApplicationAuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ApplicationAuditEvent(nil), s.applicationAuditEvents...)
}

// ListingAuditEvents 返回内存模式的安全事件副本，仅用于本地测试与开发态一致性验证。
func (s *Service) ListingAuditEvents() []ListingAuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ListingAuditEvent(nil), s.listingAuditEvents...)
}

func listingReviewEventType(action string) string {
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

func (s *Service) acceptApplicationInMemory(input AcceptApplicationInput) (Application, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	application, ok := s.applications[input.ApplicationID]
	if !ok || application.OwnerUserID != input.OwnerUserID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusPendingOwner {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能接受。")
	}
	listing, ok := s.listings[application.CarpoolListingID]
	if !ok || listing.OwnerUserID != input.OwnerUserID {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	listing = s.withSeatSummaryLocked(listing)
	if listing.AvailableSeats < application.SeatCount {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可用名额。")
	}
	if listing.Status != ListingStatusActive || listing.GovernanceStatus != "clear" {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	buyerMethod, buyerVersion, ok := s.contact.WechatVersionForOwnerAndScope(application.BuyerContactMethodID, application.BuyerUserID, contact.UsageScopeBuyer)
	if !ok || !buyerMethod.Enabled {
		return Application{}, contact.WechatRequiredError("buyerContactMethodId", "申请拼车前必须先配置微信联系方式。")
	}
	ownerMethod, ownerVersion, ok := s.contact.WechatVersionForOwnerAndScope(listing.OwnerContactMethodID, input.OwnerUserID, contact.UsageScopeCarpoolOwner)
	if !ok || !ownerMethod.Enabled {
		return Application{}, contact.WechatRequiredError("ownerContactMethodId", "车主必须先配置微信联系方式。")
	}
	now := s.now()
	session := contact.ContactSession{
		ID:              uuid.NewString(),
		BuyerUserID:     application.BuyerUserID,
		SellerUserID:    application.OwnerUserID,
		BuyerVersionID:  buyerVersion.ID,
		SellerVersionID: ownerVersion.ID,
		OpensAt:         now,
	}
	s.contact.AddSession(session)
	application.Status = ApplicationStatusJoined
	application.ContactSessionID = session.ID
	application.JoinedAt = &now
	application.DecisionReason = ""
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	membership := Membership{
		ID:                        uuid.NewString(),
		CarpoolListingID:          application.CarpoolListingID,
		CarpoolApplicationID:      application.ID,
		BuyerUserID:               application.BuyerUserID,
		OwnerUserID:               application.OwnerUserID,
		ProductPlanID:             application.ProductPlanID,
		Status:                    MembershipStatusActive,
		SeatCount:                 application.SeatCount,
		PriceMonthlyCNY:           application.PriceMonthlyCNY,
		PolicyVersionSnapshot:     application.PolicyVersionSnapshot,
		RiskNoticeCode:            application.RiskNoticeCode,
		ConditionsVersionSnapshot: application.ConditionsVersionSnapshot,
		ConditionsSnapshot:        application.ConditionsSnapshot,
		JoinedAt:                  now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
		Version:                   1,
	}
	if listing.CycleTerm != nil {
		membership.CycleTermID = listing.CycleTerm.ID
	}
	s.memberships[membership.ID] = membership
	s.memberByApp[application.ID] = membership.ID
	s.memberOrder = append(s.memberOrder, membership.ID)
	listing.ActiveBuyerMembers += application.SeatCount
	if listing.BuyerSeatCapacity-listing.OfflineOccupiedSeats-listing.ActiveBuyerMembers <= 0 {
		listing.Status = ListingStatusStopped
		listing.RecruitmentStopReason = "full"
	}
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	s.appendApplicationAuditEventLocked(application, input.OwnerUserID, "user", "carpool_application.joined", input.RequestID)
	return application, nil
}

func (s *Service) cancelApplicationInMemory(input CancelApplicationInput) (Application, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	application, ok := s.applications[input.ApplicationID]
	if !ok || application.BuyerUserID != input.BuyerUserID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusPendingOwner {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能取消；已加入后请退出拼车。")
	}
	application.Status = ApplicationStatusCancelledByBuyer
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	return application, nil
}

func (s *Service) endMembershipInMemory(input EndMembershipInput) (Membership, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	membership, ok := s.memberships[input.MembershipID]
	if !ok || !canActorEndMembership(membership, input.ActorUserID, input.ActorRole, input.TargetStatus) {
		return Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return Membership{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if membership.Status != MembershipStatusActive {
		return Membership{}, domain.NewError(http.StatusConflict, domain.CodeMembershipNotActive, "Membership not active", "当前成员关系不是可操作状态。")
	}
	membership.Status = input.TargetStatus
	membership.EndedAt = &now
	membership.EndedReason = strings.TrimSpace(input.Reason)
	membership.EndedByUserID = input.ActorUserID
	membership.UpdatedAt = now
	membership.Version++
	if listing, ok := s.listings[membership.CarpoolListingID]; ok {
		listing.ActiveBuyerMembers -= membership.SeatCount
		if listing.ActiveBuyerMembers < 0 {
			listing.ActiveBuyerMembers = 0
		}
		listing.UpdatedAt = now
		listing.Version++
		s.listings[listing.ID] = listing
	}
	s.memberships[membership.ID] = membership
	if application, ok := s.applications[membership.CarpoolApplicationID]; ok {
		s.contact.RevokeSession(application.ContactSessionID, now)
	}
	return membership, nil
}

func (s *Service) updateMembershipOwnerNoteInMemory(input UpdateMembershipOwnerNoteInput) (Membership, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	membership, ok := s.memberships[input.MembershipID]
	if !ok || membership.OwnerUserID != input.OwnerUserID {
		return Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return Membership{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	membership.OwnerNote = input.Note
	membership.UpdatedAt = s.now()
	membership.Version++
	s.memberships[membership.ID] = membership
	return membership, nil
}

func validateCreateListingInput(input CreateListingInput, plan catalog.ProductPlan) *domain.AppError {
	if strings.TrimSpace(input.ProductPlanID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeProductPlanResolutionRequired, "Product plan required", "必须选择产品套餐。", "productPlanId", "required", "必须选择产品套餐。")
	}
	if strings.TrimSpace(input.OwnerContactMethodID) == "" {
		return contact.WechatRequiredError("ownerContactMethodId", "发布拼车前必须先配置微信联系方式。")
	}
	if err := validatePlanPublishAllowed(plan); err != nil {
		return err
	}
	if strings.TrimSpace(input.Title) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Title required", "必须填写车源标题。", "title", "required", "必须填写车源标题。")
	}
	if err := validateListingText("title", input.Title, 120); err != nil {
		return err
	}
	if strings.TrimSpace(input.Summary) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Summary required", "必须填写车源说明。", "summary", "required", "必须填写车源说明。")
	}
	if err := validateListingText("summary", input.Summary, 2000); err != nil {
		return err
	}
	if strings.TrimSpace(input.AccessArrangement) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access arrangement required", "必须说明席位或站外访问安排。", "accessArrangement", "required", "必须说明席位或站外访问安排。")
	}
	if err := validateListingText("accessArrangement", input.AccessArrangement, 2000); err != nil {
		return err
	}
	method := strings.TrimSpace(input.DistributionMethod)
	if method == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution method required", "必须选择分发方式。", "distributionMethod", "required", "必须选择分发方式。")
	}
	if method != ListingDistributionMethodSub2API && method != ListingDistributionMethodAccountLogin && method != ListingDistributionMethodOther {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution method invalid", "分发方式不正确。", "distributionMethod", "invalid", "分发方式只能选择 Sub2API、账号登录或其他。")
	}
	if strings.TrimSpace(input.DistributionMethodNote) == "" && method == ListingDistributionMethodOther {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution note required", "选择其他分发方式时必须填写说明。", "distributionMethodNote", "required", "请填写其他分发方式说明。")
	}
	if err := validateListingText("distributionMethodNote", input.DistributionMethodNote, 500); err != nil {
		return err
	}
	if strings.TrimSpace(input.RegionCode) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Region required", "必须填写开通区。", "regionCode", "required", "必须填写开通区。")
	}
	if err := validateListingText("regionCode", input.RegionCode, 64); err != nil {
		return err
	}
	if strings.TrimSpace(input.RegionName) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Region name required", "必须填写开通区名称。", "regionName", "required", "必须填写开通区名称。")
	}
	if err := validateListingText("regionName", input.RegionName, 64); err != nil {
		return err
	}
	if err := validateCycleTermInput(input.CycleTerm); err != nil {
		return err
	}
	if strings.TrimSpace(input.SourceURL) != "" {
		if err := validateEvidenceURL(input.SourceURL); err != nil {
			return err
		}
	}
	if amount, ok := parseNonNegativeDecimal(input.PriceMonthlyCNY); !ok || amount.Sign() < 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Price invalid", "月费格式不正确。", "priceMonthlyCny", "invalid", "月费必须是非负数字。")
	}
	if multiplier, ok := parseNonNegativeDecimal(input.ServiceMultiplier); !ok || multiplier.Cmp(big.NewRat(1, 1)) != 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Service multiplier invalid", "拼车倍率固定为 1。", "serviceMultiplier", "fixed_value", "拼车倍率必须为 1。")
	}
	if strings.TrimSpace(input.DailyQuotaAmount) != "" {
		if quota, ok := parseNonNegativeDecimal(input.DailyQuotaAmount); !ok || quota.Sign() <= 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Daily spend limit invalid", "每日最大花费额度格式不正确。", "dailySpendLimitUsd", "invalid", "每日最大花费额度必须是大于 0 的数字。")
		}
	}
	if strings.TrimSpace(input.WeeklyQuotaAmount) != "" {
		if quota, ok := parseNonNegativeDecimal(input.WeeklyQuotaAmount); !ok || quota.Sign() <= 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Weekly spend limit invalid", "每周最大花费额度格式不正确。", "weeklySpendLimitUsd", "invalid", "每周最大花费额度必须是大于 0 的数字。")
		}
	}
	if input.FollowsOfficialQuotaReset == nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Official quota reset selection required", "必须选择额度是否跟随官方重置。", "followsOfficialQuotaReset", "required", "请选择额度是否跟随官方重置。")
	}
	if strings.TrimSpace(input.VPSRegion) != "" {
		if err := validateListingText("vpsRegion", input.VPSRegion, 64); err != nil {
			return err
		}
	}
	if err := validateListingChoice("openingChannelCode", input.OpeningChannelCode, input.CustomOpeningChannel, validOpeningChannelCodes()); err != nil {
		return err
	}
	if err := validateListingChoice("paymentMethodCode", input.PaymentMethodCode, input.CustomPaymentMethod, validPaymentMethodCodes()); err != nil {
		return err
	}
	if input.BuyerSeatCapacity <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Seat count invalid", "买家名额必须大于 0。", "buyerSeatCapacity", "invalid", "买家名额必须大于 0。")
	}
	if input.OfflineOccupiedSeats < 0 || input.OfflineOccupiedSeats >= input.BuyerSeatCapacity {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSeatUnavailable, "Seat unavailable", "线下已占名额必须小于买家总名额。", "offlineOccupiedSeats", "invalid", "线下已占名额必须小于买家总名额。")
	}
	if err := validateRiskAcknowledgement(input.RiskAcknowledgement, plan); err != nil {
		return err
	}
	return nil
}

func validateListingText(field, value string, maxRunes int) *domain.AppError {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxRunes {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text too long", "文本内容过长。", field, "too_long", "文本内容过长。")
	}
	if strings.ContainsAny(value, "\x00") || domain.LooksLikeSecretContent(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在平台填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func validateCycleTermInput(input CycleTermInput) *domain.AppError {
	if strings.TrimSpace(input.BillingPeriod) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Billing period required", "必须填写账期。", "cycleTerm.billingPeriod", "required", "必须填写账期。")
	}
	if input.NoticeDays < 0 || input.NoticeDays > 365 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Notice days invalid", "退出通知天数不正确。", "cycleTerm.noticeDays", "invalid", "退出通知天数必须在 0 到 365 之间。")
	}
	if strings.TrimSpace(input.ExitPolicy) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Exit policy required", "必须填写退出规则。", "cycleTerm.exitPolicy", "required", "必须填写退出规则。")
	}
	if strings.TrimSpace(input.UsageRules) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Usage rules required", "必须填写使用规则。", "cycleTerm.usageRules", "required", "必须填写使用规则。")
	}
	return nil
}

func validatePlanPublishAllowed(plan catalog.ProductPlan) *domain.AppError {
	switch plan.PublishPolicy {
	case "allowed":
		return nil
	case "blocked":
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeInvalidStateTransition, "Product plan blocked", "该产品当前不允许发布车源。", "productPlanId", "blocked", "该产品当前不允许发布车源。")
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeInvalidStateTransition, "Product plan info only", "该产品当前仅开放行情信息，不开放拼车发布。", "productPlanId", "info_only", "该产品当前仅开放行情信息。")
	}
}

func requireLinuxDoBindingForPublish(user auth.User) *domain.AppError {
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "linux.do binding required", "发布拼车前需要完成 linux.do 身份绑定。", "linuxDoBinding", "required", "需要先完成 linux.do 身份绑定。")
	}
	return nil
}

func validateCreateApplicationInput(input CreateApplicationInput, listing Listing, plan catalog.ProductPlan) *domain.AppError {
	if strings.TrimSpace(input.ListingID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须选择车源。", "listingId", "required", "必须选择车源。")
	}
	if input.BuyerUserID == listing.OwnerUserID {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Cannot apply to own carpool", "不能申请自己的车源。")
	}
	if strings.TrimSpace(input.BuyerContactMethodID) == "" {
		return contact.WechatRequiredError("buyerContactMethodId", "申请拼车前必须先配置微信联系方式。")
	}
	if listing.AvailableSeats <= 0 {
		return domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可申请名额。")
	}
	return validateRiskAcknowledgement(input.RiskAcknowledgement, plan)
}

func validateReviewInput(input ReviewInput) *domain.AppError {
	if strings.TrimSpace(input.ListingID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须提供车源。", "listingId", "required", "必须提供车源。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review reason required", "审核动作必须填写原因。", "reason", "required", "必须填写审核原因。")
	}
	if input.Status == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Status required", "必须提供目标状态。", "status", "required", "必须提供目标状态。")
	}
	return nil
}

func validateAcceptApplicationInput(input AcceptApplicationInput) *domain.AppError {
	if strings.TrimSpace(input.ApplicationID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供申请。", "applicationId", "required", "必须提供申请。")
	}
	return nil
}

func validateRejectApplicationInput(input RejectApplicationInput) *domain.AppError {
	if strings.TrimSpace(input.ApplicationID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供申请。", "applicationId", "required", "必须提供申请。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写拒绝原因。", "reason", "required", "必须填写拒绝原因。")
	}
	return nil
}

func validateCancelApplicationInput(input CancelApplicationInput) *domain.AppError {
	if strings.TrimSpace(input.ApplicationID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供申请。", "applicationId", "required", "必须提供申请。")
	}
	return nil
}

func validateEndMembershipInput(input EndMembershipInput) *domain.AppError {
	if strings.TrimSpace(input.MembershipID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership required", "必须提供成员关系。", "membershipId", "required", "必须提供成员关系。")
	}
	if input.ActorRole == JoinActorBuyer && input.TargetStatus == MembershipStatusLeft {
		return nil
	}
	if input.ActorRole == JoinActorOwner && input.TargetStatus == MembershipStatusRemoved {
		return nil
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership action invalid", "成员关系操作不正确。", "targetStatus", "invalid", "成员关系操作不正确。")
}

func validateMembershipOwnerNoteInput(input UpdateMembershipOwnerNoteInput) *domain.AppError {
	if strings.TrimSpace(input.MembershipID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership required", "必须提供成员关系。", "membershipId", "required", "必须提供成员关系。")
	}
	if utf8.RuneCountInString(input.Note) > 500 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Owner note too long", "车主备注不能超过 500 个字符。", "note", "too_long", "车主备注不能超过 500 个字符。")
	}
	return nil
}

func validateRiskAcknowledgement(ack *RiskAcknowledgement, plan catalog.ProductPlan) *domain.AppError {
	if !plan.RiskAckRequired {
		return nil
	}
	if ack == nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeRiskAckRequired, "Risk acknowledgement required", "该产品需要确认风险告知。", "riskAcknowledgement", "required", "必须确认当前风险告知。")
	}
	if strings.TrimSpace(ack.RiskNoticeCode) != plan.RiskNoticeCode || ack.PolicyVersion != plan.PolicyVersion {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeRiskAckRequired, "Risk acknowledgement stale", "风险告知版本不匹配，请刷新后重新确认。", "riskAcknowledgement", "stale", "风险告知版本不匹配。")
	}
	return nil
}

func normalizedRiskAck(ack *RiskAcknowledgement, now time.Time) *RiskAcknowledgement {
	if ack == nil {
		return nil
	}
	normalized := *ack
	normalized.RiskNoticeCode = strings.TrimSpace(normalized.RiskNoticeCode)
	if normalized.AcknowledgedAt.IsZero() {
		normalized.AcknowledgedAt = now
	}
	return &normalized
}

func newApplication(input CreateApplicationInput, listing Listing, now time.Time) Application {
	return Application{
		ID:                        uuid.NewString(),
		CarpoolListingID:          listing.ID,
		BuyerUserID:               input.BuyerUserID,
		OwnerUserID:               listing.OwnerUserID,
		ProductPlanID:             listing.ProductPlanID,
		BuyerContactMethodID:      input.BuyerContactMethodID,
		Status:                    ApplicationStatusPendingOwner,
		SeatCount:                 1,
		ListingTitleSnapshot:      listing.Title,
		PriceMonthlyCNY:           listing.PriceMonthlyCNY,
		PolicyVersionSnapshot:     listing.PolicyVersion,
		RiskNoticeCode:            listing.RiskNoticeCode,
		ConditionsVersionSnapshot: listing.ConditionsVersion,
		ConditionsSnapshot:        NewListingConditionsSnapshot(listing),
		AcceptedConditionsVersion: listing.ConditionsVersion,
		ConditionsAcceptedAt:      now,
		RequestID:                 strings.TrimSpace(input.RequestID),
		CreatedAt:                 now,
		UpdatedAt:                 now,
		Version:                   1,
	}
}

func NewListingConditionsSnapshot(listing Listing) ListingConditionsSnapshot {
	snapshot := ListingConditionsSnapshot{
		Title:                                 listing.Title,
		PriceMonthlyCNY:                       listing.PriceMonthlyCNY,
		DailySpendLimitUSD:                    listing.DailyQuotaAmount,
		WeeklySpendLimitUSD:                   listing.WeeklyQuotaAmount,
		BuyerSeatCapacity:                     listing.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  listing.OfflineOccupiedSeats,
		RegionCode:                            listing.RegionCode,
		RegionName:                            listing.RegionName,
		VPSRegion:                             listing.VPSRegion,
		SupportsMainlandChinaDirectConnection: listing.SupportsMainlandChinaDirectConnection,
		DistributionMethod:                    listing.DistributionMethod,
		DistributionMethodNote:                listing.DistributionMethodNote,
		ProvidesAdminAccount:                  listing.ProvidesAdminAccount,
		AccessArrangement:                     listing.AccessArrangement,
		PolicyVersion:                         listing.PolicyVersion,
		RiskNoticeCode:                        listing.RiskNoticeCode,
	}
	if listing.CycleTerm != nil {
		term := *listing.CycleTerm
		snapshot.CycleTerm = &term
	}
	if listing.FollowsOfficialQuotaReset != nil {
		snapshot.FollowsOfficialQuotaReset = *listing.FollowsOfficialQuotaReset
	}
	if listing.OpeningChannelCode != nil {
		snapshot.OpeningChannelCode = *listing.OpeningChannelCode
	}
	if listing.CustomOpeningChannel != nil {
		snapshot.CustomOpeningChannel = *listing.CustomOpeningChannel
	}
	if listing.PaymentMethodCode != nil {
		snapshot.PaymentMethodCode = *listing.PaymentMethodCode
	}
	if listing.CustomPaymentMethod != nil {
		snapshot.CustomPaymentMethod = *listing.CustomPaymentMethod
	}
	return snapshot
}

func canUpdateListingStatus(currentStatus, nextStatus, action string) bool {
	switch action {
	case "approve":
		return nextStatus == ListingStatusActive && currentStatus == ListingStatusPendingReview
	case "reject":
		return nextStatus == ListingStatusRejected && currentStatus == ListingStatusPendingReview
	case "request_changes":
		return nextStatus == ListingStatusChangesRequested && (currentStatus == ListingStatusPendingReview || currentStatus == ListingStatusActive)
	}
	switch nextStatus {
	case ListingStatusActive:
		return currentStatus == ListingStatusPendingReview
	case ListingStatusRejected:
		return currentStatus == ListingStatusPendingReview
	case ListingStatusChangesRequested:
		return currentStatus == ListingStatusPendingReview || currentStatus == ListingStatusActive
	default:
		return false
	}
}

func canUpdateListingGovernance(listing Listing, action string) bool {
	published := listing.Status == ListingStatusActive || listing.Status == ListingStatusStopped
	switch action {
	case "pause":
		return published && listing.GovernanceStatus == "clear"
	case "restore":
		return published && listing.GovernanceStatus == "removed"
	default:
		return false
	}
}

func isOngoingApplicationStatus(status string) bool {
	return status == ApplicationStatusPendingOwner
}

func canActorEndMembership(membership Membership, userID, actorRole, targetStatus string) bool {
	switch actorRole {
	case JoinActorBuyer:
		return targetStatus == MembershipStatusLeft && membership.BuyerUserID == userID
	case JoinActorOwner:
		return targetStatus == MembershipStatusRemoved && membership.OwnerUserID == userID
	default:
		return false
	}
}

func validateEvidenceURL(raw string) *domain.AppError {
	if len(raw) > 2048 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 过长。", "sourceUrl", "too_long", "来源 URL 过长。")
	}
	if strings.ContainsAny(raw, "\x00\r\n\t") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 包含控制字符。", "sourceUrl", "control_character", "来源 URL 包含控制字符。")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 必须是 https。", "sourceUrl", "https_required", "来源 URL 必须是 https。")
	}
	if parsed.User != nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 不能包含 userinfo。", "sourceUrl", "userinfo_forbidden", "来源 URL 不能包含 userinfo。")
	}
	if parsed.Fragment != "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 不能包含 fragment。", "sourceUrl", "fragment_forbidden", "来源 URL 不能包含 fragment。")
	}
	for key := range parsed.Query() {
		normalized := strings.ToLower(key)
		switch normalized {
		case "key", "token", "apikey", "api_key", "access_token", "refresh_token", "session", "cookie", "password":
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "来源 URL 不能包含认证参数。", "sourceUrl", "secret_query", "来源 URL 不能包含认证参数。")
		}
	}
	decoded, _ := url.QueryUnescape(parsed.EscapedPath() + "?" + parsed.RawQuery)
	if looksLikeSecret(decoded) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "来源 URL 看起来包含认证秘密。", "sourceUrl", "secret_content", "来源 URL 看起来包含认证秘密。")
	}
	return nil
}

func parseNonNegativeDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() < 0 {
		return nil, false
	}
	return rat, true
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizedProvidesAdminAccount(distributionMethod string, value bool) bool {
	if strings.TrimSpace(distributionMethod) == ListingDistributionMethodAccountLogin {
		return false
	}
	return value
}

func validOpeningChannelCodes() map[string]struct{} {
	return map[string]struct{}{
		ListingOpeningChannelWeb: {}, ListingOpeningChannelIOSAppStore: {}, ListingOpeningChannelGooglePlay: {},
		ListingOpeningChannelTeamSeat: {}, ListingOpeningChannelOther: {},
	}
}

func validPaymentMethodCodes() map[string]struct{} {
	return map[string]struct{}{
		ListingPaymentMethodCreditCard: {}, ListingPaymentMethodVirtualCard: {}, ListingPaymentMethodApplePay: {},
		ListingPaymentMethodGooglePay: {}, ListingPaymentMethodAppStoreGiftCard: {}, ListingPaymentMethodGooglePlayGiftCard: {},
		ListingPaymentMethodPayPal: {}, ListingPaymentMethodUCard: {}, ListingPaymentMethodOther: {},
	}
}

func validateListingChoice(field, code, custom string, valid map[string]struct{}) *domain.AppError {
	code = strings.TrimSpace(code)
	custom = strings.TrimSpace(custom)
	if code == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Selection required", "必须选择一个选项。", field, "required", "请选择一个选项。")
	}
	if _, ok := valid[code]; !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Selection invalid", "选择项不正确。", field, "invalid", "选择项不正确。")
	}
	customField := "customOpeningChannel"
	if field == "paymentMethodCode" {
		customField = "customPaymentMethod"
	}
	if code == "other" && custom == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Custom value required", "选择其他时必须填写具体内容。", customField, "required", "请填写具体内容。")
	}
	if code != "other" && custom != "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Custom value unexpected", "只有选择其他时才能填写自定义内容。", customField, "unexpected", "请清空自定义内容。")
	}
	return validateListingText(customField, custom, 64)
}

func looksLikeSecret(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{"bearer ", "api_key=", "apikey=", "access_token=", "refresh_token=", "session=", "cookie=", "password=", "api key", "sub2api key", "secret=", "token="}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
