package proxmox

// GetStorages retrieves the list of storages from Proxmox.
func GetStorages(client *Client) (interface{}, error) {
	return client.Get("/storage")
}
