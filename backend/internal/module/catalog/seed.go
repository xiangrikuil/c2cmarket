package catalog

import "time"

func SeedProductCategories() []ProductCategory {
	return []ProductCategory{
		{Lifecycle: activeLifecycle("gpt"), ID: "00000000-0000-0000-0000-000000000101", Code: "gpt", DisplayName: "GPT", SortOrder: 10},
		{Lifecycle: activeLifecycle("claude"), ID: "00000000-0000-0000-0000-000000000102", Code: "claude", DisplayName: "Claude", SortOrder: 20},
		{Lifecycle: deprecatedLifecycle(), ID: "00000000-0000-0000-0000-000000000103", Code: "cursor", DisplayName: "Cursor", SortOrder: 40},
		{Lifecycle: deprecatedLifecycle(), ID: "00000000-0000-0000-0000-000000000104", Code: "gemini", DisplayName: "Gemini", SortOrder: 50},
		{Lifecycle: deprecatedLifecycle(), ID: "00000000-0000-0000-0000-000000000105", Code: "perplexity", DisplayName: "Perplexity", SortOrder: 60},
		{Lifecycle: activeLifecycle("grok"), ID: "00000000-0000-0000-0000-000000000106", Code: "grok", DisplayName: "Grok", SortOrder: 30},
		{Lifecycle: deprecatedLifecycle(), ID: "00000000-0000-0000-0000-000000000199", Code: "other", DisplayName: "其他", SortOrder: 999},
	}
}

func SeedProductPlans(now time.Time) []ProductPlan {
	return []ProductPlan{
		{
			Lifecycle:            activeLifecycle("gpt"),
			ID:                   "00000000-0000-0000-0000-000000000303",
			CategoryID:           "00000000-0000-0000-0000-000000000101",
			CategoryCode:         "gpt",
			ProviderCode:         "openai",
			Slug:                 "chatgpt-pro-20x-web",
			DisplayName:          "ChatGPT Pro 20x Web",
			Description:          "个人订阅费用分摊，高风险需确认。",
			PublishPolicy:        "allowed",
			AccessMode:           "personal_account_cost_share",
			ProviderPolicyStatus: "known_restricted",
			RiskLevel:            "high",
			RiskAckRequired:      true,
			RiskNoticeCode:       "openai_subscription_carpool",
			PolicyVersion:        1,
			PolicyNote:           "C2CMarket 当前开放该品类，不代表服务提供商认可。",
			QuotaLabel:           "额度",
			QuotaUnit:            "USD",
			QuotaPeriod:          "monthly",
			SortOrder:            30,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		{
			Lifecycle:            activeLifecycle("claude"),
			ID:                   "00000000-0000-0000-0000-000000000401",
			CategoryID:           "00000000-0000-0000-0000-000000000102",
			CategoryCode:         "claude",
			ProviderCode:         "anthropic",
			Slug:                 "claude-pro",
			DisplayName:          "Claude Pro",
			Description:          "社区 Claude Pro 拼车品类。",
			PublishPolicy:        "allowed",
			AccessMode:           "owner_managed_access",
			ProviderPolicyStatus: "unknown",
			RiskLevel:            "elevated",
			RiskAckRequired:      false,
			PolicyVersion:        1,
			PolicyNote:           "需说明成员、席位或站外访问安排。",
			QuotaLabel:           "额度",
			QuotaUnit:            "USD",
			QuotaPeriod:          "monthly",
			SortOrder:            50,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		{
			Lifecycle:            activeLifecycle("grok"),
			ID:                   "00000000-0000-0000-0000-000000000601",
			CategoryID:           "00000000-0000-0000-0000-000000000106",
			CategoryCode:         "grok",
			ProviderCode:         "xai",
			Slug:                 "grok-premium",
			DisplayName:          "Grok Premium",
			Description:          "社区 Grok 订阅拼车品类。",
			PublishPolicy:        "allowed",
			AccessMode:           "owner_managed_access",
			ProviderPolicyStatus: "unknown",
			RiskLevel:            "elevated",
			RiskAckRequired:      false,
			PolicyVersion:        1,
			PolicyNote:           "需说明成员、席位或站外访问安排。",
			QuotaLabel:           "额度",
			QuotaUnit:            "USD",
			QuotaPeriod:          "monthly",
			SortOrder:            60,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
	}
}

func SeedAPIModelProviders(now time.Time) []APIModelProvider {
	return []APIModelProvider{
		{
			Lifecycle:        activeLifecycle("gpt"),
			ID:               "00000000-0000-0000-0000-000000000c01",
			ProviderCategory: "gpt",
			Code:             "openai",
			DisplayName:      "OpenAI",
			SortOrder:        10,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			Lifecycle:        activeLifecycle("claude"),
			ID:               "00000000-0000-0000-0000-000000000c02",
			ProviderCategory: "claude",
			Code:             "anthropic",
			DisplayName:      "Anthropic",
			SortOrder:        20,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			Lifecycle:        deprecatedLifecycle(),
			ID:               "00000000-0000-0000-0000-000000000c03",
			ProviderCategory: "gemini",
			Code:             "google",
			DisplayName:      "Google",
			SortOrder:        30,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			Lifecycle:        deprecatedLifecycle(),
			ID:               "00000000-0000-0000-0000-000000000c04",
			ProviderCategory: "perplexity",
			Code:             "perplexity",
			DisplayName:      "Perplexity",
			SortOrder:        40,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			Lifecycle:        deprecatedLifecycle(),
			ID:               "00000000-0000-0000-0000-000000000c05",
			ProviderCategory: "other",
			Code:             "openrouter",
			DisplayName:      "OpenRouter",
			SortOrder:        50,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			Lifecycle:        activeLifecycle("grok"),
			ID:               "00000000-0000-0000-0000-000000000c06",
			ProviderCategory: "grok",
			Code:             "xai",
			DisplayName:      "xAI",
			SortOrder:        30,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
}

func SeedAPIModels(now time.Time) []APIModelCatalog {
	validFrom := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	return []APIModelCatalog{
		{
			Lifecycle:                  activeLifecycle("gpt"),
			ID:                         "00000000-0000-0000-0000-000000000a01",
			ProviderID:                 "00000000-0000-0000-0000-000000000c01",
			ModelKey:                   "gpt-4.1",
			Capabilities:               []string{"text"},
			SortOrder:                  10,
			CurrentPriceVersionID:      "00000000-0000-0000-0000-000000000b01",
			CurrentPriceSourceURL:      "https://platform.openai.com/docs/pricing",
			CurrentPriceSourceVersion:  "seed-2026-06-22",
			CurrentPriceValidFrom:      &validFrom,
			InputPricePerMillion:       "2.000000",
			CachedInputPricePerMillion: "0.500000",
			OutputPricePerMillion:      "8.000000",
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			Lifecycle:                  activeLifecycle(""),
			ID:                         "00000000-0000-0000-0000-000000000a02",
			ProviderID:                 "00000000-0000-0000-0000-000000000c01",
			ModelKey:                   "gpt-4.1-mini",
			Capabilities:               []string{"text"},
			SortOrder:                  20,
			CurrentPriceVersionID:      "00000000-0000-0000-0000-000000000b02",
			CurrentPriceSourceURL:      "https://platform.openai.com/docs/pricing",
			CurrentPriceSourceVersion:  "seed-2026-06-22",
			CurrentPriceValidFrom:      &validFrom,
			InputPricePerMillion:       "0.400000",
			CachedInputPricePerMillion: "0.100000",
			OutputPricePerMillion:      "1.600000",
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			Lifecycle:                 activeLifecycle("grok"),
			ID:                        "00000000-0000-0000-0000-000000000a31",
			ProviderID:                "00000000-0000-0000-0000-000000000c06",
			ProviderCategory:          "grok",
			ProviderCode:              "xai",
			Provider:                  "xAI",
			ProviderStatus:            StatusActive,
			ModelKey:                  "grok-4",
			Capabilities:              []string{"text"},
			SortOrder:                 310,
			CurrentPriceSourceVersion: "manual-seed-2026-08-14",
			CreatedAt:                 now,
			UpdatedAt:                 now,
		},
	}
}

func activeLifecycle(coreKey string) Lifecycle {
	return Lifecycle{
		CoreKey:               coreKey,
		Status:                StatusActive,
		EffectiveStatus:       StatusActive,
		EffectiveStatusSource: EffectiveStatusSourceSelf,
		Version:               1,
		IdentityLocked:        coreKey != "",
		IdentityLockReason:    coreIdentityLockReason(coreKey),
	}
}

func applyLifecycleProjection(lifecycle Lifecycle) Lifecycle {
	if lifecycle.EffectiveStatus == "" {
		lifecycle.EffectiveStatus = lifecycle.Status
	}
	if lifecycle.EffectiveStatusSource == "" {
		lifecycle.EffectiveStatusSource = EffectiveStatusSourceSelf
	}
	return lifecycle
}

func deprecatedLifecycle() Lifecycle {
	return Lifecycle{
		Status:                StatusDeprecated,
		EffectiveStatus:       StatusDeprecated,
		EffectiveStatusSource: EffectiveStatusSourceSelf,
		StatusReason:          "首发目录范围收口",
		Version:               1,
	}
}

func coreIdentityLockReason(coreKey string) string {
	if coreKey == "" {
		return ""
	}
	return "核心目录身份由系统管理，不能修改。"
}
