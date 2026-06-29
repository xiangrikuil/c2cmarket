package catalog

import "time"

func SeedProductCategories() []ProductCategory {
	return []ProductCategory{
		{ID: "00000000-0000-0000-0000-000000000101", Code: "gpt", DisplayName: "GPT", SortOrder: 10, Active: true},
		{ID: "00000000-0000-0000-0000-000000000102", Code: "claude", DisplayName: "Claude", SortOrder: 20, Active: true},
		{ID: "00000000-0000-0000-0000-000000000103", Code: "cursor", DisplayName: "Cursor", SortOrder: 30, Active: true},
		{ID: "00000000-0000-0000-0000-000000000104", Code: "gemini", DisplayName: "Gemini", SortOrder: 40, Active: true},
		{ID: "00000000-0000-0000-0000-000000000105", Code: "perplexity", DisplayName: "Perplexity", SortOrder: 50, Active: true},
		{ID: "00000000-0000-0000-0000-000000000199", Code: "other", DisplayName: "其他", SortOrder: 999, Active: true},
	}
}

func SeedProductPlans(now time.Time) []ProductPlan {
	return []ProductPlan{
		{
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
			Active:               true,
			SortOrder:            30,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		{
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
			Active:               true,
			SortOrder:            50,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
	}
}

func SeedAPIModelProviders(now time.Time) []APIModelProvider {
	return []APIModelProvider{
		{
			ID:               "00000000-0000-0000-0000-000000000c01",
			ProviderCategory: "gpt",
			Code:             "openai",
			DisplayName:      "OpenAI",
			Active:           true,
			SortOrder:        10,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "00000000-0000-0000-0000-000000000c02",
			ProviderCategory: "claude",
			Code:             "anthropic",
			DisplayName:      "Anthropic",
			Active:           true,
			SortOrder:        20,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "00000000-0000-0000-0000-000000000c03",
			ProviderCategory: "gemini",
			Code:             "google",
			DisplayName:      "Google",
			Active:           true,
			SortOrder:        30,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "00000000-0000-0000-0000-000000000c04",
			ProviderCategory: "perplexity",
			Code:             "perplexity",
			DisplayName:      "Perplexity",
			Active:           true,
			SortOrder:        40,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               "00000000-0000-0000-0000-000000000c05",
			ProviderCategory: "other",
			Code:             "openrouter",
			DisplayName:      "OpenRouter",
			Active:           true,
			SortOrder:        50,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
}

func SeedAPIModels(now time.Time) []APIModelCatalog {
	validFrom := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	return []APIModelCatalog{
		{
			ID:                         "00000000-0000-0000-0000-000000000a01",
			ProviderID:                 "00000000-0000-0000-0000-000000000c01",
			ModelKey:                   "gpt-4.1",
			DisplayName:                "GPT-4.1",
			Capabilities:               []string{"text"},
			Active:                     true,
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
			ID:                         "00000000-0000-0000-0000-000000000a02",
			ProviderID:                 "00000000-0000-0000-0000-000000000c01",
			ModelKey:                   "gpt-4.1-mini",
			DisplayName:                "GPT-4.1 mini",
			Capabilities:               []string{"text"},
			Active:                     true,
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
	}
}
