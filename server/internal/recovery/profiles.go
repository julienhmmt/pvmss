package recovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// legacyProfileConfig is the JSON shape stored in vm_profiles.config.
// Fields match backend/state/settings.go's VMProfileConfig struct.
type legacyProfileConfig struct {
	Sockets int    `json:"sockets"`
	Cores   int    `json:"cores"`
	RAMGB   int    `json:"ram_gb"`
	DiskGB  int    `json:"disk_gb"`
	DiskBus string `json:"disk_bus"`
}

// MapProfiles reads the legacy vm_profiles table, parses each config JSON
// blob, and returns rows for catalog_profiles. A profile whose JSON fails
// to parse or is missing a required field is skipped with a named reason
// (per-row error isolation, plan.md research decisions).
func MapProfiles(ctx context.Context, legacyDB *sql.DB) ([]ProfileRow, []SkipReason, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT id, name, config, enabled FROM vm_profiles ORDER BY id`)
	if err != nil {
		return nil, nil, fmt.Errorf("query vm_profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		out   []ProfileRow
		skips []SkipReason
	)

	for rows.Next() {
		var (
			id, name, configJSON string
			enabled              bool
		)
		if err := rows.Scan(&id, &name, &configJSON, &enabled); err != nil {
			return nil, nil, fmt.Errorf("scan vm_profiles: %w", err)
		}

		var cfg legacyProfileConfig
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			skips = append(skips, SkipReason{
				Row:    id,
				Reason: fmt.Sprintf("config JSON parse error: %v", err),
			})

			continue
		}

		if cfg.DiskBus == "" {
			skips = append(skips, SkipReason{
				Row:    id,
				Reason: "config JSON missing disk_bus field",
			})

			continue
		}

		if cfg.Sockets <= 0 || cfg.Cores <= 0 || cfg.RAMGB <= 0 || cfg.DiskGB <= 0 {
			skips = append(skips, SkipReason{
				Row:    id,
				Reason: fmt.Sprintf("config JSON has non-positive resource values: sockets=%d cores=%d ram_gb=%d disk_gb=%d", cfg.Sockets, cfg.Cores, cfg.RAMGB, cfg.DiskGB),
			})

			continue
		}

		out = append(out, ProfileRow{
			ID:       id,
			Label:    name,
			CPUCores: cfg.Cores,
			MemoryMB: cfg.RAMGB * 1024, // GB → MB
			DiskGB:   cfg.DiskGB,
			Bus:      cfg.DiskBus,
			Enabled:  enabled,
		})
	}

	return out, skips, rows.Err()
}

func upsertProfile(ctx context.Context, v04DB *sql.DB, cluster string, r ProfileRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_profiles (cluster, id, label, cpu_cores, memory_mb, disk_gb, bus, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster, id) DO UPDATE SET
			label = excluded.label,
			cpu_cores = excluded.cpu_cores,
			memory_mb = excluded.memory_mb,
			disk_gb = excluded.disk_gb,
			bus = excluded.bus,
			enabled = excluded.enabled`,
		cluster, r.ID, r.Label, r.CPUCores, r.MemoryMB, r.DiskGB, r.Bus, r.Enabled)
	if err != nil {
		return fmt.Errorf("upsert catalog_profiles: %w", err)
	}

	return nil
}
