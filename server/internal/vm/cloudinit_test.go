//nolint:wsl_v5 // domain tests keep ordered state transitions and assertions together
package vm_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

type testRefresher struct{}

func (testRefresher) Refresh(context.Context) (time.Time, error) { return time.Now(), nil }

// Fixture constants centralised to satisfy goconst (min-occurrences 4).
const (
	// testPasswordValue is the throwaway password the password-path tests
	// apply through the guest agent.
	testPasswordValue = "hunter2-change-me"
	// testActionSetCloudInitPassword is the fake's agent password-apply action.
	testActionSetCloudInitPassword = "set_cloudinit_password"
	// testStorageLocal is the fake-local storage name the fixtures share.
	testStorageLocal = "local"
	// testUserUbuntu is the cloud-init user the fixtures share.
	testUserUbuntu = "ubuntu"
	// testActionCreate is the fake's VM-create action name.
	testActionCreate = "create"
	// testActionAttachCloudInitSnippet is the fake's snippet-attach action name.
	testActionAttachCloudInitSnippet = "attach_cloudinit_snippet"
	// testActionStart is the fake's VM-start action name.
	testActionStart = "start"
	// testBIOSOVMF is the Proxmox bios value selecting UEFI firmware.
	testBIOSOVMF = "ovmf"
)

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
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "cloudinit.db"), LogLevel: testLogLevel, LogFormat: testLogFormat, LogOutput: testLogOutput})
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
	if config.User != cluster.FakeCloudInitUser || config.IPMode != cluster.CloudInitIPModeStatic || config.Password != "" {
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
	user := testUserUbuntu
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
	if len(calls) != 2 || calls[0].Action != "ensure_cloudinit_drive" || calls[1].Action != testActionSetCloudInitConfig {
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
	// The fake now rejects reboot on a stopped VM. VM 101 is stopped in
	// the pristine dataset - start it first so the reboot succeeds.
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}
	user := testUserUbuntu
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

// documentDeps builds SetCloudInitDocument deps for VM 101 (alice).
func documentDeps(index *inventory.Index, st *store.Store) vm.CloudInitDocumentDeps {
	return vm.CloudInitDocumentDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName,
		VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st,
	}
}

// TestSetCloudInitDocument_SwitchesToPublishedTemplate - switching attaches
// the template's published file, records it, audits, and writes nothing.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitDocument_SwitchesToPublishedTemplate(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	tmplID := createTestTemplate(t, st)
	filename := publishedFilename(t, st, tmplID)

	if err := vm.SetCloudInitDocument(context.Background(), documentDeps(index, st), tmplID); err != nil {
		t.Fatalf("SetCloudInitDocument: %v", err)
	}

	if got := attachedFilename(t, 101); got != filename {
		t.Errorf("attached %q, want %q", got, filename)
	}

	doc, found, err := vm.GetCloudInitDocument(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, st)
	if err != nil || !found || doc.TemplateID != tmplID || doc.Filename != filename || doc.Legacy {
		t.Fatalf("document = %+v/%v/%v", doc, found, err)
	}

	entries, err := st.QueryAudit(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Action != "edit_cloudinit_document" {
		t.Fatalf("audit = %+v, err %v", entries, err)
	}
}

// TestSetCloudInitDocument_EmptyDetaches - "" clears cicustom and forgets
// the row; the shared file is never removed.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitDocument_EmptyDetaches(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	tmplID := createTestTemplate(t, st)

	if err := vm.SetCloudInitDocument(context.Background(), documentDeps(index, st), tmplID); err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := vm.SetCloudInitDocument(context.Background(), documentDeps(index, st), ""); err != nil {
		t.Fatalf("detach: %v", err)
	}

	if got := attachedFilename(t, 101); got != "" {
		t.Errorf("last attach = %q, want a detach", got)
	}

	if _, found, _ := vm.GetCloudInitDocument(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, st); found {
		t.Error("document row survived the detach")
	}

	for _, c := range cluster.FakeCalls() {
		if c.Action == "remove_cloudinit_snippet" {
			t.Fatalf("detach removed a shared file: %+v", c)
		}
	}
}

// TestSetCloudInitDocument_ReplacesLegacyPerVMFile - a VM that used a legacy
// per-VM document gets the published template; the legacy file and row go.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitDocument_ReplacesLegacyPerVMFile(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	tmplID := createTestTemplate(t, st)

	if err := st.PutCloudInitSnippet(context.Background(), testClusterName, 101, testStorageLocal, "pvmss-101.yml", "#cloud-config\n", "alice"); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	legacy, found, err := vm.GetCloudInitDocument(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, st)
	if err != nil || !found || !legacy.Legacy {
		t.Fatalf("legacy document = %+v/%v/%v", legacy, found, err)
	}

	if err := vm.SetCloudInitDocument(context.Background(), documentDeps(index, st), tmplID); err != nil {
		t.Fatalf("SetCloudInitDocument: %v", err)
	}

	removed := slices.ContainsFunc(cluster.FakeCalls(), func(c cluster.FakeCall) bool {
		return c.Action == "remove_cloudinit_snippet" && c.Filename == "pvmss-101.yml"
	})
	if !removed {
		t.Error("legacy per-VM file not removed")
	}

	if _, found, _ := st.GetCloudInitSnippet(context.Background(), testClusterName, 101); found {
		t.Error("legacy row survived")
	}
}

// TestSetCloudInitDocument_Refusals - unknown template, never published, not
// on the VM's node, and a foreign VM are refused before any attach.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitDocument_Refusals(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, st *store.Store) string
		actor   auth.Identity
		wantErr error
	}{
		{name: "unknown", setup: func(*testing.T, *store.Store) string { return "nope" }, actor: cloudAliceIdentity(), wantErr: vm.ErrNotApproved},
		{name: "never published", setup: func(t *testing.T, st *store.Store) string {
			t.Helper()

			tmpl, err := catalog.CreateCloudInitTemplate(context.Background(), st, testClusterName, "Draft", testCloudInitContent)
			if err != nil {
				t.Fatalf("create: %v", err)
			}

			return tmpl.ID
		}, actor: cloudAliceIdentity(), wantErr: vm.ErrCloudInitNotPublished},
		{name: "not on node", setup: func(t *testing.T, st *store.Store) string {
			t.Helper()
			cluster.SetFakeSnippetVisibility(false)
			t.Cleanup(func() { cluster.SetFakeSnippetVisibility(true) })

			return createTestTemplate(t, st)
		}, actor: cloudAliceIdentity(), wantErr: vm.ErrCloudInitNotPublished},
		{name: "foreign VM", setup: createTestTemplate, actor: auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}, wantErr: vm.ErrForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := cloudInitIndex(t)
			st := cloudInitStore(t)
			id := tc.setup(t, st)

			deps := documentDeps(index, st)
			deps.Actor = tc.actor

			if err := vm.SetCloudInitDocument(context.Background(), deps, id); !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}

			if got := attachedFilename(t, 101); got != "" {
				t.Fatalf("refused switch attached %q", got)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_RejectsMalformedSSHKeys(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	// A pasted multi-line block must be rejected before it ever reaches
	// Proxmox, where it would smuggle extra keys into authorized_keys.
	// The structured config write must not happen.
	keys := []string{"ssh-ed25519 AAAA-good\nssh-ed25519 AAAA-smuggled"}
	_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{}}, cluster.CloudInitUpdate{SSHKeys: &keys}, false)
	if err == nil {
		t.Fatal("SetCloudInitConfig accepted a multi-line ssh key, want rejection")
	}
	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == testActionSetCloudInitConfig {
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
func TestAddCloudInitSSHKey_ValidatesAndAudits(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	bad := "ssh-rsa AAAA-bad\ninjected"
	assertBadKeyRejected(t, index, st, bad)

	key := "ssh-ed25519 AAAA-injected demo@laptop"
	assertValidKeyInjectedAndAudited(t, index, st, key)
	assertKeyReadableBack(t, index, key)
}

// assertBadKeyRejected verifies that a malformed key is rejected before the
// agent is ever called, so the injected key never reaches authorized_keys.
func assertBadKeyRejected(t *testing.T, index *inventory.Index, st *store.Store, bad string) {
	t.Helper()

	if err := vm.AddCloudInitSSHKey(context.Background(), vm.AddCloudInitSSHKeyDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st}, cluster.FakeCloudInitUser, bad); !errors.Is(err, vm.ErrSSHKeyInvalid) {
		t.Fatalf("err = %v, want ErrSSHKeyInvalid", err)
	}

	if len(cluster.FakeCallsFor(101)) != 0 {
		t.Fatalf("bad key reached the fake: %+v", cluster.FakeCallsFor(101))
	}
}

// assertValidKeyInjectedAndAudits verifies that a valid key is injected via the
// guest agent, merged into the cloud-init config, and the action is audited.
func assertValidKeyInjectedAndAudited(t *testing.T, index *inventory.Index, st *store.Store, key string) {
	t.Helper()

	if err := vm.AddCloudInitSSHKey(context.Background(), vm.AddCloudInitSSHKeyDeps{Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st}, cluster.FakeCloudInitUser, key); err != nil {
		t.Fatalf("AddCloudInitSSHKey: %v", err)
	}

	var sawAgent, sawCfgSync bool

	for _, c := range cluster.FakeCallsFor(101) {
		switch c.Action {
		case "add_ssh_key":
			sawAgent = true

			if c.Content != key || c.Name != cluster.FakeCloudInitUser {
				t.Errorf("add_ssh_key call = %+v", c)
			}
		case testActionSetCloudInitConfig:
			sawCfgSync = true
		}
	}

	if !sawAgent {
		t.Fatal("expected an add_ssh_key agent call")
	}

	if !sawCfgSync {
		t.Fatal("expected the key to be merged into the cloud-init config")
	}

	entries, err := st.QueryAudit(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Action != "edit_cloudinit_sshkey" {
		t.Fatalf("audit = %+v, err %v", entries, err)
	}
}

// assertKeyReadableBack verifies the merged key is readable through
// GetCloudInitConfig (the agent call is best-effort; the structured config is
// the source of truth).
func assertKeyReadableBack(t *testing.T, index *inventory.Index, key string) {
	t.Helper()

	got, gerr := vm.GetCloudInitConfig(context.Background(), index, cloudAliceIdentity(), testClusterName, 101, cluster.Fake{})
	if gerr != nil {
		t.Fatalf("GetCloudInitConfig after inject: %v", gerr)
	}

	if !slices.Contains(got.SSHKeys, key) {
		t.Fatalf("injected key not present in config after AddCloudInitSSHKey: %+v", got.SSHKeys)
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
	password := testPasswordValue
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
		case testActionSetCloudInitConfig:
			sawConfig = true
			if c.CloudInitData.Password != "" {
				t.Errorf("config carried a cleartext password: %+v", c.CloudInitData)
			}
		case testActionSetCloudInitPassword:
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

// TestSetCloudInitConfig_RejectsNonIPv4StaticAddresses.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_RejectsNonIPv4StaticAddresses(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	cases := []struct {
		name      string
		ipAddress string
		gateway   string
	}{
		{"IPv6 address", "2001:db8::1/64", "2001:db8::1"},
		{"gateway as IPv6", "10.0.0.1/24", "2001:db8::1"},
		{"missing CIDR prefix", "10.0.0.1", "10.0.0.254"},
		{"out of range octet", "10.0.0.256/24", "10.0.0.254"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := cluster.CloudInitIPModeStatic
			_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
				Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
				Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
			}, cluster.CloudInitUpdate{
				IPAddress: &tc.ipAddress, Gateway: &tc.gateway, IPMode: &mode,
			}, false)
			if !errors.Is(err, vm.ErrInvalidCloudInitConfig) {
				t.Fatalf("error = %v, want ErrInvalidCloudInitConfig", err)
			}
		})
	}
}

// TestSetCloudInitConfig_PasswordUsesResolvedCiUser is the
// regression test: the password lands on the VM's own ciuser read from the
// live config - never a hardcoded root (a cloud image's root is locked).
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordUsesResolvedCiUser(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}

	password := testPasswordValue
	if _, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
	}, cluster.CloudInitUpdate{Password: &password}, false); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	assertPasswordAppliedTo(t, cluster.FakeCloudInitUser)
}

// TestSetCloudInitConfig_PasswordPrefersPatchUser verifies the resolution
// order: a patch that changes ciuser AND sets a password applies the password
// to the NEW user, not the previous one.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordPrefersPatchUser(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}

	password := testPasswordValue
	user := testUserUbuntu
	if _, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
	}, cluster.CloudInitUpdate{User: &user, Password: &password}, false); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	assertPasswordAppliedTo(t, user)
}

// TestSetCloudInitConfig_PasswordWithoutUser_Refused verifies the no-fallback
// Neither the patch nor the live config defines a ciuser,
// so the password is refused with ErrNoCloudInitUser instead of being applied
// to a guessed account.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordWithoutUser_Refused(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	// A freshly created fake VM carries agent=1 (mirroring the real create
	// path) but no ciuser until one is written.
	vmid := createBareFakeVM(t, index)

	password := testPasswordValue
	_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: vmid,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
	}, cluster.CloudInitUpdate{Password: &password}, false)
	if !errors.Is(err, vm.ErrNoCloudInitUser) {
		t.Fatalf("error = %v, want ErrNoCloudInitUser", err)
	}

	for _, c := range cluster.FakeCallsFor(vmid) {
		if c.Action == testActionSetCloudInitPassword {
			t.Fatalf("a password was applied without a ciuser: %+v", c)
		}
	}
}

// TestSetCloudInitConfig_PasswordAgentDisabled_Refused verifies the immediate pre-flight
// refusal when agent= is absent from the VM config: no
// agent call is emitted and the error is actionable, not opaque.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordAgentDisabled_Refused(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	if err := (cluster.Fake{}).Action(context.Background(), cluster.FakeNode01, 101, "start"); err != nil {
		t.Fatalf("start VM 101 for test setup: %v", err)
	}

	password := testPasswordValue
	_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: agentDisabledReader{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
	}, cluster.CloudInitUpdate{Password: &password}, false)
	if !errors.Is(err, vm.ErrGuestAgentDisabled) {
		t.Fatalf("error = %v, want ErrGuestAgentDisabled", err)
	}

	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == "ping_guest_agent" || c.Action == testActionSetCloudInitPassword {
			t.Fatalf("an agent call was emitted with the agent disabled: %+v", c)
		}
	}
}

// TestSetCloudInitConfig_PasswordVMStopped_Refused verifies the pre-flight against the LIVE
// status: a stopped VM is refused with
// ErrVMNotRunning before any agent call.
//
//nolint:paralleltest // serial: shared fake dataset
func TestSetCloudInitConfig_PasswordVMStopped_Refused(t *testing.T) {
	index := cloudInitIndex(t)
	st := cloudInitStore(t)

	password := testPasswordValue
	_, err := vm.SetCloudInitConfig(context.Background(), vm.CloudInitConfigDeps{
		Index: index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Reader: cluster.Fake{}, Writer: cluster.Fake{}, Audit: st, Refresher: testRefresher{},
		StatusReader: stoppedStatusReader{},
	}, cluster.CloudInitUpdate{Password: &password}, false)
	if !errors.Is(err, vm.ErrVMNotRunning) {
		t.Fatalf("error = %v, want ErrVMNotRunning", err)
	}

	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == "ping_guest_agent" || c.Action == testActionSetCloudInitPassword {
			t.Fatalf("an agent call was emitted on a stopped VM: %+v", c)
		}
	}
}

// assertPasswordAppliedTo checks the fake recorded the agent password apply
// for exactly the given user.
func assertPasswordAppliedTo(t *testing.T, user string) {
	t.Helper()

	var sawAgent bool

	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action != testActionSetCloudInitPassword {
			continue
		}

		sawAgent = true

		if c.Name != user {
			t.Fatalf("password applied to %q, want %q", c.Name, user)
		}
	}

	if !sawAgent {
		t.Fatal("expected a set_cloudinit_password agent call")
	}
}

// createBareFakeVM registers a fresh VM in the fake dataset and returns its
// VMID. The fake's CreateVM seeds agent=1 (mirroring the real create path)
// but no ciuser.
func createBareFakeVM(t *testing.T, index *inventory.Index) int {
	t.Helper()

	vmid := 300
	if _, err := (cluster.Fake{}).CreateVM(context.Background(), cluster.VMSpec{
		VMID: vmid, Node: cluster.FakeNode01, Name: "bare-vm", Pool: cluster.FakePoolAlice,
		Tags: []string{"pvmss"},
	}); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	// Rebuild the index so the new VM is resolvable through the ownership gate.
	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	fresh := inventory.BuildIndex(snapshot)
	fresh.RefreshedAt = time.Now()
	*index = fresh

	return vmid
}

// agentDisabledReader hides the agent flag so the pre-flight sees an
// agent-less VM (the fake's seeded configs carry agent=1).
type agentDisabledReader struct{ cluster.Fake }

func (r agentDisabledReader) GetCloudInitConfig(ctx context.Context, node string, vmid int) (cluster.CloudInitConfig, error) {
	config, err := r.Fake.GetCloudInitConfig(ctx, node, vmid)
	config.Agent = false

	return config, err
}

// stoppedStatusReader reports the VM as stopped regardless of fake state.
type stoppedStatusReader struct{}

func (stoppedStatusReader) VMStatus(context.Context, string, int) (cluster.VMLiveStatus, error) {
	return cluster.VMLiveStatus{Status: cluster.VMStopped}, nil
}
