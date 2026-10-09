package migrations

import (
	"farkle-server/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	register(migrationFunc{
		name: "20261009-init_tables",
		f:    initTables,
	})
}

// initTables 先建表（仅在表不存在时），再在 tables 表为空时写入初始大厅桌子；
// 两步放在同一个迁移中，保证种子数据总在表创建之后写入
func initTables(db *gorm.DB) error {
	if err := createTables(db); err != nil {
		return err
	}
	return seedDefaultTables(db)
}

// createTables 已有表的结构变更需通过新的迁移完成
func createTables(db *gorm.DB) error {
	for _, m := range []any{&models.Table{}, &models.User{}, &models.MatchRecord{}} {
		if db.Migrator().HasTable(m) {
			continue
		}
		if err := db.Migrator().CreateTable(m); err != nil {
			return err
		}
	}
	return nil
}

// seedDefaultTables 在 tables 表为空时写入初始大厅桌子，已有桌子时不做任何改动
func seedDefaultTables(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Table{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tables := []models.Table{
		{ID: "apple", Name: "Apple", MaxPlayers: models.MaxPlayersPerTable},
		{ID: "butter", Name: "Butter", MaxPlayers: models.MaxPlayersPerTable},
		{ID: "charlie", Name: "Charlie", MaxPlayers: models.MaxPlayersPerTable},
		{ID: "delta", Name: "Delta", MaxPlayers: models.MaxPlayersPerTable},
		{ID: "echo", Name: "Echo", MaxPlayers: models.MaxPlayersPerTable},
		{ID: "foxtrot", Name: "Foxtrot", MaxPlayers: models.MaxPlayersPerTable},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&tables).Error
}
