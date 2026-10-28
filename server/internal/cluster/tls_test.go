package cluster

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

// selfSignedPair returns a PEM cert and PEM EC key.
func selfSignedPair(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
}

func TestValidateTLS(t *testing.T) {
	t.Parallel()
	cert, key := selfSignedPair(t)
	otherCert, otherKey := selfSignedPair(t)
	_ = otherCert

	tests := []struct {
		name    string
		in      store.ClusterTLS
		wantErr string
	}{
		{"empty ok", store.ClusterTLS{}, ""},
		{"ca ok", store.ClusterTLS{CAPEM: cert}, ""},
		{"garbage ca", store.ClusterTLS{CAPEM: "not pem"}, "not valid PEM"},
		{"key in ca field", store.ClusterTLS{CAPEM: key}, "only contain CERTIFICATE"},
		{"oversize ca", store.ClusterTLS{CAPEM: strings.Repeat("a", MaxTLSPEMBytes+1)}, "larger than"},
		{"bad fingerprint", store.ClusterTLS{NodeFingerprints: "abc"}, "SHA-256"},
		{"cert without key", store.ClusterTLS{ClientCertPEM: cert}, "together"},
		{"mismatched pair", store.ClusterTLS{ClientCertPEM: cert, ClientKeyPEM: otherKey}, "do not match"},
		{"pair ok", store.ClusterTLS{ClientCertPEM: cert, ClientKeyPEM: key}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ValidateTLS(tt.in)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}

			if err != nil && strings.Contains(err.Error(), "-----BEGIN") {
				t.Fatalf("error leaks key material: %v", err)
			}
		})
	}
}

func TestNormalizeFingerprints(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("x"))
	h := hex.EncodeToString(sum[:])
	colon := strings.ToUpper(strings.Join(splitEvery(h, 2), ":"))

	got, err := NormalizeFingerprints(colon + "\n" + h)
	if err != nil || len(got) != 2 || got[0] != h || got[1] != h {
		t.Fatalf("got %v, %v", got, err)
	}
}

func splitEvery(s string, n int) []string {
	var out []string
	for i := 0; i < len(s); i += n {
		out = append(out, s[i:i+n])
	}

	return out
}

func TestBuildTLSConfig(t *testing.T) {
	t.Parallel()
	cert, key := selfSignedPair(t)

	cfg, err := BuildTLSConfig(store.ClusterTLS{CAPEM: cert, ClientCertPEM: cert, ClientKeyPEM: key}, false)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MinVersion != tls.VersionTLS12 || cfg.InsecureSkipVerify || cfg.RootCAs == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	insecure, _ := BuildTLSConfig(store.ClusterTLS{}, true)
	if !insecure.InsecureSkipVerify {
		t.Fatal("insecure flag lost")
	}
}

func TestTLSHandshakeAgainstCAAndPin(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer srv.Close()

	leaf := srv.Certificate()
	leafPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}))
	sum := sha256.Sum256(leaf.Raw)
	fp := hex.EncodeToString(sum[:])
	wrongCert, _ := selfSignedPair(t)

	get := func(material store.ClusterTLS) error {
		cfg, err := BuildTLSConfig(material, false)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
		if err != nil {
			return err
		}

		resp, err := newProxmoxHTTPClientTLS(cfg).Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}

		return err
	}

	if err := get(store.ClusterTLS{}); err == nil {
		t.Fatal("system roots must reject the test server")
	}

	if err := get(store.ClusterTLS{CAPEM: leafPEM}); err != nil {
		t.Fatalf("trusted node cert rejected: %v", err)
	}

	if err := get(store.ClusterTLS{CAPEM: wrongCert}); err == nil {
		t.Fatal("wrong CA accepted")
	}

	if err := get(store.ClusterTLS{NodeFingerprints: fp}); err != nil {
		t.Fatalf("pinned fingerprint rejected: %v", err)
	}

	err := get(store.ClusterTLS{NodeFingerprints: strings.Repeat("ab", 32)})
	if !errors.Is(err, ErrTLSVerify) {
		t.Fatalf("pin mismatch err = %v, want ErrTLSVerify", err)
	}

	if !strings.Contains(TLSFailureReason(err), "fingerprint") {
		t.Fatalf("reason = %q", TLSFailureReason(err))
	}
}
