package carpool

import (
	"context"
	"math/big"
	"net/http"
	"net/url"
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

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	catalog     ProductPlanResolver
	contact     *contact.Service
	idempotency *idempotency.Service

	listings     map[string]Listing
	listingOrder []string
	applications map[string]Application
	appOrder     []string
	memberships  map[string]Membership
	memberByApp  map[string]string
	memberOrder  []string
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
	if _, _, ok := s.contact.VersionForOwner(listing.OwnerContactMethodID, user.ID); !ok {
		return Listing{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用或不属于当前用户。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	return listing, nil
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
		Title:                  strings.TrimSpace(input.Title),
		Summary:                strings.TrimSpace(input.Summary),
		AccessArrangement:      strings.TrimSpace(input.AccessArrangement),
		DistributionMethod:     strings.TrimSpace(input.DistributionMethod),
		DistributionMethodNote: strings.TrimSpace(input.DistributionMethodNote),
		ProvidesAdminAccount:   input.ProvidesAdminAccount,
		RegionCode:             strings.TrimSpace(input.RegionCode),
		RegionName:             strings.TrimSpace(input.RegionName),
		SourceURL:              strings.TrimSpace(input.SourceURL),
		PriceMonthlyCNY:        strings.TrimSpace(input.PriceMonthlyCNY),
		ServiceMultiplier:      strings.TrimSpace(input.ServiceMultiplier),
		MonthlyQuotaAmount:     strings.TrimSpace(input.MonthlyQuotaAmount),
		QuotaLabel:             strings.TrimSpace(plan.QuotaLabel),
		QuotaUnit:              strings.TrimSpace(plan.QuotaUnit),
		QuotaPeriod:            strings.TrimSpace(plan.QuotaPeriod),
		BuyerSeatCapacity:      input.BuyerSeatCapacity,
		ActiveBuyerMembers:     input.ActiveBuyerMembers,
		Status:                 status,
		PolicyVersion:          plan.PolicyVersion,
		RiskNoticeCode:         plan.RiskNoticeCode,
		RiskAckRequired:        plan.RiskAckRequired,
		CreatedAt:              now,
		UpdatedAt:              now,
		Version:                1,
	}
	listing.ReservedSeats = 0
	listing.AvailableSeats = listing.BuyerSeatCapacity - listing.ActiveBuyerMembers
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
	if _, _, ok := s.contact.VersionForOwner(listing.OwnerContactMethodID, user.ID); !ok {
		return Listing{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用或不属于当前用户。")
	}
	s.listings[listing.ID] = listing
	s.listingOrder = append(s.listingOrder, listing.ID)
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) UpdateListing(ctx context.Context, user auth.User, input UpdateListingInput) (Listing, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ListingID) == "" {
		return Listing{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Listing required", "必须提供车源。", "listingId", "required", "必须提供车源。")
	}
	plan, appErr := s.productPlan(ctx, input.ProductPlanID)
	if appErr != nil {
		return Listing{}, appErr
	}
	if err := validateCreateListingInput(CreateListingInput{
		OwnerUserID:            user.ID,
		ProductPlanID:          input.ProductPlanID,
		OwnerContactMethodID:   input.OwnerContactMethodID,
		CycleTerm:              input.CycleTerm,
		Title:                  input.Title,
		Summary:                input.Summary,
		AccessArrangement:      input.AccessArrangement,
		DistributionMethod:     input.DistributionMethod,
		DistributionMethodNote: input.DistributionMethodNote,
		ProvidesAdminAccount:   input.ProvidesAdminAccount,
		RegionCode:             input.RegionCode,
		RegionName:             input.RegionName,
		SourceURL:              input.SourceURL,
		PriceMonthlyCNY:        input.PriceMonthlyCNY,
		ServiceMultiplier:      input.ServiceMultiplier,
		MonthlyQuotaAmount:     input.MonthlyQuotaAmount,
		BuyerSeatCapacity:      input.BuyerSeatCapacity,
		ActiveBuyerMembers:     input.ActiveBuyerMembers,
		RiskAcknowledgement:    input.RiskAcknowledgement,
	}, plan); err != nil {
		return Listing{}, err
	}

	now := s.now()
	ack := normalizedRiskAck(input.RiskAcknowledgement, now)
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
	if listing.Status != ListingStatusDraft && listing.Status != ListingStatusChangesRequested {
		return Listing{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源状态不能修改。")
	}
	if _, _, ok := s.contact.VersionForOwner(input.OwnerContactMethodID, user.ID); !ok {
		return Listing{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用或不属于当前用户。")
	}

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
	listing.ProvidesAdminAccount = input.ProvidesAdminAccount
	listing.RegionCode = strings.TrimSpace(input.RegionCode)
	listing.RegionName = strings.TrimSpace(input.RegionName)
	listing.SourceURL = strings.TrimSpace(input.SourceURL)
	listing.PriceMonthlyCNY = strings.TrimSpace(input.PriceMonthlyCNY)
	listing.ServiceMultiplier = strings.TrimSpace(input.ServiceMultiplier)
	listing.MonthlyQuotaAmount = strings.TrimSpace(input.MonthlyQuotaAmount)
	listing.QuotaLabel = strings.TrimSpace(plan.QuotaLabel)
	listing.QuotaUnit = strings.TrimSpace(plan.QuotaUnit)
	listing.QuotaPeriod = strings.TrimSpace(plan.QuotaPeriod)
	listing.BuyerSeatCapacity = input.BuyerSeatCapacity
	listing.ActiveBuyerMembers = input.ActiveBuyerMembers
	listing.PolicyVersion = plan.PolicyVersion
	listing.RiskNoticeCode = plan.RiskNoticeCode
	listing.RiskAckRequired = plan.RiskAckRequired
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	return s.withSeatSummaryLocked(listing), nil
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
	if _, _, ok := s.contact.VersionForOwner(listing.OwnerContactMethodID, user.ID); !ok {
		return Listing{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用或不属于当前用户。")
	}
	now := s.now()
	listing.Status = ListingStatusActive
	listing.ReviewedByAdminID = ""
	listing.ReviewedAt = nil
	listing.ReviewReason = ""
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) PublicListings(ctx context.Context, page domain.PageRequest) (domain.Page[Listing], *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListPublicCarpoolListings(ctx, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireReservationsLocked(s.now())
	var listings []Listing
	for _, id := range s.listingOrder {
		listing := s.withSeatSummaryLocked(s.listings[id])
		if listing.Status == ListingStatusActive {
			listings = append(listings, listing)
		}
	}
	return domain.PageItems(listings, page), nil
}

func (s *Service) PublicListing(ctx context.Context, listingID string) (Listing, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicCarpoolListing(ctx, listingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireReservationsLocked(s.now())
	listing, ok := s.listings[listingID]
	if !ok || listing.Status != ListingStatusActive {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	return s.withSeatSummaryLocked(listing), nil
}

func (s *Service) MyListings(ctx context.Context, user auth.User) ([]Listing, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolListingsByOwner(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireReservationsLocked(s.now())
	var listings []Listing
	for _, id := range s.listingOrder {
		listing := s.withSeatSummaryLocked(s.listings[id])
		if listing.OwnerUserID == user.ID {
			listings = append(listings, listing)
		}
	}
	return listings, nil
}

func (s *Service) AdminListings(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[Listing], *domain.AppError) {
	if !user.IsAdmin {
		return domain.Page[Listing]{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListAdminCarpoolListings(ctx, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireReservationsLocked(s.now())
	listings := make([]Listing, 0, len(s.listingOrder))
	for _, id := range s.listingOrder {
		listings = append(listings, s.withSeatSummaryLocked(s.listings[id]))
	}
	return domain.PageItems(listings, page), nil
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
	s.expireReservationsLocked(s.now())
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
	s.expireReservationsLocked(s.now())
	listing, ok := s.listings[input.ListingID]
	if !ok {
		return Listing{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool listing not found", "车源不存在。")
	}
	if input.ExpectedVersion > 0 && listing.Version != input.ExpectedVersion {
		return Listing{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateListingStatus(listing.Status, input.Status, input.Action) {
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
	listing.Status = input.Status
	listing.ReviewedByAdminID = user.ID
	listing.ReviewedAt = &now
	listing.ReviewReason = strings.TrimSpace(input.Reason)
	listing.UpdatedAt = now
	listing.Version++
	s.listings[listing.ID] = listing
	return s.withSeatSummaryLocked(listing), nil
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
	s.expireReservationsLocked(s.now())
	listing, ok := s.listings[input.ListingID]
	if !ok || listing.Status != ListingStatusActive {
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
	if _, _, ok := s.contact.VersionForOwner(input.BuyerContactMethodID, user.ID); !ok {
		return Application{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "买家联系方式不可用或不属于当前用户。")
	}
	now := s.now()
	application := newApplication(input, listing, now)
	s.applications[application.ID] = application
	s.appOrder = append(s.appOrder, application.ID)
	return application, nil
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
	s.expireReservationsLocked(s.now())
	listing, ok := s.listings[strings.TrimSpace(listingID)]
	if !ok || listing.Status != ListingStatusActive {
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
	return EvaluateApplicationEligibility(EligibilityContext{Listing: listing, Plan: plan, CurrentUserID: user.ID, HasOngoingApplication: hasApplication, HasActiveMembership: hasMembership}), nil
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
	return EvaluateApplicationEligibility(EligibilityContext{Listing: listing, Plan: plan, CurrentUserID: user.ID, HasOngoingApplication: hasApplication, HasActiveMembership: hasMembership})
}

func (s *Service) MyApplications(ctx context.Context, user auth.User) ([]Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolApplicationsByBuyer(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var applications []Application
	for _, id := range s.appOrder {
		s.expireReservationsLocked(s.now())
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
	s.expireReservationsLocked(s.now())
	application, ok := s.applications[applicationID]
	if !ok || application.BuyerUserID != user.ID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	return application, nil
}

func (s *Service) OwnerApplications(ctx context.Context, user auth.User) ([]Application, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListCarpoolApplicationsByOwner(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireReservationsLocked(s.now())
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
	s.expireReservationsLocked(s.now())
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
	return application, nil
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

func (s *Service) WithdrawAcceptanceWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input WithdrawAcceptanceInput, buildCompletion ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = userID
	if err := validateWithdrawAcceptanceInput(input); err != nil {
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
		_, completion, appErr := s.repo.WithdrawCarpoolAcceptanceWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	application, appErr := s.withdrawAcceptanceInMemory(input)
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

func (s *Service) ConfirmApplicationJoinWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ConfirmApplicationJoinInput, buildCompletion ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.ActorUserID = userID
	if err := validateConfirmApplicationJoinInput(input); err != nil {
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
		_, completion, appErr := s.repo.ConfirmCarpoolApplicationJoinWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	application, appErr := s.confirmApplicationJoinInMemory(input)
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

func (s *Service) ConfirmMembershipCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ConfirmMembershipCompleteInput, buildCompletion MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.ActorUserID = userID
	if err := validateConfirmMembershipCompleteInput(input); err != nil {
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
		_, completion, appErr := s.repo.ConfirmCarpoolMembershipCompleteWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	membership, appErr := s.confirmMembershipCompleteInMemory(input)
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

func (s *Service) productPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError) {
	if s.catalog == nil {
		return catalog.ProductPlan{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "产品目录服务不可用。")
	}
	return s.catalog.ProductPlan(ctx, planID)
}

func (s *Service) withSeatSummaryLocked(listing Listing) Listing {
	reserved := 0
	for _, application := range s.applications {
		if application.CarpoolListingID == listing.ID && isUnexpiredReservation(application, s.now()) {
			reserved += application.SeatCount
		}
	}
	listing.ReservedSeats = reserved
	listing.AvailableSeats = listing.BuyerSeatCapacity - listing.ActiveBuyerMembers - reserved
	if listing.AvailableSeats < 0 {
		listing.AvailableSeats = 0
	}
	return listing
}

func (s *Service) expireReservationsLocked(now time.Time) {
	for id, application := range s.applications {
		if application.Status == ApplicationStatusAcceptedReserved && application.ReservationExpiresAt != nil && !now.Before(*application.ReservationExpiresAt) {
			application.Status = ApplicationStatusExpired
			application.UpdatedAt = now
			application.Version++
			s.applications[id] = application
			s.contact.RevokeSession(application.ContactSessionID, now)
		}
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
	if !ok || listing.OwnerUserID != input.OwnerUserID || listing.Status != ListingStatusActive {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前车源不可接受申请。")
	}
	listing = s.withSeatSummaryLocked(listing)
	if listing.AvailableSeats < application.SeatCount {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", "当前车源没有可预留名额。")
	}
	buyerMethod, buyerVersion, ok := s.contact.VersionForOwner(application.BuyerContactMethodID, application.BuyerUserID)
	if !ok || !buyerMethod.Enabled {
		return Application{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "买家联系方式不可用或不属于当前用户。")
	}
	ownerMethod, ownerVersion, ok := s.contact.VersionForOwner(listing.OwnerContactMethodID, input.OwnerUserID)
	if !ok || !ownerMethod.Enabled {
		return Application{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用或不属于当前用户。")
	}
	now := s.now()
	reservationExpiresAt := now.Add(JoinConfirmationDuration)
	session := contact.ContactSession{
		ID:              uuid.NewString(),
		BuyerUserID:     application.BuyerUserID,
		SellerUserID:    application.OwnerUserID,
		BuyerVersionID:  buyerVersion.ID,
		SellerVersionID: ownerVersion.ID,
		OpensAt:         now,
		EndsAt:          reservationExpiresAt,
	}
	s.contact.AddSession(session)
	application.Status = ApplicationStatusAcceptedReserved
	application.ContactSessionID = session.ID
	application.ReservationExpiresAt = &reservationExpiresAt
	application.JoinConfirmationDeadline = &reservationExpiresAt
	application.DecisionReason = ""
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
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
	if application.Status != ApplicationStatusPendingOwner && application.Status != ApplicationStatusAcceptedReserved {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能取消；已加入后请退出拼车。")
	}
	shouldRevokeContact := application.Status == ApplicationStatusAcceptedReserved
	application.Status = ApplicationStatusCancelledByBuyer
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	if shouldRevokeContact {
		s.contact.RevokeSession(application.ContactSessionID, now)
	}
	return application, nil
}

func (s *Service) withdrawAcceptanceInMemory(input WithdrawAcceptanceInput) (Application, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	application, ok := s.applications[input.ApplicationID]
	if !ok || application.OwnerUserID != input.OwnerUserID {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusAcceptedReserved {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能撤回接受。")
	}
	application.Status = ApplicationStatusCancelledByOwner
	application.DecisionReason = strings.TrimSpace(input.Reason)
	application.DecidedAt = &now
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	s.contact.RevokeSession(application.ContactSessionID, now)
	return application, nil
}

func (s *Service) confirmApplicationJoinInMemory(input ConfirmApplicationJoinInput) (Application, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	application, ok := s.applications[input.ApplicationID]
	if !ok {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if !canActorConfirmJoin(application, input.ActorUserID, input.ActorRole) {
		return Application{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool application not found", "上车申请不存在。")
	}
	if input.ExpectedVersion > 0 && application.Version != input.ExpectedVersion {
		return Application{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if application.Status != ApplicationStatusAcceptedReserved {
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前申请状态不能确认加入。")
	}
	if application.JoinConfirmationDeadline == nil || !now.Before(*application.JoinConfirmationDeadline) {
		application.Status = ApplicationStatusExpired
		application.UpdatedAt = now
		application.Version++
		s.applications[application.ID] = application
		s.contact.RevokeSession(application.ContactSessionID, now)
		return Application{}, domain.NewError(http.StatusConflict, domain.CodeJoinConfirmationExpired, "Join confirmation expired", "确认加入期限已过。")
	}
	switch input.ActorRole {
	case JoinActorBuyer:
		if application.BuyerConfirmedAt != nil {
			return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "买家已确认加入。")
		}
		application.BuyerConfirmedAt = &now
	case JoinActorOwner:
		if application.OwnerConfirmedAt != nil {
			return Application{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "车主已确认加入。")
		}
		application.OwnerConfirmedAt = &now
	}
	if application.BuyerConfirmedAt != nil && application.OwnerConfirmedAt != nil {
		joinedAt := now
		application.Status = ApplicationStatusJoined
		application.JoinedAt = &joinedAt
		application.ReservationExpiresAt = nil
		listing := s.listings[application.CarpoolListingID]
		listing.ActiveBuyerMembers += application.SeatCount
		listing.UpdatedAt = now
		listing.Version++
		s.listings[listing.ID] = listing
		membership := Membership{
			ID:                    uuid.NewString(),
			CarpoolListingID:      application.CarpoolListingID,
			CarpoolApplicationID:  application.ID,
			BuyerUserID:           application.BuyerUserID,
			OwnerUserID:           application.OwnerUserID,
			ProductPlanID:         application.ProductPlanID,
			Status:                MembershipStatusActive,
			SeatCount:             application.SeatCount,
			PriceMonthlyCNY:       application.PriceMonthlyCNY,
			PolicyVersionSnapshot: application.PolicyVersionSnapshot,
			RiskNoticeCode:        application.RiskNoticeCode,
			JoinedAt:              joinedAt,
			CreatedAt:             now,
			UpdatedAt:             now,
			Version:               1,
		}
		s.memberships[membership.ID] = membership
		s.memberByApp[application.ID] = membership.ID
		s.memberOrder = append(s.memberOrder, membership.ID)
	}
	application.UpdatedAt = now
	application.Version++
	s.applications[application.ID] = application
	return application, nil
}

func (s *Service) confirmMembershipCompleteInMemory(input ConfirmMembershipCompleteInput) (Membership, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	membership, ok := s.memberships[input.MembershipID]
	if !ok || !canActorConfirmMembership(membership, input.ActorUserID, input.ActorRole) {
		return Membership{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "成员关系不存在。")
	}
	if input.ExpectedVersion > 0 && membership.Version != input.ExpectedVersion {
		return Membership{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if membership.Status != MembershipStatusActive {
		return Membership{}, domain.NewError(http.StatusConflict, domain.CodeMembershipNotActive, "Membership not active", "当前成员关系不是可操作状态。")
	}
	switch input.ActorRole {
	case JoinActorBuyer:
		if membership.BuyerCompletedAt != nil {
			return Membership{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "买家已确认完成。")
		}
		membership.BuyerCompletedAt = &now
	case JoinActorOwner:
		if membership.OwnerCompletedAt != nil {
			return Membership{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "车主已确认完成。")
		}
		membership.OwnerCompletedAt = &now
	}
	if membership.BuyerCompletedAt != nil && membership.OwnerCompletedAt != nil {
		membership.Status = MembershipStatusCompleted
		membership.CompletedAt = &now
		membership.EndedAt = &now
		membership.EndedReason = "双方确认周期完成。"
		membership.EndedByUserID = input.ActorUserID
		if listing, ok := s.listings[membership.CarpoolListingID]; ok {
			listing.ActiveBuyerMembers -= membership.SeatCount
			if listing.ActiveBuyerMembers < 0 {
				listing.ActiveBuyerMembers = 0
			}
			listing.UpdatedAt = now
			listing.Version++
			s.listings[listing.ID] = listing
		}
	}
	membership.UpdatedAt = now
	membership.Version++
	s.memberships[membership.ID] = membership
	return membership, nil
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

func validateCreateListingInput(input CreateListingInput, plan catalog.ProductPlan) *domain.AppError {
	if strings.TrimSpace(input.ProductPlanID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeProductPlanResolutionRequired, "Product plan required", "必须选择产品套餐。", "productPlanId", "required", "必须选择产品套餐。")
	}
	if strings.TrimSpace(input.OwnerContactMethodID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "发布车源必须选择车主联系方式。", "ownerContactMethodId", "required", "必须选择车主联系方式。")
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
	if method != ListingDistributionMethodSub2API && method != ListingDistributionMethodOther {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution method invalid", "分发方式不正确。", "distributionMethod", "invalid", "分发方式只能选择 Sub2API 或其他。")
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
	if multiplier, ok := parseNonNegativeDecimal(input.ServiceMultiplier); !ok || multiplier.Sign() <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Service multiplier invalid", "倍率格式不正确。", "serviceMultiplier", "invalid", "倍率必须是大于 0 的数字。")
	}
	if quota, ok := parseNonNegativeDecimal(input.MonthlyQuotaAmount); !ok || quota.Sign() <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Monthly quota invalid", "每月额度格式不正确。", "monthlyQuotaAmount", "invalid", "每月额度必须是大于 0 的数字。")
	}
	if input.BuyerSeatCapacity <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Seat count invalid", "买家名额必须大于 0。", "buyerSeatCapacity", "invalid", "买家名额必须大于 0。")
	}
	if input.ActiveBuyerMembers < 0 || input.ActiveBuyerMembers >= input.BuyerSeatCapacity {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSeatUnavailable, "Seat unavailable", "已占用名额必须小于买家总名额。", "activeBuyerMembers", "invalid", "已占用名额必须小于买家总名额。")
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
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "申请上车必须选择联系方式。", "buyerContactMethodId", "required", "必须选择联系方式。")
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

func validateWithdrawAcceptanceInput(input WithdrawAcceptanceInput) *domain.AppError {
	if strings.TrimSpace(input.ApplicationID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供申请。", "applicationId", "required", "必须提供申请。")
	}
	return nil
}

func validateConfirmApplicationJoinInput(input ConfirmApplicationJoinInput) *domain.AppError {
	if strings.TrimSpace(input.ApplicationID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Application required", "必须提供申请。", "applicationId", "required", "必须提供申请。")
	}
	switch input.ActorRole {
	case JoinActorBuyer, JoinActorOwner:
		return nil
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Actor role invalid", "确认角色不正确。", "actorRole", "invalid", "确认角色不正确。")
	}
}

func validateConfirmMembershipCompleteInput(input ConfirmMembershipCompleteInput) *domain.AppError {
	if strings.TrimSpace(input.MembershipID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership required", "必须提供成员关系。", "membershipId", "required", "必须提供成员关系。")
	}
	switch input.ActorRole {
	case JoinActorBuyer, JoinActorOwner:
		return nil
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Actor role invalid", "确认角色不正确。", "actorRole", "invalid", "确认角色不正确。")
	}
}

func validateEndMembershipInput(input EndMembershipInput) *domain.AppError {
	if strings.TrimSpace(input.MembershipID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership required", "必须提供成员关系。", "membershipId", "required", "必须提供成员关系。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写结束原因。", "reason", "required", "必须填写结束原因。")
	}
	if input.ActorRole == JoinActorBuyer && input.TargetStatus == MembershipStatusLeft {
		return nil
	}
	if input.ActorRole == JoinActorOwner && input.TargetStatus == MembershipStatusRemoved {
		return nil
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Membership action invalid", "成员关系操作不正确。", "targetStatus", "invalid", "成员关系操作不正确。")
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
		ID:                    uuid.NewString(),
		CarpoolListingID:      listing.ID,
		BuyerUserID:           input.BuyerUserID,
		OwnerUserID:           listing.OwnerUserID,
		ProductPlanID:         listing.ProductPlanID,
		BuyerContactMethodID:  input.BuyerContactMethodID,
		Status:                ApplicationStatusPendingOwner,
		SeatCount:             1,
		ListingTitleSnapshot:  listing.Title,
		PriceMonthlyCNY:       listing.PriceMonthlyCNY,
		PolicyVersionSnapshot: listing.PolicyVersion,
		RiskNoticeCode:        listing.RiskNoticeCode,
		CreatedAt:             now,
		UpdatedAt:             now,
		Version:               1,
	}
}

func canUpdateListingStatus(currentStatus, nextStatus, action string) bool {
	switch action {
	case "approve":
		return nextStatus == ListingStatusActive && currentStatus == ListingStatusPendingReview
	case "reject":
		return nextStatus == ListingStatusRejected && currentStatus == ListingStatusPendingReview
	case "request_changes":
		return nextStatus == ListingStatusChangesRequested && currentStatus == ListingStatusPendingReview
	case "pause":
		return nextStatus == ListingStatusPaused && currentStatus == ListingStatusActive
	case "restore":
		return nextStatus == ListingStatusActive && currentStatus == ListingStatusPaused
	}
	switch nextStatus {
	case ListingStatusActive:
		return currentStatus == ListingStatusPendingReview
	case ListingStatusRejected:
		return currentStatus == ListingStatusPendingReview
	case ListingStatusChangesRequested:
		return currentStatus == ListingStatusPendingReview
	case ListingStatusPaused:
		return currentStatus == ListingStatusActive
	default:
		return false
	}
}

func isOngoingApplicationStatus(status string) bool {
	return status == ApplicationStatusPendingOwner || status == ApplicationStatusAcceptedReserved
}

func isUnexpiredReservation(application Application, now time.Time) bool {
	return application.Status == ApplicationStatusAcceptedReserved &&
		application.ReservationExpiresAt != nil &&
		now.Before(*application.ReservationExpiresAt)
}

func canActorConfirmJoin(application Application, userID, actorRole string) bool {
	switch actorRole {
	case JoinActorBuyer:
		return application.BuyerUserID == userID
	case JoinActorOwner:
		return application.OwnerUserID == userID
	default:
		return false
	}
}

func canActorConfirmMembership(membership Membership, userID, actorRole string) bool {
	switch actorRole {
	case JoinActorBuyer:
		return membership.BuyerUserID == userID
	case JoinActorOwner:
		return membership.OwnerUserID == userID
	default:
		return false
	}
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
