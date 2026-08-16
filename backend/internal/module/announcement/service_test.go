package announcement

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func TestPublishAnnouncementUsesActionTimeForOldDraft(t *testing.T) {
	now := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createAnnouncementForPublishTest(t, service, admin, now.Add(-10*24*time.Hour), nil)

	published, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("publish announcement: %v", appErr)
	}
	if published.Status != StatusPublished || DisplayStatus(published, now) != StatusPublished {
		t.Fatalf("expected published status, got stored=%q display=%q", published.Status, DisplayStatus(published, now))
	}
	if !published.PublishAt.Equal(now) || !published.UpdatedAt.Equal(now) {
		t.Fatalf("expected action timestamps at %s, got publishAt=%s updatedAt=%s", now, published.PublishAt, published.UpdatedAt)
	}

	rows, appErr := service.AdminAnnouncements(context.Background(), admin)
	if appErr != nil {
		t.Fatalf("list admin announcements: %v", appErr)
	}
	found := false
	for _, row := range rows {
		if row.ID == published.ID {
			found = DisplayStatus(row, now) == StatusPublished
			break
		}
	}
	if !found {
		t.Fatal("published announcement was not visible in the published admin state")
	}
}

func TestPublishAnnouncementPreservesFutureSchedule(t *testing.T) {
	now := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	plannedAt := now.Add(24 * time.Hour)
	expiresAt := plannedAt.Add(2 * time.Hour)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createAnnouncementForPublishTest(t, service, admin, plannedAt, &expiresAt)

	scheduled, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("schedule announcement: %v", appErr)
	}
	if scheduled.Status != StatusScheduled || DisplayStatus(scheduled, now) != StatusScheduled {
		t.Fatalf("expected scheduled status, got stored=%q display=%q", scheduled.Status, DisplayStatus(scheduled, now))
	}
	if !scheduled.PublishAt.Equal(plannedAt) || !scheduled.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected scheduled timestamps publishAt=%s updatedAt=%s", scheduled.PublishAt, scheduled.UpdatedAt)
	}
}

func TestPublishAnnouncementRejectsElapsedEndTime(t *testing.T) {
	now := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Hour)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createAnnouncementForPublishTest(t, service, admin, now.Add(-10*24*time.Hour), &expiresAt)

	_, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed || appErr.Detail != "结束时间已过，请调整结束时间后再发布。" || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "expireAt" {
		t.Fatalf("expected expireAt validation error, got %#v", appErr)
	}

	unchanged, getErr := service.AdminAnnouncement(context.Background(), admin, item.ID)
	if getErr != nil {
		t.Fatalf("read unchanged announcement: %v", getErr)
	}
	if unchanged.Status != StatusDraft || !unchanged.PublishAt.Equal(item.PublishAt) || unchanged.Version != item.Version {
		t.Fatalf("failed publish mutated announcement: before=%+v after=%+v", item, unchanged)
	}
}

func TestResolvePublishTransitionRejectsEndTimeBeforeFutureSchedule(t *testing.T) {
	now := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	publishAt := now.Add(24 * time.Hour)
	expireAt := publishAt.Add(-time.Minute)
	item := Announcement{Status: StatusDraft, PublishAt: publishAt, ExpireAt: &expireAt}

	_, _, appErr := ResolvePublishTransition(item, now)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed || appErr.Detail != "结束时间必须晚于计划发布时间，请调整后再发布。" || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "expireAt" {
		t.Fatalf("expected future publish window validation error, got %#v", appErr)
	}
}

func TestDisplayStatusDerivesLifecycleAtReadTime(t *testing.T) {
	publishAt := time.Date(2026, 8, 10, 4, 0, 0, 0, time.UTC)
	expireAt := publishAt.Add(2 * time.Hour)
	item := Announcement{Status: StatusScheduled, PublishAt: publishAt, ExpireAt: &expireAt}

	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{name: "before publish", now: publishAt.Add(-time.Nanosecond), want: StatusScheduled},
		{name: "at publish", now: publishAt, want: StatusPublished},
		{name: "before expiry", now: expireAt.Add(-time.Nanosecond), want: StatusPublished},
		{name: "at expiry", now: expireAt, want: StatusExpired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DisplayStatus(item, tc.now); got != tc.want {
				t.Fatalf("DisplayStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContentUpdatedAtOnlyTracksNormalizedUserVisibleContent(t *testing.T) {
	now := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createAnnouncementForPublishTest(t, service, admin, now.Add(-time.Hour), nil)
	createdContentUpdatedAt := item.ContentUpdatedAt
	if !createdContentUpdatedAt.Equal(now) {
		t.Fatalf("create contentUpdatedAt = %s, want %s", createdContentUpdatedAt, now)
	}

	now = now.Add(time.Hour)
	managementForm := formFromAnnouncement(item)
	managementForm.Title = "  " + item.Title + "  "
	managementForm.Summary = "  " + item.Summary + "  "
	managementForm.ContentMarkdown = "\n" + item.ContentMarkdown + "\n"
	managementForm.Channels = []string{ChannelMessageCenter, ChannelHomeBanner}
	managementForm.IsPinned = true
	managementForm.IsDismissible = false
	managementOnly, appErr := service.UpdateAnnouncement(context.Background(), admin, item.ID, managementForm)
	if appErr != nil {
		t.Fatalf("update management fields: %v", appErr)
	}
	if !managementOnly.ContentUpdatedAt.Equal(createdContentUpdatedAt) {
		t.Fatalf("management update advanced contentUpdatedAt from %s to %s", createdContentUpdatedAt, managementOnly.ContentUpdatedAt)
	}
	if !managementOnly.UpdatedAt.Equal(now) {
		t.Fatalf("management update updatedAt = %s, want %s", managementOnly.UpdatedAt, now)
	}

	now = now.Add(time.Hour)
	contentForm := formFromAnnouncement(managementOnly)
	contentForm.Title = managementOnly.Title + "（更新）"
	contentEdited, appErr := service.UpdateAnnouncement(context.Background(), admin, item.ID, contentForm)
	if appErr != nil {
		t.Fatalf("update visible content: %v", appErr)
	}
	if !contentEdited.ContentUpdatedAt.Equal(now) {
		t.Fatalf("content update contentUpdatedAt = %s, want %s", contentEdited.ContentUpdatedAt, now)
	}

	now = now.Add(time.Hour)
	published, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("publish announcement: %v", appErr)
	}
	if !published.ContentUpdatedAt.Equal(contentEdited.ContentUpdatedAt) {
		t.Fatalf("publish advanced contentUpdatedAt from %s to %s", contentEdited.ContentUpdatedAt, published.ContentUpdatedAt)
	}

	now = now.Add(time.Hour)
	offline, appErr := service.OfflineAnnouncement(context.Background(), admin, item.ID, "公告生命周期测试完成")
	if appErr != nil {
		t.Fatalf("offline announcement: %v", appErr)
	}
	if !offline.ContentUpdatedAt.Equal(contentEdited.ContentUpdatedAt) {
		t.Fatalf("offline advanced contentUpdatedAt from %s to %s", contentEdited.ContentUpdatedAt, offline.ContentUpdatedAt)
	}

	now = now.Add(time.Hour)
	duplicated, appErr := service.DuplicateAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("duplicate announcement: %v", appErr)
	}
	if !duplicated.ContentUpdatedAt.Equal(now) {
		t.Fatalf("duplicate contentUpdatedAt = %s, want %s", duplicated.ContentUpdatedAt, now)
	}
}

func TestCriticalAnnouncementValidationMatrix(t *testing.T) {
	valid := FormInput{
		Title:           "紧急服务通知",
		Summary:         "用于验证紧急公告渠道和确认知悉约束。",
		ContentMarkdown: "## 紧急服务通知\n\n请确认当前服务状态。",
		Category:        CategoryRisk,
		Level:           LevelCritical,
		Channels:        []string{ChannelMessageCenter, ChannelGlobalBar, ChannelModal},
		Audience:        Audience{Type: AudienceAll},
		IsDismissible:   false,
		RequiresAck:     true,
		PublishAt:       time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC),
	}
	if appErr := validateForm(valid); appErr != nil {
		t.Fatalf("valid critical form: %v", appErr)
	}

	cases := []struct {
		name   string
		mutate func(*FormInput)
		field  string
	}{
		{name: "missing modal", mutate: func(form *FormInput) { form.Channels = []string{ChannelMessageCenter, ChannelGlobalBar} }, field: "level"},
		{name: "missing acknowledgement", mutate: func(form *FormInput) { form.RequiresAck = false }, field: "level"},
		{name: "dismissible", mutate: func(form *FormInput) { form.IsDismissible = true }, field: "level"},
		{name: "important modal", mutate: func(form *FormInput) { form.Level = LevelImportant; form.RequiresAck = false }, field: "channels"},
		{name: "normal global", mutate: func(form *FormInput) { form.Level = LevelNormal; form.RequiresAck = false; form.IsDismissible = true }, field: "level"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := valid
			form.Channels = append([]string(nil), valid.Channels...)
			tc.mutate(&form)
			appErr := validateForm(form)
			if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != tc.field {
				t.Fatalf("expected %s validation error, got %#v", tc.field, appErr)
			}
		})
	}
}

func TestAnnouncementReceiptActionsRemainDistinctAndVersioned(t *testing.T) {
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createCriticalAnnouncement(t, service, admin, now.Add(-time.Hour))
	item, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("publish critical announcement: %v", appErr)
	}
	user := auth.User{ID: "announcement-user"}

	now = now.Add(time.Minute)
	read, appErr := service.MarkRead(context.Background(), user, item.ID)
	if appErr != nil {
		t.Fatalf("mark read: %v", appErr)
	}
	if read.ReadAt == nil || read.FirstSeenAt != nil || read.AcknowledgedAt != nil {
		t.Fatalf("read action conflated receipt timestamps: %+v", read)
	}

	now = now.Add(time.Minute)
	acknowledged, appErr := service.Acknowledge(context.Background(), user, item.ID)
	if appErr != nil {
		t.Fatalf("acknowledge critical announcement: %v", appErr)
	}
	if acknowledged.ReadAt == nil || acknowledged.FirstSeenAt == nil || acknowledged.AcknowledgedAt == nil {
		t.Fatalf("acknowledgement did not preserve read and record delivery: %+v", acknowledged)
	}
	if _, appErr := service.Dismiss(context.Background(), user, item.ID); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("critical announcement dismiss should fail, got %#v", appErr)
	}
}

func TestAnnouncementDeliveryRevisionIgnoresManagementAndOrderingChanges(t *testing.T) {
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin := announcementTestAdmin()
	item := createCriticalAnnouncement(t, service, admin, now.Add(-time.Hour))
	item, appErr := service.PublishAnnouncement(context.Background(), admin, item.ID)
	if appErr != nil {
		t.Fatalf("publish critical announcement: %v", appErr)
	}
	user := auth.User{ID: "announcement-user"}
	if _, appErr := service.Acknowledge(context.Background(), user, item.ID); appErr != nil {
		t.Fatalf("acknowledge critical announcement: %v", appErr)
	}

	now = now.Add(time.Hour)
	management := formFromAnnouncement(item)
	management.IsPinned = true
	management.Channels = []string{ChannelModal, ChannelGlobalBar, ChannelMessageCenter}
	updated, appErr := service.UpdateAnnouncement(context.Background(), admin, item.ID, management)
	if appErr != nil {
		t.Fatalf("management update: %v", appErr)
	}
	if updated.Version != item.Version {
		t.Fatalf("management update changed delivery revision from %d to %d", item.Version, updated.Version)
	}

	rows, appErr := service.UserAnnouncements(context.Background(), user)
	if appErr != nil {
		t.Fatalf("read user announcements: %v", appErr)
	}
	current := findAnnouncementByID(t, rows, item.ID)
	if !IsAcknowledged(current) {
		t.Fatal("management update invalidated acknowledgement")
	}

	now = now.Add(time.Hour)
	material := formFromAnnouncement(updated)
	material.Title += "（更新）"
	revised, appErr := service.UpdateAnnouncement(context.Background(), admin, item.ID, material)
	if appErr != nil {
		t.Fatalf("material update: %v", appErr)
	}
	if revised.Version != item.Version+1 {
		t.Fatalf("material update revision = %d, want %d", revised.Version, item.Version+1)
	}
	rows, _ = service.UserAnnouncements(context.Background(), user)
	if IsAcknowledged(findAnnouncementByID(t, rows, item.ID)) {
		t.Fatal("material update retained stale acknowledgement")
	}
}

func TestAnnouncementAudienceMatchingAndDeliveryOrdering(t *testing.T) {
	linuxDO := auth.User{ID: "linuxdo", LinuxDoBinding: &auth.LinuxDoBinding{Bound: true}}
	student := auth.User{ID: "student", StudentClaim: &auth.StudentEmailClaim{}}
	admin := auth.User{ID: "admin", IsAdmin: true}
	studentAdmin := auth.User{ID: "student-admin", IsAdmin: true, StudentClaim: &auth.StudentEmailClaim{}}

	cases := []struct {
		name     string
		audience Audience
		user     auth.User
		want     bool
	}{
		{name: "linuxdo buyer", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleBuyer}}, user: linuxDO, want: true},
		{name: "student buyer", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleBuyer}}, user: student, want: true},
		{name: "linuxdo merchant", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleMerchant}}, user: linuxDO, want: true},
		{name: "student not merchant", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleMerchant}}, user: student, want: false},
		{name: "durable admin", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleAdmin}}, user: admin, want: true},
		{name: "student-only admin excluded", audience: Audience{Type: AudienceRoles, Roles: []string{AudienceRoleAdmin}}, user: studentAdmin, want: false},
		{name: "specific match", audience: Audience{Type: AudienceSpecificUsers, UserIDs: []string{"linuxdo"}}, user: linuxDO, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := audienceMatchesUser(tc.audience, tc.user); got != tc.want {
				t.Fatalf("audienceMatchesUser() = %t, want %t", got, tc.want)
			}
		})
	}

	publishedAt := time.Date(2026, 8, 16, 4, 0, 0, 0, time.UTC)
	items := []Announcement{
		{ID: "important", Level: LevelImportant, IsPinned: true, PublishAt: publishedAt.Add(time.Hour)},
		{ID: "critical-b", Level: LevelCritical, PublishAt: publishedAt},
		{ID: "critical-a", Level: LevelCritical, PublishAt: publishedAt},
	}
	SortByDeliveryPriority(items)
	if items[0].ID != "critical-a" || items[1].ID != "critical-b" || items[2].ID != "important" {
		t.Fatalf("unexpected delivery order: %v", []string{items[0].ID, items[1].ID, items[2].ID})
	}
}

func createCriticalAnnouncement(t *testing.T, service *Service, admin auth.User, publishAt time.Time) Announcement {
	t.Helper()
	item, appErr := service.CreateAnnouncement(context.Background(), admin, FormInput{
		Title:           "紧急服务通知",
		Summary:         "用于验证紧急公告强触达和确认回执。",
		ContentMarkdown: "## 紧急服务通知\n\n请确认当前服务状态。",
		Category:        CategoryRisk,
		Level:           LevelCritical,
		Channels:        []string{ChannelMessageCenter, ChannelGlobalBar, ChannelModal},
		Audience:        Audience{Type: AudienceAll},
		IsDismissible:   false,
		RequiresAck:     true,
		PublishAt:       publishAt,
	})
	if appErr != nil {
		t.Fatalf("create critical announcement: %v", appErr)
	}
	return item
}

func findAnnouncementByID(t *testing.T, items []Announcement, id string) Announcement {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("announcement %s not found", id)
	return Announcement{}
}

func createAnnouncementForPublishTest(t *testing.T, service *Service, admin auth.User, publishAt time.Time, expireAt *time.Time) Announcement {
	t.Helper()
	item, appErr := service.CreateAnnouncement(context.Background(), admin, FormInput{
		Title:           "公告发布时间测试",
		Summary:         "用于验证公告立即发布、定时发布与结束时间规则。",
		ContentMarkdown: "## 公告发布时间测试\n\n这是一条用于自动化验证的公告正文。",
		Category:        CategoryPlatform,
		Level:           LevelNormal,
		Channels:        []string{ChannelMessageCenter},
		IsDismissible:   true,
		PublishAt:       publishAt,
		ExpireAt:        expireAt,
	})
	if appErr != nil {
		t.Fatalf("create announcement: %v", appErr)
	}
	return item
}

func formFromAnnouncement(item Announcement) FormInput {
	return FormInput{
		Title:           item.Title,
		Summary:         item.Summary,
		ContentMarkdown: item.ContentMarkdown,
		Category:        item.Category,
		Level:           item.Level,
		Channels:        append([]string(nil), item.Channels...),
		Audience:        item.Audience,
		IsPinned:        item.IsPinned,
		IsDismissible:   item.IsDismissible,
		RequiresAck:     item.RequiresAck,
		CTALabel:        item.CTALabel,
		CTAURL:          item.CTAURL,
		PublishAt:       item.PublishAt,
		ExpireAt:        item.ExpireAt,
	}
}

func announcementTestAdmin() auth.User {
	return auth.User{ID: "admin-announcement-test", DisplayName: "公告测试管理员", IsAdmin: true}
}
