package migrations

import "embed"

// FS contains the immutable application migrations used by the migration
// command and integration tests.
//
//go:embed *.sql
var FS embed.FS
