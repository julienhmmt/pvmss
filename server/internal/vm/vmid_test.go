package vm_test

import (
	"errors"
	"testing"

	"pvmss/server/internal/vm"
)

func TestAllocateVMID(t *testing.T) {
	tests := []struct {
		name    string
		lo, hi  int
		used    []int
		want    int
		wantErr error
	}{
		{"empty range start", 9000, 9999, nil, 9000, nil},
		{"skips used", 9000, 9999, []int{9000, 9001, 9003}, 9002, nil},
		{"ignores ids outside range", 9000, 9999, []int{100, 101, 10000}, 9000, nil},
		{"last free id", 200, 202, []int{200, 201}, 202, nil},
		{"single id free", 500, 500, nil, 500, nil},
		{"exhausted", 200, 202, []int{202, 200, 201}, 0, vm.ErrVMIDRangeExhausted},
		{"single id taken", 500, 500, []int{500}, 0, vm.ErrVMIDRangeExhausted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vm.AllocateVMID(tt.lo, tt.hi, tt.used)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("got (%d, %v), want (%d, %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
