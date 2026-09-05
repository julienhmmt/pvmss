package cluster

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

	if got.IPMode != CloudInitIPModeStatic || got.IPAddress != "10.0.0.42/24" || got.Gateway != "10.0.0.1" {
		t.Errorf("ip = %+v", got)
	}

	if got.DNSServer != "10.0.0.1" || got.SearchDomain != "example.internal" {
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

func TestProxmox_EnsureCloudInitDrive_UsesExistingDiskStorage(t *testing.T) {
	t.Parallel()

	var gotValue string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotValue = r.FormValue("ide3")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.EnsureCloudInitDrive(context.Background(), testNodeName, testVMID); err != nil {
		t.Fatalf("EnsureCloudInitDrive: %v", err)
	}

	if gotValue != "local-lvm:cloudinit" {
		t.Errorf("ide3 = %q, want local-lvm:cloudinit", gotValue)
	}
}

func TestProxmox_EnsureCloudInitDrive_NoOpWhenPresent(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"ide3":"local-lvm:vm-101-cloudinit,media=cdrom"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not rewrite an existing cloud-init drive")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.EnsureCloudInitDrive(context.Background(), testNodeName, testVMID); err != nil {
		t.Fatalf("EnsureCloudInitDrive: %v", err)
	}
}

func TestProxmox_SetCloudInitConfig_EnsuresDriveThenWrites(t *testing.T) {
	t.Parallel()

	var driveEnsured bool

	var gotIPConfig string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			if r.FormValue("ide3") == "local-lvm:cloudinit" {
				driveEnsured = true
			}

			if v := r.FormValue("ipconfig0"); v != "" {
				gotIPConfig = v
			}

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	config := CloudInitConfig{User: FakeCloudInitUser, IPMode: CloudInitIPModeStatic, IPAddress: "10.0.0.42/24", Gateway: "10.0.0.1"}
	if err := p.SetCloudInitConfig(context.Background(), testNodeName, testVMID, config); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	if !driveEnsured {
		t.Error("expected the cloud-init drive to be ensured before the config write")
	}

	if gotIPConfig != "ip=10.0.0.42/24,gw=10.0.0.1" {
		t.Errorf("ipconfig0 = %q", gotIPConfig)
	}
}

func TestProxmox_PushCloudInitSnippet(t *testing.T) {
	t.Parallel()

	var gotContentField, gotFilename, gotFileBody string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/storage/local/upload", func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			if err := r.ParseMultipartForm(1 << 20); err != nil { //nolint:gosec // test-only mock server (httptest, local loopback), body already bounded by MaxBytesReader above
				t.Fatalf("parse multipart: %v", err)
			}

			gotContentField = r.FormValue("content")

			file, header, err := r.FormFile("filename")
			if err != nil {
				t.Fatalf("form file: %v", err)
			}
			defer func() { _ = file.Close() }()

			gotFilename = header.Filename

			gotFileBody = readMultipartFileBody(t, file)

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.PushCloudInitSnippet(context.Background(), testNodeName, "local", testSnippetFilename, testVMID, "#cloud-config\n")
	if err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}

	if gotContentField != "snippets" {
		t.Errorf("content field = %q, want snippets", gotContentField)
	}

	if gotFilename != testSnippetFilename {
		t.Errorf("filename = %q", gotFilename)
	}

	if gotFileBody != "#cloud-config\n" {
		t.Errorf("file body = %q", gotFileBody)
	}
}

// TestProxmox_FindSnippetStorage_NotFound verifies the refusal when the node
// has no snippet-capable storage at all.
func TestProxmox_FindSnippetStorage_NotFound(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/storage", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.FindSnippetStorage(context.Background(), testNodeName)
	if err == nil {
		t.Fatal("expected an error when no snippet-capable storage exists")
	}
}

// TestProxmox_SetCloudInitConfig_SSHKeysSurviveProxmoxDecode is the ticket-01
// regression test. Proxmox percent-decodes sshkeys with Perl's uri_unescape,
// which does NOT turn '+' back into a space; url.QueryEscape encodes a space
// AS '+', so every key written that way reached the guest as
// "ssh-ed25519+AAAA...". The wire body must therefore carry %2520 (the form
// encoding of the literal "%20" PathEscape produced), never %2B.
func TestProxmox_SetCloudInitConfig_SSHKeysSurviveProxmoxDecode(t *testing.T) {
	t.Parallel()

	var rawBody string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			rawBody = string(body)

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	config := CloudInitConfig{SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice@laptop"}, IPMode: CloudInitIPModeDHCP}
	if err := p.SetCloudInitConfig(context.Background(), testNodeName, testVMID, config); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	// The space between key type and blob must travel as %2520 on the wire
	// (form-encoding of the percent-encoded %20), never as %2B (the
	// form-encoding of QueryEscape's '+').
	if !strings.Contains(rawBody, "%2520") {
		t.Errorf("wire body = %q, want a %%2520-encoded space (PathEscape), got QueryEscape's plus", rawBody)
	}

	if strings.Contains(rawBody, "%2B") {
		t.Errorf("body carries a form-encoded '+' for a space: %s", rawBody)
	}
}

// TestEncodeSSHKeys_OnlyProxmoxSafeCharacters is the live regression test:
// Proxmox's own sshkeys format validator rejected a real key (an email-style
// "user@host" comment, universal in ssh-keygen output) with "HTTP 400:
// sshkeys: invalid format - invalid urlencoded string" — url.PathEscape
// leaves '@' unescaped (RFC3986 allows it raw in a path segment), but
// Proxmox's validator does not accept it raw. Only unreserved characters and
// %XX sequences may appear in the output; anything else reproduces the live
// 400 on the next SSH key with a "user@host" comment.
func TestEncodeSSHKeys_OnlyProxmoxSafeCharacters(t *testing.T) {
	t.Parallel()

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEEHKEQ6FLrn8b85ClMxvu04DbAiyMZ5tf5ktL4xEpSZ mettmett@JH-LVL10"

	encoded := encodeSSHKeys([]string{key})

	for _, r := range encoded {
		safe := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '.' || r == '_' || r == '~' || r == '%'
		if !safe {
			t.Fatalf("encoded sshkeys %q contains %q, not a Proxmox-safe character", encoded, r)
		}
	}

	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		t.Fatalf("PathUnescape: %v", err)
	}

	if decoded != key {
		t.Errorf("round-tripped key = %q, want %q", decoded, key)
	}
}

// TestProxmox_SetCloudInitConfig_SSHKeysRoundTrip simulates Proxmox's exact
// decode chain — form-decode on receipt, then uri_unescape (decodes %XX,
// leaves '+') when generating the seed — and asserts the key read back is
// byte-identical, including a base64 blob containing a literal '+'.
func TestProxmox_SetCloudInitConfig_SSHKeysRoundTrip(t *testing.T) {
	t.Parallel()

	keyWithPlus := "ssh-ed25519 AAAAB3NzaC1+abc== alice@laptop"
	want := []string{keyWithPlus, "ssh-ed25519 BBBB bob@laptop"}

	var stored string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			// Proxmox stores the form-decoded value verbatim in the config.
			stored = r.FormValue("sshkeys")

			writeJSONFixture(t, w, `{"data":null}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			// A cloud-init drive is already present, so the ensure step
			// short-circuits and the test exercises only the sshkeys path.
			writeJSONFixture(t, w, `{"data":{"ide3":"local-lvm:vm-101-cloudinit,media=cdrom","sshkeys":`+strconv.Quote(stored)+`}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	config := CloudInitConfig{SSHKeys: want, IPMode: CloudInitIPModeDHCP}
	if err := p.SetCloudInitConfig(context.Background(), testNodeName, testVMID, config); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	// The stored config must carry %20 for spaces — a '+' would decode to a
	// space and corrupt the key inside the guest.
	if strings.Contains(stored, "+ ") {
		t.Errorf("stored sshkeys = %q, want spaces percent-encoded", stored)
	}

	got, err := p.GetCloudInitConfig(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if len(got.SSHKeys) != len(want) || got.SSHKeys[0] != want[0] || got.SSHKeys[1] != want[1] {
		t.Errorf("round-tripped keys = %+v, want %+v", got.SSHKeys, want)
	}
}

// TestProxmox_FindSnippetStorage_PrefersSharedActive verifies the selection
// rule (ticket 04): inactive storages are skipped, and a shared storage wins
// over a node-local one so a later migration cannot orphan the snippet.
func TestProxmox_FindSnippetStorage_PrefersSharedActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rows string
		want string
	}{
		{name: "shared active preferred over local", rows: `[{"storage":"local","active":1,"shared":0},{"storage":"cephfs","active":1,"shared":1}]`, want: "cephfs"},
		{name: "inactive storage ignored", rows: `[{"storage":"local","active":0,"shared":0},{"storage":"local-2","active":1,"shared":0}]`, want: "local-2"},
		{name: "only inactive storages is not found", rows: `[{"storage":"local","active":0,"shared":0}]`, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/nodes/node01/storage", func(w http.ResponseWriter, _ *http.Request) {
					writeJSONFixture(t, w, `{"data":`+tc.rows+`}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			got, err := p.FindSnippetStorage(context.Background(), testNodeName)
			if tc.want == "" {
				if err == nil {
					t.Fatal("expected an error when no active snippet storage exists")
				}

				return
			}

			if err != nil {
				t.Fatalf("FindSnippetStorage: %v", err)
			}

			if got != tc.want {
				t.Errorf("storage = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProxmox_AttachCloudInitSnippet verifies the snippet is wired to the VM
// via cicustom=vendor= (MERGE semantics, so generated user-data is preserved)
// when a filename is given, and that an empty filename clears the cicustom key
// instead of setting it. Fixes the silent no-op reported in REPORT.md §4/addendum.
//

func TestProxmox_AttachCloudInitSnippet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		wantKey  string
		wantVal  string
	}{
		{name: "vendor slot merges snippet", filename: testSnippetFilename, wantKey: "cicustom", wantVal: "vendor=local:snippets/pvmss-101.yml"},
		{name: "empty filename detaches", filename: "", wantKey: "delete", wantVal: "cicustom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			attachCloudInitSnippetSubtest(t, tc.filename, tc.wantKey, tc.wantVal)
		})
	}
}

func attachCloudInitSnippetSubtest(t *testing.T, filename, wantKey, wantVal string) {
	t.Helper()

	seen := map[string]string{}

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		// The VM already carries a cloud-init drive, so the ensure step
		// (ticket 03) short-circuits and only the cicustom PUT happens.
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"ide3":"local-lvm:vm-101-cloudinit,media=cdrom"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", recordCloudInitConfigForm(t, seen))
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.AttachCloudInitSnippet(context.Background(), testNodeName, "local", filename, testVMID); err != nil {
		t.Fatalf("AttachCloudInitSnippet: %v", err)
	}

	if seen[wantKey] != wantVal {
		t.Errorf("%s = %q, want %q (seen %+v)", wantKey, seen[wantKey], wantVal, seen)
	}
}

func recordCloudInitConfigForm(t *testing.T, seen map[string]string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		for _, k := range []string{"cicustom", "delete"} {
			if v := r.FormValue(k); v != "" {
				seen[k] = v
			}
		}

		writeJSONFixture(t, w, `{"data":null}`)
	}
}

// TestProxmox_AttachCloudInitSnippet_EnsuresDriveFirst is the ticket-03
// regression test: without a cloud-init drive, Proxmox silently ignores
// cicustom, so the attach must provision the drive first — exactly like
// SetCloudInitConfig does. A detach (empty filename) needs no drive.
func TestProxmox_AttachCloudInitSnippet_EnsuresDriveFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		filename   string
		configData string
		wantPuts   int
		wantFirst  string
	}{
		{
			name:       "VM without a drive gets one before the attach",
			filename:   testSnippetFilename,
			configData: `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`,
			wantPuts:   2,
			wantFirst:  "ide3",
		},
		{
			name:       "VM with a drive attaches directly",
			filename:   testSnippetFilename,
			configData: `{"data":{"ide3":"local-lvm:vm-101-cloudinit,media=cdrom"}}`,
			wantPuts:   1,
		},
		{
			name:       "detach never provisions a drive",
			filename:   "",
			configData: `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`,
			wantPuts:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var putKeys []string

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
					writeJSONFixture(t, w, tc.configData)
				})
				mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", recordConfigPut(t, &putKeys))
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			if err := p.AttachCloudInitSnippet(context.Background(), testNodeName, "local", tc.filename, testVMID); err != nil {
				t.Fatalf("AttachCloudInitSnippet: %v", err)
			}

			if len(putKeys) != tc.wantPuts {
				t.Fatalf("PUT sequence = %+v, want %d writes", putKeys, tc.wantPuts)
			}

			if tc.wantFirst != "" && putKeys[0] != tc.wantFirst {
				t.Errorf("first PUT = %q, want %q (drive before cicustom)", putKeys[0], tc.wantFirst)
			}
		})
	}
}

// recordConfigPut returns a PUT /config handler that records the ordered
// sequence of config keys the attach loop writes (ide3, cicustom, or delete).
// Extracted from TestProxmox_AttachCloudInitSnippet_EnsuresDriveFirst to keep
// its cognitive complexity under the go:S3776 limit.
func recordConfigPut(t *testing.T, putKeys *[]string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		switch {
		case r.FormValue("ide3") != "":
			*putKeys = append(*putKeys, "ide3")
		case r.FormValue("cicustom") != "":
			*putKeys = append(*putKeys, "cicustom")
		case r.FormValue("delete") != "":
			*putKeys = append(*putKeys, "delete")
		}

		writeJSONFixture(t, w, `{"data":null}`)
	}
}

// TestProxmox_SetCloudInitPassword_Agent verifies the password is applied via
// the guest-agent endpoint (writes /etc/shadow), never cipassword (REPORT.md §1),
// and that the username is the caller-resolved ciuser — never a hardcoded root
// (ticket 02: a cloud image's root is locked).
func TestProxmox_SetCloudInitPassword_Agent(t *testing.T) {
	t.Parallel()

	var gotPath, gotUser, gotPassword string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/agent/set-user-password", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotPath = r.URL.Path
			gotUser = r.FormValue("username")
			gotPassword = r.FormValue("password")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetCloudInitPassword(context.Background(), testNodeName, testVMID, "debian", "s3cret"); err != nil {
		t.Fatalf("SetCloudInitPassword: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/set-user-password" {
		t.Errorf("path = %q, want agent/set-user-password", gotPath)
	}

	if gotUser != "debian" || gotPassword != "s3cret" {
		t.Errorf("username/password = %q/%q, want debian/s3cret", gotUser, gotPassword)
	}
}

// TestProxmox_SetCloudInitPassword_UserUnknown verifies that the guest agent's
// "user does not exist" rejection maps to the retryable ErrGuestUserUnknown
// sentinel (ticket 05: cloud-init creates the account mid-boot).
func TestProxmox_SetCloudInitPassword_UserUnknown(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/agent/set-user-password", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data":null,"errors":"user 'debian' does not exist"}`))
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.SetCloudInitPassword(context.Background(), testNodeName, testVMID, "debian", "s3cret")
	if !errors.Is(err, ErrGuestUserUnknown) {
		t.Fatalf("err = %v, want ErrGuestUserUnknown", err)
	}
}

// TestProxmox_PingGuestAgent verifies the probe hits the agent ping endpoint.
func TestProxmox_PingGuestAgent(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/agent/ping", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":{}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.PingGuestAgent(context.Background(), testNodeName, testVMID); err != nil {
		t.Fatalf("PingGuestAgent: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/ping" {
		t.Errorf("path = %q, want agent/ping", gotPath)
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

// TestProxmox_AddSSHKey_AgentExec verifies the key is passed as a positional
// argv to a fixed script (no shell interpolation), so a crafted key cannot
// break out, and that the guest's exit status is honoured (REPORT.md §2/#2).
//
//nolint:wsl_v5 // test keeps exec/exec-status handlers adjacent
func TestProxmox_AddSSHKey_AgentExec(t *testing.T) {
	t.Parallel()

	var gotCommands []string
	var gotExit int

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/agent/exec", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			gotCommands = r.Form["command"]
			writeJSONFixture(t, w, `{"data":{"pid":4242}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/agent/exec-status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"exited":true,"exitcode":`+strconv.Itoa(gotExit)+`}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	// Success path: positional argv must be [/bin/sh, -c, <script>, user, key].
	gotExit = 0
	if err := p.AddSSHKey(context.Background(), testNodeName, testVMID, FakeCloudInitUser, "ssh-ed25519 AAAA demo@laptop"); err != nil {
		t.Fatalf("AddSSHKey: %v", err)
	}
	if len(gotCommands) != 5 || gotCommands[0] != "/bin/sh" || gotCommands[1] != "-c" || gotCommands[3] != FakeCloudInitUser || gotCommands[4] != "ssh-ed25519 AAAA demo@laptop" {
		t.Fatalf("agent argv = %+v, want [/bin/sh -c <script> debian <key>]", gotCommands)
	}
	// The key must travel as a bare argv element, never interpolated into the
	// script body (which is the constant sshKeyAddScript).
	if strings.Contains(gotCommands[2], "demo@laptop") {
		t.Fatal("key was interpolated into the script body instead of passed as argv")
	}

	// Exit code 3 => user not found on guest.
	gotExit = 3
	if err := p.AddSSHKey(context.Background(), testNodeName, testVMID, "ghost", "ssh-ed25519 AAAA x"); !errors.Is(err, ErrSSHKeyUserUnknown) {
		t.Fatalf("err = %v, want ErrSSHKeyUserUnknown", err)
	}

	// Any other non-zero exit surfaces as a failure.
	gotExit = 1
	if err := p.AddSSHKey(context.Background(), testNodeName, testVMID, FakeCloudInitUser, "ssh-ed25519 AAAA x"); err == nil {
		t.Fatal("expected a failure for non-zero guest exit")
	}
}

// readMultipartFileBody reads the entire contents of a multipart file into a
// string. Extracted from the TestProxmox_PushCloudInitSnippet handler closure
// to satisfy the cognitive-complexity ceiling (go:S3776); read logic unchanged.
func readMultipartFileBody(t *testing.T, file multipart.File) string {
	t.Helper()

	var body strings.Builder

	buf := make([]byte, 512)

	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			body.Write(buf[:n])
		}

		if readErr != nil {
			break
		}
	}

	return body.String()
}

// withAgentExecTiming temporarily shortens the agent-exec polling constants so
// tests can exercise the ticker/deadline loop in milliseconds instead of real
// seconds. It restores the originals on cleanup. The waitAgentExec tests are
// NOT parallel because they mutate these package-level vars.
func withAgentExecTiming(t *testing.T, poll, wait time.Duration) {
	t.Helper()

	prevPoll, prevWait := agentExecPoll, maxAgentExecWait
	agentExecPoll = poll
	maxAgentExecWait = wait

	t.Cleanup(func() {
		agentExecPoll = prevPoll
		maxAgentExecWait = prevWait
	})
}

// newAgentExecStatusServer builds a test Proxmox server whose exec-status
// handler delegates to respond, passing the 1-based request count so callers
// can vary the response per poll. The returned counter lets callers assert the
// request bound.
func newAgentExecStatusServer(t *testing.T, respond func(count int) string) (*httptest.Server, *int32) {
	t.Helper()

	var count int32

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/agent/exec-status", func(w http.ResponseWriter, _ *http.Request) {
			n := atomic.AddInt32(&count, 1)
			writeJSONFixture(t, w, respond(int(n)))
		})
	})

	return srv, &count
}

// TestProxmox_WaitAgentExec_EventualCompletion verifies the loop polls until
// the guest reports completion: the first few polls return "running", then one
// returns exited with exit code 0. The request count equals the number of
// polls actually made (no busy-loop hammering).
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_EventualCompletion(t *testing.T) {
	withAgentExecTiming(t, 5*time.Millisecond, 200*time.Millisecond)

	srv, countPtr := newAgentExecStatusServer(t, func(n int) string {
		if n < 3 {
			return `{"data":{"exited":false}}`
		}

		return `{"data":{"exited":true,"exitcode":0}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	start := time.Now()

	if err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242); err != nil {
		t.Fatalf("waitAgentExec: %v", err)
	}

	elapsed := time.Since(start)

	if got := atomic.LoadInt32(countPtr); got != 3 {
		t.Errorf("request count = %d, want 3 (first two running, third completed)", got)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want well under the 200ms deadline", elapsed)
	}
}

// TestProxmox_WaitAgentExec_Timeout verifies that when the guest never reports
// completion, the loop gives up after maxAgentExecWait with the named timeout
// error and a request count bounded by maxAgentExecWait/agentExecPoll (not
// hundreds).
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_Timeout(t *testing.T) {
	withAgentExecTiming(t, 5*time.Millisecond, 60*time.Millisecond)

	srv, countPtr := newAgentExecStatusServer(t, func(_ int) string {
		return `{"data":{"exited":false}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "guest agent did not report exec completion") {
		t.Errorf("err = %q, want the named timeout error mentioning qemu-guest-agent", err.Error())
	}

	// maxAgentExecWait/agentExecPoll = 12, plus the immediate first poll =>
	// ~13. Allow jitter but reject a busy loop (hundreds).
	if got := atomic.LoadInt32(countPtr); got > 20 {
		t.Errorf("request count = %d, want bounded by ~maxAgentExecWait/agentExecPoll (not hundreds)", got)
	}
}

// TestProxmox_WaitAgentExec_ExitCodes preserves the exit-code mapping: 0 is
// success, 3 is ErrSSHKeyUserUnknown, any other non-zero surfaces the guest
// stderr. These resolve on the immediate first poll so timing is irrelevant.
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_ExitCodes(t *testing.T) {
	withAgentExecTiming(t, 50*time.Millisecond, time.Second)

	cases := []struct {
		name    string
		exit    int
		wantErr error
		wantMsg string
	}{
		{"exit 0 is success", 0, nil, ""},
		{"exit 3 is user unknown", 3, ErrSSHKeyUserUnknown, ""},
		{"exit 1 surfaces stderr", 1, nil, "guest agent ssh-key add failed (exit 1)"},
	}

	for _, tc := range cases {
		//nolint:paralleltest // serial: parent mutates package-level timing vars
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newAgentExecStatusServer(t, func(_ int) string {
				return `{"data":{"exited":true,"exitcode":` + strconv.Itoa(tc.exit) + `,"err-data":"boom"}}`
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			err := p.waitAgentExec(context.Background(), testNodeName, testVMID, 4242)
			assertAgentExecError(t, err, tc.wantErr, tc.wantMsg)
		})
	}
}

// assertAgentExecError checks the waitAgentExec error against the expected
// sentinel, message substring, or nil. Extracted from
// TestProxmox_WaitAgentExec_ExitCodes to keep its cognitive complexity under
// go:S3776's ceiling.
func assertAgentExecError(t *testing.T, err, wantErr error, wantMsg string) {
	t.Helper()

	switch {
	case wantErr != nil:
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	case wantMsg != "":
		if err == nil || !strings.Contains(err.Error(), wantMsg) {
			t.Fatalf("err = %v, want message containing %q", err, wantMsg)
		}
	default:
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	}
}

// TestProxmox_WaitAgentExec_ContextCancelled verifies that cancelling the
// context mid-loop terminates promptly (within one poll interval) rather than
// waiting for the deadline.
//
//nolint:paralleltest // serial: mutates package-level agentExecPoll/maxAgentExecWait
func TestProxmox_WaitAgentExec_ContextCancelled(t *testing.T) {
	withAgentExecTiming(t, 20*time.Millisecond, 5*time.Second)

	srv, _ := newAgentExecStatusServer(t, func(_ int) string {
		return `{"data":{"exited":false}}`
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := p.waitAgentExec(ctx, testNodeName, testVMID, 4242)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, want prompt cancellation (well under the 5s deadline)", elapsed)
	}
}
