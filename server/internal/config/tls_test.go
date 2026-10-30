package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"strings"
	"testing"
	"time"
)

type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte
}

func issue(t *testing.T, cn string, parent *testCert, usage x509.ExtKeyUsage) testCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	signer, signerKey := tpl, key
	if parent == nil {
		tpl.IsCA, tpl.BasicConstraintsValid = true, true
		tpl.KeyUsage = x509.KeyUsageCertSign
	} else {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{usage}
		signer, signerKey = parent.cert, parent.key
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}

	c, _ := x509.ParseCertificate(der)

	return testCert{cert: c, key: key, der: der}
}

func writePEM(t *testing.T, dir, name string, c testCert) (certPath, keyPath string) {
	t.Helper()

	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")
	kb, _ := x509.MarshalECPrivateKey(c.key)

	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.der}), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		t.Fatal(err)
	}

	return certPath, keyPath
}

func TestLoad_TLSSettings(t *testing.T) {
	dir := t.TempDir()
	ca := issue(t, "ca", nil, 0)
	caPath, _ := writePEM(t, dir, "ca", ca)
	crt, key := writePEM(t, dir, "srv", issue(t, "srv", &ca, x509.ExtKeyUsageServerAuth))

	cases := []struct {
		name, cert, key, ca, auth, wantErr, wantMode string
	}{
		{name: "off", wantMode: "http"},
		{name: "tls", cert: crt, key: key, wantMode: "tls"},
		{name: "mtls default require", cert: crt, key: key, ca: caPath, wantMode: "mtls"},
		{name: "mtls optional", cert: crt, key: key, ca: caPath, auth: "optional", wantMode: "tls"},
		{name: "cert without key", cert: crt, wantErr: "must be set together"},
		{name: "ca without cert", ca: caPath, wantErr: "requires PVMSS_TLS_CERT_FILE"},
		{name: "auth without ca", cert: crt, key: key, auth: "require", wantErr: "requires PVMSS_TLS_CLIENT_CA_FILE"},
		{name: "bad auth", cert: crt, key: key, ca: caPath, auth: "maybe", wantErr: "must be"},
		{name: "missing file", cert: crt + ".nope", key: key, wantErr: "PVMSS_TLS_CERT_FILE"},
		{name: "ca not pem", cert: crt, key: key, ca: key, wantErr: "no PEM certificate"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setMetricsBaseEnv(t)
			t.Setenv("PVMSS_TLS_CERT_FILE", c.cert)
			t.Setenv("PVMSS_TLS_KEY_FILE", c.key)
			t.Setenv("PVMSS_TLS_CLIENT_CA_FILE", c.ca)
			t.Setenv("PVMSS_TLS_CLIENT_AUTH", c.auth)

			cfg, err := config.Load()
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want %q", err, c.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if got := cfg.TLSMode(); got != c.wantMode {
				t.Fatalf("TLSMode = %q, want %q", got, c.wantMode)
			}
		})
	}
}

func TestServerTLSConfig_RequiresClientCertificate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ca := issue(t, "ca", nil, 0)
	caPath, _ := writePEM(t, dir, "ca", ca)
	crt, key := writePEM(t, dir, "srv", issue(t, "srv", &ca, x509.ExtKeyUsageServerAuth))
	client := issue(t, "client", &ca, x509.ExtKeyUsageClientAuth)

	cfg := config.Configuration{TLSCertFile: crt, TLSKeyFile: key, TLSClientCAFile: caPath, TLSClientAuth: config.ClientAuthRequire}

	tlsCfg, err := cfg.ServerTLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	srv.TLS = tlsCfg

	srv.StartTLS()
	defer srv.Close()

	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)

	get := func(certs []tls.Certificate) error {
		c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: certs}}}

		resp, err := c.Get(srv.URL) //nolint:noctx // test
		if err == nil {
			_ = resp.Body.Close()
		}

		return err
	}

	if err := get(nil); err == nil {
		t.Fatal("request without client certificate succeeded")
	}

	if err := get([]tls.Certificate{{Certificate: [][]byte{client.der}, PrivateKey: client.key}}); err != nil {
		t.Fatalf("request with client certificate: %v", err)
	}
}
