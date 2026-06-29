package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/validator"
	"net/http"
	"time"
)

func decodeJSON(r *http.Request, dst any) *domain.AppError {
	return validator.DecodeJSON(r, dst)
}

func decodeStrictJSON[T any](r *http.Request) ([]byte, T, *domain.AppError) {
	return validator.DecodeStrictJSON[T](r)
}

func decodeStrictJSONOnly[T any](r *http.Request) (T, *domain.AppError) {
	return validator.DecodeStrictJSONOnly[T](r)
}
func parseOptionalTime(value string) (time.Time, error) {
	return validator.ParseOptionalTime(value)
}

func parseRequiredTime(value, field string) (time.Time, *domain.AppError) {
	return validator.ParseRequiredTime(value, field)
}

func requireIfMatchVersion(r *http.Request) (int64, *domain.AppError) {
	return validator.RequireIfMatchVersion(r)
}

func requestHash(method, routeKey string, body []byte) string {
	return validator.RequestHash(method, routeKey, body)
}

func requestIDFrom(r *http.Request) string {
	return middleware.RequestIDFromRequest(r)
}
