package main

import (
	"log"
	"net/http"
	"os"

	"farkle-server/config"
	"farkle-server/repositories"
	"farkle-server/routes"
	"farkle-server/services"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 当前对局运行时不依赖数据库；保留初始化以维持现有启动行为，
	// 并为后续用户与对局记录落库预留。
	if _, err := repositories.OpenDB(cfg.Database.DSN); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	hub := services.NewHub(cfg)
	handler := routes.NewRouter(hub)

	log.Printf("Server listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, handler))
}
