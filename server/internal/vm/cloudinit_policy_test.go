package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

func TestSetCloudInitSnippet_DisabledBeforeValidationAndResolve(t *testing.T) {
	t.Parallel()
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	service := policy.New(st, inventory.NewProjectionFromIndex(index), cluster.Fake{})

	gabarit, err := service.Gabarit(context.Background(), testClusterName)
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	gabarit.AllowCustomYAML = false
	if err := service.SetGabarit(context.Background(), testClusterName, gabarit); err != nil {
		t.Fatalf("SetGabarit: %v", err)
	}

	for _, testCase := range []struct {
		name  string
		index *inventory.Index
	}{
		{name: "invalid content", index: index},
		{name: "ownership index unavailable", index: nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cluster.ResetFake()

			err := vm.SetCloudInitSnippet(context.Background(), vm.CloudInitSnippetDeps{Index: testCase.index, Actor: cloudAliceIdentity(), ClusterName: testClusterName, VMID: 101, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: st, Service: service}, "not yaml")
			if !errors.Is(err, vm.ErrCustomYAMLDisabled) {
				t.Fatalf("error = %v, want ErrCustomYAMLDisabled", err)
			}

			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("disabled snippet reached cluster: %+v", calls)
			}
		})
	}
}
