//nolint:wsl_v5 // domain tests keep ordered state transitions and assertions together
package vm_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

type testRefresher struct{}

func (testRefresher) Refresh(context.Context) (time.Time, error) { return time.Now(), nil }

func cloudInitIndex(t *testing.T) *inventory.Index {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	index.RefreshedAt = time.Now()
	return &index
}

func cloudInitStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "cloudinit.db"), LogLevel: "info", LogFormat: "json", LogOutput: "stdout"})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func cloudAliceIdentity() auth.Identity {
	return auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
}

//nolint:paralleltest // serial: shared fake dataset
func TestGetCloudInitConfig_OwnerAndForbidden(t *testing.T) {
	index := cloudInitIndex(t)
	config, err := vm.GetCloudInitConfig(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, cluster.Fake{})
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}
	if config.User != "debian" || config.IPMode != cluster.CloudInitIPModeStatic || config.Password != "" {
		t.Fatalf("config = %+v", config)
	}

	_, err = vm.GetCloudInitConfig(context.Background(), index, auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}, testClusterName, 101, cluster.Fake{})
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("non-owner error = %v, want ErrForbidden", err)
	}
}

//nolint:gocyclo,paralleltest // ordered test uses shared fake dataset and audit store
func TestSetCloudInitConfig_MergesDHCPAuditsAndDoesNotReboot(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	user := "ubuntu"
	mode := cluster.CloudInitIPModeDHCP
	keys := []string{"ssh-ed25519 AAAA-new"}
	rebooted, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{User: &user, IPMode: &mode, SSHKeys: &keys}, false)
	if err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}
	if rebooted {
		t.Fatal("rebooted = true, want false")
	}
	got, err := vm.GetCloudInitConfig(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, cluster.Fake{})
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if got.User != user || got.IPMode != mode || got.IPAddress != "" || got.Gateway != "" || len(got.SSHKeys) != 1 {
		t.Fatalf("updated config = %+v", got)
	}
	calls := cluster.FakeCallsFor(101)
	if len(calls) != 2 || calls[0].Action != "ensure_cloudinit_drive" || calls[1].Action != "set_cloudinit_config" {
		t.Fatalf("calls = %+v, want ensure then set", calls)
	}
	entries, err := st.QueryAudit(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Action != "edit_cloudinit_config" || entries[0].Actor != cluster.FakeUserAlice {
		t.Fatalf("audit = %+v, err %v", entries, err)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_RebootNowCallsT05Once(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	// T001b: the fake now rejects reboot on a stopped VM. VM 101 is stopped in
	// the pristine dataset — start it first so the reboot succeeds.
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}
	user := "ubuntu"
	rebooted, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{User: &user}, true)
	if err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}
	if !rebooted {
		t.Fatal("rebooted = false, want true")
	}
	var rebootCalls int
	for _, call := range cluster.FakeCallsFor(101) {
		if call.Action == "reboot" {
			rebootCalls++
		}
	}
	if rebootCalls != 1 {
		t.Fatalf("reboot calls = %d, want 1; calls = %+v", rebootCalls, cluster.FakeCallsFor(101))
	}
}

//nolint:paralleltest,gocyclo // serial: shared fake dataset; table of assertions
func TestSetCloudInitSnippet_PersistsTargetPushesAndPreservesClear(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	service := policy.New(st, inventory.NewProjectionFromIndex(index), cluster.Fake{})
	content := "#cloud-config\nusers: {}\n"
	if err := vm.SetCloudInitSnippet(context.Background(), vm.CloudInitSnippetDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st, Service: service}, content); err != nil {
		t.Fatalf("SetCloudInitSnippet: %v", err)
	}
	calls := cluster.FakeCallsFor(101)
	if len(calls) != 2 || calls[0].Action != "push_cloudinit_snippet" || calls[0].Storage != cluster.FakeSnippetStorage || calls[0].Filename != "pvmss-101.yml" || calls[0].Content != content {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[1].Action != "attach_cloudinit_snippet" || calls[1].Storage != cluster.FakeSnippetStorage || calls[1].Filename != "pvmss-101.yml" {
		t.Fatalf("attach call = %+v, want attach of pvmss-101.yml", calls[1])
	}
	if err := vm.SetCloudInitSnippet(context.Background(), vm.CloudInitSnippetDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st, Service: service}, ""); err != nil {
		t.Fatalf("clear snippet: %v", err)
	}
	snippet, found, err := st.GetCloudInitSnippet(context.Background(), testClusterName, 101)
	if err != nil || !found || snippet.Content != "" {
		t.Fatalf("cleared snippet = %+v, found %v, err %v", snippet, found, err)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitSnippet_ValidationAndPushFailure(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	service := policy.New(st, inventory.NewProjectionFromIndex(index), cluster.Fake{})
	if err := vm.SetCloudInitSnippet(context.Background(), vm.CloudInitSnippetDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st, Service: service}, "not yaml"); !errors.Is(err, cloudinit.ErrSnippetPrefix) {
		t.Fatalf("invalid error = %v, want ErrSnippetPrefix", err)
	}
	pushErr := errors.New("push unavailable")
	cluster.SetFakeCloudInitPushError(pushErr)
	err := vm.SetCloudInitSnippet(context.Background(), vm.CloudInitSnippetDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st, Service: service}, "#cloud-config\n")
	if !errors.Is(err, vm.ErrSnippetPushFailed) {
		t.Fatalf("push error = %v, want ErrSnippetPushFailed", err)
	}
	if _, found, readErr := st.GetCloudInitSnippet(context.Background(), testClusterName, 101); readErr != nil || !found {
		t.Fatalf("snippet after push failure found %v, err %v; want committed row", found, readErr)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_RejectsMalformedSSHKeys(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	// A pasted multi-line block must be rejected before it ever reaches
	// Proxmox, where it would smuggle extra keys into authorized_keys
	// (REPORT.md §2/#3). The structured config write must not happen.
	keys := []string{"ssh-ed25519 AAAA-good\nssh-ed25519 AAAA-smuggled"}
	_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{SSHKeys: &keys}, false)
	if err == nil {
		t.Fatal("SetCloudInitConfig accepted a multi-line ssh key, want rejection")
	}
	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == "set_cloudinit_config" {
			t.Fatalf("a multi-line ssh key reached the config write: %+v", c)
		}
	}

	// A well-formed key still passes validation.
	good := []string{"ssh-ed25519 AAAA-good-comment"}
	if _, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{SSHKeys: &good}, false); err != nil {
		t.Fatalf("SetCloudInitConfig rejected a valid ssh key: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordUsesGuestAgentNotCipassword(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	// VM 101 is stopped in the pristine dataset; the guest agent requires a
	// running guest, so start it first so the agent path can succeed.
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}
	password := "hunter2-change-me"
	rebooted, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{Password: &password}, false)
	if err != nil {
		t.Fatalf("SetCloudInitConfig with password: %v", err)
	}
	if rebooted {
		t.Fatal("rebooted = true, want false (rebootNow not requested)")
	}
	calls := cluster.FakeCallsFor(101)
	var sawConfig, sawAgent bool
	for _, c := range calls {
		switch c.Action {
		case "set_cloudinit_config":
			sawConfig = true
			if c.CloudInitData.Password != "" {
				t.Errorf("config carried a cleartext password: %+v", c.CloudInitData)
			}
		case "set_cloudinit_password":
			sawAgent = true
		}
	}
	if !sawConfig {
		t.Fatal("expected a set_cloudinit_config call")
	}
	if !sawAgent {
		t.Fatal("expected the password to be applied via the guest agent (set_cloudinit_password), not cipassword")
	}
}
