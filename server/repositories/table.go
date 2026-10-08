package repositories

import (
	"farkle-server/models"

	"gorm.io/gorm"
)

type TableRepository interface {
	List() ([]models.Table, error)
}

type tableRepository struct {
	db *gorm.DB
}

func NewTableRepository(db *gorm.DB) TableRepository {
	return &tableRepository{db: db}
}

// List 按 id 升序返回所有桌子；当前阶段每桌人数固定为 MaxPlayersPerTable，统一收紧
func (r *tableRepository) List() ([]models.Table, error) {
	var tables []models.Table
	if err := r.db.Order("id").Find(&tables).Error; err != nil {
		return nil, err
	}
	for i := range tables {
		tables[i].MaxPlayers = models.MaxPlayersPerTable
	}
	return tables, nil
}
