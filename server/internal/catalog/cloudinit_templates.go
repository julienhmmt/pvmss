package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/store"
	"strings"
	"time"
)

// ErrDuplicateCloudInitTemplate is returned when a template's derived slug
// already exists for the cluster (409).
var ErrDuplicateCloudInitTemplate = errors.New("duplicate cloud-init template")

// ErrCloudInitTemplateNotFound is returned when a template id does not exist or
// is disabled (for the enabled-only single lookup used by vm.Create).
var ErrCloudInitTemplateNotFound = errors.New("cloud-init template not found")

// ErrInvalidCloudInitTemplate is returned when template fields are missing or
// content fails T08's validation (400).
var ErrInvalidCloudInitTemplate = errors.New("invalid cloud-init template")

// CloudInitTemplate is one catalog_cloudinit_templates row. The same struct is
// returned by both the admin (all rows) and catalog (enabled-only) readers;
// each audience's handler calls the reader appropriate to its own audience,
// matching T11's identical split for profiles.
type CloudInitTemplate struct {
	ID        string
	Label     string
	Content   string
	Enabled   bool
	CreatedAt string
	UpdatedAt string
}

// DeriveCloudInitTemplateID converts a label to a lowercase hyphenated slug,
// mirroring T11's DeriveProfileID convention (e.g. "Web server" → "web-server").
// The slug is the template's permanent id.
func DeriveCloudInitTemplateID(label string) string {
	lowered := strings.ToLower(label)
	slug := slugRe.ReplaceAllString(lowered, "-")

	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "template"
	}

	return slug
}

// ListCloudInitTemplates returns every template for the cluster (including
// disabled ones), ordered by id — the admin list endpoint's data source.
func ListCloudInitTemplates(ctx context.Context, st *store.Store, cluster string) ([]CloudInitTemplate, error) {
	rows, err := st.CatalogCloudInitTemplatesAll(ctx, cluster)
	if err != nil {
		return nil, err
	}

	out := make([]CloudInitTemplate, len(rows))
	for i, r := range rows {
		out[i] = CloudInitTemplate{
			ID: r.ID, Label: r.Label, Content: r.Content,
			Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}

	return out, nil
}

// CloudInitTemplates returns only enabled templates for the cluster, ordered by
// id — T06's catalog reader's data source (FR-004). Disabled templates remain
// visible only through ListCloudInitTemplates (the admin list).
func CloudInitTemplates(ctx context.Context, st *store.Store, cluster string) ([]CloudInitTemplate, error) {
	rows, err := st.CatalogCloudInitTemplatesEnabled(ctx, cluster)
	if err != nil {
		return nil, err
	}

	out := make([]CloudInitTemplate, len(rows))
	for i, r := range rows {
		out[i] = CloudInitTemplate{
			ID: r.ID, Label: r.Label, Content: r.Content,
			Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}

	return out, nil
}

// FindCloudInitTemplate returns a single enabled template by id — used by
// vm.Create to resolve a template's content server-side before allocating a
// VMID (FR-006). Returns ErrCloudInitTemplateNotFound when the id is absent or
// disabled. Named FindCloudInitTemplate to mirror catalog.FindProfile (T11's
// own single-lookup convention), since Go forbids a type and function sharing
// the name CloudInitTemplate.
func FindCloudInitTemplate(ctx context.Context, st *store.Store, cluster, id string) (CloudInitTemplate, error) {
	templates, err := CloudInitTemplates(ctx, st, cluster)
	if err != nil {
		return CloudInitTemplate{}, err
	}

	for _, t := range templates {
		if t.ID == id {
			return t, nil
		}
	}

	return CloudInitTemplate{}, fmt.Errorf("%w: %q", ErrCloudInitTemplateNotFound, id)
}

// CreateCloudInitTemplate derives a slug from the label, validates the content
// with T08's cloudinit.Validate (FR-003), rejects slug collisions with
// ErrDuplicateCloudInitTemplate (409), and inserts the new row. The new
// template is enabled by default.
func CreateCloudInitTemplate(ctx context.Context, st *store.Store, cluster, label, content string) (CloudInitTemplate, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return CloudInitTemplate{}, fmt.Errorf("%w: label is required", ErrInvalidCloudInitTemplate)
	}

	if err := cloudinit.Validate(content); err != nil {
		return CloudInitTemplate{}, fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	id := DeriveCloudInitTemplateID(label)

	exists, err := st.CloudInitTemplateExists(ctx, cluster, id)
	if err != nil {
		return CloudInitTemplate{}, err
	}

	if exists {
		return CloudInitTemplate{}, ErrDuplicateCloudInitTemplate
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.InsertCloudInitTemplate(ctx, cluster, id, label, content, stamp, stamp); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return CloudInitTemplate{}, ErrDuplicateCloudInitTemplate
		}

		return CloudInitTemplate{}, err
	}

	return CloudInitTemplate{
		ID: id, Label: label, Content: content, Enabled: true,
		CreatedAt: stamp, UpdatedAt: stamp,
	}, nil
}

// UpdateCloudInitTemplate changes an existing template's label and content,
// re-validating the content with T08's cloudinit.Validate (FR-003). Returns
// ErrCloudInitTemplateNotFound if the id does not exist (404).
func UpdateCloudInitTemplate(ctx context.Context, st *store.Store, cluster, id, label, content string) (CloudInitTemplate, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return CloudInitTemplate{}, fmt.Errorf("%w: label is required", ErrInvalidCloudInitTemplate)
	}

	if err := cloudinit.Validate(content); err != nil {
		return CloudInitTemplate{}, fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	exists, err := st.CloudInitTemplateExists(ctx, cluster, id)
	if err != nil {
		return CloudInitTemplate{}, err
	}

	if !exists {
		return CloudInitTemplate{}, ErrCloudInitTemplateNotFound
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.UpdateCloudInitTemplate(ctx, cluster, id, label, content, stamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CloudInitTemplate{}, ErrCloudInitTemplateNotFound
		}

		return CloudInitTemplate{}, err
	}

	return readCloudInitTemplateBack(ctx, st, cluster, id)
}

// DeleteCloudInitTemplate removes a template row. Returns
// ErrCloudInitTemplateNotFound if the id does not exist. Has no cascade —
// nothing references a template by id after creation (FR-009).
func DeleteCloudInitTemplate(ctx context.Context, st *store.Store, cluster, id string) error {
	exists, err := st.CloudInitTemplateExists(ctx, cluster, id)
	if err != nil {
		return err
	}

	if !exists {
		return ErrCloudInitTemplateNotFound
	}

	if err := st.DeleteCloudInitTemplate(ctx, cluster, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCloudInitTemplateNotFound
		}

		return err
	}

	return nil
}

// SetCloudInitTemplateEnabled toggles the enabled state for one template. A
// toggle is an upsert on the enabled column, never a delete. Returns
// ErrCloudInitTemplateNotFound if the id does not exist.
func SetCloudInitTemplateEnabled(ctx context.Context, st *store.Store, cluster, id string, enabled bool) error {
	exists, err := st.CloudInitTemplateExists(ctx, cluster, id)
	if err != nil {
		return err
	}

	if !exists {
		return ErrCloudInitTemplateNotFound
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.SetCloudInitTemplateEnabled(ctx, cluster, id, enabled, stamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCloudInitTemplateNotFound
		}

		return err
	}

	return nil
}

// readCloudInitTemplateBack returns the current row state after an update.
func readCloudInitTemplateBack(ctx context.Context, st *store.Store, cluster, id string) (CloudInitTemplate, error) {
	rows, err := st.CatalogCloudInitTemplatesAll(ctx, cluster)
	if err != nil {
		return CloudInitTemplate{}, err
	}

	for _, r := range rows {
		if r.ID == id {
			return CloudInitTemplate{
				ID: r.ID, Label: r.Label, Content: r.Content,
				Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			}, nil
		}
	}

	return CloudInitTemplate{}, ErrCloudInitTemplateNotFound
}
