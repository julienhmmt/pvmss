package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Client certificate modes for PVMSS_TLS_CLIENT_AUTH.
const (
	ClientAuthRequire  = "require"
	ClientAuthOptional = "optional"
)

// loadTLSSettings reads the optional native TLS listener settings. All unset
// keeps plain HTTP (TLS terminated by a reverse proxy). Cert and key enable
// HTTPS; a client CA adds mutual TLS. The files are loaded here so a bad path
// fails startup, not the first handshake.
func loadTLSSettings(cfg *Configuration) error {
	cfg.TLSCertFile = strings.TrimSpace(os.Getenv("PVMSS_TLS_CERT_FILE"))
	cfg.TLSKeyFile = strings.TrimSpace(os.Getenv("PVMSS_TLS_KEY_FILE"))
	cfg.TLSClientCAFile = strings.TrimSpace(os.Getenv("PVMSS_TLS_CLIENT_CA_FILE"))
	cfg.TLSClientAuth = strings.ToLower(strings.TrimSpace(os.Getenv("PVMSS_TLS_CLIENT_AUTH")))

	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return errors.New("PVMSS_TLS_CERT_FILE and PVMSS_TLS_KEY_FILE must be set together")
	}

	if cfg.TLSCertFile == "" && cfg.TLSClientCAFile != "" {
		return errors.New("PVMSS_TLS_CLIENT_CA_FILE requires PVMSS_TLS_CERT_FILE and PVMSS_TLS_KEY_FILE")
	}

	switch {
	case cfg.TLSClientCAFile == "" && cfg.TLSClientAuth != "":
		return errors.New("PVMSS_TLS_CLIENT_AUTH requires PVMSS_TLS_CLIENT_CA_FILE")
	case cfg.TLSClientCAFile != "" && cfg.TLSClientAuth == "":
		cfg.TLSClientAuth = ClientAuthRequire
	case cfg.TLSClientAuth != "" && cfg.TLSClientAuth != ClientAuthRequire && cfg.TLSClientAuth != ClientAuthOptional:
		return fmt.Errorf("PVMSS_TLS_CLIENT_AUTH must be %q or %q, got %q", ClientAuthRequire, ClientAuthOptional, cfg.TLSClientAuth)
	}

	_, err := cfg.ServerTLSConfig()

	return err
}

// ServerTLSConfig builds the listener's tls.Config; nil when TLS is off.
func (c Configuration) ServerTLSConfig() (*tls.Config, error) {
	if c.TLSCertFile == "" {
		return nil, nil //nolint:nilnil // nil config means plain HTTP
	}

	cert, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("PVMSS_TLS_CERT_FILE/PVMSS_TLS_KEY_FILE: %w", err)
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	if c.TLSClientCAFile == "" {
		return cfg, nil
	}

	pem, err := os.ReadFile(c.TLSClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("PVMSS_TLS_CLIENT_CA_FILE: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("PVMSS_TLS_CLIENT_CA_FILE holds no PEM certificate")
	}

	cfg.ClientCAs = pool
	cfg.ClientAuth = tls.RequireAndVerifyClientCert

	if c.TLSClientAuth == ClientAuthOptional {
		cfg.ClientAuth = tls.VerifyClientCertIfGiven
	}

	return cfg, nil
}

// TLSMode names the listener mode for the startup banner: http, tls or mtls.
func (c Configuration) TLSMode() string {
	switch {
	case c.TLSCertFile == "":
		return "http"
	case c.TLSClientCAFile == "" || c.TLSClientAuth == ClientAuthOptional:
		return "tls"
	default:
		return "mtls"
	}
}
