package server

import (
	core "c2c-market/backend/internal/module/core"
	"net/http"
)

func NewRouter() http.Handler {
	return NewServer(core.NewService(), ServerOptions{EnableDevAuth: true})
}
