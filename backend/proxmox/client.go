package proxmox

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	px "github.com/Telmate/proxmox-api-go/proxmox"
)

// Client is a wrapper around the Proxmox API client.
type Client struct {
	*px.Client
	HttpClient *http.Client
	AuthToken  string
}

// NewClient creates a new Proxmox API client.
func NewClient(apiURL, apiTokenID, apiTokenSecret string, insecureSkipVerify bool) (*Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	httpClient := &http.Client{Transport: tr}

	// We still use the library's client for other potential calls, but we'll use our own for GET.
	pxClient, err := px.NewClient(apiURL, httpClient, "", nil, "", 300)
	if err != nil {
		return nil, err
	}

	authToken := fmt.Sprintf("%s=%s", apiTokenID, apiTokenSecret)
	pxClient.SetAPIToken(apiTokenID, apiTokenSecret) // Set for library calls

	return &Client{pxClient, httpClient, authToken}, nil
}

// Get performs a direct, manual GET request to the Proxmox API, bypassing the library entirely.
func (c *Client) Get(path string) (map[string]interface{}, error) {
	url := c.ApiUrl + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.AuthToken))

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
