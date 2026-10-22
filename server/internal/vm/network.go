package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
)

var (
	// ErrBridgeNotApproved reports a bridge absent from the catalog.
	ErrBridgeNotApproved = errors.New("bridge is not approved")
	// ErrNetworkCardsExceedLimit reports a network-card bound violation.
	ErrNetworkCardsExceedLimit = errors.New("network cards exceed limit")
	// ErrInvalidNetworkModel reports an unsupported NIC model.
	ErrInvalidNetworkModel = errors.New("invalid network model")
	// ErrDuplicateNetworkIndex reports repeated interface indexes.
	ErrDuplicateNetworkIndex = errors.New("duplicate network interface index")
)

// NetworkDependencies contains the resolved VM write dependencies for network updates.
type NetworkDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Writer      cluster.Writer
	Resources   catalog.Resources
	Policy      *policy.Policy
	Gabarit     policy.Gabarit
	Audit       AuditRecorder
	Refresher   IndexRefresher
}

// UpdateNetwork replaces a VM's desired network interface list while preserving
// server-derived MAC and guest-agent IP fields.
func UpdateNetwork(ctx context.Context, deps NetworkDependencies, requested []cluster.NetworkInterface) ([]cluster.NetworkInterface, error) {
	entity, err := resolveNetworkTarget(deps)
	if err != nil {
		return nil, err
	}

	gabarit, err := resolveGabarit(ctx, deps.Policy, deps.Gabarit, deps.ClusterName, func(g policy.Gabarit) bool { return g.MaxNetworkCards > 0 })
	if err != nil {
		return nil, err
	}

	// A zero gabarit field means no cap.
	if gabarit.MaxNetworkCards != 0 && len(requested) > gabarit.MaxNetworkCards {
		return nil, ErrNetworkCardsExceedLimit
	}

	existing := networkByIndex(entity.NetworkInterfaces)
	updated := make([]cluster.NetworkInterface, 0, len(requested))

	seen := make(map[int]bool, len(requested))
	for _, iface := range requested {
		if seen[iface.Index] {
			return nil, ErrDuplicateNetworkIndex
		}

		seen[iface.Index] = true
		if !deps.Resources.HasBridge(iface.Bridge, entity.Node) {
			return nil, fmt.Errorf("%w: %s", ErrBridgeNotApproved, iface.Bridge)
		}

		if !allowedNetworkModels[iface.Model] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidNetworkModel, iface.Model)
		}

		// Server-owned NIC state (MAC, guest-agent IPs) and options the request
		// cannot set (firewall, mtu) come from the live NIC, not the request.
		current := existing[iface.Index]
		iface.MAC = current.MAC
		iface.IPAddresses = append([]string(nil), current.IPAddresses...)
		iface.Firewall = current.Firewall
		iface.MTU = current.MTU

		if !deps.Actor.IsAdmin {
			iface = keepTenantIsolation(iface, current, gabarit.IsolationVLANTag)
		}

		updated = append(updated, iface)
	}

	if err := deps.Writer.UpdateNetwork(ctx, entity.Node, entity.VMID, updated); err != nil {
		return nil, fmt.Errorf("update network: %w", err)
	}

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, "network_update"); err != nil {
		return nil, fmt.Errorf("record network audit: %w", err)
	}

	if _, err := deps.Refresher.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("refresh inventory after network write: %w", err)
	}

	return updated, nil
}

// keepTenantIsolation pins the VLAN and rate limit of a non-admin's NIC: they
// isolate tenants, so the NIC stays on the imposed isolation VLAN (or its
// current one) at its current rate, whatever the request asked.
func keepTenantIsolation(iface, current cluster.NetworkInterface, isolationVLANTag int) cluster.NetworkInterface {
	iface.VLAN = current.VLAN
	if isolationVLANTag > 0 {
		iface.VLAN = &isolationVLANTag
	}

	iface.RateMbps = current.RateMbps

	return iface
}

func resolveNetworkTarget(deps NetworkDependencies) (Entity, error) {
	if deps.Index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
}

func networkByIndex(interfaces []cluster.NetworkInterface) map[int]cluster.NetworkInterface {
	result := make(map[int]cluster.NetworkInterface, len(interfaces))
	for _, iface := range interfaces {
		result[iface.Index] = iface
	}

	return result
}
