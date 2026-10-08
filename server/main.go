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

	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := repositories.OpenDB(cfg.Db.DSN())
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	tables, err := repositories.NewTableRepository(db.DB).List()
	if err != nil {
		log.Fatalf("加载桌子失败: %v", err)
	}
	if len(tables) == 0 {
		log.Fatal("数据库中没有桌子，请先执行 server/sql/seed_tables.sql")
	}

	hub := services.NewHub(tables)
	handler := routes.NewRouter(hub)

	log.Printf("Server listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, handler))
}
