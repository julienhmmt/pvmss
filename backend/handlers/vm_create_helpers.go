package handlers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
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
func countVMsInPool(ctx context.Context, poolName string) (int, error) {
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return 0, fmt.Errorf("failed to create resty client: %w", err)
	}
	var resp struct {
		Data struct {
			Members []struct {
				Type string `json:"type"`
				VMID int    `json:"vmid"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolName), &resp); err != nil {
		return 0, err
	}
	count := 0
	for _, m := range resp.Data.Members {
		if m.VMID > 0 && (strings.EqualFold(m.Type, "qemu") || strings.EqualFold(m.Type, "lxc")) {
			count++
		}
	}
	return count, nil
}
