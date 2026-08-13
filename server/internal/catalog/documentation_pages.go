//nolint:wsl_v5 // validation and store mapping keep each field check adjacent
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/store"
	"strings"
	"time"
)

// ErrDuplicateDocumentationPage is returned when a page's derived slug already
// exists for the requested language (409).
var ErrDuplicateDocumentationPage = errors.New("duplicate documentation page")

// ErrDocumentationPageNotFound is returned when a page id does not exist (404).
var ErrDocumentationPageNotFound = errors.New("documentation page not found")

// ErrInvalidDocumentationPage is returned when page fields are missing or fail
// validation (400).
var ErrInvalidDocumentationPage = errors.New("invalid documentation page")

// ErrSystemDocumentationPage is returned when an action is refused because the
// page is a built-in system page (delete, id/lang change) (403).
var ErrSystemDocumentationPage = errors.New("documentation page is system-protected")

// DocumentationPage is one documentation_pages row (issue #53).
type DocumentationPage struct {
	ID        string
	Lang      string
	Title     string
	Category  string
	BodyMD    string
	Audience  string
	Enabled   bool
	IsSystem  bool
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

// DocumentationAudienceUser and DocumentationAudienceAdmin are the two allowed
// audience values; user pages are public, admin pages require an admin caller.
const (
	DocumentationAudienceUser  = "user"
	DocumentationAudienceAdmin = "admin"

	// maxDocumentationBodyBytes bounds body_md to 256 KB.
	maxDocumentationBodyBytes = 256 * 1024
	// maxDocumentationTitleLen bounds title to 120 characters.
	maxDocumentationTitleLen = 120
	// maxDocumentationCategoryLen bounds category to 40 characters.
	maxDocumentationCategoryLen = 40
)

// DeriveDocumentationPageID converts a title to a lowercase hyphenated slug,
// mirroring DeriveCloudInitTemplateID (e.g. "Getting started" →
// "getting-started"). The slug is the page's permanent id.
func DeriveDocumentationPageID(title string) string {
	lowered := strings.ToLower(title)
	slug := slugRe.ReplaceAllString(lowered, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "page"
	}

	return slug
}

// toPage maps a store row to the catalog struct.
func toPage(r store.DocumentationPageRow) DocumentationPage {
	return DocumentationPage{
		ID: r.ID, Lang: r.Lang, Title: r.Title, Category: r.Category, BodyMD: r.BodyMD,
		Audience: r.Audience, Enabled: r.Enabled, IsSystem: r.IsSystem, SortOrder: r.SortOrder,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// ListDocumentationPages returns every page (including disabled ones), ordered
// by sort_order then title — the admin list endpoint's data source.
func ListDocumentationPages(ctx context.Context, st *store.Store) ([]DocumentationPage, error) {
	rows, err := st.DocumentationPagesAll(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]DocumentationPage, len(rows))
	for i, r := range rows {
		out[i] = toPage(r)
	}

	return out, nil
}

// EnabledDocumentationPages returns only enabled pages, ordered by sort_order
// then title — the public reader's data source.
func EnabledDocumentationPages(ctx context.Context, st *store.Store) ([]DocumentationPage, error) {
	rows, err := st.DocumentationPagesEnabled(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]DocumentationPage, len(rows))
	for i, r := range rows {
		out[i] = toPage(r)
	}

	return out, nil
}

// GetDocumentationPage returns a single page by (id, lang) with an en fallback:
// when the requested lang is absent, the en row is returned if present. Returns
// ErrDocumentationPageNotFound when neither the requested lang nor en exists.
func GetDocumentationPage(ctx context.Context, st *store.Store, id, lang string) (DocumentationPage, error) {
	row, err := st.GetDocumentationPage(ctx, id, lang)
	if err == nil {
		return toPage(row), nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return DocumentationPage{}, err
	}

	// en fallback: only attempt when the requested lang was not already en.
	if lang == "en" {
		return DocumentationPage{}, fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
	}

	fallback, ferr := st.GetDocumentationPage(ctx, id, "en")
	if ferr != nil {
		if errors.Is(ferr, sql.ErrNoRows) {
			return DocumentationPage{}, fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return DocumentationPage{}, ferr
	}

	return toPage(fallback), nil
}

// validateDocumentationPage checks the shared field constraints used by create
// and update.
func validateDocumentationPage(title, lang, category, bodyMD, audience string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidDocumentationPage)
	}

	if len(title) > maxDocumentationTitleLen {
		return fmt.Errorf("%w: title exceeds %d characters", ErrInvalidDocumentationPage, maxDocumentationTitleLen)
	}

	if lang != "en" && lang != "fr" {
		return fmt.Errorf("%w: lang must be en or fr", ErrInvalidDocumentationPage)
	}

	if len(category) > maxDocumentationCategoryLen {
		return fmt.Errorf("%w: category exceeds %d characters", ErrInvalidDocumentationPage, maxDocumentationCategoryLen)
	}

	if strings.TrimSpace(bodyMD) == "" {
		return fmt.Errorf("%w: body is required", ErrInvalidDocumentationPage)
	}

	if len(bodyMD) > maxDocumentationBodyBytes {
		return fmt.Errorf("%w: body exceeds %d bytes", ErrInvalidDocumentationPage, maxDocumentationBodyBytes)
	}

	if audience != DocumentationAudienceUser && audience != DocumentationAudienceAdmin {
		return fmt.Errorf("%w: audience must be user or admin", ErrInvalidDocumentationPage)
	}

	return nil
}

// CreateDocumentationPage derives a slug from the title, validates the fields,
// rejects slug collisions with ErrDuplicateDocumentationPage (409), and inserts
// the new row. The new page is enabled by default and is not a system page.
func CreateDocumentationPage(ctx context.Context, st *store.Store, title, lang, category, bodyMD, audience string) (DocumentationPage, error) {
	title = strings.TrimSpace(title)
	category = strings.TrimSpace(category)
	if err := validateDocumentationPage(title, lang, category, bodyMD, audience); err != nil {
		return DocumentationPage{}, err
	}

	id := DeriveDocumentationPageID(title)

	exists, err := st.DocumentationPageExists(ctx, id, lang)
	if err != nil {
		return DocumentationPage{}, err
	}

	if exists {
		return DocumentationPage{}, ErrDuplicateDocumentationPage
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	row := store.DocumentationPageRow{
		ID: id, Lang: lang, Title: title, Category: category, BodyMD: bodyMD,
		Audience: audience, Enabled: true, IsSystem: false, SortOrder: 0,
		CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := st.InsertDocumentationPage(ctx, row); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return DocumentationPage{}, ErrDuplicateDocumentationPage
		}

		return DocumentationPage{}, err
	}

	return toPage(row), nil
}

// UpdateDocumentationPage changes an existing page's mutable fields, revalidating
// them. The id and lang are immutable; is_system pages may be edited but never
// deleted or re-identified. Returns ErrDocumentationPageNotFound if the
// (id, lang) does not exist (404).
func UpdateDocumentationPage(ctx context.Context, st *store.Store, id, lang, title, category, bodyMD, audience string, enabled bool, sortOrder int) (DocumentationPage, error) {
	title = strings.TrimSpace(title)
	category = strings.TrimSpace(category)
	if err := validateDocumentationPage(title, lang, category, bodyMD, audience); err != nil {
		return DocumentationPage{}, err
	}

	current, err := st.GetDocumentationPage(ctx, id, lang)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentationPage{}, fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return DocumentationPage{}, err
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	updater := st.UpdateDocumentationPage
	if current.IsSystem {
		updater = st.UpdateSystemDocumentationPage
	}

	if err := updater(ctx, id, lang, title, category, bodyMD, audience, enabled, sortOrder, stamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentationPage{}, fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return DocumentationPage{}, err
	}

	return readDocumentationPageBack(ctx, st, id, lang)
}

// DeleteDocumentationPage removes a non-system page row. Returns
// ErrDocumentationPageNotFound if the (id, lang) does not exist, and
// ErrSystemDocumentationPage if the page is a built-in system page.
func DeleteDocumentationPage(ctx context.Context, st *store.Store, id, lang string) error {
	current, err := st.GetDocumentationPage(ctx, id, lang)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return err
	}

	if current.IsSystem {
		return fmt.Errorf("%w: %q", ErrSystemDocumentationPage, id)
	}

	if err := st.DeleteDocumentationPage(ctx, id, lang); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return err
	}

	return nil
}

// SetDocumentationPageEnabled toggles the enabled state for one page. A toggle
// is an upsert on the enabled column, never a delete. Returns
// ErrDocumentationPageNotFound if the (id, lang) does not exist.
func SetDocumentationPageEnabled(ctx context.Context, st *store.Store, id, lang string, enabled bool) error {
	exists, err := st.DocumentationPageExists(ctx, id, lang)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
	}

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.SetDocumentationPageEnabled(ctx, id, lang, enabled, stamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return err
	}

	return nil
}

// readDocumentationPageBack returns the current row state after an update.
func readDocumentationPageBack(ctx context.Context, st *store.Store, id, lang string) (DocumentationPage, error) {
	row, err := st.GetDocumentationPage(ctx, id, lang)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DocumentationPage{}, fmt.Errorf("%w: %q", ErrDocumentationPageNotFound, id)
		}

		return DocumentationPage{}, err
	}

	return toPage(row), nil
}
