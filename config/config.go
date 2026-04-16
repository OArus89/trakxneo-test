package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host      string          `yaml:"host"`
	Gateway   GatewayConfig   `yaml:"gateway"`
	API       APIConfig       `yaml:"api"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	Postgres  PostgresConfig  `yaml:"postgres"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Dragonfly DragonflyConfig `yaml:"dragonfly"`
	Auth      AuthConfig      `yaml:"auth"`
}

type GatewayConfig struct {
	TeltonikaPort int `yaml:"teltonika_port"`
	RuptelaPort   int `yaml:"ruptela_port"`
}

type APIConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type WebSocketConfig struct {
	Port int    `yaml:"port"`
	URL  string `yaml:"url"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.User, p.Password, p.Host, p.Port, p.Database)
}

type ClickHouseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
}

type DragonflyConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (d DragonflyConfig) Addr() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type envFile struct {
	Environments map[string]Config `yaml:"environments"`
	Default      string            `yaml:"default"`
}

// Load reads the env.yaml config for the given target environment.
// If target is empty, uses TARGET env var or the default from config.
func Load(target string) (*Config, error) {
	if target == "" {
		target = os.Getenv("TARGET")
	}

	_, filename, _, _ := runtime.Caller(0)
	configPath := filepath.Join(filepath.Dir(filename), "env.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var ef envFile
	if err := yaml.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if target == "" {
		target = ef.Default
	}

	cfg, ok := ef.Environments[target]
	if !ok {
		return nil, fmt.Errorf("unknown environment %q (available: %v)", target, keys(ef.Environments))
	}

	// Allow overriding the host IP via TARGET_HOST env var
	if hostOverride := os.Getenv("TARGET_HOST"); hostOverride != "" {
		cfg.Host = hostOverride
		cfg.API.BaseURL = fmt.Sprintf("http://%s:%d", hostOverride, cfg.API.Port)
		cfg.WebSocket.URL = fmt.Sprintf("wss://%s:%d/ws", hostOverride, cfg.WebSocket.Port)
		cfg.Postgres.Host = hostOverride
		cfg.ClickHouse.Host = hostOverride
		cfg.Dragonfly.Host = hostOverride
	}

	// Allow overriding auth credentials
	if email := os.Getenv("AUTH_EMAIL"); email != "" {
		cfg.Auth.Username = email
	}
	if pw := os.Getenv("AUTH_PASSWORD"); pw != "" {
		cfg.Auth.Password = pw
	}

	return &cfg, nil
}

func keys(m map[string]Config) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
