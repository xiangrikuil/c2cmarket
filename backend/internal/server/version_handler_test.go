package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"c2c-market/backend/internal/buildinfo"
	"c2c-market/backend/internal/database"
)

func TestVersionHandlerReturnsInjectedBuildAndMigrationMetadata(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.GitCommit
	originalBuildTime := buildinfo.BuildTime
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.GitCommit = originalCommit
		buildinfo.BuildTime = originalBuildTime
	})

	buildinfo.Version = "v1.2.3"
	buildinfo.GitCommit = "0123456789abcdef0123456789abcdef01234567"
	buildinfo.BuildTime = "2026-07-26T12:00:00Z"

	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	response := httptest.NewRecorder()
	(&Server{}).handleVersion(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var payload VersionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	want := VersionResponse{
		Service:                  "c2c-market-backend",
		Version:                  buildinfo.Version,
		GitCommit:                buildinfo.GitCommit,
		BuildTime:                buildinfo.BuildTime,
		ExpectedMigrationVersion: database.ExpectedMigrationVersion,
	}
	if payload != want {
		t.Fatalf("version response = %+v, want %+v", payload, want)
	}
}
