package apihealth

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const preflightTokenTTL = 10 * time.Minute

type preflightGrant struct {
	OwnerUserID           string
	ConnectionID          string
	ExpectedVersion       int64
	CanonicalBaseURL      string
	CredentialFingerprint string
	ProbeModel            string
	ProbeProtocol         string
	Result                PreflightResult
	ExpiresAt             time.Time
}

type preflightStore struct {
	mu     sync.Mutex
	grants map[string]preflightGrant
}

func newPreflightToken() (string, error) {
	value := make([]byte, 32)
	if _, err := cryptorand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func credentialFingerprint(credential string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(credential)))
	return hex.EncodeToString(sum[:])
}

func (store *preflightStore) issue(grant preflightGrant, now time.Time) (string, error) {
	token, err := newPreflightToken()
	if err != nil {
		return "", err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.grants == nil {
		store.grants = make(map[string]preflightGrant)
	}
	for existingToken, existing := range store.grants {
		if !existing.ExpiresAt.After(now) {
			delete(store.grants, existingToken)
		}
	}
	grant.ExpiresAt = now.Add(preflightTokenTTL)
	store.grants[token] = grant
	return token, nil
}

func (store *preflightStore) consume(token string, now time.Time) (preflightGrant, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return preflightGrant{}, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	grant, found := store.grants[token]
	delete(store.grants, token)
	if !found || !grant.ExpiresAt.After(now) {
		return preflightGrant{}, false
	}
	return grant, true
}
