package proxmox

// GetVMBRs retrieves the list of network bridges from a specific node.
func GetVMBRs(client *Client, node string) (map[string]interface{}, error) {
	path := "/nodes/" + node + "/network"
	return client.Get(path)
}
