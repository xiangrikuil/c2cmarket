package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/announcement"
)

func TestAnnouncementDTOProjectionsKeepTargetingAndReceiptsPrivate(t *testing.T) {
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	acknowledgedAt := now.Add(time.Minute)
	item := announcement.Announcement{
		ID:              "00000000-0000-0000-0000-000000000109",
		Slug:            "critical-delivery",
		Title:           "紧急服务通知",
		Summary:         "用于验证公告响应投影不会泄露指定用户和回执。",
		ContentMarkdown: "## 紧急服务通知\n\n请确认当前服务状态。",
		Category:        announcement.CategoryRisk,
		Level:           announcement.LevelCritical,
		Status:          announcement.StatusPublished,
		Channels:        []string{announcement.ChannelMessageCenter, announcement.ChannelGlobalBar, announcement.ChannelModal},
		Audience: announcement.Audience{
			Type:    announcement.AudienceSpecificUsers,
			UserIDs: []string{"00000000-0000-0000-0000-000000000001"},
		},
		RequiresAck:      true,
		IsPinned:         true,
		PublishAt:        now,
		ContentUpdatedAt: now,
		Version:          2,
		CreatedBy:        "00000000-0000-0000-0000-000000000002",
		UpdatedBy:        "00000000-0000-0000-0000-000000000002",
		CreatedAt:        now,
		UpdatedAt:        now,
		Receipt: &announcement.Receipt{
			AnnouncementID:      "00000000-0000-0000-0000-000000000109",
			AnnouncementVersion: 2,
			AcknowledgedAt:      &acknowledgedAt,
		},
	}

	userDTO := toAnnouncementDTO(item)
	if len(userDTO.Audience.UserIDs) != 0 || userDTO.Receipt == nil {
		t.Fatalf("user projection should hide target ids and retain own receipt: %+v", userDTO)
	}
	adminDTO := toAdminAnnouncementDTO(item)
	if len(adminDTO.Audience.UserIDs) != 1 {
		t.Fatalf("admin projection lost explicit target ids: %+v", adminDTO.Audience)
	}

	item.Audience = announcement.Audience{Type: announcement.AudienceAll}
	encoded, err := json.Marshal(toPublicAnnouncementDTO(item))
	if err != nil {
		t.Fatalf("marshal public announcement: %v", err)
	}
	if !strings.Contains(string(encoded), `"isPinned":true`) {
		t.Fatalf("public projection lost delivery ordering metadata: %s", encoded)
	}
	for _, forbidden := range []string{"userIds", "receipt", "createdBy", "updatedBy", "operator"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("public projection leaked %q: %s", forbidden, encoded)
		}
	}
}
