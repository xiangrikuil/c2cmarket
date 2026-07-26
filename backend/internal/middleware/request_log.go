package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func WithRequestLogging(logger *log.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	var writeMu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		entry, err := json.Marshal(struct {
			Event      string  `json:"event"`
			Method     string  `json:"method"`
			Path       string  `json:"path"`
			Status     int     `json:"status"`
			DurationMS float64 `json:"duration_ms"`
			RequestID  string  `json:"request_id"`
			ClientIP   string  `json:"client_ip"`
		}{
			Event:      "http_request",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     status,
			DurationMS: float64(time.Since(startedAt).Microseconds()) / 1000,
			RequestID:  RequestIDFromRequest(r),
			ClientIP:   ClientIPFromRequest(r),
		})
		if err == nil {
			writeMu.Lock()
			_, _ = logger.Writer().Write(append(entry, '\n'))
			writeMu.Unlock()
		}
	})
}
