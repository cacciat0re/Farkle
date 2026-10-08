package config

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "config.yml"

	SQLDialectPostgres = "postgres"
)

type dbConfig struct {
	Dialect  string `yaml:"dialect"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"ssl_mode"`
}

type serverConfig struct {
	Addr string `yaml:"addr"`
}

// Config 服务端静态配置（对应 config.yml，缺省值见 defaultConfig）。
// 桌子列表不在此处，而是存放于数据库 tables 表中。
type Config struct {
	Db     dbConfig     `yaml:"db"`
	Server serverConfig `yaml:"server"`
}

// DSN 根据数据库配置拼装连接串，密码等特殊字符由 net/url 转义
func (c *dbConfig) DSN() string {
	u := url.URL{
		Scheme:   c.Dialect,
		User:     url.UserPassword(c.Username, c.Password),
		Host:     net.JoinHostPort(c.Host, fmt.Sprint(c.Port)),
		Path:     "/" + c.Name,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}
	return u.String()
}

// defaultConfig 未在 yaml 中声明的字段使用的缺省值
func defaultConfig() *Config {
	return &Config{
		Db: dbConfig{
			Dialect:  SQLDialectPostgres,
			Host:     "localhost",
			Port:     5432,
			Username: "postgres",
			SSLMode:  "disable",
		},
		Server: serverConfig{
			Addr: ":8080",
		},
	}
}

// LoadConfig 从 yaml 加载配置；文件缺失或字段非法时返回错误
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s（可参考 config.default.yml 创建）: %w", path, err)
	}

	cfg := defaultConfig()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 拼写错误或已废弃的字段直接报错，避免静默使用缺省值
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Db.Dialect != SQLDialectPostgres {
		return fmt.Errorf("不支持的数据库类型 db.dialect=%q，当前仅支持 %s", c.Db.Dialect, SQLDialectPostgres)
	}
	if c.Db.Name == "" {
		return fmt.Errorf("配置缺少 db.name")
	}
	if c.Server.Addr == "" {
		return fmt.Errorf("配置缺少 server.addr")
	}
	return nil
}
