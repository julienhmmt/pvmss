package cluster

import (
	"bytes"
	"testing"
)

func TestReverseByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want byte
	}{
		{0x00, 0x00},
		{0xFF, 0xFF},
		{0x01, 0x80}, // 00000001 -> 10000000
		{0x80, 0x01}, // 10000000 -> 00000001
		{0xA5, 0xA5}, // 10100101 is a palindrome
		{0x0F, 0xF0},
	}

	for _, c := range cases {
		if got := reverseByte(c.in); got != c.want {
			t.Errorf("reverseByte(%#02x) = %#02x, want %#02x", c.in, got, c.want)
		}
	}
}

func TestVNCDESKey(t *testing.T) {
	t.Parallel()

	// A password of exactly 8 non-zero bytes: every byte gets bit-reversed,
	// nothing gets padded.
	key := vncDESKey("password")

	want := []byte{
		reverseByte('p'), reverseByte('a'), reverseByte('s'), reverseByte('s'),
		reverseByte('w'), reverseByte('o'), reverseByte('r'), reverseByte('d'),
	}
	if !bytes.Equal(key, want) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "password", key, want)
	}

	// Shorter than 8 bytes: only the significant bytes are reversed, the
	// rest stays zero-padded (VNC's password truncation rule).
	short := vncDESKey("ab")

	wantShort := []byte{reverseByte('a'), reverseByte('b'), 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(short, wantShort) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "ab", short, wantShort)
	}

	// Longer than 8 bytes: only the first 8 are significant (matches
	// Proxmox's own truncation of the VNC ticket-as-password).
	long := vncDESKey("abcdefghij")

	wantLong := []byte{
		reverseByte('a'), reverseByte('b'), reverseByte('c'), reverseByte('d'),
		reverseByte('e'), reverseByte('f'), reverseByte('g'), reverseByte('h'),
	}
	if !bytes.Equal(long, wantLong) {
		t.Errorf("vncDESKey(%q) = %x, want %x", "abcdefghij", long, wantLong)
	}

	if got := len(vncDESKey("")); got != 8 {
		t.Errorf("vncDESKey(\"\") length = %d, want 8", got)
	}
}
