package cluster

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"pvmss/server/internal/store"
)

const (
	// MaxTLSPEMBytes caps each PEM field a client may submit.
	MaxTLSPEMBytes = 64 * 1024
	// maxTLSFingerprints caps the pinned-fingerprint list.
	maxTLSFingerprints = 64
	sha256HexLen       = sha256.Size * 2
)

// CertInfo is the non-secret summary of a stored certificate.
type CertInfo struct {
	Subject  string
	NotAfter time.Time
}

// parseCertificates decodes a PEM bundle that must hold only CERTIFICATE
// blocks (a stray private key in the CA field is refused, not stored).
func parseCertificates(field, data string) ([]*x509.Certificate, error) {
	if len(data) > MaxTLSPEMBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", field, MaxTLSPEMBytes)
	}
	var certs []*x509.Certificate
	rest := []byte(data)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("%s may only contain CERTIFICATE blocks, found %s", field, block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s holds an invalid certificate", field)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 || len(strings.TrimSpace(string(rest))) > 0 {
		return nil, fmt.Errorf("%s is not valid PEM certificate data", field)
	}
	return certs, nil
}

// NormalizeFingerprints parses SHA-256 fingerprints (one per line or comma
// separated, with or without colons) into lowercase hex lines.
func NormalizeFingerprints(raw string) ([]string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' || r == ' ' || r == '\r' || r == '\t' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToLower(strings.ReplaceAll(f, ":", ""))
		if raw, err := hex.DecodeString(f); err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("fingerprint %q is not a SHA-256 hex digest (%d hex characters)", truncate(f, 16), sha256HexLen)
		}
		out = append(out, f)
	}
	if len(out) > maxTLSFingerprints {
		return nil, fmt.Errorf("at most %d fingerprints are allowed", maxTLSFingerprints)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ValidateTLS checks submitted TLS material and returns it normalized
// (fingerprints canonicalized). Errors are safe to show to an admin: they
// never echo key material.
func ValidateTLS(t store.ClusterTLS) (store.ClusterTLS, error) {
	t.CAPEM = strings.TrimSpace(t.CAPEM)
	t.ClientCertPEM = strings.TrimSpace(t.ClientCertPEM)
	t.ClientKeyPEM = strings.TrimSpace(t.ClientKeyPEM)
	if t.CAPEM != "" {
		if _, err := parseCertificates("caPem", t.CAPEM); err != nil {
			return t, err
		}
	}
	fps, err := NormalizeFingerprints(t.NodeFingerprints)
	if err != nil {
		return t, err
	}
	t.NodeFingerprints = strings.Join(fps, "\n")
	if (t.ClientCertPEM == "") != (t.ClientKeyPEM == "") {
		return t, errors.New("client certificate and client key must be provided together")
	}
	if t.ClientCertPEM != "" {
		if len(t.ClientCertPEM) > MaxTLSPEMBytes || len(t.ClientKeyPEM) > MaxTLSPEMBytes {
			return t, fmt.Errorf("client certificate or key is larger than %d bytes", MaxTLSPEMBytes)
		}
		if _, err := tls.X509KeyPair([]byte(t.ClientCertPEM), []byte(t.ClientKeyPEM)); err != nil {
			return t, errors.New("client certificate and key are invalid or do not match")
		}
	}
	return t, nil
}

// CertSummary describes the first certificate of a PEM bundle ("" gives nil).
func CertSummary(data string) *CertInfo {
	certs, err := parseCertificates("pem", data)
	if err != nil || len(certs) == 0 {
		return nil
	}
	return &CertInfo{Subject: certs[0].Subject.String(), NotAfter: certs[0].NotAfter}
}

// BuildTLSConfig builds the tls.Config for a cluster: TLS 1.2 minimum;
// RootCAs from the stored CA; pinned SHA-256 leaf fingerprints, when given,
// replace chain and hostname checks (Proxmox node certificates are usually
// self-signed or issued for another name); an optional client certificate.
func BuildTLSConfig(t store.ClusterTLS, insecureSkipVerify bool) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12} //nolint:gosec // G402 false positive: floor is 1.2
	if insecureSkipVerify {
		cfg.InsecureSkipVerify = true //nolint:gosec // operator-configured per cluster; defaults to false
	} else {
		if err := applyTrust(cfg, t); err != nil {
			return nil, err
		}
	}
	if t.ClientCertPEM != "" {
		pair, err := tls.X509KeyPair([]byte(t.ClientCertPEM), []byte(t.ClientKeyPEM))
		if err != nil {
			return nil, errors.New("client certificate and key are invalid")
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

func applyTrust(cfg *tls.Config, t store.ClusterTLS) error {
	fps, err := NormalizeFingerprints(t.NodeFingerprints)
	if err != nil {
		return err
	}
	if len(fps) > 0 {
		cfg.InsecureSkipVerify = true //nolint:gosec // replaced by the fingerprint pin below
		cfg.VerifyPeerCertificate = pinVerifier(fps)
		return nil
	}
	if t.CAPEM == "" {
		return nil
	}
	certs, err := parseCertificates("caPem", t.CAPEM)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	for _, c := range certs {
		pool.AddCert(c)
	}
	cfg.RootCAs = pool
	return nil
}

func pinVerifier(fps []string) func([][]byte, [][]*x509.Certificate) error {
	return func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 {
			return fmt.Errorf("%w: server presented no certificate", ErrTLSVerify)
		}
		sum := sha256.Sum256(raw[0])
		got := hex.EncodeToString(sum[:])
		for _, want := range fps {
			if subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
				return nil
			}
		}
		return fmt.Errorf("%w: certificate fingerprint %s does not match any pinned fingerprint", ErrTLSVerify, got)
	}
}

// TLSFailureReason turns a TLS verification error into a short, specific
// message for the admin Test result.
func TLSFailureReason(err error) string {
	var unknown x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	switch {
	case errors.As(err, &unknown):
		return "the certificate is not signed by a trusted CA - add the cluster CA certificate (pve-root-ca.pem) in the TLS settings"
	case errors.As(err, &host):
		return "the certificate does not match the cluster host name - pin the node fingerprint or use the name in the certificate"
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		return "the certificate is expired or not yet valid"
	case strings.Contains(err.Error(), "does not match any pinned fingerprint"):
		return "the certificate fingerprint does not match any pinned fingerprint"
	default:
		return "the certificate could not be verified"
	}
}
