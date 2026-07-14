package server

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
)

const (
	realtimeHeartbeatInterval = 15 * time.Second
	realtimeWriteTimeout      = 5 * time.Second
)

var realtimeClientPayload = `{"schemaVersion":1,"topics":["all-live"]}`

func (s *Server) handleMyEvents(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	subscription, err := s.realtimeHub.Subscribe(user.ID, user.IsAdmin)
	if err != nil {
		writeProblem(w, r, domain.NewError(http.StatusServiceUnavailable, domain.CodeInternalError, "Realtime unavailable", "实时更新暂时不可用，请稍后重试。"))
		return
	}
	defer subscription.Close()

	controller := http.NewResponseController(w)
	if err := setRealtimeWriteDeadline(controller, time.Time{}); err != nil {
		writeProblem(w, r, domain.NewError(http.StatusServiceUnavailable, domain.CodeInternalError, "Realtime unavailable", "实时更新暂时不可用，请稍后重试。"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := writeRealtimeFrame(w, controller, "retry: 3000\n\nevent: ready\ndata: "+realtimeClientPayload+"\n\n"); err != nil {
		return
	}

	heartbeat := time.NewTicker(realtimeHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case _, ok := <-subscription.Events():
			if !ok {
				return
			}
			if err := writeRealtimeEvent(w, controller, "invalidate"); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeRealtimeFrame(w, controller, ": heartbeat\n\n"); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func writeRealtimeEvent(w http.ResponseWriter, controller *http.ResponseController, eventName string) error {
	return writeRealtimeFrame(w, controller, fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, realtimeClientPayload))
}

func writeRealtimeFrame(w http.ResponseWriter, controller *http.ResponseController, frame string) error {
	if err := setRealtimeWriteDeadline(controller, time.Now().Add(realtimeWriteTimeout)); err != nil {
		return err
	}
	defer func() {
		_ = setRealtimeWriteDeadline(controller, time.Time{})
	}()
	if _, err := fmt.Fprint(w, frame); err != nil {
		return err
	}
	return controller.Flush()
}

func setRealtimeWriteDeadline(controller *http.ResponseController, deadline time.Time) error {
	err := controller.SetWriteDeadline(deadline)
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}
