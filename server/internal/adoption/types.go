// Package adoption detects cluster-side leftovers from a previous PVMSS
// instance (pvmss-* pools unknown to managed_pools, pvmss-tagged VMs, tags
// absent from catalog_tags) and re-registers the ones an administrator
// selects. It only writes PVMSS-side rows: the VMs, pools and tags themselves
// already exist on the Proxmox cluster.
package adoption

import "pvmss/server/internal/cluster"

// VMRef is the small view of a VM the report carries: enough to name it in
// the list the administrator reviews.
type VMRef struct {
	VMID   int
	Name   string
	Node   string
	Status string
	Pool   string
	Tags   []string
}

// OrphanPool is a pvmss-* pool present on the cluster that this instance
// does not manage, with its current member VMs.
type OrphanPool struct {
	Name    string
	Comment string
	Members []VMRef
}

// OrphanTag is a tag carried by at least one reported VM that catalog_tags
// does not know. Color is the suggested palette value applied on adoption.
type OrphanTag struct {
	Name    string
	VMCount int
	Color   string
}

// HiddenPool is a dismissed orphan pool reported as a summary only - the
// member list stays in the visible shape so hidden entries keep the payload
// small.
type HiddenPool struct {
	Name        string
	Comment     string
	MemberCount int
}

// HiddenTag is a dismissed orphan tag. It keeps the full OrphanTag shape -
// the suggested color is needed so a hidden tag can still be adopted.
type HiddenTag = OrphanTag

// HiddenLooseVM is a dismissed loose VM: enough to name it in the hidden
// list.
type HiddenLooseVM struct {
	VMID int
	Name string
}

// Report is the read-only result of Scan: every orphan candidate for one
// cluster. LooseVMs are pvmss-tagged VMs outside any pvmss-* pool - listed
// for visibility only, nothing is re-registered for them. The Hidden*
// slices carry the items the administrator dismissed via the recovery page;
// they stay in the report so hidden items are never silently forgotten.
type Report struct {
	Pools          []OrphanPool
	LooseVMs       []VMRef
	Tags           []OrphanTag
	HiddenPools    []HiddenPool
	HiddenLooseVMs []HiddenLooseVM
	HiddenTags     []HiddenTag
}

// Selection is the subset of the report the administrator chooses to adopt.
type Selection struct {
	Pools []string
	Tags  []string
}

// AdoptedTag is a catalog tag created (or already present) by Adopt.
type AdoptedTag struct {
	Name  string
	Color string
}

// Result lists what Adopt actually registered.
type Result struct {
	Pools []string
	Tags  []AdoptedTag
}

// palette assigns a deterministic color to each adopted tag by its position
// in the sorted orphan tag list. The previous instance's colors lived in its
// database and cannot be recovered; the administrator can recolor afterwards
// via the tags page. Same values as the recovery package's defaultPalette.
var palette = []string{
	"#3b82f6", // blue
	"#10b981", // emerald
	"#f59e0b", // amber
	"#ef4444", // red
	"#8b5cf6", // violet
	"#ec4899", // pink
	"#14b8a6", // teal
	"#f97316", // orange
}

func vmRef(machine cluster.VM) VMRef {
	return VMRef{
		VMID:   machine.VMID,
		Name:   machine.Name,
		Node:   machine.Node,
		Status: string(machine.Status),
		Pool:   machine.Pool,
		Tags:   machine.Tags,
	}
}
