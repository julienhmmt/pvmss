package cluster

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

	err := p.PushCloudInitSnippet(context.Background(), testNodeName, "local", "pvmss-101.yml", testVMID, "#cloud-config\n")
	if err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}

	if gotContentField != "snippets" {
		t.Errorf("content field = %q, want snippets", gotContentField)
	}

	if gotFilename != "pvmss-101.yml" {
		t.Errorf("filename = %q", gotFilename)
	}

	if gotFileBody != "#cloud-config\n" {
		t.Errorf("file body = %q", gotFileBody)
	}
}

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

// TestProxmox_AttachCloudInitSnippet verifies the snippet is wired to the VM
// via cicustom=vendor= (MERGE semantics, so generated user-data is preserved)
// when a filename is given, and that an empty filename clears the cicustom key
// instead of setting it. Fixes the silent no-op reported in REPORT.md §4/addendum.
//
//nolint:wsl_v5 // test table keeps brace groups tight
func TestProxmox_AttachCloudInitSnippet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		wantKey  string
		wantVal  string
	}{
		{name: "vendor slot merges snippet", filename: "pvmss-101.yml", wantKey: "cicustom", wantVal: "vendor=local:snippets/pvmss-101.yml"},
		{name: "empty filename detaches", filename: "", wantKey: "delete", wantVal: "cicustom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			seen := map[string]string{}

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("parse form: %v", err)
					}
					for _, k := range []string{"cicustom", "delete"} {
						if v := r.FormValue(k); v != "" {
							seen[k] = v
						}
					}
					writeJSONFixture(t, w, `{"data":null}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			if err := p.AttachCloudInitSnippet(context.Background(), testNodeName, "local", tc.filename, testVMID); err != nil {
				t.Fatalf("AttachCloudInitSnippet: %v", err)
			}

			if seen[tc.wantKey] != tc.wantVal {
				t.Errorf("%s = %q, want %q (seen %+v)", tc.wantKey, seen[tc.wantKey], tc.wantVal, seen)
			}
		})
	}
}

// TestProxmox_SetCloudInitPassword_Agent verifies the password is applied via
// the guest-agent endpoint (writes /etc/shadow), never cipassword (REPORT.md §1).
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

	if err := p.SetCloudInitPassword(context.Background(), testNodeName, testVMID, "s3cret"); err != nil {
		t.Fatalf("SetCloudInitPassword: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/set-user-password" {
		t.Errorf("path = %q, want agent/set-user-password", gotPath)
	}

	if gotUser != "root" || gotPassword != "s3cret" {
		t.Errorf("username/password = %q/%q, want root/s3cret", gotUser, gotPassword)
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
