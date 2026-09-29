package cluster

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestProxmoxCheckSnippet(t *testing.T) {
	t.Parallel()

	const file = "pvmss-tpl-web-0123456789ab.yml"

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"type":"cluster","name":"lab"},
				{"type":"node","name":"n2","online":1},
				{"type":"node","name":"n1","online":1},
				{"type":"node","name":"n3","online":0}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/n1/storage/local/content", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"volid":"local:snippets/`+file+`"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/n2/storage/local/content", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal, SnippetStorage: "local"}

	got, err := p.CheckSnippet(context.Background(), file)
	if err != nil {
		t.Fatalf("CheckSnippet: %v", err)
	}

	want := []NodeSnippetStatus{
		{Node: "n1", Present: true},
		{Node: "n2"},
		{Node: "n3", Error: "node is offline"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("node %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestProxmoxCheckSnippet_NoStorage(t *testing.T) {
	t.Parallel()

	if _, err := (Proxmox{}).CheckSnippet(context.Background(), "pvmss-x.yml"); !errors.Is(err, ErrSnippetWriteUnavailable) {
		t.Errorf("err = %v, want ErrSnippetWriteUnavailable", err)
	}
}
