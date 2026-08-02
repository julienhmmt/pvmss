package store

// schemaV1 creates the schema_migrations tracking table.
const schemaV1 = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
)`

// Migrations maps each schema version to its forward-only DDL.
// Versions must be consecutive integers starting at 1.
var Migrations = map[int]string{
	1: schemaV1,
}
