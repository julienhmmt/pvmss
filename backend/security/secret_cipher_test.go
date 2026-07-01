package security

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "a-32-byte-or-longer-session-secret-value"
	plaintext := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----\n"

	enc, err := EncryptSecret(plaintext, secret)
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if !IsEncrypted(enc) {
		t.Fatalf("expected encrypted prefix, got %q", enc)
	}
	if enc == plaintext {
		t.Fatal("ciphertext equals plaintext")
	}

	got, err := DecryptSecret(enc, secret)
	if err != nil {
		t.Fatalf("DecryptSecret: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptEmptyReturnsEmpty(t *testing.T) {
	enc, err := EncryptSecret("", "secret")
	if err != nil {
		t.Fatalf("EncryptSecret empty: %v", err)
	}
	if enc != "" {
		t.Fatalf("expected empty, got %q", enc)
	}
}

func TestDecryptPlaintextPassthrough(t *testing.T) {
	// A value without the prefix is returned unchanged.
	got, err := DecryptSecret("not-encrypted", "secret")
	if err != nil {
		t.Fatalf("DecryptSecret passthrough: %v", err)
	}
	if got != "not-encrypted" {
		t.Fatalf("got %q", got)
	}
}

func TestDecryptWrongSecretFails(t *testing.T) {
	enc, err := EncryptSecret("data", "correct-secret-correct-secret-32b")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if _, err := DecryptSecret(enc, "wrong-secret-wrong-secret-wrong-32"); err == nil {
		t.Fatal("expected decrypt failure with wrong secret")
	}
}

func TestEncryptEmptySecretFails(t *testing.T) {
	if _, err := EncryptSecret("data", ""); err == nil {
		t.Fatal("expected error with empty secret")
	}
}
