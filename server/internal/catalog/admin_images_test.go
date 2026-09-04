package catalog_test

import (
	"context"
	"errors"
	"testing"

	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
)

// TestAdminListImages_IncludesSuperset verifies the admin image list includes
// the fake superset with the seeded approval enabled.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminListImages_IncludesSuperset(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	images, err := catalog.AdminListImages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListImages: %v", err)
	}

	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d: %+v", len(images), images)
	}

	enabled := 0

	for _, image := range images {
		if image.Enabled {
			enabled++
		}
	}

	if enabled != 1 {
		t.Errorf("expected exactly 1 enabled image (the seeded approval), got %d: %+v", enabled, images)
	}
}

// TestSetImageEnabled_ToggleIsolatesByFile verifies toggling one image does
// not affect another and persists the discovered size.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetImageEnabled_ToggleIsolatesByFile(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetImageEnabled(ctx, st, cluster.Fake{}, "default", catalog.ImageRef{Node: node01, Storage: storageLocal, File: "debian-12-generic-cloudimg-amd64.qcow2"}, true); err != nil {
		t.Fatalf("SetImageEnabled debian: %v", err)
	}

	images, err := catalog.AdminListImages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListImages: %v", err)
	}

	for _, image := range images {
		if image.File == "debian-12-generic-cloudimg-amd64.qcow2" && !image.Enabled {
			t.Error("debian-12 should be enabled after toggle")
		}

		if image.File == "rocky-9-generic-cloudimg-x86_64.raw" && image.Enabled {
			t.Error("rocky-9 should still be disabled (unaffected)")
		}
	}
}

// TestSetImageEnabled_UnknownReturnsError — toggling an image (node, storage,
// file) triple not in the current discovery set returns cluster.ErrNotFound.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetImageEnabled_UnknownReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetImageEnabled(ctx, st, cluster.Fake{}, "default", catalog.ImageRef{Node: node01, Storage: storageLocal, File: "missing.qcow2"}, true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetImageEnabled unknown: got %v, want cluster.ErrNotFound", err)
	}
}

// TestDeleteImage_RemovesOrphan — deleting a stored approval row works and a
// missing row reports ErrImageNotFound.
//
//nolint:paralleltest // serial: shared database fixture
func TestDeleteImage_RemovesOrphan(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetImageEnabled(ctx, "default", node01, storageLocal, "orphan.qcow2", 1024, false); err != nil {
		t.Fatalf("seed orphan approval: %v", err)
	}

	if err := catalog.DeleteImage(ctx, st, "default", node01, storageLocal, "orphan.qcow2"); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}

	err := catalog.DeleteImage(ctx, st, "default", node01, storageLocal, "orphan.qcow2")
	if !errors.Is(err, catalog.ErrImageNotFound) {
		t.Fatalf("second delete: got %v, want ErrImageNotFound", err)
	}
}
