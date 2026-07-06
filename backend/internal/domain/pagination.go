package domain

import (
	"encoding/base64"
	"encoding/json"
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
	Offset int `json:"offset"`
}

func PageItems[T any](items []T, request PageRequest) Page[T] {
	limit := request.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := decodePageOffset(request.Cursor)
	if offset >= len(items) {
		return Page[T]{Items: []T{}}
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
	return page
}

func decodePageOffset(cursor string) int {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0
	}
	body, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	var payload pageOffsetCursor
	if err := json.Unmarshal(body, &payload); err != nil || payload.Offset < 0 {
		return 0
	}
	return payload.Offset
}

func encodePageOffset(offset int) string {
	body, _ := json.Marshal(pageOffsetCursor{Offset: offset})
	return base64.RawURLEncoding.EncodeToString(body)
}
