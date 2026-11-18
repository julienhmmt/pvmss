package httpapi

import (
	"pvmss/server/internal/catalog"
	"testing"
)

func TestHardwareISOs_SharedOfferedOnce(t *testing.T) {
	t.Parallel()

	got := hardwareISOs([]catalog.ISO{
		{Storage: "ceph", Node: "n1", File: "debian.iso"},
		{Storage: "ceph", Node: "n2", File: "debian.iso"},
		{Storage: "local", Node: "n1", File: "alpine.iso"},
	})

	if len(got) != 2 || got[0].VolID != "ceph:iso/debian.iso" || got[1].VolID != "local:iso/alpine.iso" {
		t.Fatalf("got %+v, want one entry per volume", got)
	}
}
