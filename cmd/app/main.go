package main

import (
	"log"
	"log/slog"
	"path/filepath"

	"github.com/ReilBleem13/MessangerV2/internal/repository/cache"
	"github.com/ReilBleem13/MessangerV2/internal/repository/database"
	"github.com/ReilBleem13/MessangerV2/internal/server"
	"github.com/ReilBleem13/MessangerV2/internal/service"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func main() {
	cache.NewRedisClient()
	slog.Info("Redis inited")

	dsn := "host=localhost port=5432 user=test password=test dbname=test sslmode=disable"
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
		server.WithMigrateDown(func() error {
			return goose.DownTo(database.Client().DB, migrationsPath, 0)
		}),
	)
	server.Run(":8080")
}
