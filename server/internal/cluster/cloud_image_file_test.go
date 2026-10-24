package cluster

import "testing"

func TestIsCloudImageFile(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"ubuntu-24.04.qcow2": true,
		"DEBIAN-12.QCOW2":    true,
		"disk.raw":           true,
		"appliance.vmdk":     true,
		"appliance.ova":      false,
		"appliance.ovf":      false,
		"noble-cloudimg.img": false,
		"ubuntu-24.04.iso":   false,
		"notes.qcow2.txt":    false,
	}

	for name, want := range cases {
		if got := IsCloudImageFile(name); got != want {
			t.Errorf("IsCloudImageFile(%q) = %v, want %v", name, got, want)
		}
	}
}
