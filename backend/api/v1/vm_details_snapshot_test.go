package apiv1

import "testing"

func TestSupportsSnapshotVMState_SupportedStorage(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]interface{}
	}{
		{
			name: "qcow2 disk",
			cfg:  map[string]interface{}{"scsi0": "local:100/vm-100-disk-0.qcow2,size=32G"},
		},
		{
			name: "ceph storage",
			cfg:  map[string]interface{}{"virtio0": "ceph-vms:vm-100-disk-0,size=32G"},
		},
		{
			name: "rbd storage",
			cfg:  map[string]interface{}{"sata0": "rbd:vm-100-disk-0,size=32G"},
		},
		{
			name: "zfs storage",
			cfg:  map[string]interface{}{"scsi0": "local-zfs:vm-100-disk-0,size=32G"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !supportsSnapshotVMState(test.cfg) {
				t.Fatalf("expected storage to support vmstate")
			}
		})
	}
}

func TestSupportsSnapshotVMState_UnsupportedStorage(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]interface{}
	}{
		{
			name: "raw dir disk",
			cfg:  map[string]interface{}{"scsi0": "local:100/vm-100-disk-0.raw,size=32G"},
		},
		{
			name: "lvm disk",
			cfg:  map[string]interface{}{"virtio0": "local-lvm:vm-100-disk-0,size=32G"},
		},
		{
			name: "cdrom only",
			cfg:  map[string]interface{}{"ide2": "local:iso/debian.iso,media=cdrom"},
		},
		{
			name: "mixed storage (qcow2 + lvm)",
			cfg: map[string]interface{}{
				"scsi0": "local:100/vm-100-disk-0.qcow2,size=32G",
				"scsi1": "local-lvm:vm-100-disk-1,size=32G",
			},
		},
		{
			name: "mixed storage (zfs + raw)",
			cfg: map[string]interface{}{
				"virtio0": "local-zfs:vm-100-disk-0,size=32G",
				"virtio1": "local:100/vm-100-disk-1.raw,size=32G",
			},
		},
		{
			name: "no disks at all",
			cfg:  map[string]interface{}{},
		},
		{
			name: "partial name match (cephfs)",
			cfg:  map[string]interface{}{"scsi0": "cephfs:vm-100-disk-0,size=32G"},
		},
		{
			name: "partial name match (zfsbackup)",
			cfg:  map[string]interface{}{"virtio0": "zfsbackup:vm-100-disk-0,size=32G"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if supportsSnapshotVMState(test.cfg) {
				t.Fatalf("expected storage not to support vmstate")
			}
		})
	}
}

func TestSupportsSnapshotVMState_MultipleDisksAllSupported(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]interface{}
	}{
		{
			name: "multiple qcow2 disks",
			cfg: map[string]interface{}{
				"scsi0": "local:100/vm-100-disk-0.qcow2,size=32G",
				"scsi1": "local:100/vm-100-disk-1.qcow2,size=32G",
				"scsi2": "local:100/vm-100-disk-2.qcow2,size=32G",
			},
		},
		{
			name: "multiple zfs disks",
			cfg: map[string]interface{}{
				"virtio0": "local-zfs:vm-100-disk-0,size=32G",
				"virtio1": "local-zfs:vm-100-disk-1,size=32G",
			},
		},
		{
			name: "mixed supported storage (qcow2 + zfs)",
			cfg: map[string]interface{}{
				"scsi0":   "local:100/vm-100-disk-0.qcow2,size=32G",
				"virtio0": "local-zfs:vm-100-disk-1,size=32G",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !supportsSnapshotVMState(test.cfg) {
				t.Fatalf("expected storage to support vmstate")
			}
		})
	}
}
