package cluster

import "testing"

func TestParseDiskValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		value       string
		wantStorage string
		wantSizeGB  int
	}{
		{"simple", "local-lvm:vm-101-disk-0,size=32G", FakeStorageLocalLVM, 32},
		{"options before size", "local-lvm:vm-101-disk-0,cache=writeback,size=64G", FakeStorageLocalLVM, 64},
		{"megabytes round down", "local:vm-101-disk-1,size=512M", FakeStorageLocal, 0},
		{"no size option", "local:vm-101-disk-2", FakeStorageLocal, 0},
		{"malformed, no colon", "not-a-disk-value", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage, sizeGB := parseDiskValue(tc.value)
			if storage != tc.wantStorage || sizeGB != tc.wantSizeGB {
				t.Errorf("parseDiskValue(%q) = (%q, %d), want (%q, %d)", tc.value, storage, sizeGB, tc.wantStorage, tc.wantSizeGB)
			}
		})
	}
}

func TestParseProxmoxSizeGB(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"gigabytes", "32G", 32},
		{"lowercase unit", "32g", 32},
		{"terabytes", "2T", 2048},
		{"megabytes", "2048M", 2},
		{"kilobytes negligible", "1024K", 0},
		{"bare bytes", "34359738368", 32},
		{"empty", "", 0},
		{"garbage", "not-a-size", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := parseProxmoxSizeGB(tc.raw); got != tc.want {
				t.Errorf("parseProxmoxSizeGB(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseCDROM(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  proxmoxVMConfig
		want CDROMState
	}{
		{"absent, no ide2 key", proxmoxVMConfig{}, CDROMState{State: CDROMAbsent}},
		{"empty, none media", proxmoxVMConfig{cdromDiskKey: "none,media=cdrom"}, CDROMState{State: CDROMEmpty}},
		{"mounted", proxmoxVMConfig{cdromDiskKey: cdromMountedValue}, CDROMState{State: CDROMMounted, ISOVolID: "local:iso/debian-12.iso"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := parseCDROM(tc.cfg); got != tc.want {
				t.Errorf("parseCDROM(%+v) = %+v, want %+v", tc.cfg, got, tc.want)
			}
		})
	}
}

func TestParseNetValue(t *testing.T) {
	t.Parallel()

	t.Run("model, mac, bridge, vlan, rate", func(t *testing.T) {
		t.Parallel()

		nic := parseNetValue(0, "virtio=BC:24:11:AA:BB:CC,bridge=vmbr0,tag=100,rate=10")

		if nic.Model != string(DiskBusVirtio) || nic.MAC != "BC:24:11:AA:BB:CC" || nic.Bridge != FakeBridgeVMbr0 {
			t.Fatalf("nic = %+v", nic)
		}

		if nic.VLAN == nil || *nic.VLAN != 100 {
			t.Fatalf("vlan = %v, want 100", nic.VLAN)
		}

		if nic.RateMbps == nil || *nic.RateMbps != 10 {
			t.Fatalf("rate = %v, want 10", nic.RateMbps)
		}
	})

	t.Run("bare model, no mac", func(t *testing.T) {
		t.Parallel()

		nic := parseNetValue(1, "virtio,bridge=vmbr1")

		if nic.Model != string(DiskBusVirtio) || nic.MAC != "" || nic.Bridge != FakeBridgeVMbr1 {
			t.Fatalf("nic = %+v", nic)
		}
	})
}

func TestParseBootOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  proxmoxVMConfig
		want []string
	}{
		{"modern order form", proxmoxVMConfig{"boot": "order=scsi0;ide2;net0"}, []string{diskKeySCSI0, cdromDiskKey, "net0"}},
		{"no boot key", proxmoxVMConfig{}, nil},
		{"legacy flag form, unrecognized", proxmoxVMConfig{"boot": "cdn"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := parseBootOrder(tc.cfg)
			if len(got) != len(tc.want) {
				t.Fatalf("parseBootOrder(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseBootOrder(%+v)[%d] = %q, want %q", tc.cfg, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSplitProxmoxTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"multiple", "pvmss;prod;team-a", []string{FakeTagPvmss, "prod", "team-a"}},
		{"empty", "", nil},
		{"trims whitespace", " pvmss ; prod ", []string{FakeTagPvmss, "prod"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := splitProxmoxTags(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("splitProxmoxTags(%q) = %v, want %v", tc.raw, got, tc.want)
			}

			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("splitProxmoxTags(%q)[%d] = %q, want %q", tc.raw, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseDisks_SkipsReservedSlots(t *testing.T) {
	t.Parallel()

	cfg := proxmoxVMConfig{
		"scsi0": "local-lvm:vm-101-disk-0,size=32G",
		"ide2":  "local:iso/debian-12.iso,media=cdrom", // CD-ROM, must not appear as a disk
		"ide3":  "local:vm-101-cloudinit,media=cdrom",  // cloud-init drive, must not appear as a disk
	}

	disks, total := parseDisks(cfg)
	if len(disks) != 1 {
		t.Fatalf("disks = %+v, want exactly one (scsi0)", disks)
	}

	if disks[0].Key != diskKeySCSI0 || disks[0].Storage != FakeStorageLocalLVM || disks[0].SizeGB != 32 {
		t.Fatalf("disks[0] = %+v", disks[0])
	}

	want := int64(32) * 1024 * 1024 * 1024
	if total != want {
		t.Fatalf("total = %d, want %d", total, want)
	}
}

func TestEncodeNetValue(t *testing.T) {
	t.Parallel()

	t.Run("with mac, vlan, firewall", func(t *testing.T) {
		t.Parallel()

		vlan := 100

		got := encodeNetValue(NetworkInterface{Model: "virtio", MAC: "AA:BB:CC:DD:EE:FF", Bridge: "vmbr0", VLAN: &vlan})
		want := "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=100,firewall=1"

		if got != want {
			t.Errorf("encodeNetValue = %q, want %q", got, want)
		}
	})

	t.Run("no mac, default model, firewall always emitted", func(t *testing.T) {
		t.Parallel()

		got := encodeNetValue(NetworkInterface{Bridge: FakeBridgeVMbr1})
		want := "virtio,bridge=vmbr1,firewall=1"

		if got != want {
			t.Errorf("encodeNetValue = %q, want %q", got, want)
		}
	})

	t.Run("mtu emitted when set", func(t *testing.T) {
		t.Parallel()

		got := encodeNetValue(NetworkInterface{Bridge: FakeBridgeVMbr0, MTU: 9000})
		want := "virtio,bridge=vmbr0,firewall=1,mtu=9000"

		if got != want {
			t.Errorf("encodeNetValue = %q, want %q", got, want)
		}
	})
}
