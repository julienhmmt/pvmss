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
	index := cloudInitIndex(t)
	st := cloudInitStore(t)
	service := policy.New(st, inventory.NewProjectionFromIndex(index), cluster.Fake{})
	gabarit, err := service.Gabarit(context.Background(), testClusterName)
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}
	gabarit.AllowCustomYaml = false
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
			cluster.ResetFake()
			err := vm.SetCloudInitSnippet(context.Background(), testCase.index, cloudAliceIdentity(), testClusterName, 101, "not yaml", cluster.Fake{}, cluster.Fake{}, st, service)
			if !errors.Is(err, vm.ErrCustomYAMLDisabled) {
				t.Fatalf("error = %v, want ErrCustomYAMLDisabled", err)
			}
			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("disabled snippet reached cluster: %+v", calls)
			}
		})
	}
}
