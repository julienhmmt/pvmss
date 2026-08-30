package cluster

import "testing"

// TestStorageSnapshotCapability — ticket 07: the (plugin, format) rule that
// replaced the hardcoded 4-entry table. Block-backed plugins snapshot
// natively; file-backed plugins only with qcow2 disks; plain lvm, iscsi and
// raw-on-file cannot snapshot at all.
func TestStorageSnapshotCapability(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		pluginType   string
		format       string
		wantSnapshot bool
		wantVMState  bool
	}{
		{"zfspool any format", "zfspool", "", true, true},
		{"lvmthin any format", "lvmthin", testDiskFormatRaw, true, true},
		{"rbd any format", "rbd", "", true, true},
		{"btrfs any format", "btrfs", "", true, true},
		{"dir qcow2", "dir", testDiskFormatQCow2, true, true},
		{"nfs qcow2", "nfs", testDiskFormatQCow2, true, true},
		{"cifs qcow2", "cifs", testDiskFormatQCow2, true, true},
		{"cephfs qcow2", "cephfs", testDiskFormatQCow2, true, true},
		{"dir raw", "dir", testDiskFormatRaw, false, false},
		{"nfs raw", "nfs", testDiskFormatRaw, false, false},
		{"dir no format", "dir", "", false, false},
		{"lvm non-thin", "lvm", "", false, false},
		{"iscsi", "iscsi", "", false, false},
		{"unknown plugin", "glusterfs", testDiskFormatQCow2, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			canSnapshot, canVMState := StorageSnapshotCapability(tc.pluginType, tc.format)

			if canSnapshot != tc.wantSnapshot {
				t.Errorf("StorageSnapshotCapability(%q, %q) canSnapshot = %t, want %t", tc.pluginType, tc.format, canSnapshot, tc.wantSnapshot)
			}

			if canVMState != tc.wantVMState {
				t.Errorf("StorageSnapshotCapability(%q, %q) canVMState = %t, want %t", tc.pluginType, tc.format, canVMState, tc.wantVMState)
			}
		})
	}
}
