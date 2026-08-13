package server

import (
	"net/http"

	"c2c-market/backend/internal/module/auth"
)

func requireCapability(w http.ResponseWriter, r *http.Request, user auth.User, capability string) bool {
	appErr := auth.RequireCapability(user, capability)
	if appErr == nil {
		return true
	}
	writeProblem(w, r, appErr)
	return false
}

func requireActorCapability(w http.ResponseWriter, r *http.Request, actor auth.BusinessActor, capability string) bool {
	appErr := auth.RequireProjectedCapability(actor.Capabilities, capability)
	if appErr == nil {
		return true
	}
	writeProblem(w, r, appErr)
	return false
}
