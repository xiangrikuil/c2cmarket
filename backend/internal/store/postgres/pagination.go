package postgres

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

const storePageCursorVersion = 1

type keysetCursor struct {
	Version int    `json:"v"`
	Time    string `json:"t"`
	ID      string `json:"id"`
}

type keysetPosition struct {
	Time time.Time
	ID   string
}

func normalizePageRequest(request domain.PageRequest) domain.PageRequest {
	if request.Limit < 1 {
		request.Limit = 20
	}
	if request.Limit > 100 {
		request.Limit = 100
	}
	request.Cursor = strings.TrimSpace(request.Cursor)
	return request
}

func decodeKeysetCursor(value string) (keysetPosition, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return keysetPosition{}, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return keysetPosition{}, invalidPageCursorError()
	}
	var payload keysetCursor
	if err := json.Unmarshal(body, &payload); err != nil {
		return keysetPosition{}, invalidPageCursorError()
	}
	if payload.Version != storePageCursorVersion || strings.TrimSpace(payload.ID) == "" || strings.TrimSpace(payload.Time) == "" {
		return keysetPosition{}, invalidPageCursorError()
	}
	sortTime, err := time.Parse(time.RFC3339Nano, payload.Time)
	if err != nil {
		return keysetPosition{}, invalidPageCursorError()
	}
	id := strings.TrimSpace(payload.ID)
	if _, err := uuid.Parse(id); err != nil {
		return keysetPosition{}, invalidPageCursorError()
	}
	return keysetPosition{Time: sortTime, ID: id}, nil
}

func encodeKeysetCursor(sortTime time.Time, id string) string {
	body, _ := json.Marshal(keysetCursor{
		Version: storePageCursorVersion,
		Time:    sortTime.UTC().Format(time.RFC3339Nano),
		ID:      strings.TrimSpace(id),
	})
	return base64.RawURLEncoding.EncodeToString(body)
}

func pageFromItems[T any](items []T, request domain.PageRequest, cursorFor func(T) (time.Time, string)) domain.Page[T] {
	request = normalizePageRequest(request)
	page := domain.Page[T]{Items: items}
	if len(items) <= request.Limit {
		return page
	}
	visible := append([]T(nil), items[:request.Limit]...)
	last := visible[len(visible)-1]
	sortTime, id := cursorFor(last)
	next := encodeKeysetCursor(sortTime, id)
	page.Items = visible
	page.NextCursor = &next
	return page
}

func invalidPageCursorError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}
