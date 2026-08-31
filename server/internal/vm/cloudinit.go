package vm

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
	"time"
)

const snippetFilenamePrefix = "pvmss-"

// wrapJoin wraps two errors using the "%w: %w" verb so both are matchable via
// errors.Is. Centralized to avoid duplicating the format literal.
func wrapJoin(sentinel, cause error) error {
	return fmt.Errorf("%w: %w", sentinel, cause)
}

// isIPv4Prefix reports whether s is a valid IPv4 CIDR prefix.
func isIPv4Prefix(s string) bool {
	prefix, err := netip.ParsePrefix(s)
	return err == nil && prefix.Addr().Is4()
}

// isIPv4Address reports whether s is a valid IPv4 host address.
func isIPv4Address(s string) bool {
	addr, err := netip.ParseAddr(s)
	return err == nil && addr.Is4()
}

var (
	// ErrInvalidCloudInitConfig reports malformed effective structured state.
	ErrInvalidCloudInitConfig = errors.New("invalid cloud-init config")
	// ErrSSHKeyInvalid reports a public key that failed cloudinit validation.
	ErrSSHKeyInvalid = errors.New("invalid ssh public key")
	// ErrSnippetPushFailed reports a committed snippet that was not applied upstream.
	ErrSnippetPushFailed = errors.New("cloud-init snippet push failed")
	// ErrCustomYAMLDisabled reports an administrator-disabled snippet editor.
	ErrCustomYAMLDisabled = errors.New("custom yaml disabled")
	// ErrNoCloudInitUser reports a password request on a VM whose patch and
	// live config define no ciuser. The password is refused, never applied to
	// a guessed account: a cloud image's root is locked, so a fallback to
	// root would silently write the password where nobody can log in
	// (ticket 02).
	ErrNoCloudInitUser = errors.New("no cloud-init user defined")
	// ErrGuestAgentDisabled reports agent= absent from the VM config — the
	// QEMU guest agent cannot answer, so the password cannot be applied
	// (ticket 05's immediate, actionable pre-flight refusal).
	ErrGuestAgentDisabled = errors.New("guest agent not enabled")
	// ErrVMNotRunning reports a password attempt on a VM that is not running.
	ErrVMNotRunning = errors.New("vm not running")
	// ErrGuestAgentUnreachable reports that the guest agent never answered
	// within the bounded wait, so the password was not applied (ticket 05).
	ErrGuestAgentUnreachable = errors.New("guest agent unreachable")
)

// GetCloudInitConfig reads live structured state after the shared ownership gate.
func GetCloudInitConfig(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, reader cluster.CloudInitReader) (cluster.CloudInitConfig, error) {
	entity, err := resolveCloudInitTarget(index, actor, clusterName, vmid)
	if err != nil {
		return cluster.CloudInitConfig{}, err
	}

	config, err := reader.GetCloudInitConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return cluster.CloudInitConfig{}, fmt.Errorf("read cloud-init config: %w", err)
	}

	return config, nil
}

// CloudInitConfigDeps groups the shared dependencies and resolution context
// for SetCloudInitConfig. It collapses the eleven positional parameters the
// function used to take (SonarQube go:S107).
type CloudInitConfigDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.CloudInitReader
	Writer      cluster.Writer
	Audit       AuditRecorder
	Refresher   IndexRefresher
	// StatusReader reads the VM's live power state for the password
	// pre-flight (ticket 05). Optional: when nil the running check is
	// skipped and the bounded agent ping is the only readiness gate.
	StatusReader cluster.VMStatusReader
}

// SetCloudInitConfig merges and writes a partial update, optionally using T05's reboot action.
func SetCloudInitConfig(ctx context.Context, deps CloudInitConfigDeps, update cluster.CloudInitUpdate, rebootNow bool) (bool, error) {
	index := deps.Index
	actor := deps.Actor
	clusterName := deps.ClusterName
	vmid := deps.VMID
	reader := deps.Reader
	writer := deps.Writer
	audit := deps.Audit
	refresher := deps.Refresher

	if err := validateCloudInitUpdate(update); err != nil {
		return false, err
	}

	entity, err := resolveCloudInitTarget(index, actor, clusterName, vmid)
	if err != nil {
		return false, err
	}

	current, err := reader.GetCloudInitConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return false, fmt.Errorf("read cloud-init config before update: %w", err)
	}

	effective, err := mergeCloudInitConfig(current, update)
	if err != nil {
		return false, err
	}

	if err := writer.SetCloudInitConfig(ctx, entity.Node, entity.VMID, effective); err != nil {
		return false, fmt.Errorf("write cloud-init config: %w", err)
	}

	// Apply the password via the QEMU guest agent (writes /etc/shadow only),
	// never through cipassword (whose crypt hash lands on the seed drive and
	// is readable by the tenant — REPORT.md §1). The pre-flight refuses
	// immediately with an actionable error when the agent is disabled or the
	// VM is not running; the bounded wait inside applyCloudInitPassword
	// covers the "running but still booting" case (ticket 05).
	if update.Password != nil && *update.Password != "" {
		if err := applyCloudInitPasswordFlow(ctx, deps, writer, entity, current, effective, *update.Password); err != nil {
			return false, err
		}
	}

	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, "edit_cloudinit_config"); err != nil {
		return false, fmt.Errorf("record cloud-init config audit: %w", err)
	}

	if !rebootNow {
		return false, nil
	}

	if err := Action(ctx, BulkDeps{Actor: actor, Writer: writer, Audit: audit, Refresher: refresher}, index, clusterName, vmid, "reboot"); err != nil {
		return false, fmt.Errorf("reboot after cloud-init update: %w", err)
	}

	return true, nil
}

// applyCloudInitPasswordFlow runs the whole password step: the pre-flight
// refusals, the ciuser resolution (patch value first, then the live config —
// never a fallback to a locked root, ticket 02), and the bounded agent wait.
func applyCloudInitPasswordFlow(ctx context.Context, deps CloudInitConfigDeps, writer cluster.Writer, entity Entity, current, effective cluster.CloudInitConfig, password string) error {
	if err := preflightGuestAgent(ctx, deps, entity, current); err != nil {
		return err
	}

	if effective.User == "" {
		return ErrNoCloudInitUser
	}

	if err := applyCloudInitPassword(ctx, writer, entity, effective.User, password); err != nil {
		return fmt.Errorf("apply cloud-init password via guest agent: %w", err)
	}

	return nil
}

// agentPingPoll is the interval between guest-agent pings while waiting for
// the guest to become reachable; maxAgentPingWait bounds the total wait for
// both the ping phase and the missing-account retries. Vars so tests can
// shorten them, mirroring agentExecPoll in the cluster package.
var (
	agentPingPoll    = 2 * time.Second
	maxAgentPingWait = 30 * time.Second
)

// preflightGuestAgent refuses a password request that cannot succeed before
// anything is written: the guest agent must be enabled in the VM config, and
// the VM must be running (read live, not from the up-to-30s-stale projection —
// ADR 0001). Both refusals are immediate and actionable where the raw agent
// error today is opaque (ticket 05).
func preflightGuestAgent(ctx context.Context, deps CloudInitConfigDeps, entity Entity, current cluster.CloudInitConfig) error {
	if !current.Agent {
		return ErrGuestAgentDisabled
	}

	if deps.StatusReader == nil {
		return nil
	}

	status, err := deps.StatusReader.VMStatus(ctx, entity.Node, entity.VMID)
	if err != nil {
		// The live read is a courtesy check; an unreachable reader must not
		// block the operation — the bounded ping below is the real gate.
		return nil //nolint:nilerr // deliberate fall-through: the ping is the authoritative gate
	}

	if status.Status != cluster.VMRunning {
		return ErrVMNotRunning
	}

	return nil
}

// applyCloudInitPassword waits for the QEMU guest agent within a bounded
// window, then applies the password to user, retrying while cloud-init has
// not yet created the account (cc_users_groups runs tens of seconds after
// Proxmox reports the VM running). The password is never stored, so the whole
// operation must succeed inside one synchronous request — hence the bounded,
// in-request wait instead of ProxMate's persisted-retry subsystem (ticket 05).
func applyCloudInitPassword(ctx context.Context, writer cluster.Writer, entity Entity, user, password string) error {
	deadline := time.NewTimer(maxAgentPingWait)
	defer deadline.Stop()

	ticker := time.NewTicker(agentPingPoll)
	defer ticker.Stop()

	agentReady := false
	for {
		if !agentReady {
			if err := writer.PingGuestAgent(ctx, entity.Node, entity.VMID); err != nil {
				if !waitNextProbe(ctx, deadline.C, ticker.C) {
					return fmt.Errorf("%w after %s (is qemu-guest-agent installed and running?): %w", ErrGuestAgentUnreachable, maxAgentPingWait, err)
				}

				continue
			}

			agentReady = true
		}

		err := writer.SetCloudInitPassword(ctx, entity.Node, entity.VMID, user, password)
		if err == nil || !errors.Is(err, cluster.ErrGuestUserUnknown) {
			return err
		}

		if !waitNextProbe(ctx, deadline.C, ticker.C) {
			return fmt.Errorf("guest account %q did not appear within %s: %w", user, maxAgentPingWait, err)
		}
	}
}

// waitNextProbe blocks until the next probe tick, the deadline, or context
// cancellation. ok is false when the caller must stop (deadline or ctx).
func waitNextProbe(ctx context.Context, deadline, tick <-chan time.Time) (ok bool) {
	select {
	case <-ctx.Done():
		return false
	case <-deadline:
		return false
	case <-tick:
		return true
	}
}

// GetCloudInitSnippet reads one snippet after the shared ownership gate.
func GetCloudInitSnippet(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, st *store.Store) (store.CloudInitSnippet, bool, error) {
	if _, err := resolveCloudInitTarget(index, actor, clusterName, vmid); err != nil {
		return store.CloudInitSnippet{}, false, err
	}

	return st.GetCloudInitSnippet(ctx, clusterName, vmid)
}

// CloudInitSnippetDeps groups the shared dependencies and resolution context
// for SetCloudInitSnippet. It collapses the ten positional parameters the
// function used to take (SonarQube go:S107).
type CloudInitSnippetDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.CloudInitReader
	Writer      cluster.Writer
	Store       *store.Store
	Service     *policy.Policy
}

// SetCloudInitSnippet validates, persists, pushes, and audits one custom snippet.
func SetCloudInitSnippet(ctx context.Context, deps CloudInitSnippetDeps, content string) error {
	index := deps.Index
	actor := deps.Actor
	clusterName := deps.ClusterName
	vmid := deps.VMID
	reader := deps.Reader
	writer := deps.Writer
	st := deps.Store
	service := deps.Service

	if service == nil {
		return policy.ErrUnavailable
	}

	gabarit, err := service.Gabarit(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("read gabarit: %w", err)
	}

	if !gabarit.AllowCustomYAML {
		return ErrCustomYAMLDisabled
	}

	if err := cloudinit.Validate(content); err != nil {
		return err
	}

	entity, err := resolveCloudInitTarget(index, actor, clusterName, vmid)
	if err != nil {
		return err
	}

	storage, filename, err := resolveSnippetArtifact(ctx, st, reader, entity, clusterName, vmid)
	if err != nil {
		return err
	}

	if err := st.PutCloudInitSnippet(ctx, clusterName, vmid, storage, filename, content, actor.Username); err != nil {
		return err
	}

	if err := writer.PushCloudInitSnippet(ctx, entity.Node, storage, filename, vmid, content); err != nil {
		return wrapJoin(ErrSnippetPushFailed, err)
	}

	// Point the VM at the just-pushed snippet via the vendor-data slot. Before
	// this step the file lived in storage but was never referenced by the VM,
	// so the guest never received it (REPORT.md addendum: silent no-op).
	if err := writer.AttachCloudInitSnippet(ctx, entity.Node, storage, filename, vmid); err != nil {
		return wrapJoin(ErrSnippetPushFailed, err)
	}

	if err := st.RecordAction(ctx, actor.Username, clusterName, vmid, "edit_cloudinit_snippet"); err != nil {
		return fmt.Errorf("record cloud-init snippet audit: %w", err)
	}

	return nil
}

// resolveSnippetArtifact returns the storage and filename for a cloud-init
// snippet, reusing the existing record when present and discovering storage
// from the cluster reader when creating a new one.
func resolveSnippetArtifact(ctx context.Context, st *store.Store, reader cluster.CloudInitReader, entity Entity, clusterName string, vmid int) (storage, filename string, err error) {
	existing, found, err := st.GetCloudInitSnippet(ctx, clusterName, vmid)
	if err != nil {
		return "", "", fmt.Errorf("read existing cloud-init snippet: %w", err)
	}

	storage = existing.Storage
	if !found {
		storage, err = reader.FindSnippetStorage(ctx, entity.Node)
		if err != nil {
			return "", "", fmt.Errorf("resolve cloud-init snippet storage: %w", err)
		}
	}

	filename = fmt.Sprintf("%s%d.yml", snippetFilenamePrefix, vmid)
	if found && existing.Filename != "" {
		filename = existing.Filename
	}

	return storage, filename, nil
}

func resolveCloudInitTarget(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	if index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(index, actor, clusterName, vmid)
}

// AddCloudInitSSHKeyDeps groups the shared dependencies for AddCloudInitSSHKey.
type AddCloudInitSSHKeyDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.CloudInitReader
	Writer      cluster.Writer
	Audit       AuditRecorder
}

// AddCloudInitSSHKey injects a single public key into the running guest's
// authorized_keys via the QEMU guest agent, without a reboot. The key is
// validated first (REPORT.md §2/#3), so a malformed or multi-line value is
// rejected before it reaches the agent. On success the key is also merged into
// the cloud-init config best-effort so later duplicate/rebuild flows keep a
// truthful key set; that sync failing does not roll back the live injection,
// which is the source of truth (mirrors ProxMate's addGuestSshKey).
func AddCloudInitSSHKey(ctx context.Context, deps AddCloudInitSSHKeyDeps, user, key string) error {
	if err := cloudinit.ValidateSSHKey(key); err != nil {
		return wrapJoin(ErrSSHKeyInvalid, err)
	}

	entity, err := resolveCloudInitTarget(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return err
	}

	if err := deps.Writer.AddSSHKey(ctx, entity.Node, deps.VMID, user, key); err != nil {
		return fmt.Errorf("inject ssh key via guest agent: %w", err)
	}

	// Best-effort config sync: keep ciuser/sshkeys/ipconfig0 truthful for
	// future duplicate/rebuild flows. A read or write failure here must not
	// hide the successful agent injection, which is what actually put the key
	// on the guest.
	current, err := deps.Reader.GetCloudInitConfig(ctx, entity.Node, deps.VMID)
	if err == nil {
		if slices.Contains(current.SSHKeys, key) {
			return recordSSHKeyAudit(ctx, deps, user, key)
		}

		current.SSHKeys = append(current.SSHKeys, key)
		if err := deps.Writer.SetCloudInitConfig(ctx, entity.Node, deps.VMID, current); err != nil {
			// The key is already live in the guest; surface a softer note only
			// if we can attribute it, but never fail the whole operation.
			return recordSSHKeyAudit(ctx, deps, user, key)
		}
	}

	return recordSSHKeyAudit(ctx, deps, user, key)
}

func recordSSHKeyAudit(ctx context.Context, deps AddCloudInitSSHKeyDeps, _, _ string) error {
	if deps.Audit == nil {
		return nil
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "edit_cloudinit_sshkey"); err != nil {
		return fmt.Errorf("record cloud-init ssh-key audit: %w", err)
	}

	return nil
}

//nolint:wsl_v5 // validation branches are intentionally kept adjacent by field
func validateCloudInitUpdate(update cluster.CloudInitUpdate) error {
	if update.IPMode != nil && *update.IPMode != cluster.CloudInitIPModeDHCP && *update.IPMode != cluster.CloudInitIPModeStatic {
		return fmt.Errorf("%w: ipMode must be dhcp or static", ErrInvalidCloudInitConfig)
	}
	if update.IPAddress != nil && *update.IPAddress != "" && !isIPv4Prefix(*update.IPAddress) {
		return fmt.Errorf("%w: invalid ipAddress", ErrInvalidCloudInitConfig)
	}
	if update.Gateway != nil && *update.Gateway != "" && !isIPv4Address(*update.Gateway) {
		return fmt.Errorf("%w: invalid gateway", ErrInvalidCloudInitConfig)
	}

	// Reject malformed or multi-line SSH keys before they reach Proxmox: a
	// pasted multi-line value would smuggle extra keys into authorized_keys
	// (REPORT.md §2/#3, mirrors ProxMate's isValidPublicKey guard).
	if update.SSHKeys != nil {
		if err := cloudinit.ValidateSSHKeys(*update.SSHKeys); err != nil {
			return wrapJoin(ErrInvalidCloudInitConfig, err)
		}
	}

	return nil
}

//nolint:wsl_v5 // field merge branches intentionally form one patch operation
func mergeCloudInitConfig(current cluster.CloudInitConfig, update cluster.CloudInitUpdate) (cluster.CloudInitConfig, error) {
	if current.IPMode == "" {
		current.IPMode = cluster.CloudInitIPModeDHCP
	}
	if update.User != nil {
		current.User = *update.User
	}
	if update.SSHKeys != nil {
		current.SSHKeys = append([]string(nil), (*update.SSHKeys)...)
	}
	if update.IPMode != nil {
		current.IPMode = *update.IPMode
		if current.IPMode == cluster.CloudInitIPModeDHCP {
			current.IPAddress = ""
			current.Gateway = ""
		}
	}
	if update.IPAddress != nil {
		current.IPAddress = *update.IPAddress
	}
	if update.Gateway != nil {
		current.Gateway = *update.Gateway
	}
	if update.DNSServer != nil {
		current.DNSServer = *update.DNSServer
	}
	if update.SearchDomain != nil {
		current.SearchDomain = *update.SearchDomain
	}

	if err := validateCloudInitConfig(current); err != nil {
		return cluster.CloudInitConfig{}, err
	}

	return current, nil
}

//nolint:wsl_v5 // validation branches are intentionally ordered by failure precedence
func validateCloudInitConfig(config cluster.CloudInitConfig) error {
	if config.IPMode != cluster.CloudInitIPModeDHCP && config.IPMode != cluster.CloudInitIPModeStatic {
		return fmt.Errorf("%w: ipMode must be dhcp or static", ErrInvalidCloudInitConfig)
	}
	if config.IPMode != cluster.CloudInitIPModeStatic {
		return nil
	}
	if config.IPAddress == "" || config.Gateway == "" {
		return fmt.Errorf("%w: static mode requires ipAddress and gateway", ErrInvalidCloudInitConfig)
	}
	if !isIPv4Prefix(config.IPAddress) {
		return fmt.Errorf("%w: invalid ipAddress", ErrInvalidCloudInitConfig)
	}
	if !isIPv4Address(config.Gateway) {
		return fmt.Errorf("%w: invalid gateway", ErrInvalidCloudInitConfig)
	}

	return nil
}
