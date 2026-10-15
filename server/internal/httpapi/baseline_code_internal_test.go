package httpapi

import (
	"pvmss/server/internal/vm"
	"testing"
)

func TestBaselineErrorCode_MapsDeliveryStates(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		vm.ErrCloudInitWriteUnavailable.Error(): "cloudinit_write_unavailable",
		"":                                      "no_document",
		"no published cloud-init document on this cluster":   "no_document",
		"proxmox PUT /nodes/n/qemu/1/config: HTTP 500: boom": "delivery_failed",
	}

	for text, want := range cases {
		if got := baselineErrorCode(vm.BaselineStateNotDelivered, text); got != want {
			t.Errorf("%q -> %q, want %q", text, got, want)
		}
	}

	if got := baselineErrorCode("applied", ""); got != "" {
		t.Errorf("applied -> %q, want none", got)
	}
}
