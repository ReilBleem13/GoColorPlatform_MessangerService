package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	App      App
	Database Database
	Redis    Redis
	JWT      JWT
}

type App struct {
	Port string `env:"PORT" env-required:"true"`
}

type JWT struct {
	Secret                 string `env:"JWT_SECRET" env-required:"true"`
	AccessExpirationMin    int    `env:"JWT_ACCESS_EXP_MIN" env-required:"true"`
	RefreshExpirationHours int    `env:"JWT_REFRESH_EXP_HOURS" env-required:"true"`
}

type Redis struct {
	Host string `env:"REDIS_HOST" env-required:"true"`
	Port string `env:"REDIS_PORT" env-required:"true"`
}

type Database struct {
	Host     string `env:"POSTGRES_HOST" env-required:"true"`
	Port     string `env:"POSTGRES_PORT" env-required:"true"`
	User     string `env:"POSTGRES_USER" env-required:"true"`
	DBName   string `env:"POSTGRES_DB" env-required:"true"`
	Password string `env:"POSTGRES_PASSWORD" env-required:"true"`
	SSLMode  string `env:"POSTGRES_SSLMODE" env-required:"true"`
}

func (d Database) DSN() string {
	return fmt.Sprintf(
		`host=%s port=%s user=%s password=%s dbname=%s sslmode=%s`,
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

func Load() (*Config, error) {
	cfg := &Config{}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read environment variables: %w", err)
	}
	return cfg, nil
}
