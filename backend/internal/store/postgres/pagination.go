package postgres

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
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

type scalarKeysetCursor struct {
	Version int    `json:"v"`
	Sort    string `json:"s"`
	Value   string `json:"value"`
	ID      string `json:"id"`
}

type scalarKeysetPosition struct {
	Value string
	ID    string
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

func decodeScalarKeysetCursor(value, sortMode string) (scalarKeysetPosition, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return scalarKeysetPosition{}, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return scalarKeysetPosition{}, invalidPageCursorError()
	}
	var payload scalarKeysetCursor
	if err := json.Unmarshal(body, &payload); err != nil {
		return scalarKeysetPosition{}, invalidPageCursorError()
	}
	payload.Sort = strings.TrimSpace(payload.Sort)
	payload.Value = strings.TrimSpace(payload.Value)
	payload.ID = strings.TrimSpace(payload.ID)
	if payload.Version != storePageCursorVersion || payload.Sort != strings.TrimSpace(sortMode) || payload.Value == "" || payload.ID == "" {
		return scalarKeysetPosition{}, invalidPageCursorError()
	}
	if _, err := uuid.Parse(payload.ID); err != nil {
		return scalarKeysetPosition{}, invalidPageCursorError()
	}
	return scalarKeysetPosition{Value: payload.Value, ID: payload.ID}, nil
}

func encodeScalarKeysetCursor(sortMode, value, id string) string {
	body, _ := json.Marshal(scalarKeysetCursor{
		Version: storePageCursorVersion,
		Sort:    strings.TrimSpace(sortMode),
		Value:   strings.TrimSpace(value),
		ID:      strings.TrimSpace(id),
	})
	return base64.RawURLEncoding.EncodeToString(body)
}

func validateNonNegativeDecimalCursor(position scalarKeysetPosition) *domain.AppError {
	value := strings.TrimSpace(position.Value)
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 || strings.Contains(value, "/") {
		return invalidPageCursorError()
	}
	return nil
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

func pageFromScalarItems[T any](items []T, request domain.PageRequest, sortMode string, cursorFor func(T) (string, string)) domain.Page[T] {
	request = normalizePageRequest(request)
	page := domain.Page[T]{Items: items}
	if len(items) <= request.Limit {
		return page
	}
	visible := append([]T(nil), items[:request.Limit]...)
	last := visible[len(visible)-1]
	value, id := cursorFor(last)
	next := encodeScalarKeysetCursor(sortMode, value, id)
	page.Items = visible
	page.NextCursor = &next
	return page
}

func invalidPageCursorError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}
