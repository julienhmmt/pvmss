package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"time"
	"unicode"
)

// ErrProtectedTag is returned when attempting to delete the mandatory pvmss
// tag (FR-014). The handler translates this to 403.
var ErrProtectedTag = errors.New("protected tag")

// ErrDuplicateTag is returned when a tag name already exists (409).
var ErrDuplicateTag = errors.New("duplicate tag")

// ErrTagNotFound is returned when a tag name does not exist (404).
var ErrTagNotFound = errors.New("tag not found")

// ErrInvalidTagName is returned when a tag name fails validation (400).
var ErrInvalidTagName = errors.New("invalid tag name")

// ErrInvalidTagColor is returned when a tag color is empty or malformed (400).
var ErrInvalidTagColor = errors.New("invalid tag color")

// ProtectedTagName is the one tag that cannot be deleted.
const ProtectedTagName = "pvmss"

// TagWithCount is one catalog_tags row with a live VM count computed from the
// inventory projection (FR-015: never stored).
type TagWithCount struct {
	Name      string
	Color     string
	VMCount   int
	Protected bool
}

// validateTagName checks the 1-50 alphanumeric rule (FR-013). The pvmss tag
// itself passes validation (it is alphanumeric).
func validateTagName(name string) error {
	if len(name) < 1 || len(name) > 50 {
		return fmt.Errorf("%w: name must be 1-50 characters", ErrInvalidTagName)
	}

	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("%w: name must be alphanumeric", ErrInvalidTagName)
		}
	}

	return nil
}

// ListTags returns every tag for the cluster with a live VM count computed
// from the inventory projection (FR-015). The pvmss tag is marked protected.
//
// FR-014 makes the pvmss tag mandatory and undeletable for every cluster. The
// V9 migration seeds it only for the "default" cluster (the only cluster at
// T06); non-default clusters get it lazily here via ensurePvmssTag, so the
// admin surface never lists a cluster without it.
func ListTags(ctx context.Context, st *store.Store, projection *inventory.Projection, cluster string) ([]TagWithCount, error) {
	if err := ensurePvmssTag(ctx, st, cluster); err != nil {
		return nil, err
	}

	rows, err := st.CatalogTags(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// Build a tag-to-VM-count map from the live inventory projection. The
	// projection may be nil when tags are not used (e.g. tests that only
	// exercise nodes/storages/bridges/isos) — guard before dereferencing.
	tagCounts := make(map[string]int)

	if projection != nil {
		if idx := projection.Load(); idx != nil {
			for _, vm := range idx.ByVMID {
				for _, tag := range vm.Tags {
					tagCounts[tag]++
				}
			}
		}
	}

	out := make([]TagWithCount, len(rows))
	for i, r := range rows {
		out[i] = TagWithCount{
			Name:      r.Name,
			Color:     r.Color,
			VMCount:   tagCounts[r.Name],
			Protected: r.Name == ProtectedTagName,
		}
	}

	return out, nil
}

// ensurePvmssTag inserts the mandatory pvmss tag for the cluster if it does
// not already exist (FR-014). Idempotent — safe to call on every ListTags.
func ensurePvmssTag(ctx context.Context, st *store.Store, cluster string) error {
	exists, err := st.TagExists(ctx, cluster, ProtectedTagName)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if err := st.InsertTag(ctx, cluster, ProtectedTagName, "#4f46e5", createdAt); err != nil {
		// A concurrent insert between our TagExists check and InsertTag is
		// fine — the tag exists, which is all we need.
		if errors.Is(err, store.ErrDuplicate) {
			return nil
		}

		return err
	}

	return nil
}

// CreateTag validates the name (1-50 alphanumeric, FR-013), rejects duplicates
// with ErrDuplicateTag (409), and inserts the new row.
func CreateTag(ctx context.Context, st *store.Store, cluster, name, color string) (TagWithCount, error) {
	name = strings.TrimSpace(name)
	if err := validateTagName(name); err != nil {
		return TagWithCount{}, err
	}

	if color == "" {
		color = "#4f46e5"
	}

	exists, err := st.TagExists(ctx, cluster, name)
	if err != nil {
		return TagWithCount{}, err
	}

	if exists {
		return TagWithCount{}, ErrDuplicateTag
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if err := st.InsertTag(ctx, cluster, name, color, createdAt); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return TagWithCount{}, ErrDuplicateTag
		}

		return TagWithCount{}, err
	}

	return TagWithCount{
		Name:      name,
		Color:     color,
		VMCount:   0,
		Protected: name == ProtectedTagName,
	}, nil
}

// SetTagColor updates the color of an existing tag. The pvmss tag's color can
// be changed (spec Acceptance Scenario 3.2: protection is delete-only).
// Returns ErrTagNotFound if the tag does not exist.
func SetTagColor(ctx context.Context, st *store.Store, cluster, name, color string) (TagWithCount, error) {
	if color == "" {
		return TagWithCount{}, fmt.Errorf("%w: color is required", ErrInvalidTagColor)
	}

	exists, err := st.TagExists(ctx, cluster, name)
	if err != nil {
		return TagWithCount{}, err
	}

	if !exists {
		return TagWithCount{}, ErrTagNotFound
	}

	if err := st.UpdateTagColor(ctx, cluster, name, color); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TagWithCount{}, ErrTagNotFound
		}

		return TagWithCount{}, err
	}

	// Read back to return the full row.
	tags, err := st.CatalogTags(ctx, cluster)
	if err != nil {
		return TagWithCount{}, err
	}

	for _, t := range tags {
		if t.Name == name {
			return TagWithCount{
				Name:      t.Name,
				Color:     t.Color,
				VMCount:   0, // Recomputed by handler via projection if needed
				Protected: t.Name == ProtectedTagName,
			}, nil
		}
	}

	return TagWithCount{}, ErrTagNotFound
}

// DeleteTag removes a tag row. Returns ErrProtectedTag (403) if the tag is
// pvmss, or ErrTagNotFound (404) if it does not exist.
func DeleteTag(ctx context.Context, st *store.Store, cluster, name string) error {
	if name == ProtectedTagName {
		return ErrProtectedTag
	}

	exists, err := st.TagExists(ctx, cluster, name)
	if err != nil {
		return err
	}

	if !exists {
		return ErrTagNotFound
	}

	if err := st.DeleteTag(ctx, cluster, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTagNotFound
		}

		return err
	}

	return nil
}
