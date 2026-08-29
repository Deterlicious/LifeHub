package riverinfra

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const (
	Schema        = "river"
	TargetVersion = 7
)

var schemaNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func Migrate(ctx context.Context, databaseURL string) error {
	return MigrateSchema(ctx, databaseURL, Schema, TargetVersion)
}

func Validate(ctx context.Context, databaseURL string) error {
	return ValidateSchema(ctx, databaseURL, Schema, TargetVersion)
}

func ValidateSchema(ctx context.Context, databaseURL, schema string, targetVersion int) error {
	if !schemaNamePattern.MatchString(schema) {
		return fmt.Errorf("invalid River schema")
	}
	if targetVersion != TargetVersion {
		return fmt.Errorf("River target must be %d", TargetVersion)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open River validation pool: %w", err)
	}
	defer pool.Close()
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: schema})
	if err != nil {
		return fmt.Errorf("create River validator: %w", err)
	}
	if err := rejectNewerVersion(ctx, pool, schema, targetVersion); err != nil {
		return err
	}
	existing, err := migrator.ExistingVersions(ctx)
	if err != nil {
		return fmt.Errorf("read River migration versions: %w", err)
	}
	for _, migration := range existing {
		if migration.Version > targetVersion {
			return fmt.Errorf("River schema is newer than pinned target %d", targetVersion)
		}
	}
	validation, err := migrator.Validate(ctx, &rivermigrate.ValidateOpts{TargetVersion: TargetVersion})
	if err != nil {
		return fmt.Errorf("validate River migration target: %w", err)
	}
	if !validation.OK {
		return fmt.Errorf("River migration validation failed: %v", validation.Messages)
	}
	return nil
}

// MigrateSchema exists so integration tests can prove the pinned River target
// in an isolated schema without mutating another test or deployment's queue.
func MigrateSchema(ctx context.Context, databaseURL, schema string, targetVersion int) error {
	if !schemaNamePattern.MatchString(schema) {
		return fmt.Errorf("invalid River schema")
	}
	if targetVersion != TargetVersion {
		return fmt.Errorf("River target must be %d", TargetVersion)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open River migration pool: %w", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return fmt.Errorf("create River schema: %w", err)
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: schema})
	if err != nil {
		return fmt.Errorf("create River migrator: %w", err)
	}
	if err := rejectNewerVersion(ctx, pool, schema, targetVersion); err != nil {
		return err
	}
	existing, err := migrator.ExistingVersions(ctx)
	if err != nil {
		return fmt.Errorf("read River migration versions: %w", err)
	}
	for _, migration := range existing {
		if migration.Version > targetVersion {
			return fmt.Errorf("River schema is newer than pinned target %d", targetVersion)
		}
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{TargetVersion: targetVersion}); err != nil {
		return fmt.Errorf("migrate River to version %d: %w", targetVersion, err)
	}
	validation, err := migrator.Validate(ctx, &rivermigrate.ValidateOpts{TargetVersion: targetVersion})
	if err != nil {
		return fmt.Errorf("validate River migration target: %w", err)
	}
	if !validation.OK {
		return fmt.Errorf("River migration validation failed: %v", validation.Messages)
	}
	return nil
}

func rejectNewerVersion(ctx context.Context, pool *pgxpool.Pool, schema string, targetVersion int) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".river_migration").Scan(&exists); err != nil {
		return fmt.Errorf("inspect River migration table: %w", err)
	}
	if !exists {
		return nil
	}
	var maximum *int64
	query := "SELECT max(version) FROM " + pgx.Identifier{schema, "river_migration"}.Sanitize()
	if err := pool.QueryRow(ctx, query).Scan(&maximum); err != nil {
		return fmt.Errorf("read River maximum migration version: %w", err)
	}
	if maximum != nil && *maximum > int64(targetVersion) {
		return fmt.Errorf("River schema is newer than pinned target %d", targetVersion)
	}
	return nil
}
