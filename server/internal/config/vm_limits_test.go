package config_test

import (
	"pvmss/server/internal/config"
	"testing"
)

func TestDefaultVMLimits(t *testing.T) {
	t.Parallel()

	limits := config.DefaultVMLimits()

	want := config.VMLimits{
		MaxSockets:      4,
		MaxCores:        8,
		MaxMemoryMB:     16384,
		MaxDiskPerVMGB:  500,
		MaxNetworkCards: 4,
	}
	if limits != want {
		t.Fatalf("DefaultVMLimits() = %+v, want %+v", limits, want)
	}
}
