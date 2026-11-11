package auth

import "testing"

func TestSignVerify(t *testing.T) {
	t.Parallel()

	m := &SessionManager{secret: []byte("0123456789abcdef0123456789abcdef")}
	other := &SessionManager{secret: []byte("fedcba9876543210fedcba9876543210")}

	signed := m.Sign("cluster|state.with|odd bytes")
	if got, ok := m.Verify(signed); !ok || got != "cluster|state.with|odd bytes" {
		t.Fatalf("round trip = %q, %v", got, ok)
	}

	for _, forged := range []string{"", "nodot", signed + "x", "x" + signed, other.Sign("cluster|state.with|odd bytes")} {
		if _, ok := m.Verify(forged); ok {
			t.Fatalf("Verify(%q) accepted a forged value", forged)
		}
	}
}
