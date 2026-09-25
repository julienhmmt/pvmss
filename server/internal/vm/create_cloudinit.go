package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
)

// ErrCloudInitNotPublished - the requested cloud-init template has no
// published file on the VM's node (never published, or published while the
// node was offline). The admin republishes from Admin > Cloud-init
// templates; nothing was created.
var ErrCloudInitNotPublished = errors.New("cloud-init template is not published on the selected node")

// publishedDocument is an admin-published cloud-init file resolved at plan
// time and proven present on the VM's node. TemplateID is empty for the
// standalone baseline. The zero value means "no document".
type publishedDocument struct {
	TemplateID string
	Storage    string
	Filename   string
}

func (d publishedDocument) present() bool { return d.Filename != "" }

// documentTarget identifies the VM a cloud-init document belongs to: the
// (cluster, node, vmid) triple that travels together through the create
// paths. VMID is 0 at plan time and set once the VMID is allocated.
type documentTarget struct {
	Cluster string
	Node    string
	VMID    int
}

// resolvePlanDocument resolves, before any VMID is spent, the published
// file a new VM will use:
//   - a requested template must be enabled, published, and listed on the
//     node - anything else refuses the create;
//   - an image VM without a template gets the published baseline when it
//     is on the node, otherwise it boots on the native keys and skipReason
//     says why;
//   - the plain ISO/template path without a template needs nothing.
func resolvePlanDocument(ctx context.Context, deps CreateDeps, target documentTarget, req CreateRequest) (publishedDocument, string, error) {
	templateID := req.CloudInitTemplateID

	if templateID == "" && req.Image == nil {
		return publishedDocument{}, "", nil
	}

	if templateID != "" {
		if _, err := catalog.FindCloudInitTemplate(ctx, deps.Store, target.Cluster, templateID); err != nil {
			return publishedDocument{}, "", fmt.Errorf("%w: cloud-init template %q is not approved for this cluster", ErrNotApproved, templateID)
		}
	}

	doc, err := locatePublishedDocument(ctx, deps, target, templateID)
	if err != nil {
		// A chosen template is refused; the baseline (image VM, no template)
		// degrades to a skip reason and the VM boots on its native keys.
		if templateID != "" {
			return publishedDocument{}, "", err
		}

		return publishedDocument{}, err.Error(), nil
	}

	return doc, "", nil
}

// locatePublishedDocument resolves the published file a VM will use and
// proves it is on the VM's node. A non-nil error is the reason nothing can
// be attached: a refusal for a chosen template, a skip reason for the
// baseline.
func locatePublishedDocument(ctx context.Context, deps CreateDeps, target documentTarget, templateID string) (publishedDocument, error) {
	if deps.Snippets == nil || deps.Pusher == nil {
		return publishedDocument{}, fmt.Errorf("%w: no cloud-init resolver wired", ErrClusterCreate)
	}

	storage, err := deps.Snippets.FindSnippetStorage(ctx, target.Node)
	if err != nil {
		if errors.Is(err, cluster.ErrSnippetWriteUnavailable) {
			return publishedDocument{}, fmt.Errorf("%w: %w", ErrCloudInitWriteUnavailable, err)
		}

		return publishedDocument{}, fmt.Errorf("%w: %s: enable the snippets content type of the cluster's snippet storage on this node (%w)", ErrNoSnippetStorage, target.Node, err)
	}

	filename, err := catalog.PublishedFile(ctx, deps.Store, target.Cluster, templateID)
	if err != nil {
		if errors.Is(err, catalog.ErrCloudInitTemplateNotPublished) {
			return publishedDocument{}, fmt.Errorf("%w: the document has never been published - publish it in Admin > Cloud-init templates", ErrCloudInitNotPublished)
		}

		return publishedDocument{}, err
	}

	present, err := deps.Pusher.HasSnippet(ctx, target.Node, storage, filename)
	if err != nil {
		return publishedDocument{}, fmt.Errorf("%w: list %s snippets on %s: %w", ErrClusterCreate, storage, target.Node, err)
	}

	if !present {
		return publishedDocument{}, fmt.Errorf("%w: %s:snippets/%s is not on node %s - republish in Admin > Cloud-init templates", ErrCloudInitNotPublished, storage, filename, target.Node)
	}

	return publishedDocument{TemplateID: templateID, Storage: storage, Filename: filename}, nil
}

// attachPublishedDocument points the VM's cicustom (vendor-data) at the
// published file and records which file the VM uses. The file is shared:
// nothing is written here.
func attachPublishedDocument(ctx context.Context, deps CreateDeps, actor auth.Identity, target documentTarget, doc publishedDocument) error {
	if err := deps.Pusher.AttachCloudInitSnippet(ctx, target.Node, doc.Storage, doc.Filename, target.VMID); err != nil {
		return fmt.Errorf("attach cloud-init document: %w", err)
	}

	if deps.Store == nil {
		return nil
	}

	if err := deps.Store.PutVMCloudInitDocument(ctx, target.Cluster, target.VMID, catalog.DocumentKey(doc.TemplateID), doc.Filename, actor.Username); err != nil {
		return fmt.Errorf("record cloud-init document: %w", err)
	}

	return nil
}

// applyCloudInitDocument attaches the plan-time document on the ISO and
// template paths. A failure lands on result.CloudInitPushError and keeps the
// VM stopped: the create task already ran and cannot be undone, and booting
// without the document would silently give the user a VM they did not ask
// for.
func applyCloudInitDocument(ctx context.Context, deps CreateDeps, actor auth.Identity, target documentTarget, doc publishedDocument, result *CreateResult) {
	if !doc.present() {
		return
	}

	result.CloudInitTemplateID = doc.TemplateID

	if err := attachPublishedDocument(ctx, deps, actor, target, doc); err != nil {
		deps.Log.Error("cloud-init document attach failed", "component", "vm", "cluster", target.Cluster, "vmid", target.VMID, "filename", doc.Filename, "error", err)
		result.CloudInitPushError = err.Error()
	}
}
