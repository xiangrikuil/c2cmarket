package server

import (
	"c2c-market/backend/internal/module/idempotency"
	httpresponse "c2c-market/backend/internal/response"
	"net/http"
	"strings"
)

type listResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

type problemDetails = httpresponse.ProblemDetails

func writeIdempotencyCompletion(w http.ResponseWriter, completion idempotency.Completion) {
	for name, value := range completion.Headers {
		if strings.TrimSpace(name) != "" && strings.TrimSpace(value) != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Content-Type", completion.ContentType)
	w.WriteHeader(completion.Status)
	_, _ = w.Write(completion.Body)
}

func writeNoStoreIdempotencyCompletion(w http.ResponseWriter, completion idempotency.Completion) {
	w.Header().Set("Cache-Control", "private, no-store")
	writeIdempotencyCompletion(w, completion)
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	httpresponse.WriteJSON(w, status, payload)
}

func setETag(w http.ResponseWriter, version int64) {
	httpresponse.SetETag(w, version)
}

func writeProblem(w http.ResponseWriter, r *http.Request, err error) {
	httpresponse.WriteProblem(w, r, err, requestIDFrom(r))
}
