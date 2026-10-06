package vm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
	"strings"
)

const (
	// MigrationExcludedOffline marks an approved node that is not online.
	MigrationExcludedOffline = "offline"
	// MigrationExcludedNotAllowed marks a node Proxmox's precheck refuses.
	MigrationExcludedNotAllowed = "not_allowed"

	auditActionMigrate = "vm.migrate"
)

var (
	// ErrMigrationBlocked reports a live-migration blocker such as a local resource.
	ErrMigrationBlocked = errors.New("migration blocked")
	// ErrInvalidMigrationTarget reports a target that is not a migration candidate.
	ErrInvalidMigrationTarget = errors.New("invalid migration target")
)

// MigrationStore is the persistence surface migration needs.
type MigrationStore interface {
	CatalogNodesEnabled(ctx context.Context, clusterName string) ([]store.CatalogNodeEnabled, error)
	NodePolicyRow(ctx context.Context, clusterName, node string) (store.NodePolicyRow, error)
	RecordVMActionDetail(ctx context.Context, actor, clusterName string, vmid int, action, detail string) error
}

// MigrationDependencies contains the resolved read, write, and audit dependencies.
type MigrationDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Migrator    cluster.Migrator
	Status      cluster.VMStatusReader
	Store       MigrationStore
}

// MigrationPreflight describes where a VM can go and what stops it.
type MigrationPreflight struct {
	Node       string
	Running    bool
	Lock       string
	Blockers   []string
	LocalDisks []string
	Candidates []MigrationCandidate
	Excluded   []MigrationExclusion
}

// Blocked reports whether no migration can start right now.
func (p MigrationPreflight) Blocked() bool { return p.Lock != "" || len(p.Blockers) > 0 }

// MigrationCandidate is an eligible target; Warnings never block.
type MigrationCandidate struct {
	Node     string
	Warnings []string
}

// MigrationExclusion explains why an approved node is not a candidate.
type MigrationExclusion struct {
	Node   string
	Reason string
	Detail string
}

// MigrationStart is the dispatched migration task.
type MigrationStart struct {
	UPID, Source, Target string
}

// PreflightMigration computes the migration candidates for one VM.
func PreflightMigration(ctx context.Context, deps MigrationDependencies) (MigrationPreflight, error) {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return MigrationPreflight{}, err
	}

	live, err := deps.Status.VMStatus(ctx, entity.Node, entity.VMID)
	if err != nil {
		return MigrationPreflight{}, fmt.Errorf("read vm status: %w", err)
	}

	precheck, err := deps.Migrator.MigrationPrecheck(ctx, entity.Node, entity.VMID)
	if err != nil {
		return MigrationPreflight{}, fmt.Errorf("migration precheck: %w", err)
	}

	result := MigrationPreflight{
		Node:       entity.Node,
		Running:    live.Status == cluster.VMRunning,
		Lock:       live.Lock,
		Blockers:   []string{},
		LocalDisks: nonNil(precheck.LocalDisks),
		Candidates: []MigrationCandidate{},
		Excluded:   []MigrationExclusion{},
	}
	if result.Running && len(precheck.LocalResources) > 0 {
		result.Blockers = append(result.Blockers, "local resources prevent live migration: "+strings.Join(precheck.LocalResources, ", "))
	}

	approved, err := deps.approvedNodes(ctx)
	if err != nil {
		return MigrationPreflight{}, err
	}

	machine := deps.Index.ByVMID[entity.VMID]

	for _, node := range deps.Index.Nodes {
		if node.Name == entity.Node || !approved[node.Name] {
			continue
		}

		switch {
		case node.Status != cluster.NodeOnline:
			result.Excluded = append(result.Excluded, MigrationExclusion{Node: node.Name, Reason: MigrationExcludedOffline})
		case !slices.Contains(precheck.AllowedNodes, node.Name):
			result.Excluded = append(result.Excluded, MigrationExclusion{Node: node.Name, Reason: MigrationExcludedNotAllowed, Detail: precheck.NotAllowed[node.Name]})
		default:
			warnings, err := deps.capacityWarnings(ctx, node.Name, machine)
			if err != nil {
				return MigrationPreflight{}, err
			}

			result.Candidates = append(result.Candidates, MigrationCandidate{Node: node.Name, Warnings: warnings})
		}
	}

	return result, nil
}

// StartMigration validates target against a fresh preflight, dispatches the
// migration, and audits it.
func StartMigration(ctx context.Context, deps MigrationDependencies, target string) (MigrationStart, error) {
	preflight, err := PreflightMigration(ctx, deps)
	if err != nil {
		return MigrationStart{}, err
	}

	if preflight.Lock != "" {
		return MigrationStart{}, fmt.Errorf("%w: VM %d is locked by a %s; retry once it completes", ErrVMLocked, deps.VMID, preflight.Lock)
	}

	if preflight.Blocked() {
		return MigrationStart{}, fmt.Errorf("%w: %s", ErrMigrationBlocked, strings.Join(preflight.Blockers, "; "))
	}

	if err := preflight.checkTarget(target); err != nil {
		return MigrationStart{}, err
	}

	spec := cluster.MigrateSpec{Target: target, Online: preflight.Running, WithLocalDisks: len(preflight.LocalDisks) > 0}

	upid, err := deps.Migrator.Migrate(ctx, preflight.Node, deps.VMID, spec)
	if err != nil {
		return MigrationStart{}, fmt.Errorf("dispatch migration: %w", err)
	}

	detail, err := migrationAuditDetail(preflight.Node, target)
	if err != nil {
		return MigrationStart{}, err
	}

	if err := deps.Store.RecordVMActionDetail(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, auditActionMigrate, detail); err != nil {
		return MigrationStart{}, fmt.Errorf("record migration audit: %w", err)
	}

	return MigrationStart{UPID: upid, Source: preflight.Node, Target: target}, nil
}

func (p MigrationPreflight) checkTarget(target string) error {
	if target == p.Node {
		return fmt.Errorf("%w: already on node %s", ErrInvalidMigrationTarget, target)
	}

	if slices.ContainsFunc(p.Candidates, func(c MigrationCandidate) bool { return c.Node == target }) {
		return nil
	}

	for _, excluded := range p.Excluded {
		if excluded.Node != target {
			continue
		}

		if excluded.Reason == MigrationExcludedOffline {
			return fmt.Errorf("%w: node %s is offline", ErrInvalidMigrationTarget, target)
		}

		return fmt.Errorf("%w: node %s not allowed: %s", ErrInvalidMigrationTarget, target, excluded.Detail)
	}

	return fmt.Errorf("%w: unknown or unapproved node %s", ErrInvalidMigrationTarget, target)
}

func (deps MigrationDependencies) approvedNodes(ctx context.Context) (map[string]bool, error) {
	rows, err := deps.Store.CatalogNodesEnabled(ctx, deps.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("read approved nodes: %w", err)
	}

	approved := make(map[string]bool, len(rows))
	for _, row := range rows {
		approved[row.Name] = row.Enabled
	}

	return approved, nil
}

func (deps MigrationDependencies) capacityWarnings(ctx context.Context, node string, machine cluster.VM) ([]string, error) {
	caps, err := deps.Store.NodePolicyRow(ctx, deps.ClusterName, node)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read node policy: %w", err)
	}

	return policy.NodeOverflow(deps.Index, caps, node, machine), nil
}

func migrationAuditDetail(source, target string) (string, error) {
	type change struct {
		Field string `json:"field"`
		Old   string `json:"old"`
		New   string `json:"new"`
	}

	detail, err := json.Marshal(struct {
		Summary string   `json:"summary"`
		Changes []change `json:"changes"`
	}{Summary: fmt.Sprintf("migrate %s -> %s", source, target), Changes: []change{{Field: "node", Old: source, New: target}}})
	if err != nil {
		return "", fmt.Errorf("encode migration audit: %w", err)
	}

	return string(detail), nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}
