package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/store"
	"regexp"
	"strings"
)

// ErrDuplicateProfile is returned when a profile's derived slug already exists.
var ErrDuplicateProfile = errors.New("duplicate profile")

// ErrProfileNotFound is returned when a profile id does not exist.
var ErrProfileNotFound = errors.New("profile not found")

// ErrInvalidProfile is returned when profile fields are out of range or missing.
var ErrInvalidProfile = errors.New("invalid profile")

// slugRe strips non-alphanumeric characters for slug derivation.
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// AdminProfile is one catalog_profiles row with its enabled state, as returned
// by the admin profiles list endpoint (includes disabled profiles, unlike
// catalog.Profiles which filters by enabled = 1).
type AdminProfile struct {
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
	Enabled  bool
}

// DeriveProfileID converts a label to a lowercase hyphenated slug matching
// T06's own small/medium/large style (e.g. "X-Large (8 vCPU, 16 GB, 160 GB)"
// → "x-large-8-vcpu-16-gb-160-gb"). The slug is the profile's permanent id.
func DeriveProfileID(label string) string {
	lowered := strings.ToLower(label)
	slug := slugRe.ReplaceAllString(lowered, "-")

	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "profile"
	}

	return slug
}

// validateProfileFields checks that cpuCores, memoryMB, diskGB are positive and
// bus is non-empty — the minimum the admin UI requires (FR-009).
func validateProfileFields(cpuCores, memoryMB, diskGB int, bus string) error {
	if cpuCores < 1 {
		return fmt.Errorf("%w: cpuCores must be >= 1", ErrInvalidProfile)
	}

	if memoryMB < 128 {
		return fmt.Errorf("%w: memoryMB must be >= 128", ErrInvalidProfile)
	}

	if diskGB < 1 {
		return fmt.Errorf("%w: diskGB must be >= 1", ErrInvalidProfile)
	}

	if bus == "" {
		return fmt.Errorf("%w: bus is required", ErrInvalidProfile)
	}

	return nil
}

// ListAdminProfiles returns every profile for the cluster (including disabled
// ones), ordered by id — the admin list endpoint's data source.
func ListAdminProfiles(ctx context.Context, st *store.Store, cluster string) ([]AdminProfile, error) {
	rows, err := st.CatalogProfilesEnabled(ctx, cluster)
	if err != nil {
		return nil, err
	}

	out := make([]AdminProfile, len(rows))
	for i, r := range rows {
		out[i] = AdminProfile{
			ID: r.ID, Label: r.Label, CPUCores: r.CPUCores,
			MemoryMB: r.MemoryMB, DiskGB: r.DiskGB, Bus: r.Bus, Enabled: r.Enabled,
		}
	}

	return out, nil
}

// ProfileSpec is the editable field set of a VM profile, shared by
// CreateProfile and UpdateProfile. Grouping it collapses the five positional
// field parameters those functions used to take (SonarQube go:S107).
type ProfileSpec struct {
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// CreateProfile derives a slug from the label, validates the fields, rejects
// slug collisions with ErrDuplicateProfile (409), and inserts the new row
// (FR-009). The new profile is enabled by default.
func CreateProfile(ctx context.Context, st *store.Store, cluster string, spec ProfileSpec) (AdminProfile, error) {
	label := strings.TrimSpace(spec.Label)
	if label == "" {
		return AdminProfile{}, fmt.Errorf("%w: label is required", ErrInvalidProfile)
	}

	if err := validateProfileFields(spec.CPUCores, spec.MemoryMB, spec.DiskGB, spec.Bus); err != nil {
		return AdminProfile{}, err
	}

	id := DeriveProfileID(label)

	exists, err := st.ProfileExists(ctx, cluster, id)
	if err != nil {
		return AdminProfile{}, err
	}

	if exists {
		return AdminProfile{}, ErrDuplicateProfile
	}

	if err := st.InsertProfile(ctx, cluster, id, store.ProfileValues{Label: label, CPUCores: spec.CPUCores, MemoryMB: spec.MemoryMB, DiskGB: spec.DiskGB, Bus: spec.Bus}); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return AdminProfile{}, ErrDuplicateProfile
		}

		return AdminProfile{}, err
	}

	return AdminProfile{
		ID: id, Label: label, CPUCores: spec.CPUCores,
		MemoryMB: spec.MemoryMB, DiskGB: spec.DiskGB, Bus: spec.Bus, Enabled: true,
	}, nil
}

// UpdateProfile changes an existing profile's values. Returns
// ErrProfileNotFound if the id does not exist (404).
func UpdateProfile(ctx context.Context, st *store.Store, cluster, id string, spec ProfileSpec) (AdminProfile, error) {
	label := strings.TrimSpace(spec.Label)
	if label == "" {
		return AdminProfile{}, fmt.Errorf("%w: label is required", ErrInvalidProfile)
	}

	if err := validateProfileFields(spec.CPUCores, spec.MemoryMB, spec.DiskGB, spec.Bus); err != nil {
		return AdminProfile{}, err
	}

	exists, err := st.ProfileExists(ctx, cluster, id)
	if err != nil {
		return AdminProfile{}, err
	}

	if !exists {
		return AdminProfile{}, ErrProfileNotFound
	}

	if err := st.UpdateProfile(ctx, cluster, id, store.ProfileValues{Label: label, CPUCores: spec.CPUCores, MemoryMB: spec.MemoryMB, DiskGB: spec.DiskGB, Bus: spec.Bus}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminProfile{}, ErrProfileNotFound
		}

		return AdminProfile{}, err
	}

	// Read back the enabled state (update doesn't change it).
	rows, err := st.CatalogProfilesEnabled(ctx, cluster)
	if err != nil {
		return AdminProfile{}, err
	}

	for _, r := range rows {
		if r.ID == id {
			return AdminProfile{
				ID: r.ID, Label: r.Label, CPUCores: r.CPUCores,
				MemoryMB: r.MemoryMB, DiskGB: r.DiskGB, Bus: r.Bus, Enabled: r.Enabled,
			}, nil
		}
	}

	return AdminProfile{}, ErrProfileNotFound
}

// DeleteProfile removes a profile row. Returns ErrProfileNotFound if the id
// does not exist. Has no cascade — T06 never stores a profile reference on
// the VM itself (FR-011).
func DeleteProfile(ctx context.Context, st *store.Store, cluster, id string) error {
	exists, err := st.ProfileExists(ctx, cluster, id)
	if err != nil {
		return err
	}

	if !exists {
		return ErrProfileNotFound
	}

	if err := st.DeleteProfile(ctx, cluster, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProfileNotFound
		}

		return err
	}

	return nil
}

// SetProfileEnabled toggles the enabled state for one profile. Returns
// ErrProfileNotFound if the id does not exist.
func SetProfileEnabled(ctx context.Context, st *store.Store, cluster, id string, enabled bool) error {
	exists, err := st.ProfileExists(ctx, cluster, id)
	if err != nil {
		return err
	}

	if !exists {
		return ErrProfileNotFound
	}

	if err := st.SetProfileEnabled(ctx, cluster, id, enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProfileNotFound
		}

		return err
	}

	return nil
}
