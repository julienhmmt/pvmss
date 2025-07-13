package proxmox

import (
	"crypto/tls"

	px "github.com/Telmate/proxmox-api-go/proxmox"
)

func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*px.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: insecureSkipVerify}

	client, err := px.NewClient(apiURL, nil, "", tlsConfig, "", 300)
	if err != nil {
		return nil, err
	}

	client.SetAPIToken(apiTokenID, apiTokenSecret)

	return client, nil
}
