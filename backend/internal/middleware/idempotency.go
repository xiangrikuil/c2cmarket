package middleware

import (
	"net/http"
	"strings"
)

const IdempotencyKeyHeader = "Idempotency-Key"

func IdempotencyKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(IdempotencyKeyHeader))
}
