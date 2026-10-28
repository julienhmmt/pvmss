package httpapi_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

func tlsTestPair(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "pve-root-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}

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

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_TLSNeverLeaksKey(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)
	cert, key := tlsTestPair(t)

	body, _ := json.Marshal(map[string]any{ //nolint:gosec // test fixture, not a credential
		"name": "tlsc", "url": "https://pve-t.example.com:8006/api2/json", "tokenId": "pvmss@pve!service", "tokenSecret": "s",
		"tls": map[string]string{"caPem": cert, "clientCertPem": cert, "clientKeyPem": key},
	})

	response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeCreate, HTTPMethod: http.MethodPost, Path: adminClustersPath, Body: string(body)})
	if response.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", response.Code, response.Body.String())
	}

	if strings.Contains(response.Body.String(), "PRIVATE KEY") || strings.Contains(response.Body.String(), strings.TrimSpace(key)[30:60]) {
		t.Fatalf("create response leaks key: %s", response.Body.String())
	}

	var dto struct {
		TLS struct {
			CASet        bool   `json:"caSet"`
			CASubject    string `json:"caSubject"`
			ClientKeySet bool   `json:"clientKeySet"`
		} `json:"tls"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}

	if !dto.TLS.CASet || !dto.TLS.ClientKeySet || !strings.Contains(dto.TLS.CASubject, "pve-root-ca") {
		t.Fatalf("unexpected tls dto: %+v", dto)
	}

	list := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeList, HTTPMethod: http.MethodGet, Path: adminClustersPath})
	if strings.Contains(list.Body.String(), "PRIVATE KEY") {
		t.Fatal("list response leaks key")
	}

	stored, err := fixture.store.GetCluster(context.Background(), "tlsc")
	if err != nil || stored.TLS.ClientKeyPEM != strings.TrimSpace(key) {
		t.Fatalf("key not round-tripped through encrypted store: %v", err)
	}
}

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_TLSRejectsGarbage(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)
	body := `{"name":"badtls","url":"https://pve-b.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"s","tls":{"caPem":"garbage"}}`

	response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeCreate, HTTPMethod: http.MethodPost, Path: adminClustersPath, Body: body})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}

	assertClusterErrorBody(t, response, "invalid_tls")
}
