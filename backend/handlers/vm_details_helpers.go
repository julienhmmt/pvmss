package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"pvmss/state"
)

// diskSegmentTemplate is used by the UI to render a stacked bar per disk.
type diskSegmentTemplate struct {
	Index     string
	Storage   string
	Bus       string
	SizeGB    int
	Percent   float64
	Color     string
	SizeLabel string
}

func busColor(bus string) string {
	switch bus {
	case "virtio":
		return "#f80" // primary orange
	case "scsi":
		return "#48c774" // success green
	case "sata":
		return "#ffdd57" // warning yellow
	case "ide":
		return "#7a7a7a" // grey
	default:
		return "#b5b5b5"
	}
}

func formatSizeLabelGB(sizeGB int) string {
	if sizeGB >= 1024 {
		tb := float64(sizeGB) / 1024.0
		if sizeGB%1024 == 0 {
			return fmt.Sprintf("%d TB", sizeGB/1024)
		}
		return fmt.Sprintf("%.1f TB", tb)
	}
	return fmt.Sprintf("%d GB", sizeGB)
}

// diskTemplateData represents a single disk entry for the template.
type diskTemplateData struct {
	Index     string
	Bus       string
	Number    int
	Storage   string
	SizeGB    int
	SizeLabel string
	Color     string
	Raw       string
	Exists    bool
}

// buildDisksData extracts disk definitions from VM config for all supported buses.
func buildDisksData(cfg map[string]interface{}) []diskTemplateData {
	if cfg == nil {
		return nil
	}
	busOrder := []string{state.DiskBusVirtIO, state.DiskBusSCSI, state.DiskBusSATA, state.DiskBusIDE}
	disks := make([]diskTemplateData, 0, 8)
	for _, bus := range busOrder {
		max := state.GetMaxDisksForBus(bus)
		for i := 0; i < max; i++ {
			key := fmt.Sprintf("%s%d", bus, i)
			raw := ""
			if v, ok := cfg[key].(string); ok {
				raw = strings.TrimSpace(v)
			}
			if raw == "" {
				continue
			}
			if strings.Contains(strings.ToLower(raw), "media=cdrom") {
				continue
			}
			storage := ""
			sizeGB := 0
			first := raw
			if idx := strings.Index(raw, ","); idx >= 0 {
				first = raw[:idx]
			}
			if parts := strings.SplitN(first, ":", 2); len(parts) == 2 {
				storage = strings.TrimSpace(parts[0])
				if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					sizeGB = n
				}
			}
			if sizeGB == 0 && strings.Contains(raw, "size=") {
				for _, seg := range strings.Split(raw, ",") {
					seg = strings.TrimSpace(seg)
					if strings.HasPrefix(seg, "size=") {
						val := strings.TrimPrefix(seg, "size=")
						val = strings.TrimSpace(val)
						suffix := ""
						numStr := val
						if len(val) > 0 {
							last := val[len(val)-1]
							if last == 'G' || last == 'g' || last == 'M' || last == 'm' || last == 'T' || last == 't' {
								suffix = strings.ToUpper(string(last))
								numStr = strings.TrimSpace(val[:len(val)-1])
							}
						}
						if n, err := strconv.ParseFloat(numStr, 64); err == nil {
							switch suffix {
							case "M":
								sizeGB = int(n / 1024.0)
								if sizeGB == 0 && n > 0 {
									sizeGB = 1
								}
							case "T":
								sizeGB = int(n * 1024.0)
							default:
								sizeGB = int(n)
							}
						}
						break
					}
				}
			}
			disks = append(disks, diskTemplateData{
				Index:     key,
				Bus:       bus,
				Number:    i,
				Storage:   storage,
				SizeGB:    sizeGB,
				SizeLabel: formatSizeLabelGB(sizeGB),
				Color:     busColor(bus),
				Raw:       raw,
				Exists:    true,
			})
		}
	}
	return disks
}

// Network cards display helpers
type networkCardTemplateData struct {
	Index    string
	Bridge   string
	Model    string
	MAC      string
	VLAN     string
	Rate     string
	MTU      string
	Exists   bool
	Options  []string
	LinkDown bool
}

var networkModelKeys = []string{"virtio", "e1000", "e1000e", "rtl8139", "vmxnet3"}

// parseNetworkConfig parses a netX config string into structured values.
func parseNetworkConfig(raw string) (model, mac, bridge, vlan, rate, mtu string, options []string, linkDown bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := ""
			if len(kv) > 1 {
				value = strings.TrimSpace(kv[1])
			}
			switch {
			case key == "model":
				model = value
			case key == "bridge":
				bridge = value
			case key == "tag":
				vlan = value
			case key == "rate":
				rate = value
			case key == "mtu":
				mtu = value
			case key == "link_down":
				linkDown = (value == "1" || value == "true")
			case containsString(networkModelKeys, key):
				model = key
				mac = strings.ToUpper(value)
			default:
				options = append(options, part)
			}
		} else if containsString(networkModelKeys, part) {
			model = part
		} else if part == "link_down" {
			linkDown = true
		} else {
			options = append(options, part)
		}
	}
	if model == "" {
		model = "virtio"
	}
	return
}

// buildNetworkCardsData builds template data for network cards from VM config.
func buildNetworkCardsData(cfg map[string]interface{}, maxCards int) []networkCardTemplateData {
	if maxCards <= 0 {
		maxCards = 1
	}
	cards := make([]networkCardTemplateData, maxCards)
	for i := 0; i < maxCards; i++ {
		key := fmt.Sprintf("net%d", i)
		rawVal := ""
		if cfg != nil {
			if netVal, ok := cfg[key].(string); ok {
				rawVal = netVal
			}
		}
		model, mac, bridge, vlan, rate, mtu, opts, linkDown := parseNetworkConfig(rawVal)
		cards[i] = networkCardTemplateData{
			Index:    key,
			Bridge:   bridge,
			Model:    model,
			MAC:      mac,
			VLAN:     vlan,
			Rate:     rate,
			MTU:      mtu,
			Exists:   rawVal != "",
			Options:  opts,
			LinkDown: linkDown,
		}
	}
	return cards
}
