package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// CloudInitTemplate represents a cloud-init template stored in the database.
type CloudInitTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Storage     string `json:"storage"`
	Filename    string `json:"filename"`
	YAMLContent string `json:"yaml_content"`
	Enabled     bool   `json:"enabled"`
}

// ListCloudInitTemplates returns all cloud-init templates ordered by name.
func (s *sqliteDB) ListCloudInitTemplates() ([]CloudInitTemplate, error) {
	rows, err := s.db.Query(`
		SELECT id, name, COALESCE(description,''), COALESCE(storage,''),
		       COALESCE(filename,''), yaml_content, enabled
		FROM cloudinit_templates ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query cloudinit_templates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanCloudInitRows(rows)
}

// GetCloudInitTemplate returns a single template by ID, or nil when not found.
func (s *sqliteDB) GetCloudInitTemplate(id string) (*CloudInitTemplate, error) {
	row := s.db.QueryRow(`
		SELECT id, name, COALESCE(description,''), COALESCE(storage,''),
		       COALESCE(filename,''), yaml_content, enabled
		FROM cloudinit_templates WHERE id = ?
	`, id)
	t, err := scanCloudInitRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

// CreateCloudInitTemplate inserts a new template and appends an audit entry.
func (s *sqliteDB) CreateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error {
	newJSON, _ := json.Marshal(t)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, execErr := tx.Exec(`
		INSERT INTO cloudinit_templates
		    (id, name, description, storage, filename, yaml_content, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Name, t.Description, t.Storage, t.Filename, t.YAMLContent, boolToInt(t.Enabled))
	if execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert cloudinit_template: %w", execErr)
	}
	if err := appendAudit(tx, "cloudinit_templates", t.ID, "create", "", string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpdateCloudInitTemplate replaces all fields of an existing template and appends an audit entry.
// Returns ErrNotFound when no template with the given ID exists.
func (s *sqliteDB) UpdateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT id, name, COALESCE(description,''), COALESCE(storage,''),
		       COALESCE(filename,''), yaml_content, enabled
		FROM cloudinit_templates WHERE id = ?
	`, t.ID)
	old, err := scanCloudInitRow(row)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("update cloudinit_template %q: %w", t.ID, ErrNotFound)
		}
		return fmt.Errorf("read cloudinit_template for audit: %w", err)
	}
	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(t)
	_, execErr := tx.Exec(`
		UPDATE cloudinit_templates
		SET name = ?, description = ?, storage = ?, filename = ?,
		    yaml_content = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, t.Name, t.Description, t.Storage, t.Filename, t.YAMLContent, boolToInt(t.Enabled), t.ID)
	if execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update cloudinit_template: %w", execErr)
	}
	if err := appendAudit(tx, "cloudinit_templates", t.ID, "update", string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteCloudInitTemplate removes a template by ID and appends an audit entry.
// Returns ErrNotFound when no template with the given ID exists.
func (s *sqliteDB) DeleteCloudInitTemplate(id string, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT id, name, COALESCE(description,''), COALESCE(storage,''),
		       COALESCE(filename,''), yaml_content, enabled
		FROM cloudinit_templates WHERE id = ?
	`, id)
	old, err := scanCloudInitRow(row)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delete cloudinit_template %q: %w", id, ErrNotFound)
		}
		return fmt.Errorf("read cloudinit_template for audit: %w", err)
	}
	oldJSON, _ := json.Marshal(old)
	if _, execErr := tx.Exec(`DELETE FROM cloudinit_templates WHERE id = ?`, id); execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete cloudinit_template: %w", execErr)
	}
	if err := appendAudit(tx, "cloudinit_templates", id, "delete", string(oldJSON), "", changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// scanCloudInitRows scans all rows from a ListCloudInitTemplates query.
func scanCloudInitRows(rows *sql.Rows) ([]CloudInitTemplate, error) {
	templates := []CloudInitTemplate{}
	for rows.Next() {
		var t CloudInitTemplate
		var enabled int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Storage,
			&t.Filename, &t.YAMLContent, &enabled); err != nil {
			return nil, fmt.Errorf("scan cloudinit_template: %w", err)
		}
		t.Enabled = enabled != 0
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// scanCloudInitRow scans a single *sql.Row from GetCloudInitTemplate.
func scanCloudInitRow(row *sql.Row) (*CloudInitTemplate, error) {
	var t CloudInitTemplate
	var enabled int
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Storage,
		&t.Filename, &t.YAMLContent, &enabled); err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	return &t, nil
}
