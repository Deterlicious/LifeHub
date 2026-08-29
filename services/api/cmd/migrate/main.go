package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"lifehub/services/api/internal/migrate"
	"lifehub/services/api/internal/riverinfra"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := migrate.Up(ctx, databaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	if err := riverinfra.Migrate(ctx, databaseURL); err != nil {
		slog.Error("River migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied", "river_target_version", riverinfra.TargetVersion)
}
