package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// GetEnabledNodes returns the names of all enabled nodes.
func (s *sqliteDB) GetEnabledNodes() ([]string, error) {
	return queryStringList(s.db, `SELECT name FROM enabled_nodes WHERE enabled = 1 ORDER BY name`)
}

// SetEnabledNodes atomically replaces the enabled_nodes list.
func (s *sqliteDB) SetEnabledNodes(nodes []string, changedBy string) error {
	return replaceList(s, "enabled_nodes", nodes, changedBy,
		`DELETE FROM enabled_nodes`,
		func(tx *sql.Tx, item string) error {
			_, err := tx.Exec(`INSERT INTO enabled_nodes (name, enabled) VALUES (?, 1)`, item)
			return err
		},
		`SELECT name FROM enabled_nodes WHERE enabled = 1 ORDER BY name`,
	)
}

// GetEnabledStorages returns the IDs of all enabled storages.
func (s *sqliteDB) GetEnabledStorages() ([]string, error) {
	return queryStringList(s.db, `SELECT storage_id FROM enabled_storages WHERE enabled = 1 ORDER BY storage_id`)
}

// SetEnabledStorages atomically replaces the enabled_storages list.
func (s *sqliteDB) SetEnabledStorages(storages []string, changedBy string) error {
	return replaceList(s, "enabled_storages", storages, changedBy,
		`DELETE FROM enabled_storages`,
		func(tx *sql.Tx, item string) error {
			_, err := tx.Exec(`INSERT INTO enabled_storages (storage_id, enabled) VALUES (?, 1)`, item)
			return err
		},
		`SELECT storage_id FROM enabled_storages WHERE enabled = 1 ORDER BY storage_id`,
	)
}

// GetEnabledISOs returns the names of all enabled ISOs.
func (s *sqliteDB) GetEnabledISOs() ([]string, error) {
	return queryStringList(s.db, `SELECT name FROM enabled_isos WHERE enabled = 1 ORDER BY name`)
}

// SetEnabledISOs atomically replaces the enabled_isos list.
func (s *sqliteDB) SetEnabledISOs(isos []string, changedBy string) error {
	return replaceList(s, "enabled_isos", isos, changedBy,
		`DELETE FROM enabled_isos`,
		func(tx *sql.Tx, item string) error {
			_, err := tx.Exec(`INSERT INTO enabled_isos (name, enabled) VALUES (?, 1)`, item)
			return err
		},
		`SELECT name FROM enabled_isos WHERE enabled = 1 ORDER BY name`,
	)
}

// GetEnabledVMBRs returns the names of all enabled network bridges.
func (s *sqliteDB) GetEnabledVMBRs() ([]string, error) {
	return queryStringList(s.db, `SELECT name FROM enabled_vmbrs WHERE enabled = 1 ORDER BY name`)
}

// SetEnabledVMBRs atomically replaces the enabled_vmbrs list.
func (s *sqliteDB) SetEnabledVMBRs(vmbrs []string, changedBy string) error {
	return replaceList(s, "enabled_vmbrs", vmbrs, changedBy,
		`DELETE FROM enabled_vmbrs`,
		func(tx *sql.Tx, item string) error {
			_, err := tx.Exec(`INSERT INTO enabled_vmbrs (name, enabled) VALUES (?, 1)`, item)
			return err
		},
		`SELECT name FROM enabled_vmbrs WHERE enabled = 1 ORDER BY name`,
	)
}

// GetTags returns all tag names.
func (s *sqliteDB) GetTags() ([]string, error) {
	return queryStringList(s.db, `SELECT name FROM tags ORDER BY name`)
}

// SetTags atomically replaces the tags list.
func (s *sqliteDB) SetTags(tags []string, changedBy string) error {
	return replaceList(s, "tags", tags, changedBy,
		`DELETE FROM tags`,
		func(tx *sql.Tx, item string) error {
			_, err := tx.Exec(`INSERT INTO tags (name) VALUES (?)`, item)
			return err
		},
		`SELECT name FROM tags ORDER BY name`,
	)
}

// queryer is satisfied by both *sql.DB and *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// queryStringList executes a single-column SELECT and returns results as []string.
func queryStringList(q queryer, query string) ([]string, error) {
	rows, err := q.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanStringSlice(rows)
}

// scanStringSlice scans a single-column string result set into a slice.
func scanStringSlice(rows *sql.Rows) ([]string, error) {
	result := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan list item: %w", err)
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

// replaceList atomically replaces a list table's contents and records an audit entry.
// The current state is read inside the transaction for an atomic audit trail.
func replaceList(
	s *sqliteDB,
	tableName string,
	newItems []string,
	changedBy string,
	deleteSQL string,
	insertFn func(*sql.Tx, string) error,
	selectSQL string,
) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	oldItems, err := queryStringList(tx, selectSQL)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("read current %s: %w", tableName, err)
	}
	oldJSON, _ := json.Marshal(oldItems)
	newJSON, _ := json.Marshal(newItems)
	if _, err := tx.Exec(deleteSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete %s: %w", tableName, err)
	}
	for _, item := range newItems {
		if err := insertFn(tx, item); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert into %s: %w", tableName, err)
		}
	}
	if err := appendAudit(tx, tableName, "list", "update", string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
