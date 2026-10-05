package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := OpenDB(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	hub := NewHub(cfg, db)

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
		serveLobbyWs(hub, w, req)
	})

	// 游戏桌 WebSocket（/ws/{tableId}）
	r.Get("/ws/{tableId}", func(w http.ResponseWriter, req *http.Request) {
		serveWs(hub, w, req)
	})

	log.Printf("Farkle server listening on %s（%d 张桌子）", cfg.Server.Addr, len(cfg.Tables))
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, r))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
