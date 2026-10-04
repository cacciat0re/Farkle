package main

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// User 为后续 OIDC 用户预留；游客不会写入此表。
type User struct {
	ID        string    `gorm:"primaryKey;size:64" json:"id"`
	Name      string    `gorm:"size:32;index" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MatchRecord 对局记录（每局结束后写入）
type MatchRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoomID    string    `gorm:"size:64;index" json:"roomId"`
	Players   string    `gorm:"type:json" json:"players"` // [{"id","name","score"}]
	Winner    string    `gorm:"size:64" json:"winner"`
	CreatedAt time.Time `json:"createdAt"`
}

// DB 包装 gorm，提供领域操作
type DB struct {
	*gorm.DB
}

func OpenDB(dsn string) (*DB, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := gdb.AutoMigrate(&User{}, &MatchRecord{}); err != nil {
		return nil, fmt.Errorf("迁移数据表: %w", err)
	}
	return &DB{gdb}, nil
}
