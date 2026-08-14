package domain

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

type PageRequest struct {
	Limit  int
	Cursor string
}

type Page[T any] struct {
	Items      []T
	NextCursor *string
}

type pageOffsetCursor struct {
	Offset *int `json:"offset"`
}

func PageItems[T any](items []T, request PageRequest) (Page[T], *AppError) {
	limit := request.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset, appErr := decodePageOffset(request.Cursor)
	if appErr != nil {
		return Page[T]{}, appErr
	}
	if offset >= len(items) {
		return Page[T]{Items: []T{}}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := Page[T]{
		Items: append([]T(nil), items[offset:end]...),
	}
	if end < len(items) {
		next := encodePageOffset(end)
		page.NextCursor = &next
	}
	return page, nil
}

func decodePageOffset(cursor string) (int, *AppError) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, invalidPageOffsetCursorError()
	}
	var payload pageOffsetCursor
	if err := json.Unmarshal(body, &payload); err != nil || payload.Offset == nil || *payload.Offset < 0 {
		return 0, invalidPageOffsetCursorError()
	}
	return *payload.Offset, nil
}

func encodePageOffset(offset int) string {
	body, _ := json.Marshal(pageOffsetCursor{Offset: &offset})
	return base64.RawURLEncoding.EncodeToString(body)
}

func invalidPageOffsetCursorError() *AppError {
	return NewFieldError(http.StatusUnprocessableEntity, CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}
