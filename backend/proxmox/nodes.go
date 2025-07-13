package proxmox

import (
	"context"
	"time"
)

func GetNodes(client *Client) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.GetNodeList(ctx)
}

// GetNodeNames retrieves all node names from Proxmox.
func GetNodeNames(client *Client) ([]string, error) {
	nodesResult, err := GetNodes(client)
	if err != nil {
		return nil, err
	}

	var nodeNames []string
	if nodesMap, ok := nodesResult.(map[string]interface{}); ok {
		if data, ok := nodesMap["data"].([]interface{}); ok {
			for _, item := range data {
				if nodeItem, ok := item.(map[string]interface{}); ok {
					if name, ok := nodeItem["node"].(string); ok {
						nodeNames = append(nodeNames, name)
					}
				}
			}
		}
	}
	return nodeNames, nil
}
