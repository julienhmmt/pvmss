package httpapi

import (
	"pvmss/server/internal/cluster"
	"slices"
	"testing"
)

// Issue #211: a guest bridge/VLAN sharing the NIC's MAC must not erase the
// address reported on another interface with that MAC.
func TestMergeGuestIPsKeepsAddressesAcrossSameMAC(t *testing.T) {
	t.Parallel()

	nics := []cluster.NetworkInterface{{MAC: "BC:24:11:00:00:01"}}
	guests := []cluster.GuestInterface{
		{MAC: "bc:24:11:00:00:01", IPAddresses: []string{"192.168.1.50/24"}},
		{MAC: "bc:24:11:00:00:01"},
	}

	mergeGuestIPs(nics, guests)

	if !slices.Contains(nics[0].IPAddresses, "192.168.1.50/24") {
		t.Fatalf("IPAddresses = %v, want to contain 192.168.1.50/24", nics[0].IPAddresses)
	}
}
