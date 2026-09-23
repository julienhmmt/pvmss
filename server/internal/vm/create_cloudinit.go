package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
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

// resolvePlanDocument resolves, before any VMID is spent, the published
// file a new VM will use:
//   - a requested template must be enabled, published, and listed on the
//     node - anything else refuses the create;
//   - an image VM without a template gets the published baseline when it
//     is on the node, otherwise it boots on the native keys and skipReason
//     says why;
//   - the plain ISO/template path without a template needs nothing.
func resolvePlanDocument(ctx context.Context, deps CreateDeps, clusterName string, req CreateRequest, node string) (publishedDocument, string, error) {
	templateID := req.CloudInitTemplateID
	wantsTemplate := templateID != ""

	if !wantsTemplate && req.Image == nil {
		return publishedDocument{}, "", nil
	}

	// soft turns a failure into a skip for the baseline, a refusal for a
	// template the user explicitly chose.
	soft := func(sentinel, detail error) (publishedDocument, string, error) {
		if wantsTemplate {
			return publishedDocument{}, "", fmt.Errorf("%w: %w", sentinel, detail)
		}

		return publishedDocument{}, detail.Error(), nil
	}

	if wantsTemplate {
		if _, err := catalog.FindCloudInitTemplate(ctx, deps.Store, clusterName, templateID); err != nil {
			return publishedDocument{}, "", fmt.Errorf("%w: cloud-init template %q is not approved for this cluster", ErrNotApproved, templateID)
		}
	}

	if deps.Snippets == nil || deps.Pusher == nil {
		return soft(ErrClusterCreate, errors.New("no cloud-init resolver wired"))
	}

	storage, err := deps.Snippets.FindSnippetStorage(ctx, node)
	if err != nil {
		if errors.Is(err, cluster.ErrSnippetWriteUnavailable) {
			return soft(ErrCloudInitWriteUnavailable, err)
		}

		return soft(ErrNoSnippetStorage, fmt.Errorf("%s: enable the snippets content type of the cluster's snippet storage on this node (%w)", node, err))
	}

	filename, err := catalog.PublishedFile(ctx, deps.Store, clusterName, templateID)
	if err != nil {
		if errors.Is(err, catalog.ErrCloudInitTemplateNotPublished) {
			return soft(ErrCloudInitNotPublished, errors.New("the document has never been published - publish it in Admin > Cloud-init templates"))
		}

		return publishedDocument{}, "", err
	}

	present, err := deps.Pusher.HasSnippet(ctx, node, storage, filename)
	if err != nil {
		return soft(ErrClusterCreate, fmt.Errorf("list %s snippets on %s: %w", storage, node, err))
	}

	if !present {
		return soft(ErrCloudInitNotPublished, fmt.Errorf("%s:snippets/%s is not on node %s - republish in Admin > Cloud-init templates", storage, filename, node))
	}

	return publishedDocument{TemplateID: templateID, Storage: storage, Filename: filename}, "", nil
}

// attachPublishedDocument points the VM's cicustom (vendor-data) at the
// published file and records which file the VM uses. The file is shared:
// nothing is written here.
func attachPublishedDocument(ctx context.Context, deps CreateDeps, actor auth.Identity, clusterName, node string, vmid int, doc publishedDocument) error {
	if err := deps.Pusher.AttachCloudInitSnippet(ctx, node, doc.Storage, doc.Filename, vmid); err != nil {
		return fmt.Errorf("attach cloud-init document: %w", err)
	}

	if deps.Store == nil {
		return nil
	}

	templateID := doc.TemplateID
	if templateID == "" {
		templateID = store.BaselineTemplateID
	}

	if err := deps.Store.PutVMCloudInitDocument(ctx, clusterName, vmid, templateID, doc.Filename, actor.Username); err != nil {
		return fmt.Errorf("record cloud-init document: %w", err)
	}

	return nil
}

// applyCloudInitDocument attaches the plan-time document on the ISO and
// template paths. A failure lands on result.CloudInitPushError and keeps the
// VM stopped: the create task already ran and cannot be undone, and booting
// without the document would silently give the user a VM they did not ask
// for.
func applyCloudInitDocument(ctx context.Context, deps CreateDeps, actor auth.Identity, clusterName, node string, vmid int, doc publishedDocument, result *CreateResult) {
	if !doc.present() {
		return
	}

	result.CloudInitTemplateID = doc.TemplateID

	if err := attachPublishedDocument(ctx, deps, actor, clusterName, node, vmid, doc); err != nil {
		deps.Log.Error("cloud-init document attach failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "filename", doc.Filename, "error", err)
		result.CloudInitPushError = err.Error()
	}
}
