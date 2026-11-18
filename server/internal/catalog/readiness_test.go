package catalog_test

import (
	"pvmss/server/internal/catalog"
	"slices"
	"testing"
)

func TestNodeReadiness(t *testing.T) {
	t.Parallel()

	const (
		node  = "pve1"
		other = "pve2"
	)

	full := catalog.Resources{
		Nodes:    []catalog.Node{{Name: node}},
		Storages: []catalog.Storage{{Name: storageLocal, Node: node}},
		Bridges:  []catalog.Bridge{{Name: bridgeVMbr0, Node: node}},
		ISOs:     []catalog.ISO{{Storage: storageLocal, Node: node, File: "a.iso"}},
	}
	noISO := full
	noISO.ISOs = nil
	otherNode := catalog.Resources{
		Nodes:    full.Nodes,
		Storages: []catalog.Storage{{Name: storageLocal, Node: other}},
		Bridges:  []catalog.Bridge{{Name: bridgeVMbr0, Node: other}},
		ISOs:     []catalog.ISO{{Storage: storageLocal, Node: other, File: "a.iso"}},
	}

	cases := []struct {
		name      string
		resources catalog.Resources
		templates []catalog.Template
		want      []string
	}{
		{"ready with iso", full, nil, nil},
		{"ready with template on node", noISO, []catalog.Template{{VMID: 9000, Node: node}}, nil},
		{"template on another node does not count", noISO, []catalog.Template{{VMID: 9000, Node: other}}, []string{catalog.MissingISOOrTemplate}},
		{"resources on another node only", otherNode, nil, []string{catalog.MissingStorage, catalog.MissingBridge, catalog.MissingISOOrTemplate}},
		{"empty catalog", catalog.Resources{}, nil, []string{catalog.MissingStorage, catalog.MissingBridge, catalog.MissingISOOrTemplate}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := catalog.NodeReadiness(tc.resources, tc.templates, node); !slices.Equal(got, tc.want) {
				t.Errorf("missing = %v, want %v", got, tc.want)
			}
		})
	}
}
