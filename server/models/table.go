package models

// MaxPlayersPerTable 当前阶段只开放双人桌
const MaxPlayersPerTable = 2

// Table 大厅桌子，持久化于 tables 表；服务启动时据此创建运行时桌子
type Table struct {
	ID         string `gorm:"primaryKey;size:64"`
	Name       string `gorm:"size:32;not null"`
	MaxPlayers int    `gorm:"not null;default:2"`
}
