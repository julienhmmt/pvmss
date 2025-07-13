package proxmox

// GetISOList retrieves the list of ISO images from a specific storage on a specific node.
func GetISOList(client *Client, node string, storage string) (map[string]interface{}, error) {
	// We use our manual Get method which bypasses the faulty library.
	path := "/nodes/" + node + "/storage/" + storage + "/content"
	return client.Get(path)
}
