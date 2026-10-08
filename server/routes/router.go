package routes

import (
	"encoding/json"
	"net/http"

	"farkle-server/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter 装配 HTTP API 与 WebSocket 入口。
func NewRouter(hub *services.Hub) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.AllowContentType("application/json"))

	// REST API
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		// 大厅桌子列表（含实时人数）
		r.Get("/tables", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, hub.ListTables())
		})
	})

	// 大厅状态实时推送
	r.Get("/ws/lobby", func(w http.ResponseWriter, req *http.Request) {
		serveLobbyWS(hub, w, req)
	})

	// 游戏桌 WebSocket（/ws/{tableId}）
	r.Get("/ws/{tableId}", func(w http.ResponseWriter, req *http.Request) {
		serveGameWS(hub, w, req)
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
