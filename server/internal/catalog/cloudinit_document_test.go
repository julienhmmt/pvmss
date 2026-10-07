package catalog_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

const publishTestCluster = "default"

func TestPublishedContent_MergesBaselineUnderTemplate(t *testing.T) {
	t.Parallel()

	content, err := catalog.PublishedContent("#cloud-config\npackages:\n  - nmap\nruncmd:\n  - echo hello\n")
	if err != nil {
		t.Fatalf("PublishedContent: %v", err)
	}

	for _, want := range []string{"qemu-guest-agent", "nmap", "echo hello"} {
		if !strings.Contains(content, want) {
			t.Errorf("published content missing %q:\n%s", want, content)
		}
	}
}

func TestPublishedFilename_ContentAddressed(t *testing.T) {
	t.Parallel()

	a, _ := catalog.PublishedFilename("web", "#cloud-config\na: 1\n")
	b, _ := catalog.PublishedFilename("web", "#cloud-config\na: 2\n")
	again, _ := catalog.PublishedFilename("web", "#cloud-config\na: 1\n")
	base, _ := catalog.PublishedFilename(store.BaselineTemplateID, "#cloud-config\n")

	if a == b || a != again {
		t.Errorf("filenames %q / %q / %q: want stable per content, new per edit", a, b, again)
	}

	if !strings.HasPrefix(a, "pvmss-tpl-web-") || !strings.HasPrefix(base, "pvmss-baseline-") || !strings.HasSuffix(a, ".yml") {
		t.Errorf("unexpected shapes %q, %q", a, base)
	}
}

// TestWriteCommand_WritesContentVerbatim runs the pasted command with sh and
// a stand-in pvesm, and proves the file holds the exact content.
func TestWriteCommand_WritesContentVerbatim(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pvesm := filepath.Join(dir, "pvesm")
	script := "#!/bin/sh\n# pvesm path <storage>:snippets/<name>\necho \"" + dir + "/snippets/${2#*:snippets/}\"\n"

	if err := os.WriteFile(pvesm, []byte(script), 0o700); err != nil { //nolint:gosec // test stand-in executable
		t.Fatal(err)
	}

	content := "#cloud-config\nruncmd:\n  - echo \"$HOME `id` $(date)\"\nPVMSS_EOF\n"
	cmd := catalog.WriteCommand("local", "pvmss-tpl-web-0123456789ab.yml", content)

	run := exec.CommandContext(context.Background(), "sh", "-c", cmd) //nolint:gosec // command under test

	run.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))

	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("command failed: %v\n%s\n%s", err, out, cmd)
	}

	got, err := os.ReadFile(filepath.Join(dir, "snippets", "pvmss-tpl-web-0123456789ab.yml")) //nolint:gosec // test temp dir
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != content {
		t.Errorf("file = %q, want %q", got, content)
	}
}

//nolint:gocyclo,paralleltest // serial: shared fake snippet state
func TestCheckCloudInitDocument(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	st := openCatalogStore(t)
	ctx := context.Background()

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, publishTestCluster, "Web server", "#cloud-config\npackages:\n  - nginx\n")
	if err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	status, err := catalog.CheckCloudInitDocument(ctx, cluster.Fake{}, tmpl.ID, tmpl.Content)
	if err != nil {
		t.Fatalf("CheckCloudInitDocument: %v", err)
	}

	filename, err := catalog.PublishedFile(ctx, st, publishTestCluster, tmpl.ID)
	if err != nil || filename != status.Filename {
		t.Fatalf("PublishedFile = %q/%v, want the checked file %q", filename, err, status.Filename)
	}

	if !strings.Contains(status.Command, cluster.FakeSnippetStorage+":snippets/"+filename) || !strings.Contains(status.Command, "nginx") {
		t.Errorf("command does not write %s with the template:\n%s", filename, status.Command)
	}

	if len(status.Nodes) == 0 || !status.Nodes[0].Present {
		t.Errorf("nodes = %+v, want present on the demo fake", status.Nodes)
	}

	cluster.SetFakeSnippetVisibility(false)

	status, _ = catalog.CheckCloudInitDocument(ctx, cluster.Fake{}, tmpl.ID, tmpl.Content)
	if status.Nodes[0].Present {
		t.Error("file reported present before it was pasted")
	}

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, publishTestCluster, tmpl.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, err := catalog.PublishedFile(ctx, st, publishTestCluster, tmpl.ID); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Errorf("disabled template file = %v, want ErrCloudInitTemplateNotFound", err)
	}

	if base, err := catalog.PublishedFile(ctx, st, publishTestCluster, ""); err != nil || !strings.HasPrefix(base, "pvmss-baseline-") {
		t.Errorf("baseline file = %q/%v", base, err)
	}
}

// unsafeStorageChecker reports a storage id a shell would interpret.
type unsafeStorageChecker struct{ cluster.Fake }

func (unsafeStorageChecker) SnippetStorageID() string { return "local;reboot" }

func TestCheckCloudInitDocument_RefusesUnsafeStorage(t *testing.T) {
	t.Parallel()

	if _, err := catalog.CheckCloudInitDocument(context.Background(), unsafeStorageChecker{}, "web", ""); err == nil {
		t.Fatal("unsafe storage id accepted into the pasted command")
	}
}

func TestCheckCloudInitDocument_NotConfigured(t *testing.T) {
	t.Parallel()

	if _, err := catalog.CheckCloudInitDocument(context.Background(), nil, "web", ""); !errors.Is(err, cluster.ErrSnippetWriteUnavailable) {
		t.Errorf("err = %v, want ErrSnippetWriteUnavailable", err)
	}
}
