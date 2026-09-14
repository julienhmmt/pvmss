package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// GuestNetworkInterfaces implements GuestNetworkReader via GET
// /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces. Bounded like the
// agent ping (agentPingTimeout) and never retried: an agent configured but
// not yet running pends until timeout, and the detail endpoint's best-effort
// read must not stall the page. Any error - VM stopped, agent absent,
// timeout - propagates; the caller treats it as "no live addresses".
func (p Proxmox) GuestNetworkInterfaces(ctx context.Context, node string, vmid int) ([]GuestInterface, error) {
	ctx, cancel := context.WithTimeout(ctx, agentPingTimeout)
	defer cancel()

	raw, err := p.rest().withNoRetry().do(ctx, http.MethodGet,
		fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", url.PathEscape(node), vmid), nil)
	if err != nil {
		return nil, fmt.Errorf("guest agent network-get-interfaces: %w", err)
	}

	var payload struct {
		Result []struct {
			HardwareAddress string `json:"hardware-address"`
			IPAddresses     []struct {
				IPAddress string `json:"ip-address"`
			} `json:"ip-addresses"`
		} `json:"result"`
	}
	if err := decodeData(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode guest network interfaces: %w", err)
	}

	guests := make([]GuestInterface, 0, len(payload.Result))
	for _, iface := range payload.Result {
		guest := GuestInterface{MAC: iface.HardwareAddress}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddress != "" {
				guest.IPAddresses = append(guest.IPAddresses, addr.IPAddress)
			}
		}

		guests = append(guests, guest)
	}

	return guests, nil
}
