// Package nodesetup embeds the Proxmox node setup script so PVMSS can serve
// it to the nodes (GET /api/v1/pvmss-node-setup.sh): preparing a node then
// needs no copy of the repository. tools/pvmss-node-setup.sh is the same file
// for readers of the repository; a test keeps the two identical.
package nodesetup

import (
	"bytes"
	_ "embed"
)

//go:embed pvmss-node-setup.sh
var script []byte

// Script returns the node setup script. It is a copy: callers cannot alter
// the embedded bytes.
func Script() []byte {
	return bytes.Clone(script)
}
