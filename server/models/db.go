package models

import "time"

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
