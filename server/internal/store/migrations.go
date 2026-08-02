// Package store provides SQLite-backed persistence and schema migrations.
package store

// schemaV1 creates the schema_migrations tracking table.
const schemaV1 = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
)`

// Migration is a single schema version and its forward-only DDL.
type Migration struct {
	Version int
	DDL     string
}

// Migrations is the ordered list of schema migrations.
// Versions must be consecutive integers starting at 1.
var Migrations = []Migration{
	{Version: 1, DDL: schemaV1},
}
