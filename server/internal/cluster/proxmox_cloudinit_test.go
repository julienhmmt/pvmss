package cluster

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestParseCloudInitConfig(t *testing.T) {
	t.Parallel()

	cfg := proxmoxVMConfig{
		"ciuser":       FakeCloudInitUser,
		"sshkeys":      url.QueryEscape("ssh-ed25519 AAAA demo@laptop\nssh-ed25519 BBBB demo2@laptop"),
		"ipconfig0":    "ip=10.0.0.42/24,gw=10.0.0.1",
		"nameserver":   FakeCloudInitDNS,
		"searchdomain": "example.internal",
	}

	got := parseCloudInitConfig(cfg)

	if got.User != FakeCloudInitUser || got.Password != "" {
		t.Errorf("user/password = %q/%q", got.User, got.Password)
	}

	if len(got.SSHKeys) != 2 {
		t.Fatalf("sshkeys = %v", got.SSHKeys)
	}

	if got.IPMode != CloudInitIPModeStatic || got.IPAddress != "10.0.0.42/24" || got.Gateway != FakeCloudInitDNS {
		t.Errorf("ip = %+v", got)
	}

	if got.DNSServer != FakeCloudInitDNS || got.SearchDomain != "example.internal" {
		t.Errorf("dns/search = %q/%q", got.DNSServer, got.SearchDomain)
	}
}

func TestParseCloudInitConfig_DHCPDefault(t *testing.T) {
	t.Parallel()

	got := parseCloudInitConfig(proxmoxVMConfig{})
	if got.IPMode != CloudInitIPModeDHCP {
		t.Errorf("IPMode = %q, want dhcp default", got.IPMode)
	}
}

func TestProxmox_GetCloudInitConfig(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"ciuser":"debian","ipconfig0":"ip=dhcp"}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	got, err := p.GetCloudInitConfig(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if got.User != FakeCloudInitUser || got.IPMode != CloudInitIPModeDHCP {
		t.Errorf("got = %+v", got)
	}
}

// TestAgentEnabled verifies the agent= grammar: the first comma token decides,
// so "0,frozen=1" is disabled and "1,frozen=1" enabled.
func TestAgentEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "empty is disabled", raw: "", want: false},
		{name: "0 is disabled", raw: "0", want: false},
		{name: "1 is enabled", raw: "1", want: true},
		{name: "1 with frozen is enabled", raw: "1,frozen=1", want: true},
		{name: "0 with frozen is disabled", raw: "0,frozen=1", want: false},
		{name: "enabled=1 key form is enabled", raw: "enabled=1", want: true},
		{name: "enabled=1 with type is enabled", raw: "enabled=1,type=virtio", want: true},
		{name: "enabled=0 key form is disabled", raw: "enabled=0", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := agentEnabled(tc.raw); got != tc.want {
				t.Errorf("agentEnabled(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
