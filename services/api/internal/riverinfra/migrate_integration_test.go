package riverinfra

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRiverValidationRejectsMissingAndNewerButAcceptsExactTarget(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "river_validate_" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+identifier+" CASCADE")
	})

	if err := ValidateSchema(ctx, databaseURL, schema, TargetVersion); err == nil {
		t.Fatal("missing River migrations passed validation")
	}
	if err := MigrateSchema(ctx, databaseURL, schema, TargetVersion); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchema(ctx, databaseURL, schema, TargetVersion); err != nil {
		t.Fatalf("exact target failed validation: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+identifier+".river_migration (line, version) VALUES ('main', 8)"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchema(ctx, databaseURL, schema, TargetVersion); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("newer target error = %v", err)
	}
}
