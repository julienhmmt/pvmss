package proxmox

import (
	"crypto/tls"

	px "github.com/Telmate/proxmox-api-go/proxmox"
)

type Client struct {
	*px.Client
}

func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: insecureSkipVerify}

	pxClient, err := px.NewClient(apiURL, nil, "", tlsConfig, "", 300)
	if err != nil {
		return nil, err
	}

	pxClient.SetAPIToken(apiTokenID, apiTokenSecret)

	return &Client{pxClient}, nil
}
