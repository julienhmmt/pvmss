package catalog_test

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"strings"
	"testing"
)

const validCloudInitContent = "#cloud-config\npackages:\n  - nginx\n"

// TestDeriveCloudInitTemplateID checks slug derivation from labels, mirroring
// T11's DeriveProfileID convention (lowercase, hyphenated).
func TestDeriveCloudInitTemplateID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		want  string
	}{
		{"Web server", "web-server"},
		{"DB server (postgres)", "db-server-postgres"},
		{"  spaced  ", "spaced"},
		{"!!!", "template"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tc := range tests {
		got := catalog.DeriveCloudInitTemplateID(tc.label)
		if got != tc.want {
			t.Errorf("DeriveCloudInitTemplateID(%q) = %q, want %q", tc.label, got, tc.want)
		}
	}
}

// TestCreateCloudInitTemplate_Success — a new template is created enabled by
// default and appears in both the admin list and the enabled-only catalog reader.
func TestCreateCloudInitTemplate_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent)
	if err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	if tmpl.ID != "web-server" {
		t.Errorf("tmpl.ID = %q, want %q", tmpl.ID, "web-server")
	}

	if !tmpl.Enabled {
		t.Error("new template should be enabled by default")
	}

	if tmpl.Content != validCloudInitContent {
		t.Errorf("tmpl.Content = %q, want %q", tmpl.Content, validCloudInitContent)
	}

	adminList, err := catalog.ListCloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListCloudInitTemplates: %v", err)
	}

	found := false

	for _, t2 := range adminList {
		if t2.ID == tmpl.ID {
			found = true

			if !t2.Enabled {
				t.Error("template should be enabled in admin list")
			}
		}
	}

	if !found {
		t.Error("created template not found in admin list")
	}

	enabled, err := catalog.CloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("CloudInitTemplates: %v", err)
	}

	if !cloudInitTemplateListContains(enabled, tmpl.ID) {
		t.Error("created template not found in enabled-only catalog reader")
	}
}

// TestCreateCloudInitTemplate_SlugCollision — creating a template whose label
// derives to an existing slug returns ErrDuplicateCloudInitTemplate.
func TestCreateCloudInitTemplate_SlugCollision(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	if _, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web Server", validCloudInitContent)
	if !errors.Is(err, catalog.ErrDuplicateCloudInitTemplate) {
		t.Fatalf("collision: got %v, want ErrDuplicateCloudInitTemplate", err)
	}
}

// TestCreateCloudInitTemplate_InvalidContent — content validation delegates to
// T08's cloudinit.Validate: reject missing #cloud-config prefix, reject > 16 KiB.
func TestCreateCloudInitTemplate_InvalidContent(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		label   string
		content string
	}{
		{"missing prefix", "No prefix", "packages:\n  - nginx\n"},
		{"empty label", "", validCloudInitContent},
		{"too large", "Big", "#cloud-config\n" + strings.Repeat("x", 16*1024+1)},
	}
	for _, tc := range tests {
		_, err := catalog.CreateCloudInitTemplate(ctx, st, "default", tc.label, tc.content)
		if err == nil {
			t.Errorf("CreateCloudInitTemplate(%s): expected error, got nil", tc.name)
		}
	}
}

// TestCloudInitTemplates_EnabledOnly — CloudInitTemplates (T06's catalog
// reader) and CloudInitTemplate (single lookup) return only enabled templates,
// while ListCloudInitTemplates (admin) returns every row including disabled.
func TestCloudInitTemplates_EnabledOnly(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, "default", tmpl.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	enabled, err := catalog.CloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("CloudInitTemplates: %v", err)
	}

	if cloudInitTemplateListContains(enabled, tmpl.ID) {
		t.Error("disabled template should not appear in CloudInitTemplates (enabled-only)")
	}

	if _, err := catalog.FindCloudInitTemplate(ctx, st, "default", tmpl.ID); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Fatalf("FindCloudInitTemplate on disabled: got %v, want ErrCloudInitTemplateNotFound", err)
	}

	adminList, err := catalog.ListCloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListCloudInitTemplates: %v", err)
	}

	if !cloudInitTemplateListContains(adminList, tmpl.ID) {
		t.Error("disabled template should appear in admin list (ListCloudInitTemplates)")
	}
}

// TestSetCloudInitTemplateEnabled_ToggleIsUpsert — disabling then re-enabling
// never deletes the row; the row persists with its enabled flag toggled.
func TestSetCloudInitTemplateEnabled_ToggleIsUpsert(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, "default", tmpl.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, "default", tmpl.ID, true); err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	enabled, err := catalog.CloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("CloudInitTemplates: %v", err)
	}

	if !cloudInitTemplateListContains(enabled, tmpl.ID) {
		t.Error("re-enabled template should appear in CloudInitTemplates")
	}
}

// TestSetCloudInitTemplateEnabled_NotFound — toggling a non-existent template
// returns ErrCloudInitTemplateNotFound.
func TestSetCloudInitTemplateEnabled_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, "default", "nonexistent", false); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Fatalf("got %v, want ErrCloudInitTemplateNotFound", err)
	}
}

// TestUpdateCloudInitTemplate_Success — updating label and content is reflected
// on the next read.
func TestUpdateCloudInitTemplate_Success(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := catalog.UpdateCloudInitTemplate(ctx, st, "default", tmpl.ID, "Web server (nginx + certbot)", "#cloud-config\npackages:\n  - nginx\n  - certbot\n")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Label != "Web server (nginx + certbot)" {
		t.Errorf("updated.Label = %q", updated.Label)
	}

	if !strings.Contains(updated.Content, "certbot") {
		t.Errorf("updated.Content = %q, want certbot", updated.Content)
	}
}

// TestUpdateCloudInitTemplate_NotFound — updating a non-existent template
// returns ErrCloudInitTemplateNotFound.
func TestUpdateCloudInitTemplate_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	if _, err := catalog.UpdateCloudInitTemplate(ctx, st, "default", "nonexistent", "x", validCloudInitContent); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Fatalf("got %v, want ErrCloudInitTemplateNotFound", err)
	}
}

// TestDeleteCloudInitTemplate_NoCascade — deleting a template removes it from
// every list; no cascade (FR-009 — a VM created from it keeps its own snippet).
func TestDeleteCloudInitTemplate_NoCascade(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, "default", "Web server", validCloudInitContent)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := catalog.DeleteCloudInitTemplate(ctx, st, "default", tmpl.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	adminList, err := catalog.ListCloudInitTemplates(ctx, st, "default")
	if err != nil {
		t.Fatalf("ListCloudInitTemplates: %v", err)
	}

	if cloudInitTemplateListContains(adminList, tmpl.ID) {
		t.Error("deleted template still in admin list")
	}
}

// TestDeleteCloudInitTemplate_NotFound — deleting a non-existent template
// returns ErrCloudInitTemplateNotFound.
func TestDeleteCloudInitTemplate_NotFound(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.DeleteCloudInitTemplate(ctx, st, "default", "nonexistent"); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Fatalf("got %v, want ErrCloudInitTemplateNotFound", err)
	}
}

// cloudInitTemplateListContains reports whether the list contains an entry with
// the given id.
func cloudInitTemplateListContains(list []catalog.CloudInitTemplate, id string) bool {
	for _, t := range list {
		if t.ID == id {
			return true
		}
	}

	return false
}
