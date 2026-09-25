package store_test

import (
	"context"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:gocyclo // linear put/get/list/delete round trip; splitting it would duplicate the store fixture
func TestCloudInitPublications_RoundTrip(t *testing.T) {
	t.Parallel()

	st := openClusterStore(t)
	ctx := context.Background()

	p := store.CloudInitPublication{
		Cluster: "c1", TemplateID: "web", Filename: "pvmss-tpl-web-abc.yml", ContentHash: "abc",
		PublishedAt: time.Now().UTC().Truncate(time.Millisecond),
		Nodes:       []store.NodePublication{{Node: "n1", OK: true}, {Node: "n2", Error: "offline"}},
	}
	if err := st.PutCloudInitPublication(ctx, p); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, found, err := st.GetCloudInitPublication(ctx, "c1", "web")
	if err != nil || !found {
		t.Fatalf("Get = %v/%v", found, err)
	}

	if got.Filename != p.Filename || len(got.Nodes) != 2 || got.Nodes[1].Error != "offline" || !got.PublishedAt.Equal(p.PublishedAt) {
		t.Fatalf("got %+v, want %+v", got, p)
	}

	p.Filename = "pvmss-tpl-web-def.yml"
	if err := st.PutCloudInitPublication(ctx, p); err != nil {
		t.Fatalf("Put (update): %v", err)
	}

	all, err := st.ListCloudInitPublications(ctx, "c1")
	if err != nil || len(all) != 1 || all["web"].Filename != p.Filename {
		t.Fatalf("List = %v/%v", all, err)
	}

	if err := st.DeleteCloudInitPublication(ctx, "c1", "web"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, found, _ := st.GetCloudInitPublication(ctx, "c1", "web"); found {
		t.Fatal("publication still present after delete")
	}
}

func TestVMCloudInitDocuments_RoundTrip(t *testing.T) {
	t.Parallel()

	st := openClusterStore(t)
	ctx := context.Background()

	if _, found, err := st.GetVMCloudInitDocument(ctx, "c1", 100); err != nil || found {
		t.Fatalf("empty Get = %v/%v", found, err)
	}

	if err := st.PutVMCloudInitDocument(ctx, "c1", 100, "web", "pvmss-tpl-web-abc.yml", "alice"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := st.PutVMCloudInitDocument(ctx, "c1", 100, store.BaselineTemplateID, "pvmss-baseline-abc.yml", "bob"); err != nil {
		t.Fatalf("Put (update): %v", err)
	}

	got, found, err := st.GetVMCloudInitDocument(ctx, "c1", 100)
	if err != nil || !found || got.TemplateID != store.BaselineTemplateID || got.UpdatedBy != "bob" {
		t.Fatalf("Get = %+v/%v/%v", got, found, err)
	}

	if err := st.DeleteVMCloudInitDocument(ctx, "c1", 100); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, found, _ := st.GetVMCloudInitDocument(ctx, "c1", 100); found {
		t.Fatal("row still present after delete")
	}
}
