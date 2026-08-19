//go:build integration

package store

import (
	"os"
	"testing"
)

func TestIntegrationDatabaseIsConfigured(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Fatal("TEST_DATABASE_URL is required when running with -tags=integration")
	}
}
