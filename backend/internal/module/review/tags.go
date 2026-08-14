package review

import "strings"

const (
	TagPolarityPositive = "positive"
	TagPolarityNegative = "negative"
	TagPolarityNeutral  = "neutral"
)

type tagRule struct {
	TagDefinition
	Aliases          []string
	TransactionTypes []string
	ReviewerRoles    []string
	RevieweeRoles    []string
}

var tagRules = []tagRule{
	{TagDefinition: TagDefinition{Code: "smooth_comm", Label: "沟通顺畅", Polarity: TagPolarityPositive}, Aliases: []string{"沟通顺畅"}},
	{TagDefinition: TagDefinition{Code: "quick_response", Label: "响应及时", Polarity: TagPolarityPositive}, Aliases: []string{"响应及时"}},
	{TagDefinition: TagDefinition{Code: "clear_rules", Label: "规则清晰", Polarity: TagPolarityPositive}, Aliases: []string{"规则清晰", "规则清楚"}},
	{TagDefinition: TagDefinition{Code: "good_coop", Label: "合作愉快", Polarity: TagPolarityPositive}, Aliases: []string{"合作愉快"}},
	{TagDefinition: TagDefinition{Code: "slow_response", Label: "响应较慢", Polarity: TagPolarityNegative}, Aliases: []string{"响应较慢"}},
	{TagDefinition: TagDefinition{Code: "hard_to_comm", Label: "沟通困难", Polarity: TagPolarityNegative}, Aliases: []string{"沟通困难"}},
	{TagDefinition: TagDefinition{Code: "late_change", Label: "临时变更", Polarity: TagPolarityNegative}, Aliases: []string{"临时变更"}},
	{
		TagDefinition: TagDefinition{Code: "true_desc", Label: "描述真实", Polarity: TagPolarityPositive},
		Aliases:       []string{"描述真实"}, ReviewerRoles: []string{RoleBuyer}, RevieweeRoles: []string{RoleSeller},
	},
	{
		TagDefinition: TagDefinition{Code: "clear_delivery", Label: "交付清晰", Polarity: TagPolarityPositive},
		Aliases:       []string{"交付清晰"}, TransactionTypes: []string{TransactionAPIOrder}, ReviewerRoles: []string{RoleBuyer}, RevieweeRoles: []string{RoleSeller},
	},
	{
		TagDefinition: TagDefinition{Code: "good_aftercare", Label: "售后响应及时", Polarity: TagPolarityPositive},
		Aliases:       []string{"售后响应及时"}, ReviewerRoles: []string{RoleBuyer}, RevieweeRoles: []string{RoleSeller},
	},
	{
		TagDefinition: TagDefinition{Code: "desc_diff", Label: "实际体验与描述有差异", Polarity: TagPolarityNegative},
		Aliases:       []string{"实际体验与描述有差异", "与描述不符"}, ReviewerRoles: []string{RoleBuyer}, RevieweeRoles: []string{RoleSeller},
	},
	{
		TagDefinition: TagDefinition{Code: "quick_payment", Label: "付款及时", Polarity: TagPolarityPositive},
		Aliases:       []string{"付款及时"}, ReviewerRoles: []string{RoleSeller}, RevieweeRoles: []string{RoleBuyer},
	},
	{
		TagDefinition: TagDefinition{Code: "quick_confirm", Label: "确认及时", Polarity: TagPolarityPositive},
		Aliases:       []string{"确认及时"}, ReviewerRoles: []string{RoleSeller}, RevieweeRoles: []string{RoleBuyer},
	},
	{
		TagDefinition: TagDefinition{Code: "clear_needs", Label: "需求清晰", Polarity: TagPolarityPositive},
		Aliases:       []string{"需求清晰"}, ReviewerRoles: []string{RoleSeller}, RevieweeRoles: []string{RoleBuyer},
	},
	{
		TagDefinition: TagDefinition{Code: "kept_agreement", Label: "遵守约定", Polarity: TagPolarityPositive},
		Aliases:       []string{"遵守约定"}, ReviewerRoles: []string{RoleSeller}, RevieweeRoles: []string{RoleBuyer},
	},
}

func AllowedTags(transactionType, reviewerRole, revieweeRole string) []TagDefinition {
	result := make([]TagDefinition, 0, len(tagRules))
	for _, rule := range tagRules {
		if matchesTagDimension(rule.TransactionTypes, transactionType) &&
			matchesTagDimension(rule.ReviewerRoles, reviewerRole) &&
			matchesTagDimension(rule.RevieweeRoles, revieweeRole) {
			result = append(result, rule.TagDefinition)
		}
	}
	return result
}

func AllTags() []TagDefinition {
	result := make([]TagDefinition, 0, len(tagRules))
	for _, rule := range tagRules {
		result = append(result, rule.TagDefinition)
	}
	return result
}

func NormalizeTagCodes(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		code, ok := tagCode(raw)
		if !ok {
			code = strings.TrimSpace(raw)
		}
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func DisplayTagLabels(tags []string) []string {
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		if rule, ok := findTagRule(raw); ok {
			result = append(result, rule.Label)
			continue
		}
		value := strings.TrimSpace(raw)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func ValidateTagsForScenario(tags []string, transactionType, reviewerRole, revieweeRole string) bool {
	allowed := AllowedTags(transactionType, reviewerRole, revieweeRole)
	allowedCodes := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedCodes[item.Code] = struct{}{}
	}
	for _, code := range NormalizeTagCodes(tags) {
		if _, ok := allowedCodes[code]; !ok {
			return false
		}
	}
	return true
}

func IsKnownTag(value string) bool {
	_, ok := findTagRule(value)
	return ok
}

func findTagRule(value string) (tagRule, bool) {
	trimmed := strings.TrimSpace(value)
	for _, rule := range tagRules {
		if rule.Code == trimmed {
			return rule, true
		}
		for _, alias := range rule.Aliases {
			if alias == trimmed {
				return rule, true
			}
		}
	}
	return tagRule{}, false
}

func tagCode(value string) (string, bool) {
	rule, ok := findTagRule(value)
	return rule.Code, ok
}

func matchesTagDimension(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}
