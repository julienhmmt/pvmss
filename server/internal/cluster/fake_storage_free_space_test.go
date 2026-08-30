package cluster

import (
	"context"
	"errors"
	"testing"
)

// TestFakeStorageFreeSpace_ReturnsAvailFromDataset verifies the fake's
// StorageFreeSpace returns Total - Used from the static storage dataset
// (US3/issue-04 T047).
func TestFakeStorageFreeSpace_ReturnsAvailFromDataset(t *testing.T) {
	t.Parallel()

	fake := (Fake{})

	// FakeStorageLocal on FakeNode01: Total=2199023255552, Used=879609302220.
	free, err := fake.StorageFreeSpace(context.Background(), FakeNode01, FakeStorageLocal)
	if err != nil {
		t.Fatalf("StorageFreeSpace: %v", err)
	}

	want := int64(2199023255552) - int64(879609302220)
	if free != want {
		t.Errorf("free = %d, want %d (Total - Used)", free, want)
	}
}

// TestFakeStorageFreeSpace_UnknownStorageReturnsNotFound verifies the fake
// returns ErrNotFound for an unknown (node, storage) pair.
func TestFakeStorageFreeSpace_UnknownStorageReturnsNotFound(t *testing.T) {
	t.Parallel()

	fake := (Fake{})

	_, err := fake.StorageFreeSpace(context.Background(), FakeNode01, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestFakeStorageFreeSpace_KnownStorageOnWrongNodeReturnsNotFound verifies
// the fake returns ErrNotFound when the storage exists but on a different node.
func TestFakeStorageFreeSpace_KnownStorageOnWrongNodeReturnsNotFound(t *testing.T) {
	t.Parallel()

	fake := (Fake{})

	// FakeStorageLocal exists on FakeNode01 and FakeNode02, but not FakeNode03.
	_, err := fake.StorageFreeSpace(context.Background(), FakeNode03, FakeStorageLocal)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
