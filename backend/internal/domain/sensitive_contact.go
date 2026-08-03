package domain

import (
	"regexp"
	"strings"
)

var (
	contactEmailPattern   = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,63}\b`)
	mainlandMobilePattern = regexp.MustCompile(
		`(?:^|[^\d])(?:\+?86[\s-]?)?1[3-9]\d{9}(?:$|[^\d])`,
	)
	contactAssignmentPattern = regexp.MustCompile(
		`(?i)(?:微信|wechat|weixin|qq|telegram|tg|电报|邮箱|email|e-mail|手机号|手机|电话|phone)\s*(?:号|号码|账号|id)?\s*[:=：]\s*[^\s，。；;]{3,}`,
	)
	contactLabeledValuePattern = regexp.MustCompile(
		`(?i)(?:微信|wechat|weixin|qq|telegram|tg|电报)\s*(?:号|账号|id)\s+[a-z0-9_@.+\-]{3,}`,
	)
)

// LooksLikeContactContent detects explicit contact values while allowing
// policy copy such as "do not include a phone number or WeChat ID".
func LooksLikeContactContent(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	for _, candidate := range sensitiveContentCandidates(text) {
		if contactEmailPattern.MatchString(candidate) ||
			mainlandMobilePattern.MatchString(candidate) ||
			contactAssignmentPattern.MatchString(candidate) ||
			contactLabeledValuePattern.MatchString(candidate) {
			return true
		}
	}
	return false
}
