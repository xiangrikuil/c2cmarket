package auth

import (
	"strings"
	"unicode"
)

const (
	RegistrationSourceCampaign = "campaign"
	RegistrationSourceReferral = "referral"
	RegistrationSourceDirect   = "direct"
	RegistrationSourceUnknown  = "unknown"
)

func NormalizeRegistrationAttribution(input RegistrationAttribution) RegistrationAttribution {
	source := sanitizeAttributionText(input.Source, 100)
	medium := sanitizeAttributionText(input.Medium, 100)
	campaign := sanitizeAttributionText(input.Campaign, 100)
	referrerHost := normalizeReferrerHost(input.ReferrerHost)
	result := RegistrationAttribution{
		Source:       source,
		Medium:       medium,
		Campaign:     campaign,
		ReferrerHost: referrerHost,
		LandingPath:  NormalizeAttributionPath(input.LandingPath),
	}
	switch {
	case source != "" || medium != "" || campaign != "":
		result.SourceType = RegistrationSourceCampaign
		if result.Source == "" {
			result.Source = RegistrationSourceCampaign
		}
	case referrerHost != "":
		result.SourceType = RegistrationSourceReferral
		result.Source = referrerHost
	default:
		result.SourceType = RegistrationSourceDirect
		result.Source = RegistrationSourceDirect
	}
	return result
}

func NormalizeAttributionPath(value string) string {
	path := strings.TrimSpace(strings.SplitN(strings.SplitN(value, "?", 2)[0], "#", 2)[0])
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/"
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return "/"
	}
	switch segments[0] {
	case "carpools":
		if len(segments) > 1 && segments[1] != "new" {
			return "/carpools/:id"
		}
		return "/carpools"
	case "api-market":
		if len(segments) > 1 && segments[1] != "new" {
			return "/api-market/:id"
		}
		return "/api-market"
	case "official-prices", "search", "login", "announcements":
		return "/" + segments[0]
	case "my", "merchant", "admin", "u":
		return "/" + segments[0]
	default:
		return "/other"
	}
}

func sanitizeAttributionText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	runeCount := 0
	for _, character := range value {
		if unicode.IsControl(character) {
			continue
		}
		if runeCount >= limit {
			break
		}
		builder.WriteRune(character)
		runeCount++
	}
	return strings.TrimSpace(builder.String())
}

func normalizeReferrerHost(value string) string {
	value = strings.ToLower(sanitizeAttributionText(value, 255))
	value = strings.TrimSuffix(value, ".")
	if value == "" || strings.ContainsAny(value, "/?#@: \\") {
		return ""
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '.' && character != '-' {
			return ""
		}
	}
	return value
}
