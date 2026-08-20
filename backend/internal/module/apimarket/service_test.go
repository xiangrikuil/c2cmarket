package apimarket

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/reputation"
)

type staticAPIModelResolver struct {
	models map[string]catalog.APIModelCatalog
}

func (r staticAPIModelResolver) APIModel(_ context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	return r.models[modelID], nil
}

func (r staticAPIModelResolver) APIModels(context.Context) ([]catalog.APIModelCatalog, *domain.AppError) {
	models := make([]catalog.APIModelCatalog, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models, nil
}

func TestNormalizeOwnerServiceFilterDefaultsToActiveAndRejectsUnknownViews(t *testing.T) {
	filter, appErr := NormalizeOwnerServiceFilter(OwnerServiceFilter{})
	if appErr != nil {
		t.Fatalf("normalize default owner filter: %v", appErr)
	}
	if filter.SalesView != OwnerSalesViewActive {
		t.Fatalf("expected default active sales view, got %q", filter.SalesView)
	}

	for _, salesView := range []string{
		OwnerSalesViewActive,
		OwnerSalesViewExpired,
		OwnerSalesViewPaused,
		OwnerSalesViewDraft,
		OwnerSalesViewAll,
	} {
		filter, appErr = NormalizeOwnerServiceFilter(OwnerServiceFilter{SalesView: " " + salesView + " "})
		if appErr != nil || filter.SalesView != salesView {
			t.Fatalf("expected valid sales view %q, got %+v %v", salesView, filter, appErr)
		}
	}

	_, appErr = NormalizeOwnerServiceFilter(OwnerServiceFilter{SalesView: "finished"})
	if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "salesView" {
		t.Fatalf("expected salesView validation error, got %+v", appErr)
	}
}

func TestMatchesOwnerSalesViewKeepsActiveUpcomingAndHistoryDistinct(t *testing.T) {
	if !MatchesOwnerSalesView(ServiceSalesStateSelling, OwnerSalesViewActive) {
		t.Fatal("expected selling service in active view")
	}
	if !MatchesOwnerSalesView(ServiceSalesStateUpcoming, OwnerSalesViewActive) {
		t.Fatal("expected upcoming service in active view")
	}
	if MatchesOwnerSalesView(ServiceSalesStateExpired, OwnerSalesViewActive) {
		t.Fatal("expired service must not appear in active view")
	}
	if !MatchesOwnerSalesView(ServiceSalesStateOffline, OwnerSalesViewDraft) {
		t.Fatal("offline service must remain reachable in draft view")
	}
	if !MatchesOwnerSalesView(ServiceSalesStateSoldOut, OwnerSalesViewAll) ||
		!MatchesOwnerSalesView(ServiceSalesStateArchived, OwnerSalesViewAll) {
		t.Fatal("sold-out and archived services must remain reachable in all view")
	}
}

func TestHighestPrioritySalesStateSupportsMultipleSalesChannels(t *testing.T) {
	channels := []ServiceSalesChannel{
		{Kind: ServiceSalesChannelLimitedQuota, State: ServiceSalesStateExpired},
		{Kind: ServiceSalesChannelFlexibleQuota, State: ServiceSalesStateSelling},
	}
	if got := HighestPrioritySalesState(channels); got != ServiceSalesStateSelling {
		t.Fatalf("expected selling to outrank expired, got %q", got)
	}

	channels = []ServiceSalesChannel{
		{Kind: ServiceSalesChannelLimitedQuota, State: ServiceSalesStateSoldOut},
		{Kind: ServiceSalesChannelFlexibleQuota, State: ServiceSalesStatePaused},
	}
	if got := HighestPrioritySalesState(channels); got != ServiceSalesStatePaused {
		t.Fatalf("expected paused to outrank sold out, got %q", got)
	}
}

func TestSalesSummaryForMeteredServiceUsesAuthoritativeExpiryBoundary(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(25 * time.Hour)
	service := Service{
		OwnerContactMethodID:  "contact-1",
		ProbeConnectionID:     "probe-connection-1",
		ProbeReady:            true,
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "420.000000",
		QuotaExpiresAt:        &expiresAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: PaymentMethodWechat,
			Enabled:       true,
		}},
	}

	summary := SalesSummaryForService(service, now)
	if summary.OverallState != ServiceSalesStateSelling || len(summary.Channels) != 1 {
		t.Fatalf("expected selling flexible quota summary, got %+v", summary)
	}
	channel := summary.Channels[0]
	if channel.Kind != ServiceSalesChannelFlexibleQuota ||
		channel.AvailableUSDAllowance != "420.000000" ||
		channel.ExpiresAt == nil ||
		!channel.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected flexible quota channel: %+v", channel)
	}

	service.QuotaExpiresAt = &now
	summary = SalesSummaryForService(service, now)
	if summary.OverallState != ServiceSalesStateExpired || summary.Channels[0].State != ServiceSalesStateExpired {
		t.Fatalf("expected exact cutoff to be expired, got %+v", summary)
	}
}

func TestValidateCreateInputRequiresFutureQuotaExpirationForMeteredServices(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.QuotaExpiresAt = "2026-07-07T11:59:00Z"

	err := validateCreateInput(input, now)
	if err == nil {
		t.Fatalf("expected expired quota timestamp to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "quotaExpiresAt" {
		t.Fatalf("expected quotaExpiresAt field error, got %+v", err)
	}
}

func TestValidateCreateInputRejectsManualBillingAndKeepsSupportedModes(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	manual := validMeteredCreateInput()
	manual.BillingMode = ServiceBillingModeManual

	appErr := validateCreateInput(manual, now)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed || len(appErr.FieldErrors) != 1 {
		t.Fatalf("expected stable manual billing validation error, got %+v", appErr)
	}
	fieldErr := appErr.FieldErrors[0]
	if fieldErr.Field != "billingMode" || fieldErr.Code != "unsupported" {
		t.Fatalf("expected billingMode unsupported field error, got %+v", fieldErr)
	}

	otherMetered := validMeteredCreateInput()
	otherMetered.DistributionSystem = "other"
	if appErr := validateCreateInput(otherMetered, now); appErr != nil {
		t.Fatalf("expected other + metered billing to remain supported, got %+v", appErr)
	}

	if appErr := validateCreateInput(validLimitedPackageCreateInput(), now); appErr != nil {
		t.Fatalf("expected fixed package billing to remain supported, got %+v", appErr)
	}
}

func TestValidateCreateInputAllowsOptionalLinuxDoSourceURL(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.SourceURL = " https://linux.do/t/api-quota/123456 "

	if err := validateCreateInput(input, now); err != nil {
		t.Fatalf("expected optional linux.do source URL to be valid, got %+v", err)
	}

	input.SourceURL = ""
	if err := validateCreateInput(input, now); err != nil {
		t.Fatalf("expected empty source URL to be valid, got %+v", err)
	}
}

func TestValidateCreateInputRejectsInvalidSourceURL(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.SourceURL = "https://example.com/post?token=secret"

	err := validateCreateInput(input, now)
	if err == nil {
		t.Fatalf("expected invalid source URL to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "sourceUrl" {
		t.Fatalf("expected sourceUrl field error, got %+v", err)
	}
}

func TestOrderableReasonsIncludesExpiredQuota(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	service := Service{
		OwnerContactMethodID:  "contact-1",
		ProbeConnectionID:     "probe-connection-1",
		ProbeReady:            true,
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "20.000000",
		QuotaExpiresAt:        &expiredAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: PaymentMethodWechat,
			Enabled:       true,
		}},
	}

	reasons := OrderableReasonsAt(service, now)
	if len(reasons) != 1 || reasons[0] != "quota_expired" {
		t.Fatalf("expected only quota_expired reason, got %#v", reasons)
	}
}

func TestOrderableReasonsRejectUnsupportedHistoricalBillingModes(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	for _, billingMode := range []string{ServiceBillingModeManual, "legacy_unknown_mode"} {
		service := Service{
			OwnerContactMethodID: "contact-1",
			ProbeConnectionID:    "probe-connection-1",
			ProbeReady:           true,
			BillingMode:          billingMode,
			AcceptingOrders:      true,
			PaymentWindowMinutes: 10,
			ReviewStatus:         ServiceReviewStatusApproved,
			PublicationStatus:    ServicePublicationStatusOnline,
			ModerationStatus:     ServiceModerationStatusClear,
			PaymentOptions: []PaymentOption{{
				PaymentMethod: PaymentMethodWechat,
				Enabled:       true,
			}},
		}

		reasons := OrderableReasonsAt(service, now)
		if len(reasons) != 1 || reasons[0] != "billing_mode_unsupported" {
			t.Fatalf("expected %q to be non-orderable, got %#v", billingMode, reasons)
		}
	}
}

func TestUpdateProbeConnectionRebindsAndUnbindsWithoutRebuildingService(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.services["service-1"] = Service{
		ID:                     "service-1",
		OwnerUserID:            "owner-1",
		OwnerContactMethodID:   "contact-1",
		ProbeConnectionID:      "probe-1",
		ProbeReady:             true,
		ProbeBaseURL:           "https://old.example.com/v1",
		NormalizedProbeBaseURL: "https://old.example.com/v1",
		AcceptingOrders:        true,
		ReviewStatus:           ServiceReviewStatusApproved,
		PublicationStatus:      ServicePublicationStatusOnline,
		ModerationStatus:       ServiceModerationStatusClear,
		Version:                3,
	}
	user := auth.User{ID: "owner-1"}

	rebound, appErr := manager.UpdateProbeConnection(context.Background(), user, UpdateProbeConnectionInput{
		ServiceID:         "service-1",
		ProbeConnectionID: " probe-2 ",
		ExpectedVersion:   3,
	})
	if appErr != nil {
		t.Fatalf("rebind probe connection: %v", appErr)
	}
	if rebound.ProbeConnectionID != "probe-2" || !rebound.ProbeReady || rebound.Version != 4 {
		t.Fatalf("unexpected rebound service: %+v", rebound)
	}

	unbound, appErr := manager.UpdateProbeConnection(context.Background(), user, UpdateProbeConnectionInput{
		ServiceID:         "service-1",
		ProbeConnectionID: "",
		ExpectedVersion:   4,
	})
	if appErr != nil {
		t.Fatalf("unbind probe connection: %v", appErr)
	}
	if unbound.ProbeConnectionID != "" || unbound.ProbeReady || unbound.IsOrderable || unbound.Version != 5 {
		t.Fatalf("unexpected unbound service: %+v", unbound)
	}
	if len(unbound.OrderableReasons) == 0 || unbound.OrderableReasons[0] != "probe_connection_required" {
		t.Fatalf("expected probe binding orderability reason, got %#v", unbound.OrderableReasons)
	}
}

func TestPublicServicesPaginatesFilteredOrderableServices(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"wechat-2", "alipay-1", "wechat-1"}
	manager.services["wechat-1"] = testPublicService("wechat-1", PaymentMethodWechat, now.Add(3*time.Minute))
	manager.services["alipay-1"] = testPublicService("alipay-1", PaymentMethodAlipay, now.Add(2*time.Minute))
	manager.services["wechat-2"] = testPublicService("wechat-2", PaymentMethodWechat, now.Add(time.Minute))
	if !matchesPublicServiceFilter(manager.services["wechat-1"], PublicServiceFilter{PaymentMethod: PaymentMethodWechat}) {
		t.Fatal("wechat service must match the payment-only public filter")
	}

	first, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{PaymentMethod: PaymentMethodWechat}, domain.PageRequest{Limit: 1})
	if appErr != nil {
		t.Fatalf("list first public service page: %v", appErr)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "wechat-1" || first.NextCursor == nil {
		t.Fatalf("unexpected first page: %+v", first)
	}

	second, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{PaymentMethod: PaymentMethodWechat}, domain.PageRequest{Limit: 1, Cursor: *first.NextCursor})
	if appErr != nil {
		t.Fatalf("list second public service page: %v", appErr)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "wechat-2" || second.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v", second)
	}
}

func TestPublicServicesFiltersPackagesBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"metered-newer", "package-other", "package-match"}
	manager.services["metered-newer"] = testPublicService("metered-newer", PaymentMethodWechat, now.Add(3*time.Minute))
	manager.services["package-other"] = testPublicPackageService("package-other", "model-other", 30, now.Add(2*time.Minute))
	manager.services["package-match"] = testPublicPackageService("package-match", "model-target", 7, now.Add(time.Minute))

	page, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{
		BillingMode:           ServiceBillingModeFixedPackage,
		PackageModelCatalogID: "model-target",
		PackageDurationDays:   7,
	}, domain.PageRequest{Limit: 1})
	if appErr != nil {
		t.Fatalf("list filtered package services: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "package-match" || page.NextCursor != nil {
		t.Fatalf("package filters must run before pagination: %+v", page)
	}
}

func TestNormalizePublicServiceFilterMergesRepeatedAndLegacyPackageModels(t *testing.T) {
	filter, appErr := normalizePublicServiceFilter(PublicServiceFilter{
		PackageModelCatalogIDs: []string{" model-b ", "model-a", "model-b"},
		PackageModelCatalogID:  " model-c ",
	})
	if appErr != nil {
		t.Fatalf("normalize package model filters: %v", appErr)
	}
	want := []string{"model-b", "model-a", "model-c"}
	if len(filter.PackageModelCatalogIDs) != len(want) {
		t.Fatalf("unexpected normalized model ids: got %v want %v", filter.PackageModelCatalogIDs, want)
	}
	for index := range want {
		if filter.PackageModelCatalogIDs[index] != want[index] {
			t.Fatalf("normalization must preserve first occurrence order: got %v want %v", filter.PackageModelCatalogIDs, want)
		}
	}
	if filter.PackageModelCatalogID != "" {
		t.Fatalf("legacy singular model id must be merged and cleared, got %q", filter.PackageModelCatalogID)
	}

	_, appErr = normalizePublicServiceFilter(PublicServiceFilter{PackageModelCatalogIDs: []string{"model-a", " "}})
	if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "packageModelCatalogIds" {
		t.Fatalf("expected empty repeated value rejection, got %+v", appErr)
	}

	tooMany := make([]string, 51)
	for index := range tooMany {
		tooMany[index] = "model-" + strconv.Itoa(index)
	}
	_, appErr = normalizePublicServiceFilter(PublicServiceFilter{PackageModelCatalogIDs: tooMany})
	if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Code != "max_items" {
		t.Fatalf("expected repeated model count rejection, got %+v", appErr)
	}
}

func TestPublicServicesPackageModelFiltersUseORSemanticsWithoutDuplicates(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"package-ab", "package-c"}
	packageAB := testPublicPackageService("package-ab", "model-a", 7, now.Add(2*time.Minute))
	packageAB.Models = append(packageAB.Models, ServiceModel{ID: "package-ab-model-b", ModelCatalogID: "model-b", Enabled: true})
	packageAB.Packages[0].Models = append(packageAB.Packages[0].Models, ServicePackageModel{
		ServiceModelID: "package-ab-model-b",
		ModelCatalogID: "model-b",
	})
	disabledDuration := 3
	packageAB.Models = append(packageAB.Models, ServiceModel{ID: "package-ab-model-disabled", ModelCatalogID: "model-disabled", Enabled: false})
	packageAB.Packages = append(packageAB.Packages, ServicePackage{
		ID: "package-ab-disabled-model", Enabled: true, StockAvailable: 1, DurationDays: &disabledDuration,
		PriceCNY: "1.000000", Models: []ServicePackageModel{{ServiceModelID: "package-ab-model-disabled", ModelCatalogID: "model-disabled"}},
	})
	manager.services[packageAB.ID] = packageAB
	manager.services["package-c"] = testPublicPackageService("package-c", "model-c", 7, now.Add(time.Minute))

	all, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{BillingMode: ServiceBillingModeFixedPackage}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(all.Items) != 2 {
		t.Fatalf("zero model selection must return all packages: page=%+v err=%v", all, appErr)
	}
	one, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{
		BillingMode: ServiceBillingModeFixedPackage, PackageModelCatalogIDs: []string{"model-b"},
	}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(one.Items) != 1 || one.Items[0].ID != "package-ab" {
		t.Fatalf("single model selection returned unexpected services: page=%+v err=%v", one, appErr)
	}
	many, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{
		BillingMode: ServiceBillingModeFixedPackage, PackageModelCatalogIDs: []string{"model-a", "model-b", "model-c"},
	}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(many.Items) != 2 || many.Items[0].ID != "package-ab" || many.Items[1].ID != "package-c" {
		t.Fatalf("multi-model OR selection must return each service once: page=%+v err=%v", many, appErr)
	}
	disabledOnly, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{
		BillingMode: ServiceBillingModeFixedPackage, PackageDurationDays: disabledDuration,
	}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(disabledOnly.Items) != 0 {
		t.Fatalf("package filters must ignore packages backed only by disabled service models: page=%+v err=%v", disabledOnly, appErr)
	}
}

func TestPublicServicesSortsBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"price-high", "price-low-b", "price-low-a"}
	manager.services["price-high"] = testPublicService("price-high", PaymentMethodWechat, now.Add(3*time.Minute))
	manager.services["price-low-b"] = testPublicService("price-low-b", PaymentMethodWechat, now.Add(2*time.Minute))
	manager.services["price-low-a"] = testPublicService("price-low-a", PaymentMethodWechat, now.Add(time.Minute))
	manager.services["price-high"] = withPublicServicePricing(manager.services["price-high"], "3.000000", "1.000000")
	manager.services["price-low-b"] = withPublicServicePricing(manager.services["price-low-b"], "1.000000", "3.000000")
	manager.services["price-low-a"] = withPublicServicePricing(manager.services["price-low-a"], "1.000000", "2.000000")

	filter := PublicServiceFilter{BillingMode: ServiceBillingModeMetered, Sort: PublicServiceSortPriceAsc}
	first, appErr := manager.PublicServices(context.Background(), filter, domain.PageRequest{Limit: 2})
	if appErr != nil {
		t.Fatalf("list first sorted public service page: %v", appErr)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "price-low-a" || first.Items[1].ID != "price-low-b" || first.NextCursor == nil {
		t.Fatalf("sort must run before first page: %+v", first)
	}

	second, appErr := manager.PublicServices(context.Background(), filter, domain.PageRequest{Limit: 2, Cursor: *first.NextCursor})
	if appErr != nil {
		t.Fatalf("list second sorted public service page: %v", appErr)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "price-high" || second.NextCursor != nil {
		t.Fatalf("sort must remain stable across pages: %+v", second)
	}

	minimumFilter := PublicServiceFilter{BillingMode: ServiceBillingModeMetered, Sort: PublicServiceSortMinimumPurchaseAsc}
	minimumPage, appErr := manager.PublicServices(context.Background(), minimumFilter, domain.PageRequest{Limit: 2})
	if appErr != nil {
		t.Fatalf("list minimum-purchase sorted services: %v", appErr)
	}
	if len(minimumPage.Items) != 2 || minimumPage.Items[0].ID != "price-high" || minimumPage.Items[1].ID != "price-low-a" || minimumPage.NextCursor == nil {
		t.Fatalf("minimum purchase sort must run before pagination: %+v", minimumPage)
	}
}

func TestPublicServicesSortsPackagePricesBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"package-high", "package-low-b", "package-low-a"}
	manager.services["package-high"] = testPublicPackageService("package-high", "model-target", 7, now.Add(3*time.Minute))
	manager.services["package-low-b"] = testPublicPackageService("package-low-b", "model-target", 7, now.Add(2*time.Minute))
	manager.services["package-low-a"] = testPublicPackageService("package-low-a", "model-target", 7, now.Add(time.Minute))
	manager.services["package-high"] = withPublicPackagePrice(manager.services["package-high"], "30.000000")
	manager.services["package-low-b"] = withPublicPackagePrice(manager.services["package-low-b"], "10.000000")
	manager.services["package-low-a"] = withPublicPackagePrice(manager.services["package-low-a"], "10.000000")

	filter := PublicServiceFilter{
		BillingMode:           ServiceBillingModeFixedPackage,
		PackageModelCatalogID: "model-target",
		PackageDurationDays:   7,
		Sort:                  PublicServiceSortPackagePriceAsc,
	}
	first, appErr := manager.PublicServices(context.Background(), filter, domain.PageRequest{Limit: 2})
	if appErr != nil {
		t.Fatalf("list package-price sorted services: %v", appErr)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "package-low-a" || first.Items[1].ID != "package-low-b" || first.NextCursor == nil {
		t.Fatalf("package price sort must run before pagination: %+v", first)
	}

	second, appErr := manager.PublicServices(context.Background(), filter, domain.PageRequest{Limit: 2, Cursor: *first.NextCursor})
	if appErr != nil {
		t.Fatalf("list second package-price sorted page: %v", appErr)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "package-high" || second.NextCursor != nil {
		t.Fatalf("package price sort must remain stable across pages: %+v", second)
	}
}

func TestPublicServicesSortsByMinimumPriceAcrossSelectedModels(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"package-b", "package-ab"}
	packageAB := testPublicPackageService("package-ab", "model-a", 7, now.Add(time.Minute))
	packageAB.Packages[0].PriceCNY = "30.000000"
	packageAB.Models = append(packageAB.Models, ServiceModel{ID: "package-ab-model-b", ModelCatalogID: "model-b", Enabled: true})
	packageAB.Packages = append(packageAB.Packages, ServicePackage{
		ID: "package-ab-cheap", Enabled: true, StockAvailable: 1, DurationDays: packageAB.Packages[0].DurationDays,
		PriceCNY: "5.000000", Models: []ServicePackageModel{{ServiceModelID: "package-ab-model-b", ModelCatalogID: "model-b"}},
	})
	packageB := testPublicPackageService("package-b", "model-b", 7, now.Add(2*time.Minute))
	packageB.Packages[0].PriceCNY = "10.000000"
	manager.services[packageAB.ID] = packageAB
	manager.services[packageB.ID] = packageB

	page, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{
		BillingMode: ServiceBillingModeFixedPackage, PackageModelCatalogIDs: []string{"model-a", "model-b"},
		PackageDurationDays: 7, Sort: PublicServiceSortPackagePriceAsc,
	}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(page.Items) != 2 || page.Items[0].ID != "package-ab" || page.Items[1].ID != "package-b" {
		t.Fatalf("package price sort must use the minimum across selected models: page=%+v err=%v", page, appErr)
	}
}

func TestMinimumPackagePriceIgnoresPackagesBackedOnlyByDisabledModels(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	service := testPublicPackageService("package-price", "model-active", 7, now)
	service.Packages[0].PriceCNY = "30.000000"
	service.Models = append(service.Models, ServiceModel{ID: "package-price-model-disabled", ModelCatalogID: "model-disabled", Enabled: false})
	service.Packages = append(service.Packages, ServicePackage{
		ID: "package-price-disabled", Enabled: true, StockAvailable: 1, DurationDays: service.Packages[0].DurationDays,
		PriceCNY: "1.000000", Models: []ServicePackageModel{{ServiceModelID: "package-price-model-disabled", ModelCatalogID: "model-disabled"}},
	})

	price, ok := minimumPackagePriceForPublicFilter(service, PublicServiceFilter{PackageDurationDays: 7})
	if !ok || price.RatString() != "30" {
		t.Fatalf("minimum price must use the enabled package model set, got %v ok=%v", price, ok)
	}
}

func TestPublicPackageFilterOptionsExposeOnlyPurchasableActiveModels(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	resolver := staticAPIModelResolver{models: map[string]catalog.APIModelCatalog{
		"model-openai":            {ID: "model-openai", ModelKey: "gpt-5.6", ProviderCode: "openai", ProviderCategory: "gpt", Provider: "OpenAI", Active: true, ProviderActive: true, SortOrder: 20},
		"model-xai":               {ID: "model-xai", ModelKey: "grok-4", ProviderCode: "xai", ProviderCategory: "grok", Provider: "xAI", Active: true, ProviderActive: true, SortOrder: 10},
		"model-inactive":          {ID: "model-inactive", ModelKey: "old", ProviderCode: "openai", Provider: "OpenAI", Active: false, ProviderActive: true},
		"model-provider-inactive": {ID: "model-provider-inactive", ModelKey: "paused", ProviderCode: "google", Provider: "Google", Active: true, ProviderActive: false},
		"model-sold-out":          {ID: "model-sold-out", ModelKey: "sold-out", ProviderCode: "anthropic", Provider: "Anthropic", Active: true, ProviderActive: true},
	}}
	manager := NewManager(nil, resolver, nil, func() time.Time { return now })
	service := testPublicPackageService("package-options", "model-openai", 7, now.Add(time.Minute))
	service.Models = append(service.Models,
		ServiceModel{ID: "package-options-model-xai", ModelCatalogID: "model-xai", Enabled: true},
		ServiceModel{ID: "package-options-model-inactive", ModelCatalogID: "model-inactive", Enabled: true},
		ServiceModel{ID: "package-options-model-provider-inactive", ModelCatalogID: "model-provider-inactive", Enabled: true},
		ServiceModel{ID: "package-options-model-disabled", ModelCatalogID: "model-disabled", Enabled: false},
	)
	service.Packages[0].Models = append(service.Packages[0].Models,
		ServicePackageModel{ServiceModelID: "package-options-model-xai", ModelCatalogID: "model-xai"},
		ServicePackageModel{ServiceModelID: "package-options-model-inactive", ModelCatalogID: "model-inactive"},
		ServicePackageModel{ServiceModelID: "package-options-model-provider-inactive", ModelCatalogID: "model-provider-inactive"},
	)
	disabledDuration := 30
	service.Packages = append(service.Packages, ServicePackage{
		ID: "disabled-model-package", Enabled: true, StockAvailable: 1, DurationDays: &disabledDuration,
		Models: []ServicePackageModel{{ServiceModelID: "package-options-model-disabled", ModelCatalogID: "model-disabled"}},
	})
	inactiveCatalogDuration := 3
	service.Packages = append(service.Packages, ServicePackage{
		ID: "inactive-catalog-package", Enabled: true, StockAvailable: 1, DurationDays: &inactiveCatalogDuration,
		Models: []ServicePackageModel{{ServiceModelID: "package-options-model-inactive", ModelCatalogID: "model-inactive"}},
	})
	manager.services[service.ID] = service
	manager.serviceOrder = append(manager.serviceOrder, service.ID)
	soldOut := testPublicPackageService("sold-out", "model-sold-out", 3, now.Add(2*time.Minute))
	soldOut.Packages[0].StockAvailable = 0
	manager.services[soldOut.ID] = soldOut
	manager.serviceOrder = append(manager.serviceOrder, soldOut.ID)

	options, appErr := manager.PublicPackageFilterOptions(context.Background(), ServiceBillingModeFixedPackage)
	if appErr != nil {
		t.Fatalf("list package filter options: %v", appErr)
	}
	if len(options.Models) != 2 || options.Models[0].ID != "model-openai" || options.Models[1].ID != "model-xai" {
		t.Fatalf("unexpected active purchasable model options: %+v", options.Models)
	}
	if options.Models[0].ProviderSortOrder != 10 || options.Models[1].ProviderSortOrder != 20 {
		t.Fatalf("unexpected provider ordering metadata: %+v", options.Models)
	}
	if len(options.Durations) != 1 || options.Durations[0] != 7 {
		t.Fatalf("disabled, inactive-catalog, and sold-out durations must be excluded: %+v", options.Durations)
	}
}

func TestSoldOutPackageIsHiddenFromPublicListAndDetail(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	service := testPublicPackageService("sold-out", "model-a", 7, now)
	service.Packages[0].StockAvailable = 0
	manager.services[service.ID] = service
	manager.serviceOrder = []string{service.ID}

	page, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{BillingMode: ServiceBillingModeFixedPackage}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(page.Items) != 0 {
		t.Fatalf("sold-out package must be hidden from discovery: page=%+v err=%v", page, appErr)
	}
	if _, appErr := manager.PublicService(context.Background(), service.ID); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("sold-out package detail must be hidden, got %+v", appErr)
	}

	service.Packages[0].StockAvailable = 1
	service.Models[0].Enabled = false
	manager.services[service.ID] = service
	page, appErr = manager.PublicServices(context.Background(), PublicServiceFilter{BillingMode: ServiceBillingModeFixedPackage}, domain.PageRequest{Limit: 10})
	if appErr != nil || len(page.Items) != 0 {
		t.Fatalf("package backed only by disabled service models must be hidden from discovery: page=%+v err=%v", page, appErr)
	}
	if _, appErr := manager.PublicService(context.Background(), service.ID); appErr == nil || appErr.Status != 404 {
		t.Fatalf("detail backed only by disabled service models must remain hidden, got %+v", appErr)
	}
}

func TestPublicServicesSortsRecommendationMetricsBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	manager.serviceOrder = []string{"service-low", "service-fast", "service-empty"}
	lowService := testPublicService("service-low", PaymentMethodWechat, now.Add(time.Minute))
	fastService := testPublicService("service-fast", PaymentMethodWechat, now.Add(2*time.Minute))
	manager.services["service-low"] = lowService
	manager.services["service-fast"] = fastService
	manager.services["service-empty"] = testPublicService("service-empty", PaymentMethodWechat, now.Add(3*time.Minute))
	lowResponse := 8.0
	fastResponse := 3.0
	lowRating := 4.2
	fastRating := 4.8
	lowService.Completed30d = 2
	lowService.ResponseMedianMinutes = &lowResponse
	lowService.SellerReputation = &reputation.ReputationSnapshot{
		Tier:    reputation.TierReliable,
		Metrics: reputation.ReputationMetrics{WeightedRating: &lowRating, VerifiedReviewCount: 4},
	}
	fastService.Completed30d = 5
	fastService.ResponseMedianMinutes = &fastResponse
	fastService.SellerReputation = &reputation.ReputationSnapshot{
		Tier:    reputation.TierHighTrust,
		Metrics: reputation.ReputationMetrics{WeightedRating: &fastRating, VerifiedReviewCount: 8},
	}
	manager.services["service-low"] = lowService
	manager.services["service-fast"] = fastService

	for _, test := range []struct {
		name string
		sort string
		want []string
	}{
		{name: "recommended", sort: PublicServiceSortRecommended, want: []string{"service-fast", "service-low", "service-empty"}},
		{name: "reputation", sort: PublicServiceSortReputationDesc, want: []string{"service-fast", "service-low", "service-empty"}},
		{name: "completed", sort: PublicServiceSortCompletedDesc, want: []string{"service-fast", "service-low", "service-empty"}},
		{name: "response", sort: PublicServiceSortResponseFast, want: []string{"service-fast", "service-low", "service-empty"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{Sort: test.sort}, domain.PageRequest{Limit: 2})
			if appErr != nil {
				t.Fatalf("list first sorted page: %v", appErr)
			}
			if len(first.Items) != 2 || first.Items[0].ID != test.want[0] || first.Items[1].ID != test.want[1] || first.NextCursor == nil {
				t.Fatalf("unexpected first page for %s: %+v", test.name, first)
			}
			second, appErr := manager.PublicServices(context.Background(), PublicServiceFilter{Sort: test.sort}, domain.PageRequest{Limit: 2, Cursor: *first.NextCursor})
			if appErr != nil {
				t.Fatalf("list second sorted page: %v", appErr)
			}
			if len(second.Items) != 1 || second.Items[0].ID != test.want[2] || second.NextCursor != nil {
				t.Fatalf("unexpected second page for %s: %+v", test.name, second)
			}
		})
	}
}

func testPublicService(id, paymentMethod string, updatedAt time.Time) Service {
	expiresAt := updatedAt.Add(24 * time.Hour)
	return Service{
		ID:                    id,
		OwnerContactMethodID:  "contact-1",
		ProbeConnectionID:     "probe-1",
		ProbeReady:            true,
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "100.000000",
		QuotaExpiresAt:        &expiresAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: paymentMethod,
			Enabled:       true,
		}},
		UpdatedAt: updatedAt,
	}
}

func withPublicServicePricing(service Service, priceCNYPerUSD, minimumIntentCNY string) Service {
	service.DeclaredCNYPerUSDAllowance = priceCNYPerUSD
	service.MinimumIntentCNY = minimumIntentCNY
	return service
}

func withPublicPackagePrice(service Service, priceCNY string) Service {
	service.Packages[0].PriceCNY = priceCNY
	return service
}

func testPublicPackageService(id, modelCatalogID string, durationDays int, updatedAt time.Time) Service {
	service := testPublicService(id, PaymentMethodWechat, updatedAt)
	service.BillingMode = ServiceBillingModeFixedPackage
	service.AvailableUSDAllowance = ""
	service.QuotaExpiresAt = nil
	service.Models = []ServiceModel{{
		ID:             id + "-model",
		ModelCatalogID: modelCatalogID,
		Enabled:        true,
	}}
	service.Packages = []ServicePackage{{
		ID:             id + "-package",
		Enabled:        true,
		StockAvailable: 2,
		DurationDays:   &durationDays,
		Models: []ServicePackageModel{{
			ServiceModelID: id + "-model",
			ModelCatalogID: modelCatalogID,
		}},
	}}
	return service
}

func TestPublicInventoryCountsUsesPackagesAsTheFixedPackageUnit(t *testing.T) {
	now := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	metered := testPublicService("metered-service", PaymentMethodWechat, now)
	expiresAt := now.Add(48 * time.Hour)
	metered.QuotaExpiresAt = &expiresAt
	packages := testPublicPackageService("package-service", "model-1", 3, now)
	packages.Packages = append(packages.Packages,
		ServicePackage{
			ID: "package-service-second", Enabled: true, StockAvailable: 3,
			Models: []ServicePackageModel{{ServiceModelID: "package-service-model", ModelCatalogID: "model-1"}},
		},
		ServicePackage{
			ID: "package-service-sold-out", Enabled: true, StockAvailable: 0,
			Models: []ServicePackageModel{{ServiceModelID: "package-service-model", ModelCatalogID: "model-1"}},
		},
	)
	manager.services[metered.ID] = metered
	manager.services[packages.ID] = packages
	manager.serviceOrder = []string{metered.ID, packages.ID}

	counts, appErr := manager.PublicInventoryCounts(context.Background())
	if appErr != nil {
		t.Fatalf("count public inventory: %v", appErr)
	}
	if counts.MeteredServices != 1 || counts.FixedPackages != 2 {
		t.Fatalf("unexpected public inventory counts: %+v", counts)
	}
}

func TestValidateOrderSettingsRejectsUSDTPaymentMethod(t *testing.T) {
	err := validateOrderSettingsInput(UpdateOrderSettingsInput{
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		PaymentOptions: []PaymentOptionInput{{
			PaymentMethod:       "usdt",
			Enabled:             true,
			PaymentInstructions: "TRC20 地址站外确认。",
		}},
	})
	if err == nil {
		t.Fatalf("expected USDT payment method to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "paymentOptions.0.paymentMethod" {
		t.Fatalf("expected payment method field error, got %+v", err)
	}
}

func TestBuildPaymentOptionsSkipsDisabledEmptyInstructions(t *testing.T) {
	input := UpdateOrderSettingsInput{
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		PaymentOptions: []PaymentOptionInput{
			{
				PaymentMethod:        PaymentMethodWechat,
				Enabled:              true,
				PaymentInstructions:  "微信收款二维码请按商户站外确认展示。",
				PaymentQRCodeDataURL: "data:image/png;base64,ZmFrZS1xcg==",
			},
			{
				PaymentMethod:       PaymentMethodAlipay,
				Enabled:             false,
				PaymentInstructions: " ",
			},
		},
	}
	if err := validateOrderSettingsInput(input); err != nil {
		t.Fatalf("expected disabled empty payment option placeholder to be valid, got %+v", err)
	}

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	options := buildPaymentOptions("service-1", nil, input.PaymentOptions, now)
	if len(options) != 1 {
		t.Fatalf("expected one persisted payment option, got %#v", options)
	}
	if options[0].PaymentMethod != PaymentMethodWechat || !options[0].Enabled || options[0].PaymentInstructions == "" || options[0].PaymentQRCodeDataURL == "" {
		t.Fatalf("unexpected persisted payment option: %#v", options[0])
	}
}

func TestOrderableReasonsIgnoreLegacyUSDTPaymentOption(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(25 * time.Hour)
	service := Service{
		OwnerContactMethodID:  "contact-1",
		ProbeConnectionID:     "probe-connection-1",
		ProbeReady:            true,
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "20.000000",
		QuotaExpiresAt:        &expiresAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: "usdt",
			Enabled:       true,
		}},
	}

	reasons := OrderableReasonsAt(service, now)
	if len(reasons) != 1 || reasons[0] != "payment_method_required" {
		t.Fatalf("expected legacy USDT to be ignored for orderability, got %#v", reasons)
	}
}

func TestLimitedPackageBuildIgnoresCreateIDAndRetainsUpdateIDs(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	resolver := staticAPIModelResolver{models: map[string]catalog.APIModelCatalog{
		"model-1": {
			ID:                         "model-1",
			ModelKey:                   "gpt-5.6",
			Provider:                   "OpenAI",
			Capabilities:               []string{"text"},
			CurrentPriceVersionID:      "price-version-1",
			InputPricePerMillion:       "1.000000",
			CachedInputPricePerMillion: "0.100000",
			OutputPricePerMillion:      "8.000000",
		},
	}}
	manager := NewManager(nil, resolver, nil, func() time.Time { return now })
	input := validLimitedPackageCreateInput()
	input.Packages[0].ID = "client-supplied-id"

	created, appErr := manager.buildFromInput(context.Background(), Service{}, input)
	if appErr != nil {
		t.Fatalf("build limited package service: %v", appErr)
	}
	if created.Packages[0].ID == "client-supplied-id" || created.Packages[0].ID == "" {
		t.Fatalf("expected a server-generated package id, got %q", created.Packages[0].ID)
	}
	if created.Models[0].MerchantMultiplier != "0.0100" || created.Packages[0].Models[0].ModelKey != "gpt-5.6" {
		t.Fatalf("expected exact model snapshot and declared multiplier, got %+v", created.Packages[0].Models)
	}

	packageID := created.Packages[0].ID
	modelID := created.Models[0].ID
	created.Packages[0].StockAvailable = 2
	input.Packages[0].ID = packageID
	input.Packages[0].StockTotal = 6
	updated, appErr := manager.buildFromInput(context.Background(), created, input)
	if appErr != nil {
		t.Fatalf("update limited package service: %v", appErr)
	}
	if updated.Packages[0].ID != packageID || updated.Models[0].ID != modelID {
		t.Fatalf("expected stable package/model ids, got package=%q model=%q", updated.Packages[0].ID, updated.Models[0].ID)
	}
	if updated.Packages[0].StockAvailable != 3 {
		t.Fatalf("expected available stock to preserve committed units, got %d", updated.Packages[0].StockAvailable)
	}
}

func TestValidateLimitedPackageRejectsUnsupportedDurationAndModelSubset(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	input := validLimitedPackageCreateInput()
	unsupported := 5
	input.Packages[0].DurationDays = &unsupported
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "packages.0.durationDays" {
		t.Fatalf("expected unsupported duration error, got %+v", err)
	}

	input = validLimitedPackageCreateInput()
	input.Packages[0].ModelCatalogIDs = []string{"model-not-enabled"}
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "packages.0.modelCatalogIds.0" {
		t.Fatalf("expected package model subset error, got %+v", err)
	}
}

func TestValidateCreateInputRequiresStructuredCommercialFacts(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.AccountPoolType = ""
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "accountPoolType" {
		t.Fatalf("expected accountPoolType field error, got %+v", err)
	}

	input = validMeteredCreateInput()
	input.MerchantRefundCommitment = nil
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "merchantRefundCommitment" {
		t.Fatalf("expected merchantRefundCommitment field error, got %+v", err)
	}

	input = validMeteredCreateInput()
	input.AccountPoolType = AccountPoolCustom
	input.AccountPoolCustomName = "Claude Max"
	if err := validateCreateInput(input, now); err != nil {
		t.Fatalf("expected custom account pool to be valid, got %+v", err)
	}
}

func TestValidateCreateInputRequiresExplicitPromptAuditSelection(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.PromptAuditEnabled = nil

	appErr := validateCreateInput(input, now)
	if appErr == nil || appErr.Status != 422 || len(appErr.FieldErrors) != 1 {
		t.Fatalf("expected prompt audit selection error, got %+v", appErr)
	}
	if fieldErr := appErr.FieldErrors[0]; fieldErr.Field != "promptAuditEnabled" || fieldErr.Code != "required" {
		t.Fatalf("unexpected prompt audit field error: %+v", fieldErr)
	}

	for _, enabled := range []bool{false, true} {
		input = validMeteredCreateInput()
		input.PromptAuditEnabled = &enabled
		if appErr := validateCreateInput(input, now); appErr != nil {
			t.Fatalf("expected explicit prompt audit value %v to pass, got %+v", enabled, appErr)
		}
	}
}

func TestBuildFromInputPreservesHistoricalPerformanceFactsOnUpdate(t *testing.T) {
	now := time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)
	confirmedAt := now.Add(-24 * time.Hour)
	current := Service{
		ID:                     "service-1",
		DeclaredTTFTBand:       "1_to_3s",
		PerformanceConfirmedAt: &confirmedAt,
		CreatedAt:              now.Add(-30 * 24 * time.Hour),
		Version:                7,
	}
	resolver := staticAPIModelResolver{models: map[string]catalog.APIModelCatalog{
		"model-1": {
			ID:                         "model-1",
			ModelKey:                   "gpt-5.6",
			Provider:                   "OpenAI",
			Capabilities:               []string{"text"},
			CurrentPriceVersionID:      "price-version-1",
			InputPricePerMillion:       "1.000000",
			CachedInputPricePerMillion: "0.100000",
			OutputPricePerMillion:      "8.000000",
		},
	}}
	manager := NewManager(nil, resolver, nil, func() time.Time { return now })
	input := validMeteredCreateInput()
	input.QuotaExpiresAt = now.Add(24 * time.Hour).Format(time.RFC3339)

	updated, appErr := manager.buildFromInput(context.Background(), current, input)
	if appErr != nil {
		t.Fatalf("build updated service: %v", appErr)
	}
	if updated.DeclaredTTFTBand != current.DeclaredTTFTBand || updated.PerformanceConfirmedAt == nil || !updated.PerformanceConfirmedAt.Equal(confirmedAt) {
		t.Fatalf("historical performance facts were not preserved: %+v", updated)
	}
}

func validMeteredCreateInput() CreateServiceInput {
	noRefundCommitment := false
	promptAuditEnabled := false
	return CreateServiceInput{
		OwnerContactMethodID:             "contact-1",
		ProbeConnectionID:                "probe-connection-1",
		MerchantIdentityMode:             "public_profile",
		Title:                            "GPT API quota",
		ShortDescription:                 "GPT API quota",
		DistributionSystem:               ServiceDistributionSub2API,
		BillingMode:                      ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8",
		DeclaredMaxUSDAllowancePerIntent: "500",
		AvailableUSDAllowance:            "500",
		MinimumIntentCNY:                 "20",
		MaximumIntentCNY:                 "300",
		QuotaExpiresAt:                   "2026-07-08T00:00:00Z",
		QuotaUsagePolicy:                 testQuotaUsagePolicy(),
		UsageVisibility:                  "merchant_reported",
		AccountPoolType:                  AccountPoolGPTPro20x,
		MerchantRefundCommitment:         &noRefundCommitment,
		DeclaredMaxConcurrency:           8,
		PromptAuditEnabled:               &promptAuditEnabled,
		AccessModes: []ServiceAccessModeInput{{
			AccessMode: "merchant_operated_endpoint",
		}},
		Models: []ServiceModelInput{{
			ModelCatalogID:     "model-1",
			MerchantMultiplier: "1.0000",
			Enabled:            true,
		}},
	}
}

func TestCurrentMaximumIntentCNYTracksAvailableAllowance(t *testing.T) {
	t.Parallel()

	service := Service{
		BillingMode:                      ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8",
		DeclaredMaxUSDAllowancePerIntent: "500",
		AvailableUSDAllowance:            "124.999",
		MinimumIntentCNY:                 "10",
		MaximumIntentCNY:                 "300",
	}
	if got := CurrentMaximumIntentCNY(service); got != "99.99" {
		t.Fatalf("current maximum = %q, want 99.99", got)
	}
}

func TestMeteredTailOrderAndSubCentInventoryBoundaries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	expiresAt := now.Add(48 * time.Hour)
	service := Service{
		OwnerContactMethodID:             "contact-1",
		ProbeConnectionID:                "probe-1",
		ProbeReady:                       true,
		BillingMode:                      ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8000",
		DeclaredMaxUSDAllowancePerIntent: "12.499000",
		AvailableUSDAllowance:            "12.499000",
		MinimumIntentCNY:                 "10.00",
		QuotaExpiresAt:                   &expiresAt,
		AcceptingOrders:                  true,
		PaymentWindowMinutes:             10,
		ReviewStatus:                     ServiceReviewStatusApproved,
		PublicationStatus:                ServicePublicationStatusOnline,
		ModerationStatus:                 ServiceModerationStatusClear,
		PaymentOptions:                   []PaymentOption{{PaymentMethod: PaymentMethodWechat, Enabled: true}},
	}

	if got := CurrentMaximumIntentCNY(service); got != "9.99" {
		t.Fatalf("tail maximum = %q, want 9.99", got)
	}
	if !IsTailOrder(service) {
		t.Fatal("expected remaining inventory to be a tail order")
	}
	if reasons := OrderableReasonsAt(service, now); len(reasons) != 0 {
		t.Fatalf("tail order must remain orderable, got %#v", reasons)
	}

	service.AvailableUSDAllowance = "0.012000"
	if got := CurrentMaximumIntentCNY(service); got != "0.00" {
		t.Fatalf("sub-cent maximum = %q, want 0.00", got)
	}
	if IsTailOrder(service) {
		t.Fatal("sub-cent inventory must not be a tail order")
	}
	reasons := OrderableReasonsAt(service, now)
	if len(reasons) != 1 || reasons[0] != "quota_sold_out" {
		t.Fatalf("sub-cent inventory must be sold out, got %#v", reasons)
	}
}

func validLimitedPackageCreateInput() CreateServiceInput {
	duration := 3
	refundCommitment := true
	promptAuditEnabled := true
	return CreateServiceInput{
		OwnerContactMethodID:     "contact-1",
		ProbeConnectionID:        "probe-connection-1",
		MerchantIdentityMode:     "public_profile",
		Title:                    "GPT 限时套餐",
		ShortDescription:         "按固定价格购买限时面板额度。",
		DistributionSystem:       ServiceDistributionSub2API,
		BillingMode:              ServiceBillingModeFixedPackage,
		MinimumIntentCNY:         "9.90",
		MaximumIntentCNY:         "9.90",
		UsageVisibility:          "fixed_package_only",
		AccountPoolType:          AccountPoolGPTPro5x,
		MerchantRefundCommitment: &refundCommitment,
		DeclaredMaxConcurrency:   8,
		PromptAuditEnabled:       &promptAuditEnabled,
		AccessModes: []ServiceAccessModeInput{{
			AccessMode: "fixed_package_offsite",
		}},
		Models: []ServiceModelInput{{
			ModelCatalogID:     "model-1",
			MerchantMultiplier: "0.01",
			Enabled:            true,
		}},
		Packages: []ServicePackageInput{{
			Name:             "3 天 GPT-5.6 套餐",
			PriceCNY:         "9.90",
			PanelAllowance:   "5.000000",
			QuotaUsagePolicy: testQuotaUsagePolicy(),
			DurationDays:     &duration,
			StockTotal:       5,
			Description:      "交付后 3 天内有效。",
			Enabled:          true,
			ModelCatalogIDs:  []string{"model-1"},
		}},
	}
}

func testQuotaUsagePolicy() QuotaUsagePolicy {
	return QuotaUsagePolicy{
		FiveHour: QuotaUsageLimit{Mode: QuotaLimitModeLimited, AmountUSD: "5"},
		Daily:    QuotaUsageLimit{Mode: QuotaLimitModeUnlimited},
	}
}
