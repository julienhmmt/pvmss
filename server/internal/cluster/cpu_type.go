package cluster

import (
	"slices"
	"strings"
)

// Proxmox CPU model names PVMSS selects between at VM create. kvm64 is never
// selected: it lacks SSE4.2 and POPCNT, which Windows 11 24H2 requires.
const (
	CPUTypeV3    = "x86-64-v3"
	CPUTypeV2AES = "x86-64-v2-AES"
	CPUTypeHost  = "host"
)

// Host flags each model needs, named as in /proc/cpuinfo. Taken from the
// x86-64-v* entries of qemu-server CPUConfig.pm; v3 lists +aes too.
const (
	cpuFlagsV2AES = "cx16 lahf_lm pni popcnt sse4_1 sse4_2 ssse3 aes"
	cpuFlagsV3    = cpuFlagsV2AES + " avx avx2 bmi1 bmi2 f16c fma movbe xsave abm"
)

// SelectCPUType returns the richest model every node in nodeFlags supports:
// CPUTypeV3, then CPUTypeV2AES, then CPUTypeHost. Flags that cannot be read
// (no nodes, or any node with an empty list) yield CPUTypeV2AES.
func SelectCPUType(nodeFlags [][]string) string {
	if len(nodeFlags) == 0 || slices.ContainsFunc(nodeFlags, func(flags []string) bool { return len(flags) == 0 }) {
		return CPUTypeV2AES
	}

	switch {
	case allNodesHave(nodeFlags, cpuFlagsV3):
		return CPUTypeV3
	case allNodesHave(nodeFlags, cpuFlagsV2AES):
		return CPUTypeV2AES
	default:
		return CPUTypeHost
	}
}

// allNodesHave reports whether every node's flags include all of required,
// a space-separated list.
func allNodesHave(nodeFlags [][]string, required string) bool {
	for _, flags := range nodeFlags {
		for need := range strings.FieldsSeq(required) {
			if !slices.Contains(flags, need) {
				return false
			}
		}
	}

	return true
}
