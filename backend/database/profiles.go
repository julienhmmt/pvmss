package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// VMProfileConfigBlob represents the JSON blob stored in VMProfile.Config.
// This is the serialized form of state.VMProfileConfig.
type VMProfileConfigBlob struct {
	Sockets   int    `json:"sockets"`
	Cores     int    `json:"cores"`
	RAMGB     int    `json:"ram_gb"`
	DiskGB    int    `json:"disk_gb"`
	DiskBus   string `json:"disk_bus"`
	Node      string `json:"node,omitempty"`
	Storage   string `json:"storage,omitempty"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	EnableEFI bool   `json:"enable_efi,omitempty"`
}

// VMProfile represents a VM configuration profile stored in the database.
// Config holds a JSON blob (sockets, cores, memory, disk, etc.).
type VMProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Config      string `json:"config"`
	Enabled     bool   `json:"enabled"`
}

// ListVMProfiles returns all VM profiles ordered by name.
func (s *sqliteDB) ListVMProfiles() ([]VMProfile, error) {
	rows, err := s.db.Query(`
		SELECT id, name, COALESCE(description,''), config, enabled
		FROM vm_profiles ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query vm_profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanVMProfileRows(rows)
}

// GetVMProfile returns a single profile by ID, or nil when not found.
func (s *sqliteDB) GetVMProfile(id string) (*VMProfile, error) {
	row := s.db.QueryRow(`
		SELECT id, name, COALESCE(description,''), config, enabled
		FROM vm_profiles WHERE id = ?
	`, id)
	p, err := scanVMProfileRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// CreateVMProfile inserts a new VM profile and appends an audit entry.
func (s *sqliteDB) CreateVMProfile(p *VMProfile, changedBy string) error {
	newJSON, _ := json.Marshal(p)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, execErr := tx.Exec(`
		INSERT INTO vm_profiles (id, name, description, config, enabled)
		VALUES (?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Description, p.Config, boolToInt(p.Enabled))
	if execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert vm_profile: %w", execErr)
	}
	if err := appendAudit(tx, "vm_profiles", p.ID, "create", "", string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpdateVMProfile replaces all fields of an existing profile and appends an audit entry.
// Returns ErrNotFound when no profile with the given ID exists.
func (s *sqliteDB) UpdateVMProfile(p *VMProfile, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT id, name, COALESCE(description,''), config, enabled
		FROM vm_profiles WHERE id = ?
	`, p.ID)
	old, err := scanVMProfileRow(row)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("update vm_profile %q: %w", p.ID, ErrNotFound)
		}
		return fmt.Errorf("read vm_profile for audit: %w", err)
	}
	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(p)
	_, execErr := tx.Exec(`
		UPDATE vm_profiles
		SET name = ?, description = ?, config = ?, enabled = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, p.Name, p.Description, p.Config, boolToInt(p.Enabled), p.ID)
	if execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update vm_profile: %w", execErr)
	}
	if err := appendAudit(tx, "vm_profiles", p.ID, "update", string(oldJSON), string(newJSON), changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteVMProfile removes a profile by ID and appends an audit entry.
// Returns ErrNotFound when no profile with the given ID exists.
func (s *sqliteDB) DeleteVMProfile(id string, changedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	row := tx.QueryRow(`
		SELECT id, name, COALESCE(description,''), config, enabled
		FROM vm_profiles WHERE id = ?
	`, id)
	old, err := scanVMProfileRow(row)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delete vm_profile %q: %w", id, ErrNotFound)
		}
		return fmt.Errorf("read vm_profile for audit: %w", err)
	}
	oldJSON, _ := json.Marshal(old)
	if _, execErr := tx.Exec(`DELETE FROM vm_profiles WHERE id = ?`, id); execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete vm_profile: %w", execErr)
	}
	if err := appendAudit(tx, "vm_profiles", id, "delete", string(oldJSON), "", changedBy); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// scanVMProfileRows scans all rows from a ListVMProfiles query.
func scanVMProfileRows(rows *sql.Rows) ([]VMProfile, error) {
	profiles := []VMProfile{}
	for rows.Next() {
		var p VMProfile
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Config, &enabled); err != nil {
			return nil, fmt.Errorf("scan vm_profile: %w", err)
		}
		p.Enabled = enabled != 0
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// scanVMProfileRow scans a single *sql.Row from GetVMProfile.
func scanVMProfileRow(row *sql.Row) (*VMProfile, error) {
	var p VMProfile
	var enabled int
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Config, &enabled); err != nil {
		return nil, err
	}
	p.Enabled = enabled != 0
	return &p, nil
}
