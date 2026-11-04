package cluster

import (
	"strings"
	"testing"
)

const (
	testFlagsV2AES = "cx16 lahf_lm pni popcnt sse4_1 sse4_2 ssse3 aes"
	testFlagsV3    = testFlagsV2AES + " avx avx2 bmi1 bmi2 f16c fma movbe xsave abm"
	testFlagsNoAES = "cx16 lahf_lm pni popcnt sse4_1 sse4_2 ssse3 avx avx2 bmi1 bmi2 f16c fma movbe xsave abm"
)

func TestSelectCPUType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nodeFlags [][]string
		want      string
	}{
		{"v3 flags on every node", [][]string{strings.Fields(testFlagsV3)}, CPUTypeV3},
		{"v2 and aes only", [][]string{strings.Fields(testFlagsV2AES)}, CPUTypeV2AES},
		{"no v2 flags", [][]string{strings.Fields("fpu sse2 sse3")}, CPUTypeHost},
		{"mixed nodes take the lowest common model", [][]string{strings.Fields(testFlagsV3), strings.Fields(testFlagsV2AES)}, CPUTypeV2AES},
		{"v3 on every node of several", [][]string{strings.Fields(testFlagsV3), strings.Fields(testFlagsV3)}, CPUTypeV3},
		{"aes missing from a node rules out v3 and v2-AES", [][]string{strings.Fields(testFlagsV3), strings.Fields(testFlagsNoAES)}, CPUTypeHost},
		{"no nodes is unreadable", nil, CPUTypeV2AES},
		{"empty flag list is unreadable", [][]string{strings.Fields(testFlagsV3), {}}, CPUTypeV2AES},
		{"a node with no flags beats v3 elsewhere", [][]string{{}, strings.Fields(testFlagsV3)}, CPUTypeV2AES},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SelectCPUType(tt.nodeFlags)
			if got != tt.want {
				t.Errorf("SelectCPUType() = %q, want %q", got, tt.want)
			}

			if got == "kvm64" {
				t.Errorf("SelectCPUType() returned kvm64, which PVMSS must never create on")
			}
		})
	}
}
