package apiintent

import (
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/contact"

	"github.com/google/uuid"
)

func NewIntent(input CreateIntentInput, service apimarket.Service, buyerContact contact.ContactMethod, buyerVersion contact.ContactMethodVersion, ownerContact contact.ContactMethod, ownerVersion contact.ContactMethodVersion, now time.Time) (Intent, *domain.AppError) {
	return NewIntentWithOwnerContacts(input, service, buyerContact, buyerVersion, []OwnerContactSnapshot{{
		ContactMethodID:        ownerContact.ID,
		ContactMethodVersionID: ownerVersion.ID,
		Type:                   ownerContact.Type,
		Label:                  ownerContact.Label,
	}}, now)
}

func NewIntentWithOwnerContacts(input CreateIntentInput, service apimarket.Service, buyerContact contact.ContactMethod, buyerVersion contact.ContactMethodVersion, ownerContacts []OwnerContactSnapshot, now time.Time) (Intent, *domain.AppError) {
	if buyerContact.Type != contact.MethodTypeWechat || strings.TrimSpace(buyerVersion.ID) == "" {
		return Intent{}, contact.TransactionContactRequiredError("buyerContactMethodId", "请选择有效的买家交易联系方式。")
	}
	if len(ownerContacts) != 1 || ownerContacts[0].Type != contact.MethodTypeWechat || strings.TrimSpace(ownerContacts[0].ContactMethodVersionID) == "" {
		return Intent{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Merchant contact unavailable", "商户联系方式当前不可用。")
	}
	primaryOwnerContact := ownerContacts[0]
	selectedAccessMode := strings.TrimSpace(input.SelectedAccessMode)
	selectedPackageSnapshot := ""
	quotaUsagePolicy := service.QuotaUsagePolicy
	if strings.TrimSpace(input.SelectedPackageID) != "" {
		pack, ok := findServicePackage(service, input.SelectedPackageID)
		if !ok {
			return Intent{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "选择的套餐不可用。", "selectedPackageId", "invalid", "选择的套餐不可用。")
		}
		body, err := json.Marshal(packageSnapshot(pack))
		if err != nil {
			return Intent{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		selectedPackageSnapshot = string(body)
		quotaUsagePolicy = pack.QuotaUsagePolicy
	}
	pricingSnapshot, err := servicePricingSnapshotJSON(service, quotaUsagePolicy)
	if err != nil {
		return Intent{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return Intent{
		ID:                                       uuid.NewString(),
		APIServiceID:                             service.ID,
		APIServiceOwnerUserID:                    service.OwnerUserID,
		BuyerUserID:                              input.BuyerUserID,
		OwnerUserID:                              service.OwnerUserID,
		BuyerContactMethodID:                     strings.TrimSpace(input.BuyerContactMethodID),
		BuyerContactMethodVersionID:              buyerVersion.ID,
		OwnerContactMethodID:                     primaryOwnerContact.ContactMethodID,
		OwnerContactMethodVersionID:              primaryOwnerContact.ContactMethodVersionID,
		Status:                                   StatusOpen,
		RequestedCNYAmount:                       decimalStringMust(input.RequestedCNYAmount, 2),
		RequestedUSDAllowance:                    decimalStringOptional(input.RequestedUSDAllowance, 6),
		SelectedAccessMode:                       selectedAccessMode,
		SelectedPackageID:                        strings.TrimSpace(input.SelectedPackageID),
		SelectedPackageSnapshot:                  selectedPackageSnapshot,
		ServiceVersionSnapshot:                   service.Version,
		ServiceTitleSnapshot:                     service.Title,
		DistributionSystemSnapshot:               service.DistributionSystem,
		BillingModeSnapshot:                      service.BillingMode,
		BuyerContactTypeSnapshot:                 buyerContact.Type,
		BuyerContactLabelSnapshot:                buyerContact.Label,
		OwnerContactTypeSnapshot:                 primaryOwnerContact.Type,
		OwnerContactLabelSnapshot:                primaryOwnerContact.Label,
		DeclaredCNYPerUSDAllowanceSnapshot:       decimalStringOptional(service.DeclaredCNYPerUSDAllowance, 4),
		DeclaredMaxUSDAllowancePerIntentSnapshot: decimalStringOptional(service.DeclaredMaxUSDAllowancePerIntent, 6),
		MinimumIntentCNYSnapshot:                 decimalStringMust(service.MinimumIntentCNY, 2),
		MaximumIntentCNYSnapshot:                 decimalStringOptional(service.MaximumIntentCNY, 2),
		PricingSnapshot:                          pricingSnapshot,
		QuotaUsagePolicySnapshot:                 quotaUsagePolicy,
		PromptAuditEnabledSnapshot:               service.PromptAuditEnabled,
		BuyerNote:                                strings.TrimSpace(input.BuyerNote),
		CreatedAt:                                now,
		UpdatedAt:                                now,
		Version:                                  1,
		OwnerContactSnapshots:                    append([]OwnerContactSnapshot(nil), ownerContacts...),
	}, nil
}

func servicePricingSnapshotJSON(service apimarket.Service, purchasedQuotaUsagePolicy apimarket.QuotaUsagePolicy) (string, error) {
	models := make([]map[string]any, 0, len(service.Models))
	for _, model := range service.Models {
		if !model.Enabled {
			continue
		}
		models = append(models, map[string]any{
			"id":                                  model.ID,
			"modelCatalogId":                      model.ModelCatalogID,
			"modelPriceVersionId":                 model.ModelPriceVersionID,
			"modelKey":                            model.ModelKey,
			"providerSnapshot":                    model.ProviderSnapshot,
			"capabilitiesSnapshot":                model.CapabilitiesSnapshot,
			"merchantMultiplier":                  model.MerchantMultiplier,
			"effectiveInputPricePerMillion":       model.EffectiveInputPricePerMillion,
			"effectiveCachedInputPricePerMillion": model.EffectiveCachedInputPricePerMillion,
			"effectiveOutputPricePerMillion":      model.EffectiveOutputPricePerMillion,
		})
	}
	packages := make([]map[string]any, 0, len(service.Packages))
	for _, pack := range service.Packages {
		if !pack.Enabled {
			continue
		}
		packages = append(packages, packageSnapshot(pack))
	}
	body, err := json.Marshal(map[string]any{
		"models":                      models,
		"packages":                    packages,
		"usageVisibility":             service.UsageVisibility,
		"merchantNote":                service.MerchantNote,
		"merchantSupportNote":         service.MerchantSupportNote,
		"accountPoolType":             snapshotOptionalString(service.AccountPoolType),
		"accountPoolLabel":            snapshotOptionalString(apimarket.AccountPoolLabel(service)),
		"declaredMaxConcurrency":      snapshotOptionalPositiveInt(service.DeclaredMaxConcurrency),
		"promptAuditEnabled":          service.PromptAuditEnabled,
		"merchantRefundCommitment":    service.MerchantRefundCommitment,
		"merchantRefundPolicyVersion": apimarket.MerchantRefundPolicyVersion,
		"serviceValidityExpiresAt":    service.QuotaExpiresAt,
		"quotaUsagePolicy":            quotaUsagePolicySnapshot(purchasedQuotaUsagePolicy),
	})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func snapshotOptionalString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func snapshotOptionalPositiveInt(value int) any {
	if value < 1 {
		return nil
	}
	return value
}

func packageSnapshot(pack apimarket.ServicePackage) map[string]any {
	models := make([]map[string]any, 0, len(pack.Models))
	for _, model := range pack.Models {
		models = append(models, map[string]any{
			"serviceModelId":      model.ServiceModelID,
			"modelCatalogId":      model.ModelCatalogID,
			"modelPriceVersionId": model.ModelPriceVersionID,
			"modelKey":            model.ModelKey,
			"providerSnapshot":    model.ProviderSnapshot,
			"merchantMultiplier":  model.MerchantMultiplier,
		})
	}
	return map[string]any{
		"id":               pack.ID,
		"name":             pack.Name,
		"priceCny":         pack.PriceCNY,
		"panelAllowance":   pack.PanelAllowance,
		"durationDays":     pack.DurationDays,
		"description":      pack.Description,
		"enabled":          pack.Enabled,
		"sortOrder":        pack.SortOrder,
		"models":           models,
		"quotaUsagePolicy": quotaUsagePolicySnapshot(pack.QuotaUsagePolicy),
	}
}

func quotaUsagePolicySnapshot(policy apimarket.QuotaUsagePolicy) map[string]any {
	return map[string]any{
		"fiveHour": map[string]any{
			"mode":      policy.FiveHour.Mode,
			"amountUsd": snapshotOptionalString(policy.FiveHour.AmountUSD),
		},
		"daily": map[string]any{
			"mode":      policy.Daily.Mode,
			"amountUsd": snapshotOptionalString(policy.Daily.AmountUSD),
		},
		"scope":      apimarket.QuotaLimitScopePerBuyerCredential,
		"dailyReset": apimarket.QuotaDailyResetUTCPlus8CalendarDay,
	}
}

func findServicePackage(service apimarket.Service, packageID string) (apimarket.ServicePackage, bool) {
	packageID = strings.TrimSpace(packageID)
	for _, pack := range service.Packages {
		if pack.ID == packageID {
			return pack, true
		}
	}
	return apimarket.ServicePackage{}, false
}

func decimalStringMust(value string, places int) string {
	rat, ok := parsePositiveDecimal(value)
	if !ok {
		return strings.TrimSpace(value)
	}
	return decimalString(rat, places)
}

func decimalStringOptional(value string, places int) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return decimalStringMust(value, places)
}

func parsePositiveDecimal(value string) (*big.Rat, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || rat.Sign() <= 0 {
		return nil, false
	}
	return rat, true
}

func decimalString(value *big.Rat, places int) string {
	if value == nil {
		return ""
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	num := scaled.Num()
	den := scaled.Denom()
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	doubleRemainder := new(big.Int).Mul(remainder, big.NewInt(2))
	if doubleRemainder.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	intPart := new(big.Int).Quo(quotient, scale)
	fracPart := new(big.Int).Mod(quotient, scale)
	if places == 0 {
		return intPart.String()
	}
	return intPart.String() + "." + leftPad(fracPart.String(), places)
}

func leftPad(value string, width int) string {
	for len(value) < width {
		value = "0" + value
	}
	return value
}
