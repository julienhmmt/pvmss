package cluster

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestProxmox_LogValueHidesToken(t *testing.T) {
	t.Parallel()

	const secret = "s3cr3t-token-value"

	var buf bytes.Buffer

	log := slog.New(slog.NewJSONHandler(&buf, nil))
	p := Proxmox{BaseURL: "https://pve:8006", APITokenName: "root@pam!x", APITokenValue: secret}

	log.Info("client", "proxmox", p)
	log.Info("client ptr", "proxmox", &p)

	if strings.Contains(buf.String(), secret) {
		t.Fatalf("token leaked: %s", buf.String())
	}
}
