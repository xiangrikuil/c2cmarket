package response

import (
	"c2c-market/backend/internal/domain"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type ProblemDetails struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Code      string              `json:"code"`
	Detail    string              `json:"detail"`
	Instance  string              `json:"instance"`
	RequestID string              `json:"requestId"`
	Errors    []domain.FieldError `json:"errors,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func SetETag(w http.ResponseWriter, version int64) {
	if version > 0 {
		w.Header().Set("ETag", `"`+strconv.FormatInt(version, 10)+`"`)
	}
}

func WriteProblem(w http.ResponseWriter, r *http.Request, err error, requestID string) {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		appErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "服务内部错误。")
	}
	if appErr.Status == 0 {
		appErr.Status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(appErr.Status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:      "https://c2cmarket.local/problems/" + strings.ToLower(strings.ReplaceAll(appErr.Code, "_", "-")),
		Title:     appErr.Title,
		Status:    appErr.Status,
		Code:      appErr.Code,
		Detail:    appErr.Detail,
		Instance:  r.URL.Path,
		RequestID: requestID,
		Errors:    appErr.FieldErrors,
	})
}
