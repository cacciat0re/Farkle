package migrations

import (
	"fmt"
	"sort"

	"gorm.io/gorm"
)

type migrationFunc struct {
	name string
	f    func(db *gorm.DB) error
}

var migrationFuncs []migrationFunc

func register(f migrationFunc) {
	migrationFuncs = append(migrationFuncs, f)
}

// Run 按名称顺序执行所有已注册的迁移，须在 AutoMigrate 之后调用
func Run(db *gorm.DB) error {
	sort.Slice(migrationFuncs, func(i, j int) bool {
		return migrationFuncs[i].name < migrationFuncs[j].name
	})

	for _, m := range migrationFuncs {
		if err := m.f(db); err != nil {
			return fmt.Errorf("执行迁移 %s: %w", m.name, err)
		}
	}
	return nil
}
