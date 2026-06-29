package validator

import (
	"bytes"
	"c2c-market/backend/internal/domain"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func DecodeJSON(r *http.Request, dst any) *domain.AppError {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return domain.NewError(http.StatusBadRequest, domain.CodeValidationFailed, "Invalid JSON", "请求 JSON 格式不正确或包含未知字段。")
	}
	return nil
}

func DecodeStrictJSON[T any](r *http.Request) ([]byte, T, *domain.AppError) {
	var zero T
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, zero, domain.NewError(http.StatusBadRequest, domain.CodeValidationFailed, "Invalid body", "读取请求体失败。")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var req T
	if err := decoder.Decode(&req); err != nil {
		return nil, zero, domain.NewError(http.StatusBadRequest, domain.CodeValidationFailed, "Invalid JSON", "请求 JSON 格式不正确或包含未知字段。")
	}
	return body, req, nil
}

func DecodeStrictJSONOnly[T any](r *http.Request) (T, *domain.AppError) {
	_, req, appErr := DecodeStrictJSON[T](r)
	return req, appErr
}

func ParseOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, value)
}

func ParseRequiredTime(value, field string) (time.Time, *domain.AppError) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Time invalid", "时间格式不正确。", field, "invalid", "时间必须是 ISO 8601。")
	}
	return parsed, nil
}

func RequireIfMatchVersion(r *http.Request) (int64, *domain.AppError) {
	match := strings.TrimSpace(r.Header.Get("If-Match"))
	if match == "" {
		return 0, domain.NewError(http.StatusPreconditionRequired, domain.CodePreconditionRequired, "Precondition required", "审核动作必须提供 If-Match 资源版本。")
	}
	match = strings.Trim(match, `"`)
	value, err := strconv.ParseInt(match, 10, 64)
	if err != nil || value <= 0 {
		return 0, domain.NewFieldError(http.StatusPreconditionRequired, domain.CodePreconditionRequired, "Precondition required", "If-Match 资源版本格式不正确。", "If-Match", "invalid", "If-Match 必须是正整数版本。")
	}
	return value, nil
}

func RequestHash(method, routeKey string, body []byte) string {
	sum := sha256.Sum256(append([]byte(method+" "+routeKey+"\n"), body...))
	return hex.EncodeToString(sum[:])
}
