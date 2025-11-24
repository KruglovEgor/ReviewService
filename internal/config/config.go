package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config содержит конфигурацию приложения
type Config struct {
	// Server конфигурация HTTP сервера
	Server ServerConfig
	
	// Database конфигурация PostgreSQL
	Database DatabaseConfig
	
	// App конфигурация приложения
	App AppConfig
}

// ServerConfig конфигурация HTTP сервера
type ServerConfig struct {
	Host         string        `envconfig:"SERVER_HOST" default:"0.0.0.0"`
	Port         int           `envconfig:"SERVER_PORT" default:"8080"`
	ReadTimeout  time.Duration `envconfig:"SERVER_READ_TIMEOUT" default:"10s"`
	WriteTimeout time.Duration `envconfig:"SERVER_WRITE_TIMEOUT" default:"10s"`
	IdleTimeout  time.Duration `envconfig:"SERVER_IDLE_TIMEOUT" default:"60s"`
}

// DatabaseConfig конфигурация PostgreSQL
type DatabaseConfig struct {
	Host            string        `envconfig:"DB_HOST" default:"postgres"`
	Port            int           `envconfig:"DB_PORT" default:"5432"`
	User            string        `envconfig:"DB_USER" default:"reviewservice"`
	Password        string        `envconfig:"DB_PASSWORD" default:"password"`
	Name            string        `envconfig:"DB_NAME" default:"reviewservice"`
	SSLMode         string        `envconfig:"DB_SSLMODE" default:"disable"`
	MaxOpenConns    int           `envconfig:"DB_MAX_OPEN_CONNS" default:"25"`
	MaxIdleConns    int           `envconfig:"DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime time.Duration `envconfig:"DB_CONN_MAX_LIFETIME" default:"5m"`
	MigrationsPath  string        `envconfig:"DB_MIGRATIONS_PATH" default:"file://migrations"`
}

// AppConfig конфигурация приложения
type AppConfig struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Env      string `envconfig:"APP_ENV" default:"development"`
}

// Address возвращает адрес для прослушивания HTTP сервера
func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DSN возвращает строку подключения к PostgreSQL
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{}
	
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}
	
	return cfg, nil
}
