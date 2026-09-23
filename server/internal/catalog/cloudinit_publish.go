package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"time"
)

// ErrCloudInitTemplateNotPublished reports a template (or the baseline) that
// has no published file on the cluster yet.
var ErrCloudInitTemplateNotPublished = errors.New("cloud-init document is not published on this cluster")

// publishedHashLen is the number of hex chars of the content hash kept in a
// published filename: enough to never collide within one cluster's
// templates, short enough to read in the Proxmox UI.
const publishedHashLen = 12

// PublishedContent is the exact file content published for a template: the
// generated baseline with the template merged on top. An empty template is
// the standalone baseline.
func PublishedContent(templateContent string) (string, error) {
	return cloudinit.BuildVendorData(cloudinit.BaselineInputs{UserDocument: templateContent})
}

// PublishedFilename is the immutable, content-addressed file name of a
// published document. Editing a template yields a new name, so VMs keep
// the exact file they booted with.
func PublishedFilename(templateID, content string) (filename, hash string) {
	sum := sha256.Sum256([]byte(content))
	hash = hex.EncodeToString(sum[:])

	base := "tpl-" + templateID
	if templateID == store.BaselineTemplateID {
		base = "baseline"
	}

	return fmt.Sprintf("pvmss-%s-%s.yml", base, hash[:publishedHashLen]), hash
}

// PublishCloudInitDocument publishes one document (a template, or the
// baseline when templateID is store.BaselineTemplateID) to every node of
// the cluster and records the outcome. A per-node failure is recorded, not
// returned: the returned error means nothing could be attempted.
func PublishCloudInitDocument(ctx context.Context, st *store.Store, pub cluster.SnippetPublisher, clusterName, templateID, templateContent string) (store.CloudInitPublication, error) {
	if pub == nil || !pub.PublishingEnabled() {
		return store.CloudInitPublication{}, cluster.ErrSnippetWriteUnavailable
	}

	content, err := PublishedContent(templateContent)
	if err != nil {
		return store.CloudInitPublication{}, fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	filename, hash := PublishedFilename(templateID, content)

	results, err := pub.PublishSnippet(ctx, filename, content)
	if err != nil {
		return store.CloudInitPublication{}, err
	}

	publication := store.CloudInitPublication{
		Cluster: clusterName, TemplateID: templateID, Filename: filename, ContentHash: hash,
		PublishedAt: time.Now().UTC(), Nodes: make([]store.NodePublication, len(results)),
	}

	for i, r := range results {
		publication.Nodes[i] = store.NodePublication{Node: r.Node, OK: r.OK, Error: r.Error}
	}

	if err := st.PutCloudInitPublication(ctx, publication); err != nil {
		return store.CloudInitPublication{}, err
	}

	return publication, nil
}

// PublishAllCloudInitDocuments republishes the baseline and every enabled
// template of the cluster (resync after adding a node or upgrading PVMSS,
// whose generated baseline changes the published content). Errors of
// individual templates are collected; the returned slice holds what was
// published.
func PublishAllCloudInitDocuments(ctx context.Context, st *store.Store, pub cluster.SnippetPublisher, clusterName string) ([]store.CloudInitPublication, error) {
	templates, err := CloudInitTemplates(ctx, st, clusterName)
	if err != nil {
		return nil, err
	}

	var (
		out  []store.CloudInitPublication
		errs []error
	)

	baseline, err := PublishCloudInitDocument(ctx, st, pub, clusterName, store.BaselineTemplateID, "")
	if err != nil {
		// Nothing can be published if the baseline cannot (not configured,
		// nodes unreadable): stop here.
		return nil, err
	}

	out = append(out, baseline)

	for _, t := range templates {
		p, err := PublishCloudInitDocument(ctx, st, pub, clusterName, t.ID, t.Content)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.ID, err))

			continue
		}

		out = append(out, p)
	}

	return out, errors.Join(errs...)
}

// PublishedFile returns the published filename of a template, or of the
// baseline when templateID is empty. A template must also be enabled.
func PublishedFile(ctx context.Context, st *store.Store, clusterName, templateID string) (string, error) {
	key := templateID
	if templateID == "" {
		key = store.BaselineTemplateID
	} else if _, err := FindCloudInitTemplate(ctx, st, clusterName, templateID); err != nil {
		return "", err
	}

	publication, found, err := st.GetCloudInitPublication(ctx, clusterName, key)
	if err != nil {
		return "", err
	}

	if !found {
		return "", ErrCloudInitTemplateNotPublished
	}

	return publication.Filename, nil
}

// publishAllTimeout bounds one background republication of a cluster.
const publishAllTimeout = 3 * time.Minute

// RepublishInBackground republishes every document of each named cluster
// whose client publishes, without blocking the caller: at startup (a PVMSS
// upgrade can change the generated baseline, a node may have been added)
// and after the cluster's publishing settings change. Outcomes are logged;
// the admin sees them in Admin > Cloud-init templates.
func RepublishInBackground(st *store.Store, clients cluster.ClientProvider, names []string, log *slog.Logger) {
	for _, name := range names {
		client, err := clients.Client(name)
		if err != nil {
			continue
		}

		pub, ok := client.(cluster.SnippetPublisher)
		if !ok || !pub.PublishingEnabled() {
			continue
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), publishAllTimeout)
			defer cancel()

			publications, err := PublishAllCloudInitDocuments(ctx, st, pub, name)
			if err != nil {
				log.Warn("cloud-init republication incomplete", "component", "catalog", "cluster", name, "published", len(publications), "error", err)

				return
			}

			log.Info("cloud-init documents republished", "component", "catalog", "cluster", name, "published", len(publications))
		}()
	}
}
