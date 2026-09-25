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
	"sync"
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

// IsBaseline reports whether key is the standalone baseline document key.
func IsBaseline(key string) bool { return key == store.BaselineTemplateID }

// DocumentKey maps a template id to its cloudinit_publications key: the
// empty id - the baseline an image VM with no template uses - and the
// baseline key itself both map to store.BaselineTemplateID. Every other id
// is its own key.
func DocumentKey(templateID string) string {
	if templateID == "" || IsBaseline(templateID) {
		return store.BaselineTemplateID
	}

	return templateID
}

// PublishedFilename is the immutable, content-addressed file name of a
// published document. Editing a template yields a new name, so VMs keep
// the exact file they booted with.
func PublishedFilename(templateID, content string) (filename, hash string) {
	sum := sha256.Sum256([]byte(content))
	hash = hex.EncodeToString(sum[:])

	base := "tpl-" + templateID
	if IsBaseline(templateID) {
		base = "baseline"
	}

	return fmt.Sprintf("pvmss-%s-%s.yml", base, hash[:publishedHashLen]), hash
}

// PublishRequest names the document to publish on a cluster: the template id
// (store.BaselineTemplateID for the standalone baseline) and its content
// (empty for the baseline, whose content is generated).
type PublishRequest struct {
	Cluster    string
	TemplateID string
	Content    string
}

// PublishCloudInitDocument publishes one document (a template, or the
// baseline when TemplateID is store.BaselineTemplateID) to every node of
// the cluster and records the outcome. A per-node failure is recorded, not
// returned: the returned error means nothing could be attempted.
func PublishCloudInitDocument(ctx context.Context, st *store.Store, pub cluster.SnippetPublisher, req PublishRequest) (store.CloudInitPublication, error) {
	if pub == nil || !pub.PublishingEnabled() {
		return store.CloudInitPublication{}, cluster.ErrSnippetWriteUnavailable
	}

	content, err := PublishedContent(req.Content)
	if err != nil {
		return store.CloudInitPublication{}, fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	filename, hash := PublishedFilename(req.TemplateID, content)

	results, err := pub.PublishSnippet(ctx, filename, content)
	if err != nil {
		return store.CloudInitPublication{}, err
	}

	publication := store.CloudInitPublication{
		Cluster: req.Cluster, TemplateID: req.TemplateID, Filename: filename, ContentHash: hash,
		PublishedAt: time.Now().UTC(), Nodes: nodePublications(results),
	}

	if err := st.PutCloudInitPublication(ctx, publication); err != nil {
		return store.CloudInitPublication{}, err
	}

	return publication, nil
}

// nodePublications converts the cluster's per-node publish results into the
// stored form (the only place that mapping happens).
func nodePublications(results []cluster.NodePublishResult) []store.NodePublication {
	out := make([]store.NodePublication, len(results))
	for i, r := range results {
		out[i] = store.NodePublication{Node: r.Node, OK: r.OK, Error: r.Error}
	}

	return out
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

	baseline, err := PublishCloudInitDocument(ctx, st, pub, PublishRequest{Cluster: clusterName, TemplateID: store.BaselineTemplateID})
	if err != nil {
		// Nothing can be published if the baseline cannot (not configured,
		// nodes unreadable): stop here.
		return nil, err
	}

	out = append(out, baseline)

	for _, t := range templates {
		p, err := PublishCloudInitDocument(ctx, st, pub, PublishRequest{Cluster: clusterName, TemplateID: t.ID, Content: t.Content})
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
	// The empty id and the baseline key are both the standalone baseline: no
	// template to look up.
	if templateID != "" && !IsBaseline(templateID) {
		if _, err := FindCloudInitTemplate(ctx, st, clusterName, templateID); err != nil {
			return "", err
		}
	}

	publication, found, err := st.GetCloudInitPublication(ctx, clusterName, DocumentKey(templateID))
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

// Republisher runs cloud-init republication in the background: at startup (a
// PVMSS upgrade can change the generated baseline, a node may have been
// added) and after a cluster's publishing settings change. Runs are
// serialized per cluster, and the caller can Wait for every in-flight run
// before closing the store. Each run derives its timeout from the context
// passed to Republish, so a cancelled shutdown context stops them.
type Republisher struct {
	st      *store.Store
	clients cluster.ClientProvider
	log     *slog.Logger

	mu     sync.Mutex
	locks  map[string]*sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

// NewRepublisher builds a Republisher.
func NewRepublisher(st *store.Store, clients cluster.ClientProvider, log *slog.Logger) *Republisher {
	return &Republisher{st: st, clients: clients, log: log, locks: make(map[string]*sync.Mutex)}
}

// Republish starts one background run per named cluster that can publish and
// returns without waiting. Calls for the same cluster run one at a time;
// outcomes are logged (the admin sees them in Admin > Cloud-init templates).
// After Wait, Republish is a no-op.
func (r *Republisher) Republish(ctx context.Context, names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	for _, name := range names {
		client, err := r.clients.Client(name)
		if err != nil {
			continue
		}

		pub, ok := client.(cluster.SnippetPublisher)
		if !ok || !pub.PublishingEnabled() {
			continue
		}

		r.wg.Go(func() { r.republishCluster(ctx, name, pub) })
	}
}

func (r *Republisher) republishCluster(ctx context.Context, name string, pub cluster.SnippetPublisher) {
	lock := r.lockFor(name)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(ctx, publishAllTimeout)
	defer cancel()

	publications, err := PublishAllCloudInitDocuments(ctx, r.st, pub, name)
	if err != nil {
		r.log.Warn("cloud-init republication incomplete", "component", "catalog", "cluster", name, "published", len(publications), "error", err)

		return
	}

	r.log.Info("cloud-init documents republished", "component", "catalog", "cluster", name, "published", len(publications))
}

// lockFor returns the per-cluster serialization mutex, creating it once.
func (r *Republisher) lockFor(name string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()

	mu, ok := r.locks[name]
	if !ok {
		mu = &sync.Mutex{}
		r.locks[name] = mu
	}

	return mu
}

// Wait refuses further runs and blocks until every in-flight run finishes,
// so the caller can close the store safely. Cancel the base context first so
// runs in progress stop promptly.
func (r *Republisher) Wait() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()

	r.wg.Wait()
}
