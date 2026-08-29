package migrations

import "embed"

// LatestVersion is the application schema version required by the API and worker.
const LatestVersion = 7

// FS contains the immutable application migrations used by the migration
// command and integration tests.
//
//go:embed *.sql
var FS embed.FS
