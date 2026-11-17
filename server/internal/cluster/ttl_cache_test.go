package cluster

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLCache_ServesUntilExpiryAndNeverCachesErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[[]int, int](30*time.Second, func() time.Time { return now })

	var fetches int

	fetch := func() ([]int, error) {
		fetches++

		return []int{fetches}, nil
	}

	first, _ := cache.get("k", fetch)
	again, _ := cache.get("k", fetch)

	if fetches != 1 || again[0] != first[0] {
		t.Fatalf("within TTL: fetches = %d, got %v then %v", fetches, first, again)
	}

	again[0] = 99 // callers get a copy, the cache keeps its value

	if v, _ := cache.get("k", fetch); v[0] != 1 {
		t.Errorf("cached value mutated through a returned slice: %v", v)
	}

	now = now.Add(30 * time.Second)

	if v, _ := cache.get("k", fetch); fetches != 2 || v[0] != 2 {
		t.Errorf("after TTL: fetches = %d, value %v, want refetch", fetches, v)
	}

	boom := errors.New("boom")
	failing := func() ([]int, error) { fetches++; return nil, boom }

	if _, err := cache.get("e", failing); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	if _, err := cache.get("e", failing); !errors.Is(err, boom) || fetches != 4 {
		t.Errorf("errors must not be cached: fetches = %d", fetches)
	}
}

// Back-to-back admin page loads read each node's /network once.
func TestProxmox_ListBridges_Cached(t *testing.T) {
	t.Parallel()

	var networkReads atomic.Int32

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"node":"pve1","status":"online"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/network", func(w http.ResponseWriter, _ *http.Request) {
			networkReads.Add(1)
			writeJSONFixture(t, w, `{"data":[{"iface":"vmbr0","type":"bridge","active":1}]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	for range 3 {
		if bridges, err := p.ListBridges(context.Background()); err != nil || len(bridges) != 1 {
			t.Fatalf("ListBridges = %v, %v", bridges, err)
		}
	}

	if n := networkReads.Load(); n != 1 {
		t.Errorf("/network reads = %d, want 1", n)
	}
}
