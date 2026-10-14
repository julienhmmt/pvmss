package cluster

import "strings"

// netSlotModel is the slot shared by the key-less model=MAC segment, so a
// model change (e1000 -> virtio) replaces the segment rather than duplicating it.
const netSlotModel = "model"

// netKeyBridge is the netN bridge option key (also read by parseNetValue).
const netKeyBridge = "bridge"

// netManagedSlots are the keys encodeNetValue decides on every write. Any of
// them missing from the desired value is removed from the existing netN.
var netManagedSlots = []string{netSlotModel, netKeyBridge, "tag", "rate", "firewall", "mtu"}

type netPart struct {
	slot string
	raw  string
}

func splitNetParts(value string) []netPart {
	var parts []netPart

	for raw := range strings.SplitSeq(value, ",") {
		if raw == "" {
			continue
		}

		key, _, _ := strings.Cut(raw, "=")
		parts = append(parts, netPart{slot: netSlot(key), raw: raw})
	}

	return parts
}

// netSlot maps a netN option key to its merge slot; every NIC model key shares one.
func netSlot(key string) string {
	if proxmoxNICModels[key] {
		return netSlotModel
	}

	return key
}

// mergeNetValue applies the PVMSS-controlled options of desired onto an
// existing netN value. Managed slots take the desired segment in place (or are
// dropped when desired omits them); every other option (queues, link_down,
// trunks, unknown keys) is kept in its original position. Desired segments
// with no existing counterpart are appended in desired order.
func mergeNetValue(existing, desired string) string {
	want := splitNetParts(desired)

	wanted := make(map[string]string, len(want))
	for _, part := range want {
		wanted[part.slot] = part.raw
	}

	managed := make(map[string]bool, len(netManagedSlots)+1)
	for _, slot := range netManagedSlots {
		managed[slot] = true
	}

	// macaddr duplicates the MAC carried by model=MAC; PVMSS writes the latter.
	if strings.Contains(wanted[netSlotModel], "=") {
		managed["macaddr"] = true
	}

	emitted := make(map[string]bool, len(want))
	merged := make([]string, 0, len(want))

	for _, part := range splitNetParts(existing) {
		if !managed[part.slot] {
			merged = append(merged, part.raw)

			continue
		}

		if raw, ok := wanted[part.slot]; ok && !emitted[part.slot] {
			merged = append(merged, raw)
			emitted[part.slot] = true
		}
	}

	for _, part := range want {
		if !emitted[part.slot] {
			merged = append(merged, part.raw)
			emitted[part.slot] = true
		}
	}

	return strings.Join(merged, ",")
}
