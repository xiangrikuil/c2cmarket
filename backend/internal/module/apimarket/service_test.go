package apimarket

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/catalog"
)

type staticAPIModelResolver struct {
	models map[string]catalog.APIModelCatalog
}

func (r staticAPIModelResolver) APIModel(_ context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	return r.models[modelID], nil
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
	expiresAt := now.Add(time.Hour)
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
	expiresAt := now.Add(time.Hour)
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
