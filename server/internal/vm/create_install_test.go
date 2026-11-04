package vm

import "testing"

func TestWindowsInstallHardware(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		req          CreateRequest
		bus, wantBus string
		wantNIC      string
	}{
		{"linux keeps scsi", CreateRequest{OSType: "l26"}, defaultDiskBus, defaultDiskBus, defaultNetworkModel},
		{"unset keeps virtio", CreateRequest{}, defaultNetworkModel, defaultNetworkModel, defaultNetworkModel},
		{"windows moves scsi", CreateRequest{OSType: "win11"}, defaultDiskBus, "sata", windowsNetworkModel},
		{"windows moves virtio", CreateRequest{OSType: "win10"}, defaultNetworkModel, "sata", windowsNetworkModel},
		{"windows keeps ide", CreateRequest{OSType: "win10"}, "ide", "ide", windowsNetworkModel},
		{"clone is untouched", CreateRequest{OSType: "win11", TemplateID: 9000}, defaultDiskBus, defaultDiskBus, defaultNetworkModel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := installDiskBus(tc.req, tc.bus); got != tc.wantBus {
				t.Errorf("bus = %q, want %q", got, tc.wantBus)
			}

			if got := installNICModel(tc.req); got != tc.wantNIC {
				t.Errorf("nic = %q, want %q", got, tc.wantNIC)
			}
		})
	}
}
