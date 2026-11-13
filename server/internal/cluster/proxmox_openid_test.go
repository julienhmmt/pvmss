package cluster

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

const testRedirect = "https://pvmss.example/api/v1/auth/oidc/callback"

func proxmoxOpenIDFixture(t *testing.T) *http.ServeMux {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api2/json/access/openid/auth-url", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.FormValue("realm") != "zitadel" || r.FormValue("redirect-url") != testRedirect {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		writeJSONFixture(t, w, `{"data":"https://idp.example/authorize?state=pve-state&client_id=x"}`)
	})
	mux.HandleFunc("POST /api2/json/access/openid/login", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("code") != "good" || r.FormValue("state") != "pve-state" || r.FormValue("redirect-url") != testRedirect {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		writeJSONFixture(t, w, `{"data":{"ticket":"tix-oidc","CSRFPreventionToken":"csrf-oidc","username":"alice@zitadel"}}`)
	})
	mux.HandleFunc("GET /api2/json/access/permissions", func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("PVEAuthCookie"); err != nil || cookie.Value != "tix-oidc" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		writeJSONFixture(t, w, `{"data":{"/":{"Sys.Audit":1}}}`)
	})
	mux.HandleFunc("GET /api2/json/access/acl", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONFixture(t, w, `{"data":[{"path":"/pool/pvmss-alice","ugid":"alice@zitadel","type":"user","roleid":"PVMSSUser"}]}`)
	})

	return mux
}

func TestProxmox_OpenID(t *testing.T) {
	t.Parallel()

	mux := proxmoxOpenIDFixture(t)
	srv := newProxmoxTestServer(t, func(m *http.ServeMux) { m.Handle("/", mux) })
	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}
	ctx := context.Background()

	authURL, state, err := p.OpenIDAuthURL(ctx, "zitadel", testRedirect)
	if err != nil {
		t.Fatalf("OpenIDAuthURL: %v", err)
	}

	if state != "pve-state" || authURL != "https://idp.example/authorize?state=pve-state&client_id=x" {
		t.Fatalf("authURL=%q state=%q", authURL, state)
	}

	identity, err := p.OpenIDLogin(ctx, "good", state, testRedirect)
	if err != nil {
		t.Fatalf("OpenIDLogin: %v", err)
	}

	if identity != (Identity{Username: "alice@zitadel", Pool: "pvmss-alice"}) {
		t.Fatalf("identity = %+v", identity)
	}

	if _, err := p.OpenIDLogin(ctx, "bad", state, testRedirect); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rejected code: err = %v, want ErrNotFound", err)
	}
}

func TestFake_OpenID(t *testing.T) {
	t.Parallel()

	fake := NewFake("oidc")
	ctx := context.Background()

	_, state, err := fake.OpenIDAuthURL(ctx, "zitadel", testRedirect)
	if err != nil || state == "" {
		t.Fatalf("OpenIDAuthURL: state=%q err=%v", state, err)
	}

	identity, err := fake.OpenIDLogin(ctx, FakeOpenIDCode, state, testRedirect)
	if err != nil || identity.Username != FakeUserAlice || identity.Pool == "" {
		t.Fatalf("OpenIDLogin: %+v %v", identity, err)
	}

	if _, err := fake.OpenIDLogin(ctx, "bad", state, testRedirect); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad code: err = %v", err)
	}
}
