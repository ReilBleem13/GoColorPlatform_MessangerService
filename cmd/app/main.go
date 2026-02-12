package main

import (
	"context"
	"log"
	"log/slog"
	"path/filepath"

	"github.com/ReilBleem13/MessangerV2/internal/config"
	"github.com/ReilBleem13/MessangerV2/internal/repository/cache"
	"github.com/ReilBleem13/MessangerV2/internal/repository/database"
	"github.com/ReilBleem13/MessangerV2/internal/server"
	"github.com/ReilBleem13/MessangerV2/internal/service"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("no .env file: ", err)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		return
	}

	cache.NewRedisClient(cfg.Redis.Port)
	slog.Info("Redis inited")

	dsn := cfg.Database.DSN()
	if err := database.NewPostgresClient(dsn); err != nil {
		log.Fatal(err)
	}
	slog.Info("Database inited")

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("Failed to set dialect (migrations)", "error", err)
		return
	}

	migrationsPath := filepath.Join("internal", "repository", "database", "migrations")
	if err := goose.Up(database.Client().DB, migrationsPath); err != nil {
		slog.Error("Failed to migrate up", "error", err)
		return
	}
	slog.Info("Migrations completed")

	go service.GetHub().Run()

	server := server.NewServer(
		context.Background(),
		server.WithMigrateDown(func() error {
			return goose.DownTo(database.Client().DB, migrationsPath, 0)
		}),
	)
	server.Run(":8080")
}
