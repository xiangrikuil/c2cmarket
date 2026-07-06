package server

import (
	"context"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "c2c-market-backend",
	})
}

type readinessResponse struct {
	Status                string  `json:"status"`
	Database              string  `json:"database"`
	SchemaVersion         *int64  `json:"schemaVersion,omitempty"`
	SchemaDirty           *bool   `json:"schemaDirty,omitempty"`
	ExpectedSchemaVersion int64   `json:"expectedSchemaVersion,omitempty"`
	CheckedAt             string  `json:"checkedAt"`
	Reason                *string `json:"reason,omitempty"`
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if s.readinessChecker == nil {
		writeJSON(w, http.StatusOK, readinessResponse{
			Status:    "ok",
			Database:  "not_configured",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := s.readinessChecker.Readiness(ctx)
	payload := readinessResponse{
		Status:                "ok",
		Database:              "ok",
		SchemaVersion:         status.SchemaVersion,
		SchemaDirty:           status.SchemaDirty,
		ExpectedSchemaVersion: status.ExpectedSchemaVersion,
		CheckedAt:             status.CheckedAt.UTC().Format(time.RFC3339),
	}
	code := http.StatusOK
	if !status.Configured {
		payload.Database = "not_configured"
	} else if !status.OK {
		payload.Status = "degraded"
		payload.Database = "error"
		code = http.StatusServiceUnavailable
		if status.FailureSummary != "" {
			payload.Reason = &status.FailureSummary
		}
	}
	writeJSON(w, code, payload)
}
