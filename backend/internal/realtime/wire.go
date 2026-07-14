package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	SchemaVersion = 1
	ChannelName   = "c2c_market_realtime"
)

type audience uint8

const (
	audienceUser audience = iota + 1
	audienceAdmin
	audienceAll
)

type notificationEnvelope struct {
	Version  int    `json:"v"`
	Audience string `json:"audience"`
	UserID   string `json:"userId,omitempty"`
}

type routedInvalidation struct {
	audience audience
	userID   string
}

func parseNotificationPayload(payload string) (routedInvalidation, error) {
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()

	var envelope notificationEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return routedInvalidation{}, fmt.Errorf("decode realtime notification: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return routedInvalidation{}, errors.New("decode realtime notification: multiple JSON values")
		}
		return routedInvalidation{}, fmt.Errorf("decode realtime notification trailer: %w", err)
	}
	if envelope.Version != SchemaVersion {
		return routedInvalidation{}, errors.New("unsupported realtime notification schema version")
	}

	switch envelope.Audience {
	case "user":
		if !validRoutingUserID(envelope.UserID) {
			return routedInvalidation{}, errors.New("user-scoped realtime notification requires userId")
		}
		return routedInvalidation{audience: audienceUser, userID: envelope.UserID}, nil
	case "admin":
		if envelope.UserID != "" {
			return routedInvalidation{}, errors.New("admin-scoped realtime notification must not include userId")
		}
		return routedInvalidation{audience: audienceAdmin}, nil
	case "all":
		if envelope.UserID != "" {
			return routedInvalidation{}, errors.New("global realtime notification must not include userId")
		}
		return routedInvalidation{audience: audienceAll}, nil
	default:
		return routedInvalidation{}, errors.New("unsupported realtime notification audience")
	}
}

func publishInvalidation(hub *Hub, invalidation routedInvalidation) {
	switch invalidation.audience {
	case audienceUser:
		hub.PublishUser(invalidation.userID)
	case audienceAdmin:
		hub.PublishAdmin()
	case audienceAll:
		hub.PublishAll()
	}
}
