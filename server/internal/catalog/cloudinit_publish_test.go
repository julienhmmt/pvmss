package catalog_test

import (
	"context"
	"errors"
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

//nolint:paralleltest // serial: shared fake snippet state
func TestPublishAllCloudInitDocuments(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	st := openCatalogStore(t)
	ctx := context.Background()

	if _, err := catalog.PublishedFile(ctx, st, publishTestCluster, ""); !errors.Is(err, catalog.ErrCloudInitTemplateNotPublished) {
		t.Fatalf("baseline before publish = %v, want ErrCloudInitTemplateNotPublished", err)
	}

	tmpl, err := catalog.CreateCloudInitTemplate(ctx, st, publishTestCluster, "Web server", "#cloud-config\npackages:\n  - nginx\n")
	if err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	publications, err := catalog.PublishAllCloudInitDocuments(ctx, st, cluster.Fake{}, publishTestCluster)
	if err != nil || len(publications) != 2 {
		t.Fatalf("PublishAll = %d publications, err %v; want baseline + template", len(publications), err)
	}

	for _, id := range []string{"", tmpl.ID} {
		filename, err := catalog.PublishedFile(ctx, st, publishTestCluster, id)
		if err != nil {
			t.Fatalf("PublishedFile(%q): %v", id, err)
		}

		if present, _ := (cluster.Fake{}).HasSnippet(ctx, cluster.FakeNode02, cluster.FakeSnippetStorage, filename); !present {
			t.Errorf("%s not on %s", filename, cluster.FakeNode02)
		}
	}

	if err := catalog.SetCloudInitTemplateEnabled(ctx, st, publishTestCluster, tmpl.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, err := catalog.PublishedFile(ctx, st, publishTestCluster, tmpl.ID); !errors.Is(err, catalog.ErrCloudInitTemplateNotFound) {
		t.Errorf("disabled template file = %v, want ErrCloudInitTemplateNotFound", err)
	}

	if err := catalog.DeleteCloudInitTemplate(ctx, st, publishTestCluster, tmpl.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, found, _ := st.GetCloudInitPublication(ctx, publishTestCluster, tmpl.ID); found {
		t.Error("publication survived the template delete")
	}
}

func TestPublishCloudInitDocument_NotConfigured(t *testing.T) {
	t.Parallel()

	st := openCatalogStore(t)

	if _, err := catalog.PublishCloudInitDocument(context.Background(), st, nil, publishTestCluster, store.BaselineTemplateID, ""); !errors.Is(err, cluster.ErrSnippetWriteUnavailable) {
		t.Fatalf("err = %v, want ErrSnippetWriteUnavailable", err)
	}
}
