package store_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	testSSHKeyUserAlice = "alice@pve"
	testSSHKeyUserBob   = "bob@pve"
	testSSHKeyClusterB  = "other"
	testSSHKeyMaxKeys   = 10
)

func newProfileSSHKey(t *testing.T, cluster, username, label string) store.ProfileSSHKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("convert public key: %v", err)
	}

	return store.ProfileSSHKey{
		ID:        newProfileSSHKeyID(t),
		Cluster:   cluster,
		Username:  username,
		Label:     label,
		PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		CreatedAt: time.Now().UTC(),
	}
}

func newProfileSSHKeyID(t *testing.T) string {
	t.Helper()

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("read id entropy: %v", err)
	}

	return hex.EncodeToString(buf)
}

func mustCreateProfileSSHKey(t *testing.T, st *store.Store, key store.ProfileSSHKey, maxKeys int) {
	t.Helper()

	if err := st.CreateProfileSSHKey(context.Background(), key, maxKeys); err != nil {
		t.Fatalf("CreateProfileSSHKey(%s): %v", key.Label, err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_CreateListDelete(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	second := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "laptop")
	second.CreatedAt = second.CreatedAt.Add(time.Minute)

	first := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "desktop")

	mustCreateProfileSSHKey(t, st, second, testSSHKeyMaxKeys)
	mustCreateProfileSSHKey(t, st, first, testSSHKeyMaxKeys)

	keys, err := st.ListProfileSSHKeys(ctx, testStoreCluster, testSSHKeyUserAlice)
	if err != nil {
		t.Fatalf("ListProfileSSHKeys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("listed %d keys, want 2", len(keys))
	}

	if keys[0].ID != first.ID || keys[1].ID != second.ID {
		t.Fatalf("order = [%s %s], want created_at ASC [%s %s]", keys[0].ID, keys[1].ID, first.ID, second.ID)
	}

	if keys[0].Label != "desktop" || keys[0].PublicKey != first.PublicKey {
		t.Fatalf("first row = %+v, want label desktop and the stored public key", keys[0])
	}

	if err := st.DeleteProfileSSHKey(ctx, testStoreCluster, testSSHKeyUserAlice, first.ID); err != nil {
		t.Fatalf("DeleteProfileSSHKey: %v", err)
	}

	keys, err = st.ListProfileSSHKeys(ctx, testStoreCluster, testSSHKeyUserAlice)
	if err != nil {
		t.Fatalf("ListProfileSSHKeys after delete: %v", err)
	}

	if len(keys) != 1 || keys[0].ID != second.ID {
		t.Fatalf("after delete keys = %+v, want only %s", keys, second.ID)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_EmptyListIsNonNil(t *testing.T) {
	st := openClusterStore(t)

	keys, err := st.ListProfileSSHKeys(context.Background(), testStoreCluster, "nobody@pve")
	if err != nil {
		t.Fatalf("ListProfileSSHKeys: %v", err)
	}

	if keys == nil || len(keys) != 0 {
		t.Fatalf("keys = %#v, want empty non-nil slice", keys)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_ScopeIsolation(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	aliceKey := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "alice-key")
	mustCreateProfileSSHKey(t, st, aliceKey, testSSHKeyMaxKeys)

	// Same username on another cluster sees and deletes nothing.
	if keys, err := st.ListProfileSSHKeys(ctx, testSSHKeyClusterB, testSSHKeyUserAlice); err != nil || len(keys) != 0 {
		t.Fatalf("cross-cluster list = %v, %v; want empty", keys, err)
	}

	if err := st.DeleteProfileSSHKey(ctx, testSSHKeyClusterB, testSSHKeyUserAlice, aliceKey.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-cluster delete err = %v, want sql.ErrNoRows", err)
	}

	// Another username on the same cluster sees and deletes nothing.
	if keys, err := st.ListProfileSSHKeys(ctx, testStoreCluster, testSSHKeyUserBob); err != nil || len(keys) != 0 {
		t.Fatalf("cross-user list = %v, %v; want empty", keys, err)
	}

	if err := st.DeleteProfileSSHKey(ctx, testStoreCluster, testSSHKeyUserBob, aliceKey.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-user delete err = %v, want sql.ErrNoRows", err)
	}

	// The original row survives both scoped-out attempts.
	keys, err := st.ListProfileSSHKeys(ctx, testStoreCluster, testSSHKeyUserAlice)
	if err != nil || len(keys) != 1 {
		t.Fatalf("alice keys after scoped-out deletes = %v, %v; want 1 row", keys, err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_Limit(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "one"), 2)
	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "two"), 2)

	err := st.CreateProfileSSHKey(ctx, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "three"), 2)
	if !errors.Is(err, store.ErrProfileSSHKeyLimit) {
		t.Fatalf("third key err = %v, want ErrProfileSSHKeyLimit", err)
	}

	// The limit is per (cluster, username): another scope still accepts keys.
	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testSSHKeyClusterB, testSSHKeyUserAlice, "other-cluster"), 2)
	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserBob, "bob-key"), 2)
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_DuplicateLabel(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "laptop"), testSSHKeyMaxKeys)

	err := st.CreateProfileSSHKey(ctx, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "laptop"), testSSHKeyMaxKeys)
	if !errors.Is(err, store.ErrProfileSSHKeyDuplicateLabel) {
		t.Fatalf("duplicate label err = %v, want ErrProfileSSHKeyDuplicateLabel", err)
	}

	// The same label under another scope is not a duplicate.
	mustCreateProfileSSHKey(t, st, newProfileSSHKey(t, testStoreCluster, testSSHKeyUserBob, "laptop"), testSSHKeyMaxKeys)
}

//nolint:paralleltest // serial: shared database fixture
func TestProfileSSHKeys_DuplicateFingerprint(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	key := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "original")
	mustCreateProfileSSHKey(t, st, key, testSSHKeyMaxKeys)

	// Same key blob under a different label and comment is still the same key.
	relabelled := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "copy")
	relabelled.PublicKey = key.PublicKey + " pasted-again"

	err := st.CreateProfileSSHKey(ctx, relabelled, testSSHKeyMaxKeys)
	if !errors.Is(err, store.ErrProfileSSHKeyDuplicate) {
		t.Fatalf("duplicate fingerprint err = %v, want ErrProfileSSHKeyDuplicate", err)
	}

	// Byte-identical row takes the same path.
	identical := newProfileSSHKey(t, testStoreCluster, testSSHKeyUserAlice, "identical")
	identical.PublicKey = key.PublicKey

	err = st.CreateProfileSSHKey(ctx, identical, testSSHKeyMaxKeys)
	if !errors.Is(err, store.ErrProfileSSHKeyDuplicate) {
		t.Fatalf("identical key err = %v, want ErrProfileSSHKeyDuplicate", err)
	}
}
