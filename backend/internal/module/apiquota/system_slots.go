package apiquota

import (
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
)

const (
	systemSlotKeyLayout    = "2006-01-02@15:04"
	systemSlotDuration     = 30 * time.Minute
	systemSlotRegistration = time.Hour
	systemSlotCalendarDays = 7
)

var systemSlotHours = [...]int{20}

func SystemSaleSlots(now time.Time) []SystemSaleSlot {
	location := shanghaiLocation()
	serverNow := now.UTC()
	localNow := now.In(location)
	firstDay := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	slots := make([]SystemSaleSlot, 0, systemSlotCalendarDays*len(systemSlotHours))
	for dayOffset := 0; dayOffset < systemSlotCalendarDays; dayOffset++ {
		day := firstDay.AddDate(0, 0, dayOffset)
		for _, hour := range systemSlotHours {
			startsAt := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, location)
			slots = append(slots, newSystemSaleSlot(startsAt, serverNow))
		}
	}
	return slots
}

func ResolveOpenSystemSaleSlot(key string, now time.Time) (SystemSaleSlot, *domain.AppError) {
	slot, appErr := ResolveSystemSaleSlot(key, now)
	if appErr != nil {
		return SystemSaleSlot{}, appErr
	}
	if slot.State == SystemSlotStateRegistrationOpen {
		return slot, nil
	}
	return SystemSaleSlot{}, domain.NewFieldError(
		http.StatusConflict,
		domain.CodeInvalidStateTransition,
		"Quota sale slot registration closed",
		"所选场次已停止报名，请选择更晚场次。",
		"slotKey",
		"registration_closed",
		"所选场次已停止报名，请选择更晚场次。",
	)
}

func ResolveSystemSaleSlot(key string, now time.Time) (SystemSaleSlot, *domain.AppError) {
	key = strings.TrimSpace(key)
	for _, slot := range SystemSaleSlots(now) {
		if slot.Key == key {
			return slot, nil
		}
	}
	return SystemSaleSlot{}, domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeValidationFailed,
		"Quota sale slot invalid",
		"所选场次不属于未来 7 天的平台固定场次。",
		"slotKey",
		"invalid",
		"请选择平台返回的有效场次。",
	)
}

func IsSystemSaleSlotKey(key string) bool {
	key = strings.TrimSpace(key)
	parsed, err := time.ParseInLocation(systemSlotKeyLayout, key, shanghaiLocation())
	if err != nil || parsed.Format(systemSlotKeyLayout) != key {
		return false
	}
	for _, hour := range systemSlotHours {
		if parsed.Hour() == hour && parsed.Minute() == 0 {
			return true
		}
	}
	return false
}

func newSystemSaleSlot(startsAt, serverNow time.Time) SystemSaleSlot {
	endsAt := startsAt.Add(systemSlotDuration)
	registrationClosesAt := startsAt.Add(-systemSlotRegistration)
	state := SystemSlotStateEnded
	switch {
	case serverNow.Before(registrationClosesAt):
		state = SystemSlotStateRegistrationOpen
	case serverNow.Before(startsAt):
		state = SystemSlotStateRegistrationClosed
	case serverNow.Before(endsAt):
		state = SystemSlotStateActive
	}
	return SystemSaleSlot{
		Key:                  startsAt.Format(systemSlotKeyLayout),
		StartsAt:             startsAt.UTC(),
		EndsAt:               endsAt.UTC(),
		RegistrationClosesAt: registrationClosesAt.UTC(),
		State:                state,
		ServerNow:            serverNow,
	}
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic("Asia/Shanghai time zone unavailable")
	}
	return location
}
