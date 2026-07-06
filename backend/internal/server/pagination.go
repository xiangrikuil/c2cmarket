package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

type pageResult[T any] struct {
	Items      []T
	NextCursor *string
}

type cursorPayload struct {
	Offset int `json:"offset"`
}

func paginateSlice[T any](r *http.Request, items []T) (pageResult[T], *domain.AppError) {
	limit, offset, appErr := parsePagination(r)
	if appErr != nil {
		return pageResult[T]{}, appErr
	}
	if offset >= len(items) {
		return pageResult[T]{Items: []T{}}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	result := pageResult[T]{
		Items: append([]T(nil), items[offset:end]...),
	}
	if end < len(items) {
		next := encodeCursor(end)
		result.NextCursor = &next
	}
	return result, nil
}

func writePaginatedJSON[T any](w http.ResponseWriter, r *http.Request, items []T) bool {
	page, appErr := paginateSlice(r, items)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return false
	}
	writeJSON(w, http.StatusOK, listResponse[T]{Items: page.Items, NextCursor: page.NextCursor})
	return true
}

func parsePageRequest(r *http.Request) (domain.PageRequest, *domain.AppError) {
	values := r.URL.Query()
	limit := defaultPageLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxPageLimit {
			return domain.PageRequest{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid limit", "分页 limit 必须是 1 到 100 之间的整数。", "limit", "invalid", "limit 必须是 1 到 100 之间的整数。")
		}
		limit = parsed
	}
	return domain.PageRequest{
		Limit:  limit,
		Cursor: strings.TrimSpace(values.Get("cursor")),
	}, nil
}

func writePageJSON[T any](w http.ResponseWriter, page domain.Page[T]) {
	writeJSON(w, http.StatusOK, listResponse[T]{Items: page.Items, NextCursor: page.NextCursor})
}

func parsePagination(r *http.Request) (int, int, *domain.AppError) {
	values := r.URL.Query()
	limit := defaultPageLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxPageLimit {
			return 0, 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid limit", "分页 limit 必须是 1 到 100 之间的整数。", "limit", "invalid", "limit 必须是 1 到 100 之间的整数。")
		}
		limit = parsed
	}
	offset := 0
	if raw := strings.TrimSpace(values.Get("cursor")); raw != "" {
		decoded, appErr := decodeCursor(raw)
		if appErr != nil {
			return 0, 0, appErr
		}
		offset = decoded
	}
	return limit, offset, nil
}

func encodeCursor(offset int) string {
	body, _ := json.Marshal(cursorPayload{Offset: offset})
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeCursor(value string) (int, *domain.AppError) {
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, invalidCursorError()
	}
	var payload cursorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, invalidCursorError()
	}
	if payload.Offset < 0 {
		return 0, invalidCursorError()
	}
	return payload.Offset, nil
}

func invalidCursorError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}
