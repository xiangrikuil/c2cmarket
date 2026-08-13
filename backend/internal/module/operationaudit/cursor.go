package operationaudit

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

const cursorVersion = 1

type cursorPayload struct {
	Version    int    `json:"v"`
	OccurredAt string `json:"t"`
	SourceKind string `json:"s"`
	EventID    string `json:"id"`
}

func EncodeCursor(position CursorPosition) string {
	body, _ := json.Marshal(cursorPayload{
		Version:    cursorVersion,
		OccurredAt: position.OccurredAt.UTC().Format(time.RFC3339Nano),
		SourceKind: strings.TrimSpace(position.SourceKind),
		EventID:    strings.TrimSpace(position.EventID),
	})
	return base64.RawURLEncoding.EncodeToString(body)
}

func DecodeCursor(value string) (*CursorPosition, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, invalidCursorError()
	}
	var payload cursorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, invalidCursorError()
	}
	payload.SourceKind = strings.TrimSpace(payload.SourceKind)
	payload.EventID = strings.TrimSpace(payload.EventID)
	if payload.Version != cursorVersion || !contains(SourceKinds, payload.SourceKind) {
		return nil, invalidCursorError()
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.OccurredAt))
	if err != nil {
		return nil, invalidCursorError()
	}
	if _, err := uuid.Parse(payload.EventID); err != nil {
		return nil, invalidCursorError()
	}
	return &CursorPosition{OccurredAt: occurredAt.UTC(), SourceKind: payload.SourceKind, EventID: payload.EventID}, nil
}

func invalidCursorError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid cursor", "分页 cursor 无效。", "cursor", "invalid", "cursor 无效或已过期。")
}
