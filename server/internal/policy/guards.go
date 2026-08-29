package policy

import (
	"context"
	"fmt"
	"pvmss/server/internal/auth"
)

// adminActionRecorder is the subset of the store needed to record quota
// failures without exporting a full store dependency.
type adminActionRecorder interface {
	RecordAdminAction(ctx context.Context, actor, action, targetType, targetID, detail, ip string) error
}

var quotaAuditor adminActionRecorder

// SetQuotaAuditor wires the audit recorder used by CheckQuota. It is set from
// the main composition root.
func SetQuotaAuditor(recorder adminActionRecorder) {
	quotaAuditor = recorder
}

type auditIPKey struct{}

// ContextWithAuditIP returns a context carrying the client IP for audit
// entries produced by downstream policy checks.
func ContextWithAuditIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, auditIPKey{}, ip)
}

func auditIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(auditIPKey{}).(string)
	return ip
}

// CheckQuota refuses a non-administrator whose pool has reached its allowance.
func (service *Policy) CheckQuota(ctx context.Context, clusterName string, actor auth.Identity) error {
	quota, err := service.Quota(ctx, clusterName, actor)
	if err != nil {
		return fmt.Errorf("read quota: %w", err)
	}

	if actor.IsAdmin || quota.Allowed == -1 || quota.Used < quota.Allowed {
		return nil
	}

	recordQuotaExceeded(ctx, actor, quota.Used, quota.Allowed)

	return &QuotaExceededError{Username: actor.Username, Used: quota.Used, Allowed: quota.Allowed}
}

func recordQuotaExceeded(ctx context.Context, actor auth.Identity, used, allowed int) {
	if quotaAuditor == nil {
		return
	}

	detail := fmt.Sprintf(`{"summary":"quota exceeded for %s (used %d of %d)","changes":[{"used":%d,"allowed":%d}]}`, actor.Username, used, allowed, used, allowed)
	_ = quotaAuditor.RecordAdminAction(ctx, actor.Username, "quota.exceeded", "quota", actor.Username, detail, auditIPFromContext(ctx))
}

// CheckGabarit validates the resolved initial VM hardware in field order.
func (service *Policy) CheckGabarit(ctx context.Context, clusterName string, sockets, cores, memoryMB, diskGB, networkCards int) error {
	gabarit, err := service.Gabarit(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("read gabarit: %w", err)
	}

	values := []struct {
		field              string
		requested, maximum int
	}{
		{"sockets", sockets, gabarit.MaxSockets},
		{"cores", cores, gabarit.MaxCores},
		{"memoryMB", memoryMB, gabarit.MaxMemoryMB},
		{"diskGB", diskGB, gabarit.MaxDiskPerVMGB},
		{"networkCards", networkCards, gabarit.MaxNetworkCards},
	}
	for _, value := range values {
		if value.requested != 0 && value.requested > value.maximum {
			return &GabaritExceededError{Field: value.field, Requested: value.requested, Maximum: value.maximum}
		}
	}

	return nil
}

// CheckNodeCapacity validates aggregate VM count, vCPU, RAM, and disk headroom.
// delta.ExcludeVMID removes a resizing VM's current contribution before adding
// the requested replacement values. delta.DiskGB is the provisioned disk the
// new or resized VM would add (D4c: enforced against MaxDiskGB, parallel to
// RAM).
func (service *Policy) CheckNodeCapacity(ctx context.Context, clusterName, node string, delta CapacityDelta) error {
	capacity, err := service.NodeCapacity(ctx, clusterName, node)
	if err != nil {
		return fmt.Errorf("read node capacity: %w", err)
	}

	if delta.ExcludeVMID != 0 {
		service.excludeVM(&capacity, delta.ExcludeVMID)
	}

	usedVMs := capacity.UsedVMs
	if delta.ExcludeVMID == 0 {
		usedVMs++
	}

	usedVCPUs := capacity.UsedVCPUs + delta.Sockets*delta.Cores
	usedRAMGB := capacity.UsedRAMGB + (delta.MemoryMB+1023)/1024
	usedDiskGB := capacity.UsedDiskGB + delta.DiskGB

	dimensions := make([]string, 0, 4)
	if capacity.MaxVMs > 0 && usedVMs > capacity.MaxVMs {
		dimensions = append(dimensions, dimensionVMs)
	}

	if capacity.MaxVCPUs > 0 && usedVCPUs > capacity.MaxVCPUs {
		dimensions = append(dimensions, dimensionVCPUs)
	}

	if capacity.MaxRAMGB > 0 && usedRAMGB > capacity.MaxRAMGB {
		dimensions = append(dimensions, dimensionRAM)
	}

	if capacity.MaxDiskGB > 0 && usedDiskGB > capacity.MaxDiskGB {
		dimensions = append(dimensions, dimensionDisk)
	}

	if len(dimensions) == 0 {
		return nil
	}

	return &NodeCapacityExceededError{Node: node, Dimensions: dimensions, MaxVMs: capacity.MaxVMs, MaxVCPUs: capacity.MaxVCPUs, MaxRAMGB: capacity.MaxRAMGB, MaxDiskGB: capacity.MaxDiskGB}
}

func (service *Policy) excludeVM(capacity *Capacity, vmid int) {
	if service.projection == nil || service.projection.Load() == nil {
		return
	}

	machine, ok := service.projection.Load().ByVMID[vmid]
	if !ok {
		return
	}

	if capacity.UsedVMs > 0 {
		capacity.UsedVMs--
	}

	vcpus := vmVCPUs(machine)
	if capacity.UsedVCPUs >= vcpus {
		capacity.UsedVCPUs -= vcpus
	}

	capacity.UsedRAMGB = service.ramGBExcluding(machine.Node, vmid)
	capacity.UsedDiskGB = service.diskGBExcluding(machine.Node, vmid)
}

// QuotaExceededError carries the values needed for a safe user-facing message.
type QuotaExceededError struct {
	Username      string
	Used, Allowed int
}

func (failure *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s already owns %d of %d allowed VMs", failure.Username, failure.Used, failure.Allowed)
}
func (failure *QuotaExceededError) Unwrap() error { return ErrQuotaExceeded }

// GabaritExceededError identifies the first offending VM dimension.
type GabaritExceededError struct {
	Field              string
	Requested, Maximum int
}

func (failure *GabaritExceededError) Error() string {
	switch failure.Field {
	case "diskGB":
		return fmt.Sprintf("disk size (%d GB) exceeds the configured gabarit (%d GB)", failure.Requested, failure.Maximum)
	case "memoryMB":
		return fmt.Sprintf("memory (%d MB) exceeds the configured gabarit (%d MB)", failure.Requested, failure.Maximum)
	case "networkCards":
		return fmt.Sprintf("network cards (%d) exceed the configured gabarit (%d)", failure.Requested, failure.Maximum)
	default:
		return fmt.Sprintf("%s (%d) exceeds the configured gabarit (%d)", failure.Field, failure.Requested, failure.Maximum)
	}
}
func (failure *GabaritExceededError) Unwrap() error { return ErrGabaritExceeded }

// NodeCapacityExceededError identifies the node dimensions that would overflow.
type NodeCapacityExceededError struct {
	Node                       string
	Dimensions                 []string
	MaxVMs, MaxVCPUs, MaxRAMGB int
	MaxDiskGB                  int
}

func (failure *NodeCapacityExceededError) Error() string {
	dimension := failure.Dimensions[0]

	displayDimension := dimension
	if dimension == dimensionVCPUs {
		displayDimension = dimensionVCPU
	}

	maximum := failure.MaxVCPUs
	if dimension == dimensionVMs {
		maximum = failure.MaxVMs
	}

	if dimension == dimensionRAM {
		maximum = failure.MaxRAMGB
	}

	if dimension == dimensionDisk {
		maximum = failure.MaxDiskGB
	}

	return fmt.Sprintf("node %q %s capacity (%d) would be exceeded", failure.Node, displayDimension, maximum)
}
func (failure *NodeCapacityExceededError) Unwrap() error { return ErrNodeCapacityExceeded }
