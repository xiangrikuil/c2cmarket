package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerificationCodeHash is the single challenge digest contract shared by
// email registration, bind-email, and future reset purposes.
func VerificationCodeHash(pepper []byte, purpose, subject, email, code string) string {
	mac := hmac.New(sha256.New, pepper)
	_, _ = mac.Write([]byte(strings.TrimSpace(purpose)))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.TrimSpace(subject)))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(mac.Sum(nil))
}
