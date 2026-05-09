package proxmox

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMakeRestyClient_ReusesSingleton(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	a, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", true, 10*time.Second)
	if err != nil {
		t.Fatalf("first MakeRestyClient: %v", err)
	}
	b, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", true, 30*time.Second)
	if err != nil {
		t.Fatalf("second MakeRestyClient: %v", err)
	}

	if a.client != b.client {
		t.Fatalf("expected shared *resty.Client across MakeRestyClient calls; got %p vs %p", a.client, b.client)
	}
	if a.timeout == b.timeout {
		t.Fatalf("wrapper timeout must remain per-caller; both have %s", a.timeout)
	}
}

func TestMakeRestyClient_DistinctConfigsIsolated(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	a, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secretA", true, 10*time.Second)
	if err != nil {
		t.Fatalf("client A: %v", err)
	}
	b, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secretB", true, 10*time.Second)
	if err != nil {
		t.Fatalf("client B: %v", err)
	}

	if a.client == b.client {
		t.Fatalf("expected distinct *resty.Client for distinct token secrets")
	}
}

func TestMakeRestyClient_ConcurrentReturnsSameClient(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	const n = 32
	var wg sync.WaitGroup
	clients := make([]*RestyClient, n)
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			rc, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", false, time.Duration(i+1)*time.Second)
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
				return
			}
			clients[i] = rc
		}(i)
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		if clients[i] == nil || clients[0] == nil {
			t.Fatalf("nil client at index %d", i)
		}
		if clients[i].client != clients[0].client {
			t.Fatalf("goroutine %d got distinct client: %p vs %p", i, clients[i].client, clients[0].client)
		}
	}
}

func TestSharedTransport_ReusedAcrossClients(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	a, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secretA", true, 5*time.Second)
	if err != nil {
		t.Fatalf("client A: %v", err)
	}
	b, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secretB", true, 5*time.Second)
	if err != nil {
		t.Fatalf("client B: %v", err)
	}

	tA := a.client.GetClient().Transport
	tB := b.client.GetClient().Transport
	if tA != tB {
		t.Fatalf("expected shared transport for same skipVerify; got %p vs %p", tA, tB)
	}
}

func TestSharedTransport_DistinctSkipVerify(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	a, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", true, 5*time.Second)
	if err != nil {
		t.Fatalf("client A: %v", err)
	}
	b, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret2", false, 5*time.Second)
	if err != nil {
		t.Fatalf("client B: %v", err)
	}

	tA := a.client.GetClient().Transport
	tB := b.client.GetClient().Transport
	if tA == tB {
		t.Fatalf("expected distinct transports for differing insecureSkipVerify")
	}
}

func TestMakeRestyClientCookieAuth_PerCallClient(t *testing.T) {
	ResetSharedTransports()

	a, err := MakeRestyClientCookieAuth("https://pve.example.com:8006", true, 5*time.Second)
	if err != nil {
		t.Fatalf("cookie client A: %v", err)
	}
	b, err := MakeRestyClientCookieAuth("https://pve.example.com:8006", true, 5*time.Second)
	if err != nil {
		t.Fatalf("cookie client B: %v", err)
	}

	if a.client == b.client {
		t.Fatalf("cookie-auth clients must be per-call to avoid cross-user cookie leakage")
	}
	if a.client.GetClient().Transport != b.client.GetClient().Transport {
		t.Fatalf("cookie-auth clients must still share transport for pool reuse")
	}
}

func TestRestyClient_WithRequestTimeout(t *testing.T) {
	ResetTokenClients()
	ResetSharedTransports()

	rc, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", true, 5*time.Second)
	if err != nil {
		t.Fatalf("MakeRestyClient: %v", err)
	}

	// Test 1: No caller deadline - wrapper timeout applied
	ctx, cancel := rc.withRequestTimeout(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Error("expected deadline to be set when no caller deadline exists")
	}

	// Test 2: Tighter caller deadline - preserved
	tightCtx, tightCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer tightCancel()
	ctx2, cancel2 := rc.withRequestTimeout(tightCtx)
	defer cancel2()
	dl2, ok2 := ctx2.Deadline()
	if !ok2 {
		t.Error("expected deadline to be preserved")
	}
	remaining := time.Until(dl2)
	if remaining > 3*time.Second {
		t.Errorf("caller deadline should be preserved, not extended; got remaining time %v", remaining)
	}

	// Test 3: Zero timeout - no deadline applied
	rcZero, err := MakeRestyClient("https://pve.example.com:8006", "user@pve!t", "secret", true, 0)
	if err != nil {
		t.Fatalf("MakeRestyClient with zero timeout: %v", err)
	}
	ctx3, cancel3 := rcZero.withRequestTimeout(context.Background())
	defer cancel3()
	if _, ok3 := ctx3.Deadline(); ok3 {
		t.Error("expected no deadline when wrapper timeout is zero")
	}
}
