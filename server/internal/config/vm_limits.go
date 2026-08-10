package config

// VMLimits contains the administrator's hardware bounds for VM mutations.
// These defaults are a configuration placeholder until the policy tranche makes
// them persistent and cluster-aware.
type VMLimits struct {
	MaxSockets      int
	MaxCores        int
	MaxMemoryMB     int
	MaxDiskPerVMGB  int
	MaxNetworkCards int
	MaxSnapshots    int
}

// DefaultVMLimits returns the safe demonstration bounds used by T07.
func DefaultVMLimits() VMLimits {
	return VMLimits{
		MaxSockets:      4,
		MaxCores:        8,
		MaxMemoryMB:     16384,
		MaxDiskPerVMGB:  500,
		MaxNetworkCards: 4,
		MaxSnapshots:    5,
	}
}
