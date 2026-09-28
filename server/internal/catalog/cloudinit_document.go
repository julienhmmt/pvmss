package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"pvmss/server/internal/cloudinit"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"regexp"
	"strings"
)

// publishedHashLen is the number of hex chars of the content hash kept in a
// document filename: enough to never collide within one cluster's
// templates, short enough to read in the Proxmox UI.
const publishedHashLen = 12

// PublishedContent is the exact file content of a template: the generated
// baseline with the template merged on top. An empty template is the
// standalone baseline.
func PublishedContent(templateContent string) (string, error) {
	return cloudinit.BuildVendorData(cloudinit.BaselineInputs{UserDocument: templateContent})
}

// IsBaseline reports whether key is the standalone baseline document key.
func IsBaseline(key string) bool { return key == store.BaselineTemplateID }

// DocumentKey maps a template id to its document key: the empty id - the
// baseline an image VM with no template uses - and the baseline key itself
// both map to store.BaselineTemplateID. Every other id is its own key.
func DocumentKey(templateID string) string {
	if templateID == "" || IsBaseline(templateID) {
		return store.BaselineTemplateID
	}

	return templateID
}

// PublishedFilename is the immutable, content-addressed file name of a
// document. Editing a template yields a new name, so VMs keep the exact
// file they booted with.
func PublishedFilename(templateID, content string) (filename, hash string) {
	sum := sha256.Sum256([]byte(content))
	hash = hex.EncodeToString(sum[:])

	base := "tpl-" + templateID
	if IsBaseline(templateID) {
		base = "baseline"
	}

	return fmt.Sprintf("pvmss-%s-%s.yml", base, hash[:publishedHashLen]), hash
}

// shellSafeRE is the only shape a storage id or file name may have to be
// spliced into WriteCommand: the admin runs it as root, so nothing that a
// shell would interpret gets through, whatever a stored row contains.
var shellSafeRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// heredocDelimiter ends the here-document of WriteCommand.
const heredocDelimiter = "PVMSS_EOF"

// WriteCommand is the shell command an admin pastes, as root, on each node
// that must offer the document: pvesm resolves the storage's snippets
// directory, and a quoted here-document writes the content verbatim (no
// expansion). PVMSS never runs it - the Proxmox API cannot write snippets.
func WriteCommand(storage, filename, content string) string {
	delimiter := heredocDelimiter
	for strings.Contains("\n"+content+"\n", "\n"+delimiter+"\n") {
		delimiter += "_"
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return fmt.Sprintf("F=$(pvesm path %s:snippets/%s) && mkdir -p \"${F%%/*}\" && cat > \"$F\" <<'%s'\n%s%s\n",
		storage, filename, delimiter, content, delimiter)
}

// DocumentStatus is one document as the admin sees it: the file to place,
// the command that places it, and where the Proxmox API lists it.
type DocumentStatus struct {
	TemplateID string
	Filename   string
	Command    string
	Nodes      []cluster.NodeSnippetStatus
}

// CheckCloudInitDocument computes a document's file (templateID
// store.BaselineTemplateID with empty content is the standalone baseline)
// and lists it on every node. The error means nothing could be checked.
func CheckCloudInitDocument(ctx context.Context, checker cluster.SnippetChecker, templateID, templateContent string) (DocumentStatus, error) {
	if checker == nil || checker.SnippetStorageID() == "" {
		return DocumentStatus{}, cluster.ErrSnippetWriteUnavailable
	}

	content, err := PublishedContent(templateContent)
	if err != nil {
		return DocumentStatus{}, fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	filename, _ := PublishedFilename(templateID, content)
	storage := checker.SnippetStorageID()

	if !shellSafeRE.MatchString(storage) || !shellSafeRE.MatchString(filename) {
		return DocumentStatus{}, fmt.Errorf("refusing unsafe snippet storage %q or file name %q", storage, filename)
	}

	nodes, err := checker.CheckSnippet(ctx, filename)
	if err != nil {
		return DocumentStatus{}, err
	}

	return DocumentStatus{
		TemplateID: templateID,
		Filename:   filename,
		Command:    WriteCommand(storage, filename, content),
		Nodes:      nodes,
	}, nil
}

// PublishedFile returns the filename a VM must use for a template, or for
// the baseline when templateID is empty, computed from the current content.
// Whether a node has it is proven separately (HasSnippet).
func PublishedFile(ctx context.Context, st *store.Store, clusterName, templateID string) (string, error) {
	key := DocumentKey(templateID)
	templateContent := ""

	if !IsBaseline(key) {
		tmpl, err := FindCloudInitTemplate(ctx, st, clusterName, templateID)
		if err != nil {
			return "", err
		}

		templateContent = tmpl.Content
	}

	content, err := PublishedContent(templateContent)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidCloudInitTemplate, err)
	}

	filename, _ := PublishedFilename(key, content)

	return filename, nil
}
