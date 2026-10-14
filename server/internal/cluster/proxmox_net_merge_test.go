package cluster

import "testing"

func TestMergeNetValue(t *testing.T) {
	t.Parallel()

	const mac = "BC:24:11:8A:B8:24"

	cases := []struct {
		name     string
		existing string
		desired  string
		want     string
	}{
		{
			name:     "keeps unmanaged queues and link_down when rate and bridge change",
			existing: "virtio=" + mac + ",bridge=vmbr0,firewall=1,queues=4,link_down=1",
			desired:  "virtio=" + mac + ",bridge=vmbr1,rate=50,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr1,firewall=1,queues=4,link_down=1,rate=50",
		},
		{
			name:     "nil tag removes existing tag",
			existing: "virtio=" + mac + ",bridge=vmbr0,tag=10,firewall=1,queues=4",
			desired:  "virtio=" + mac + ",bridge=vmbr0,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr0,firewall=1,queues=4",
		},
		{
			name:     "tag added when absent",
			existing: "virtio=" + mac + ",bridge=vmbr0,firewall=1,queues=4",
			desired:  "virtio=" + mac + ",bridge=vmbr0,tag=20,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr0,firewall=1,queues=4,tag=20",
		},
		{
			name:     "model change replaces model segment in place",
			existing: "e1000=" + mac + ",bridge=vmbr0,firewall=1,queues=2",
			desired:  "virtio=" + mac + ",bridge=vmbr0,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr0,firewall=1,queues=2",
		},
		{
			name:     "macaddr duplicate dropped when desired carries model MAC",
			existing: "virtio=" + mac + ",macaddr=" + mac + ",bridge=vmbr0,firewall=1",
			desired:  "virtio=" + mac + ",bridge=vmbr0,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr0,firewall=1",
		},
		{
			name:     "unknown trunk list kept in original order",
			existing: "virtio=" + mac + ",trunks=10;20,bridge=vmbr0,firewall=1",
			desired:  "virtio=" + mac + ",bridge=vmbr0,firewall=1",
			want:     "virtio=" + mac + ",trunks=10;20,bridge=vmbr0,firewall=1",
		},
		{
			name:     "empty existing yields desired",
			existing: "",
			desired:  "virtio=" + mac + ",bridge=vmbr0,firewall=1",
			want:     "virtio=" + mac + ",bridge=vmbr0,firewall=1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := mergeNetValue(tc.existing, tc.desired); got != tc.want {
				t.Errorf("mergeNetValue() = %q, want %q", got, tc.want)
			}
		})
	}
}
