package domain

import (
	"net/url"
	"regexp"
	"strings"
)

const secretDecodePasses = 2

var (
	authorizationBearerPattern = regexp.MustCompile(`(?i)\b(?:authorization\s*[:=]\s*)?bearer\s+[^\s，。；;]+`)
	secretAssignmentPattern    = regexp.MustCompile(`(?i)\b(?:authorization\s*[:=]\s*bearer|bearer|x-api-key|api[_\s-]*key|apikey|sub2api\s*key|openai_api_key|anthropic_api_key|access_token|refresh_token|session|cookie|password|passwd|pwd|secret|token|key)\s*[:=]\s*[^\s，。；;]+`)
	secretPrefixPattern        = regexp.MustCompile(`(?i)\b(?:sk-(?:proj-|ant-)?[a-z0-9_-]*|akia[0-9a-z]{8,}|ghp_[0-9a-z_]+|github_pat_[0-9a-z_]+|xoxb-[0-9a-z-]+|eyj[a-z0-9_-]*)\b`)
	jwtPattern                 = regexp.MustCompile(`(?i)\b[a-z0-9_-]{3,}\.[a-z0-9_-]{3,}\.[a-z0-9_-]{3,}\b`)
	longOpaqueTokenPattern     = regexp.MustCompile(`(?i)\b[a-z0-9_-]{40,}\b`)
	urlPattern                 = regexp.MustCompile(`(?i)https?://[^\s"'<>，。；;）)]+`)
)

// LooksLikeSecretContent only matches credential-shaped content, not bare
// educational words such as "token" or "API key" in safety copy.
func LooksLikeSecretContent(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	for _, candidate := range sensitiveContentCandidates(text) {
		if looksLikeSecretContentPlain(candidate) || containsSensitiveURL(candidate) {
			return true
		}
	}
	return false
}

func sensitiveContentCandidates(text string) []string {
	candidates := []string{text}
	current := text
	for i := 0; i < secretDecodePasses; i++ {
		decoded, err := url.QueryUnescape(current)
		if err != nil || decoded == current {
			break
		}
		candidates = append(candidates, decoded)
		current = decoded
	}
	return candidates
}

func looksLikeSecretContentPlain(text string) bool {
	lower := strings.ToLower(text)
	needles := []string{
		"-----begin",
		"sub_url=", "sub_url:", "suburl=", "suburl:",
		"trojan://", "vmess://", "ss://", "ssr://", "socks://", "socks5://", "vless://", "clash://", "hysteria://", "hy2://", "tuic://", "sub://",
		"面板账号", "面板密码",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	if authorizationBearerPattern.MatchString(text) || secretAssignmentPattern.MatchString(text) || secretPrefixPattern.MatchString(text) || jwtPattern.MatchString(text) || longOpaqueTokenPattern.MatchString(text) {
		return true
	}
	return false
}

func containsSensitiveURL(text string) bool {
	for _, raw := range urlPattern.FindAllString(text, -1) {
		parsed, err := url.Parse(strings.TrimRight(raw, ".,!?"))
		if err != nil {
			continue
		}
		if isSensitiveParsedURL(parsed, 0) {
			return true
		}
	}
	return false
}

func isSensitiveParsedURL(parsed *url.URL, depth int) bool {
	path := strings.ToLower(parsed.EscapedPath())
	if decodedPath, err := url.PathUnescape(parsed.EscapedPath()); err == nil {
		path = strings.ToLower(decodedPath)
	}
	if strings.Contains(path, "client/subscribe") || strings.Contains(path, "/subscribe") || path == "/sub" || strings.HasSuffix(path, "/sub") {
		return true
	}
	for key, values := range parsed.Query() {
		key = strings.ToLower(key)
		if isSensitiveQueryKey(key) {
			return true
		}
		for _, value := range values {
			if isSensitiveQueryValue(key, value, depth) {
				return true
			}
		}
	}
	return false
}

func isSensitiveQueryKey(key string) bool {
	return strings.Contains(key, "token") ||
		strings.Contains(key, "key") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "authorization") ||
		key == "auth" ||
		key == "jwt" ||
		strings.Contains(key, "subscribe") ||
		key == "sub"
}

func isSensitiveQueryValue(key, value string, depth int) bool {
	for _, candidate := range sensitiveContentCandidates(value) {
		lowerValue := strings.ToLower(candidate)
		if strings.Contains(lowerValue, "clash") ||
			strings.Contains(lowerValue, "vless://") ||
			strings.Contains(lowerValue, "vmess://") ||
			strings.Contains(lowerValue, "trojan://") ||
			strings.Contains(lowerValue, "ss://") ||
			strings.Contains(lowerValue, "ssr://") ||
			strings.Contains(lowerValue, "socks://") ||
			strings.Contains(lowerValue, "socks5://") ||
			strings.Contains(lowerValue, "client/subscribe") ||
			strings.Contains(lowerValue, "/subscribe") {
			return true
		}
		if depth >= secretDecodePasses || !isNestedURLQueryKey(key) {
			continue
		}
		nested, err := url.Parse(candidate)
		if err != nil || nested.Scheme == "" || nested.Host == "" {
			continue
		}
		if isSensitiveParsedURL(nested, depth+1) {
			return true
		}
	}
	return false
}

func isNestedURLQueryKey(key string) bool {
	switch key {
	case "url", "link", "redirect", "redirect_url", "target", "subscribe", "subscription", "sub":
		return true
	default:
		return false
	}
}
