package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// guestNetworkTimeout is short: a working agent answers
// in well under 100 ms, and a silent one (VM booting, agent not installed)
// otherwise cost the VM page 3 s on every load.
const guestNetworkTimeout = time.Second

// GuestNetworkInterfaces implements GuestNetworkReader via GET
// /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces. Bounded by
// guestNetworkTimeout and never retried: an agent configured but not yet
// running pends until timeout, and the VM detail page waits on this
// best-effort read. Any error - VM stopped, agent absent,
// timeout - propagates; the caller treats it as "no live addresses".
func (p Proxmox) GuestNetworkInterfaces(ctx context.Context, node string, vmid int) ([]GuestInterface, error) {
	ctx, cancel := context.WithTimeout(ctx, guestNetworkTimeout)
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
