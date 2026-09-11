package cloudinit_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/store"
	"testing"
)

// stubUserFileStore is an in-memory UserFileStore for domain tests.
type stubUserFileStore struct {
	files     map[string]store.UserCloudInitFile // key: owner+"/"+id
	insertErr error
}

func newStubUserFileStore() *stubUserFileStore {
	return &stubUserFileStore{files: map[string]store.UserCloudInitFile{}}
}

func (s *stubUserFileStore) key(owner, id string) string { return owner + "/" + id }

func (s *stubUserFileStore) ListUserCloudInitFiles(_ context.Context, owner string) ([]store.UserCloudInitFile, error) {
	var out []store.UserCloudInitFile

	for _, f := range s.files {
		if f.Owner == owner {
			out = append(out, f)
		}
	}

	return out, nil
}

func (s *stubUserFileStore) GetUserCloudInitFile(_ context.Context, owner, id string) (store.UserCloudInitFile, bool, error) {
	f, ok := s.files[s.key(owner, id)]
	return f, ok, nil
}

func (s *stubUserFileStore) CountUserCloudInitFiles(_ context.Context, owner string) (int, error) {
	n := 0

	for _, f := range s.files {
		if f.Owner == owner {
			n++
		}
	}

	return n, nil
}

func (s *stubUserFileStore) InsertUserCloudInitFile(_ context.Context, f store.UserCloudInitFile) error {
	if s.insertErr != nil {
		return s.insertErr
	}

	k := s.key(f.Owner, f.ID)
	if _, exists := s.files[k]; exists {
		return store.ErrDuplicate
	}

	s.files[k] = f

	return nil
}

func (s *stubUserFileStore) UpdateUserCloudInitFile(_ context.Context, owner, id, label, content, stamp string) error {
	k := s.key(owner, id)

	f, ok := s.files[k]
	if !ok {
		return sql.ErrNoRows
	}

	f.Label, f.Content, f.UpdatedAt = label, content, stamp
	s.files[k] = f

	return nil
}

func (s *stubUserFileStore) DeleteUserCloudInitFile(_ context.Context, owner, id string) error {
	k := s.key(owner, id)
	if _, ok := s.files[k]; !ok {
		return sql.ErrNoRows
	}

	delete(s.files, k)

	return nil
}

const (
	ucfOwnerAlice   = "alice@pve"
	ucfValidContent = "#cloud-config\npackages:\n  - htop\n"
)

func TestCreateUserFile_ValidatesAndDerivesID(t *testing.T) {
	t.Parallel()

	st := newStubUserFileStore()

	f, err := cloudinit.CreateUserFile(context.Background(), st, ucfOwnerAlice, "  Dev Box! ", ucfValidContent)
	if err != nil {
		t.Fatalf("CreateUserFile: %v", err)
	}

	if f.ID != "dev-box" {
		t.Errorf("id = %q, want slug %q", f.ID, "dev-box")
	}

	if f.Label != "Dev Box!" {
		t.Errorf("label = %q, want trimmed label", f.Label)
	}
}

func TestCreateUserFile_InvalidInputs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		label   string
		content string
		wantErr error
	}{
		{"blank label", "   ", ucfValidContent, cloudinit.ErrUserFileInvalid},
		{"empty content", "Dev box", "", cloudinit.ErrUserFileInvalid},
		{"missing marker", "Dev box", "not cloud-config", cloudinit.ErrUserFileInvalid},
		{"bad yaml", "Dev box", "#cloud-config\n[unclosed", cloudinit.ErrUserFileInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := cloudinit.CreateUserFile(context.Background(), newStubUserFileStore(), ucfOwnerAlice, tc.label, tc.content)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestCreateUserFile_InvalidWrapsValidationDetail(t *testing.T) {
	t.Parallel()

	_, err := cloudinit.CreateUserFile(context.Background(), newStubUserFileStore(), ucfOwnerAlice, "Dev box", "not cloud-config")
	if !errors.Is(err, cloudinit.ErrUserFileInvalid) || !errors.Is(err, cloudinit.ErrSnippetPrefix) {
		t.Fatalf("error = %v, want ErrUserFileInvalid wrapping ErrSnippetPrefix", err)
	}
}

func TestCreateUserFile_Limit(t *testing.T) {
	t.Parallel()

	st := newStubUserFileStore()

	for i := range cloudinit.MaxUserCloudInitFiles {
		f := store.UserCloudInitFile{Owner: ucfOwnerAlice, ID: "f" + string(rune('a'+i)), Label: "x", Content: ucfValidContent}
		if err := st.InsertUserCloudInitFile(context.Background(), f); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	_, err := cloudinit.CreateUserFile(context.Background(), st, ucfOwnerAlice, "one too many", ucfValidContent)
	if !errors.Is(err, cloudinit.ErrUserFileLimit) {
		t.Fatalf("error = %v, want ErrUserFileLimit", err)
	}
}

func TestCreateUserFile_DuplicateSlug(t *testing.T) {
	t.Parallel()

	st := newStubUserFileStore()

	if _, err := cloudinit.CreateUserFile(context.Background(), st, ucfOwnerAlice, "Dev Box", ucfValidContent); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// "dev box" slugifies to the same id.
	_, err := cloudinit.CreateUserFile(context.Background(), st, ucfOwnerAlice, "dev  box", ucfValidContent)
	if !errors.Is(err, cloudinit.ErrUserFileDuplicate) {
		t.Fatalf("error = %v, want ErrUserFileDuplicate", err)
	}
}

func TestGetUpdateDeleteUserFile_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newStubUserFileStore()

	if _, err := cloudinit.GetUserFile(ctx, st, ucfOwnerAlice, "nope"); !errors.Is(err, cloudinit.ErrUserFileNotFound) {
		t.Errorf("get: err=%v, want ErrUserFileNotFound", err)
	}

	if _, err := cloudinit.UpdateUserFile(ctx, st, ucfOwnerAlice, "nope", "x", ucfValidContent); !errors.Is(err, cloudinit.ErrUserFileNotFound) {
		t.Errorf("update: err=%v, want ErrUserFileNotFound", err)
	}

	if err := cloudinit.DeleteUserFile(ctx, st, ucfOwnerAlice, "nope"); !errors.Is(err, cloudinit.ErrUserFileNotFound) {
		t.Errorf("delete: err=%v, want ErrUserFileNotFound", err)
	}
}

func TestUpdateUserFile_RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newStubUserFileStore()

	created, err := cloudinit.CreateUserFile(ctx, st, ucfOwnerAlice, "Dev box", ucfValidContent)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := cloudinit.UpdateUserFile(ctx, st, ucfOwnerAlice, created.ID, "Renamed", "#cloud-config\nruncmd: [echo hi]\n")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Label != "Renamed" || updated.Content != "#cloud-config\nruncmd: [echo hi]\n" {
		t.Errorf("updated = %+v", updated)
	}

	if err := cloudinit.DeleteUserFile(ctx, st, ucfOwnerAlice, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := cloudinit.GetUserFile(ctx, st, ucfOwnerAlice, created.ID); !errors.Is(err, cloudinit.ErrUserFileNotFound) {
		t.Errorf("get after delete: err=%v, want ErrUserFileNotFound", err)
	}
}
