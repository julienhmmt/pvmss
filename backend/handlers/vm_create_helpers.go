package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"pvmss/proxmox"
)

// MAC address regex pattern - accepts both colon and hyphen separators.
var macAddressRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)

// ValidateMACAddress checks if a MAC address is in valid format.
func ValidateMACAddress(mac string) bool {
	if mac == "" {
		return true // Empty is valid (auto-generated later)
	}
	return macAddressRegex.MatchString(mac)
}

// NormalizeMACAddress converts a MAC address to Proxmox format (uppercase with colons).
func NormalizeMACAddress(mac string) string {
	if mac == "" {
		return ""
	}
	clean := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
	if len(clean) == 12 {
		return clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" + clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12]
	}
	return mac
}

// vmDiskCompatibleStorageTypes defines storage types that support VM disk images.
var vmDiskCompatibleStorageTypes = map[string]bool{
	"cifs":    true,
	"dir":     true,
	"iscsi":   true,
	"lvm":     true,
	"lvmthin": true,
	"nfs":     true,
	"rbd":     true,
	"zfs":     true,
}

// countVMsInPool counts the number of VMs in a user's pool (excludes templates).
func countVMsInPool(ctx context.Context, client proxmox.ClientInterface, poolName string) (int, error) {
	if client == nil {
		return 0, fmt.Errorf("proxmox client not available")
	}

	var poolResp struct {
		Data struct {
			Members []struct {
				Type     string `json:"type"`
				VMID     int    `json:"vmid"`
				Template int    `json:"template"`
			} `json:"members"`
		} `json:"data"`
	}

	if err := client.GetJSON(ctx, "/pools/"+poolName, &poolResp); err != nil {
		return 0, fmt.Errorf("failed to fetch pool members: %w", err)
	}

	count := 0
	for _, member := range poolResp.Data.Members {
		if member.Template == 1 || member.VMID <= 0 {
			continue
		}
		if strings.EqualFold(member.Type, "qemu") {
			count++
		}
	}

	return count, nil
}
