package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"lifehub/services/api/db/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Up(ctx context.Context, databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer database.Close()

	if _, err := database.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS lifehub"); err != nil {
		return fmt.Errorf("create migration schema: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	goose.SetTableName("lifehub.goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, database, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
