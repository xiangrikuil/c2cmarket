package server

import (
	"net/http"

	"c2c-market/backend/internal/buildinfo"
	"c2c-market/backend/internal/database"
)

type VersionResponse struct {
	Service                  string `json:"service"`
	Version                  string `json:"version"`
	GitCommit                string `json:"gitCommit"`
	BuildTime                string `json:"buildTime"`
	ExpectedMigrationVersion int64  `json:"expectedMigrationVersion"`
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	info := buildinfo.Current()
	writeJSON(w, http.StatusOK, VersionResponse{
		Service:                  "c2c-market-backend",
		Version:                  info.Version,
		GitCommit:                info.GitCommit,
		BuildTime:                info.BuildTime,
		ExpectedMigrationVersion: database.ExpectedMigrationVersion,
	})
}
