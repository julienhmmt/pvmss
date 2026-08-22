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
)

const snippetFilenamePrefix = "pvmss-"

var (
	// ErrInvalidCloudInitConfig reports malformed effective structured state.
	ErrInvalidCloudInitConfig = errors.New("invalid cloud-init config")
	// ErrSnippetPushFailed reports a committed snippet that was not applied upstream.
	ErrSnippetPushFailed = errors.New("cloud-init snippet push failed")
	// ErrCustomYAMLDisabled reports an administrator-disabled snippet editor.
	ErrCustomYAMLDisabled = errors.New("custom yaml disabled")
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
	// is readable by the tenant — REPORT.md §1). Requires a running guest
	// with qemu-guest-agent; a clear error is returned if the agent is down.
	if update.Password != nil && *update.Password != "" {
		if err := writer.SetCloudInitPassword(ctx, entity.Node, entity.VMID, *update.Password); err != nil {
			return false, fmt.Errorf("apply cloud-init password via guest agent: %w", err)
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
		return fmt.Errorf("%w: %w", ErrSnippetPushFailed, err)
	}

	// Point the VM at the just-pushed snippet via the vendor-data slot. Before
	// this step the file lived in storage but was never referenced by the VM,
	// so the guest never received it (REPORT.md addendum: silent no-op).
	if err := writer.AttachCloudInitSnippet(ctx, entity.Node, storage, filename, vmid); err != nil {
		return fmt.Errorf("%w: %w", ErrSnippetPushFailed, err)
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

//nolint:wsl_v5 // validation branches are intentionally kept adjacent by field
func validateCloudInitUpdate(update cluster.CloudInitUpdate) error {
	if update.IPMode != nil && *update.IPMode != cluster.CloudInitIPModeDHCP && *update.IPMode != cluster.CloudInitIPModeStatic {
		return fmt.Errorf("%w: ipMode must be dhcp or static", ErrInvalidCloudInitConfig)
	}
	if update.IPAddress != nil && *update.IPAddress != "" {
		if _, err := netip.ParsePrefix(*update.IPAddress); err != nil {
			return fmt.Errorf("%w: invalid ipAddress", ErrInvalidCloudInitConfig)
		}
	}
	if update.Gateway != nil && *update.Gateway != "" {
		if _, err := netip.ParseAddr(*update.Gateway); err != nil {
			return fmt.Errorf("%w: invalid gateway", ErrInvalidCloudInitConfig)
		}
	}

	// Reject malformed or multi-line SSH keys before they reach Proxmox: a
	// pasted multi-line value would smuggle extra keys into authorized_keys
	// (REPORT.md §2/#3, mirrors ProxMate's isValidPublicKey guard).
	if update.SSHKeys != nil {
		if err := cloudinit.ValidateSSHKeys(*update.SSHKeys); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCloudInitConfig, err)
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
	if _, err := netip.ParsePrefix(config.IPAddress); err != nil {
		return fmt.Errorf("%w: invalid ipAddress", ErrInvalidCloudInitConfig)
	}
	if _, err := netip.ParseAddr(config.Gateway); err != nil {
		return fmt.Errorf("%w: invalid gateway", ErrInvalidCloudInitConfig)
	}

	return nil
}
