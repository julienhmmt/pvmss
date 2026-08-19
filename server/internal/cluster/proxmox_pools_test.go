package cluster

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestProxmox_EnsurePoolRole_CreatesWhenAbsent(t *testing.T) {
	t.Parallel()

	var created bool

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/access/roles", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"roleid":"PVEVMUser"}]}`)
		})
		mux.HandleFunc("POST /api2/json/access/roles", func(w http.ResponseWriter, r *http.Request) {
			created = true

			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			if r.FormValue("roleid") != "PVMSSUser" {
				t.Errorf("roleid = %q", r.FormValue("roleid"))
			}

			if !strings.Contains(r.FormValue("privs"), "VM.PowerMgmt") {
				t.Errorf("privs = %q, missing VM.PowerMgmt", r.FormValue("privs"))
			}

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.EnsurePoolRole(context.Background()); err != nil {
		t.Fatalf("EnsurePoolRole: %v", err)
	}

	if !created {
		t.Fatal("expected role creation call")
	}
}

func TestProxmox_EnsurePoolRole_NoOpWhenPresent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/access/roles", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"roleid":"PVMSSUser"}]}`)
		})
		mux.HandleFunc("POST /api2/json/access/roles", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not create a role that already exists")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.EnsurePoolRole(context.Background()); err != nil {
		t.Fatalf("EnsurePoolRole: %v", err)
	}
}

func TestProxmox_EnsurePoolUser_CreatesWhenAbsent(t *testing.T) {
	t.Parallel()

	var gotUserID, gotPassword string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/access/users", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[]}`)
		})
		mux.HandleFunc("POST /api2/json/access/users", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotUserID = r.FormValue("userid")
			gotPassword = r.FormValue("password")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	username, err := p.EnsurePoolUser(context.Background(), "alice", "s3cret-pass")
	if err != nil {
		t.Fatalf("EnsurePoolUser: %v", err)
	}

	if username != "alice@pve" {
		t.Errorf("username = %q, want alice@pve", username)
	}

	if gotUserID != "alice@pve" || gotPassword != "s3cret-pass" {
		t.Errorf("sent userid=%q password=%q", gotUserID, gotPassword)
	}
}

func TestProxmox_EnsurePoolUser_NoOpWhenPresent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/access/users", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"userid":"alice@pve"}]}`)
		})
		mux.HandleFunc("POST /api2/json/access/users", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not recreate an existing user")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	username, err := p.EnsurePoolUser(context.Background(), "alice", "ignored")
	if err != nil {
		t.Fatalf("EnsurePoolUser: %v", err)
	}

	if username != "alice@pve" {
		t.Errorf("username = %q, want alice@pve", username)
	}
}

func TestProxmox_CreatePool_Idempotent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/pools", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"poolid":"alice"}]}`)
		})
		mux.HandleFunc("POST /api2/json/pools", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not recreate an existing pool")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.CreatePool(context.Background(), "alice", "Alice's pool"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
}

func TestProxmox_SetPoolACL(t *testing.T) {
	t.Parallel()

	var gotPath, gotUsers, gotRoles string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/access/acl", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotPath = r.FormValue("path")
			gotUsers = r.FormValue("users")
			gotRoles = r.FormValue("roles")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetPoolACL(context.Background(), "alice@pve", "alice", "PVMSSUser"); err != nil {
		t.Fatalf("SetPoolACL: %v", err)
	}

	if gotPath != "/pool/alice" || gotUsers != "alice@pve" || gotRoles != "PVMSSUser" {
		t.Errorf("path=%q users=%q roles=%q", gotPath, gotUsers, gotRoles)
	}
}

func TestProxmox_DeletePool_NotFound(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/pools", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.DeletePool(context.Background(), "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProxmox_DeletePool_Success(t *testing.T) {
	t.Parallel()

	var deleted bool

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/pools", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"poolid":"alice"}]}`)
		})
		mux.HandleFunc("DELETE /api2/json/pools/alice", func(w http.ResponseWriter, _ *http.Request) {
			deleted = true

			writeJSONFixture(t, w, `{"data":null}`)
		})
		mux.HandleFunc("PUT /api2/json/access/acl", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.DeletePool(context.Background(), "alice"); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}

	if !deleted {
		t.Fatal("expected DELETE /pools/alice")
	}
}

func TestProxmox_DeleteUser(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("DELETE /api2/json/access/users/alice@pve", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.DeleteUser(context.Background(), "alice@pve"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if gotPath != "/api2/json/access/users/alice@pve" {
		t.Errorf("path = %q", gotPath)
	}
}
