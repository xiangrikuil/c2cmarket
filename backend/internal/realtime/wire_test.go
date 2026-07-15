package realtime

import "testing"

func TestParseNotificationPayload(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		wantAudience audience
		wantUserID   string
	}{
		{
			name:         "user",
			payload:      `{"v":1,"audience":"user","userId":"user-a"}`,
			wantAudience: audienceUser,
			wantUserID:   "user-a",
		},
		{
			name:         "admin",
			payload:      `{"v":1,"audience":"admin"}`,
			wantAudience: audienceAdmin,
		},
		{
			name:         "all",
			payload:      `{"v":1,"audience":"all"}`,
			wantAudience: audienceAll,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalidation, err := parseNotificationPayload(test.payload)
			if err != nil {
				t.Fatalf("parseNotificationPayload() error = %v", err)
			}
			if invalidation.audience != test.wantAudience || invalidation.userID != test.wantUserID {
				t.Fatalf("parseNotificationPayload() = %#v, want audience %v user %q", invalidation, test.wantAudience, test.wantUserID)
			}
		})
	}
}

func TestParseNotificationPayloadRejectsInvalidRoutingContracts(t *testing.T) {
	payloads := map[string]string{
		"empty":                 "",
		"unsupported version":   `{"v":2,"audience":"all"}`,
		"unknown audience":      `{"v":1,"audience":"merchant"}`,
		"missing user id":       `{"v":1,"audience":"user"}`,
		"blank user id":         `{"v":1,"audience":"user","userId":"  "}`,
		"padded user id":        `{"v":1,"audience":"user","userId":" user-a "}`,
		"admin with user id":    `{"v":1,"audience":"admin","userId":"user-a"}`,
		"all with user id":      `{"v":1,"audience":"all","userId":"user-a"}`,
		"unknown field":         `{"v":1,"audience":"all","businessData":"hidden"}`,
		"multiple json values":  `{"v":1,"audience":"all"} {"v":1,"audience":"all"}`,
		"non-object json value": `[]`,
	}

	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			if _, err := parseNotificationPayload(payload); err == nil {
				t.Fatal("parseNotificationPayload() error = nil, want routing-contract error")
			}
		})
	}
}

func TestPublishInvalidationUsesParsedAudience(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	user := mustSubscribe(t, hub, "user-a", false)
	defer user.Close()
	admin := mustSubscribe(t, hub, "admin-a", true)
	defer admin.Close()

	publishInvalidation(hub, routedInvalidation{audience: audienceUser, userID: "user-a"})
	expectWakeup(t, user.Events())
	expectNoWakeup(t, admin.Events())

	publishInvalidation(hub, routedInvalidation{audience: audienceAdmin})
	expectNoWakeup(t, user.Events())
	expectWakeup(t, admin.Events())
}
