package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
)

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.metricsAuth && !matchesMetricsBearerToken(r.Header.Get("Authorization"), s.metricsToken) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		writeProblem(w, r, domain.NewError(
			http.StatusUnauthorized,
			"METRICS_AUTH_REQUIRED",
			"Metrics authentication required",
			"指标端点需要有效凭据。",
		))
		return
	}
	s.metrics.Handler().ServeHTTP(w, r)
}

func matchesMetricsBearerToken(header, expected string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || strings.TrimSpace(expected) == "" {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if provided == "" {
		return false
	}
	expectedDigest := sha256.Sum256([]byte(expected))
	providedDigest := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(expectedDigest[:], providedDigest[:]) == 1
}
