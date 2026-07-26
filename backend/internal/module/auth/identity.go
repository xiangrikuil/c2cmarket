package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
)

const (
	InitialAdminBootstrapKey = "initial-admin-v1"
	maxUsernameLength        = 24
)

type adminBootstrapRun struct {
	UserID   string
	Username string
}

func CanonicalOAuthProvider(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func CanonicalOAuthSubject(value string) string {
	return strings.TrimSpace(value)
}

func OAuthIdentityKey(provider, subject string) string {
	return CanonicalOAuthProvider(provider) + "\x00" + CanonicalOAuthSubject(subject)
}

func IsLinuxDoProvider(provider string) bool {
	return CanonicalOAuthProvider(provider) == "linux_do"
}

func OAuthUsernameCandidate(rawUsername, provider, subject string, attempt int) string {
	base := oauthUsernameBase(rawUsername)
	if attempt <= 0 {
		return truncateUsername(base, maxUsernameLength)
	}

	sum := sha256.Sum256([]byte(CanonicalOAuthProvider(provider) + "\x00" + CanonicalOAuthSubject(subject)))
	stableSuffix := hex.EncodeToString(sum[:4])
	if attempt > 1 {
		stableSuffix += "-" + strconv.Itoa(attempt)
	}
	maxBaseLength := maxUsernameLength - len(stableSuffix) - 1
	if maxBaseLength < 1 {
		maxBaseLength = 1
	}
	base = strings.Trim(truncateUsername(base, maxBaseLength), "-_")
	if base == "" {
		base = "u"
	}
	return base + "-" + stableSuffix
}

func AdminBootstrapConflictError() *domain.AppError {
	return domain.NewError(
		http.StatusConflict,
		domain.CodeAdminBootstrapConflict,
		"Administrator bootstrap conflict",
		"管理员初始化与现有账号或权限状态冲突。",
	)
}

func AdminBootstrapInconsistentError() *domain.AppError {
	return domain.NewError(
		http.StatusInternalServerError,
		domain.CodeAdminBootstrapInconsistent,
		"Administrator bootstrap inconsistent",
		"管理员初始化来源记录与账号状态不一致。",
	)
}

func oauthUsernameBase(value string) string {
	value = normalizeUsername(value)
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		}
	}
	base := strings.Trim(builder.String(), "-_")
	if len(base) < 3 {
		base = strings.Trim(base+"-user", "-_")
	}
	if base == "" {
		return "user"
	}
	return base
}

func truncateUsername(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}
