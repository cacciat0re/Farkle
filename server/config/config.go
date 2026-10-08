package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const MaxPlayersPerTable = 2

// TableConfig 一张桌子的配置
type TableConfig struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	MaxPlayers int    `yaml:"max_players"`
}

// Config 服务端配置（对应 config.yaml，全部可扩展）
type Config struct {
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Tables []TableConfig `yaml:"tables"`
}

// LoadConfig 从 yaml 加载配置；文件缺失或字段非法时返回错误
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("配置缺少 database.dsn")
	}
	if len(cfg.Tables) == 0 {
		return nil, fmt.Errorf("配置至少需要一个桌子 (tables)")
	}
	for i, t := range cfg.Tables {
		if t.ID == "" || t.Name == "" {
			return nil, fmt.Errorf("tables[%d] 缺少 id 或 name", i)
		}
		// 当前阶段只开放双人桌，配置值统一收紧为 2
		cfg.Tables[i].MaxPlayers = MaxPlayersPerTable
	}
	return &cfg, nil
}
