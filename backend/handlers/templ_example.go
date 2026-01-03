package handlers

import (
	"net/http"

	"pvmss/components"
)

func TemplExampleHandler(w http.ResponseWriter, r *http.Request) {
	vms := []components.VM{
		{
			VMID:   100,
			Name:   "web-server-prod",
			Status: "running",
			Node:   "pve-node1",
			CPU:    0.45,
			CPUs:   4,
			Memory: "8 GB",
			Disk:   "50 GB",
		},
		{
			VMID:   101,
			Name:   "db-server",
			Status: "running",
			Node:   "pve-node2",
			CPU:    0.72,
			CPUs:   8,
			Memory: "16 GB",
			Disk:   "200 GB",
		},
		{
			VMID:   102,
			Name:   "test-vm",
			Status: "stopped",
			Node:   "pve-node1",
			CPU:    0,
			CPUs:   2,
			Memory: "4 GB",
			Disk:   "20 GB",
		},
	}

	username := "demo-user"
	isAdmin := true

	components.IndexPage(vms, username, isAdmin).Render(r.Context(), w)
}
