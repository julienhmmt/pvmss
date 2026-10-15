package httpapi

import (
	"context"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
)

// adminNodeDocumentDTO is whether one node lists a document's file.
type adminNodeDocumentDTO struct {
	Node    string `json:"node"`
	Present bool   `json:"present"`
	Error   string `json:"error,omitempty"`
}

// adminDocumentDTO is a cloud-init document as the admin places it: the
// file, the command to paste on each chosen node, and where it is.
type adminDocumentDTO struct {
	Filename string                 `json:"filename"`
	Command  string                 `json:"command"`
	Nodes    []adminNodeDocumentDTO `json:"nodes"`
}

// documentFor computes a document's file and command and reads, live, which
// nodes list it through client. The returned message is set when nothing
// could be checked (feature off, nodes unreadable); per-node failures are
// in Nodes.
func documentFor(ctx context.Context, client cluster.Client, templateID, content string) (*adminDocumentDTO, string) {
	checker, ok := client.(cluster.SnippetChecker)
	if !ok {
		return nil, cluster.ErrSnippetWriteUnavailable.Error()
	}

	status, err := catalog.CheckCloudInitDocument(ctx, checker, templateID, content)
	if err != nil {
		return nil, err.Error()
	}

	nodes := make([]adminNodeDocumentDTO, len(status.Nodes))
	for i, n := range status.Nodes {
		nodes[i] = adminNodeDocumentDTO{Node: n.Node, Present: n.Present, Error: n.Error}
	}

	return &adminDocumentDTO{Filename: status.Filename, Command: status.Command, Nodes: nodes}, ""
}
