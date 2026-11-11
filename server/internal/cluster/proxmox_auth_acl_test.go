package cluster

import (
	"context"
	"net/http"
	"testing"
)

// TestProxmoxOwnedPool_ACLFallback: a non-@pve identity (OIDC, LDAP) is bound
// to the PVMSS pool its user ACL points at, never to an inherited or
// unrelated grant.
func TestProxmoxOwnedPool_ACLFallback(t *testing.T) {
	t.Parallel()

	const user = "alice@zitadel"

	cases := []struct {
		name   string
		acl    string
		status int
		want   string
	}{
		{name: "match", acl: `[{"path":"/pool/pvmss-alice","ugid":"alice@zitadel","type":"user","roleid":"PVMSSUser"}]`, want: "pvmss-alice"},
		{name: "no acl", acl: `[]`, want: ""},
		{name: "two pools first by name", acl: `[{"path":"/pool/pvmss-zed","ugid":"alice@zitadel","type":"user","roleid":"PVMSSUser"},{"path":"/pool/pvmss-bob","ugid":"alice@zitadel","type":"user","roleid":"PVMSSUser"}]`, want: "pvmss-bob"},
		{name: "non pvmss pool ignored", acl: `[{"path":"/pool/lab","ugid":"alice@zitadel","type":"user","roleid":"PVMSSUser"}]`, want: ""},
		{name: "other role ignored", acl: `[{"path":"/pool/pvmss-alice","ugid":"alice@zitadel","type":"user","roleid":"PVEAuditor"}]`, want: ""},
		{name: "group grant ignored", acl: `[{"path":"/pool/pvmss-alice","ugid":"alice@zitadel","type":"group","roleid":"PVMSSUser"}]`, want: ""},
		{name: "other user ignored", acl: `[{"path":"/pool/pvmss-bob","ugid":"bob@zitadel","type":"user","roleid":"PVMSSUser"}]`, want: ""},
		{name: "proxmox error", status: http.StatusInternalServerError, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/access/acl", func(w http.ResponseWriter, _ *http.Request) {
					if tc.status != 0 {
						w.WriteHeader(tc.status)
						return
					}

					writeJSONFixture(t, w, `{"data":`+tc.acl+`}`)
				})
			})
			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			got, err := proxmoxOwnedPool(context.Background(), p.rest(), user)
			if err != nil {
				t.Fatalf("proxmoxOwnedPool: %v", err)
			}

			if got != tc.want {
				t.Fatalf("pool = %q, want %q", got, tc.want)
			}
		})
	}
}
