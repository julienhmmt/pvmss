// Package vm — T10 console ticket store and Resolve()-gated ticket issuance.
//
// Per AC01 (see specs/011-t10-console-vnc/spec.md "Why this tranche exists"),
// the VNC ticket store stays in process memory: a map, a mutex, a TTL, and
// oldest-eviction when the fixed capacity is reached. It is never persisted
// to SQLite or any store shared between replicas — PVMSS is single-instance
// by design (SQLite on a ReadWriteOnce volume), so externalizing the store
// would add write contention for zero functional gain.
package vm

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"slices"
	"sync"
	"time"
)

// TicketTTL is the hardcoded validity window for a console ticket (plan.md
// Constraints). Long enough to cover the WebSocket upgrade round-trip, short
// enough that a stale, unconsumed ticket is not a standing capability.
// Exported so the HTTP handler can include expiresInSeconds in the response
// (contracts/vm-console.md).
const TicketTTL = 30 * time.Second

// ticketStoreCapacity caps the number of outstanding tickets. When the cap is
// reached, the oldest entry is evicted before inserting a new one — B11's
// "TTL + éviction du plus ancien", unchanged.
const ticketStoreCapacity = 256

// ErrInvalidTicket is returned by ConsoleTicketStore.Consume when the token is
// missing, expired, already consumed, or bound to a different (cluster, vmid)
// than the one in the WebSocket URL. The WebSocket handler maps this to a 400
// without upgrading the connection (FR-004, FR-005).
var ErrInvalidTicket = errors.New("invalid console ticket")

// ErrClusterConsoleUnavailable is returned by GetConsoleTicket when the
// cluster client's GetVNCTicket call fails — Proxmox is unreachable, the VM
// is not running, etc. The HTTP handler maps this to 502 console_unavailable
// (contracts/vm-console.md).
var ErrClusterConsoleUnavailable = errors.New("console unavailable")

// VNCTicket is an in-memory, single-use, TTL-bound capability binding an
// opaque token to (cluster, vmid, node, ProxmoxTicket, port). The client ever
// sees only the Token; every other field stays server-side (FR-002, FR-003).
// Never persisted, never serialized to a store (AC01).
type VNCTicket struct {
	Token         string
	Cluster       string
	VMID          int
	Node          string
	ProxmoxTicket string
	Port          int
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

// ConsoleTicketStore is the in-memory ticket store (AC01). Constructed once in
// main.go and passed into httpapi alongside every other dependency — no
// package-level singleton, no global mutable state.
type ConsoleTicketStore struct {
	mu      sync.Mutex
	tickets map[string]VNCTicket
	order   []string
}

// NewConsoleTicketStore creates an empty store.
func NewConsoleTicketStore() *ConsoleTicketStore {
	return &ConsoleTicketStore{tickets: make(map[string]VNCTicket), order: make([]string, 0, ticketStoreCapacity)}
}

// Issue generates an opaque, cryptographically random token, stores the entry,
// appends to the insertion-order slice, and evicts the oldest entry if the
// fixed capacity is exceeded (FR-003, B11). Returns the new ticket.
func (s *ConsoleTicketStore) Issue(clusterName string, vmid int, node, proxmoxTicket string, port int) VNCTicket {
	token, err := generateConsoleToken()
	if err != nil {
		// crypto/rand failure is a fatal environment condition — surface it as
		// a ticket with an empty token so the caller can detect it, and let the
		// HTTP layer turn it into a 500. This path is not reachable in practice.
		return VNCTicket{Cluster: clusterName, VMID: vmid, Node: node, ProxmoxTicket: proxmoxTicket, Port: port}
	}

	now := time.Now()
	ticket := VNCTicket{
		Token:         token,
		Cluster:       clusterName,
		VMID:          vmid,
		Node:          node,
		ProxmoxTicket: proxmoxTicket,
		Port:          port,
		IssuedAt:      now,
		ExpiresAt:     now.Add(TicketTTL),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tickets[token] = ticket
	s.order = append(s.order, token)

	if len(s.tickets) > ticketStoreCapacity {
		s.evictOldestLocked()
	}

	return ticket
}

// Consume looks up the token. Missing, expired, or a (cluster, vmid) mismatch
// against what was bound at issuance → ErrInvalidTicket. On success, the entry
// is deleted from the map BEFORE returning it — a concurrent second Consume
// for the same token cannot observe it as still present, closing the
// single-use race (FR-004). A mismatched Consume does NOT consume the ticket:
// it remains valid for its real (cluster, vmid).
func (s *ConsoleTicketStore) Consume(token, clusterName string, vmid int) (VNCTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[token]
	if !ok {
		return VNCTicket{}, ErrInvalidTicket
	}

	if time.Now().After(ticket.ExpiresAt) {
		s.removeOrderLocked(token)
		delete(s.tickets, token)
		return VNCTicket{}, ErrInvalidTicket
	}

	if ticket.Cluster != clusterName || ticket.VMID != vmid {
		return VNCTicket{}, ErrInvalidTicket
	}

	s.removeOrderLocked(token)
	delete(s.tickets, token)
	return ticket, nil
}

// evictOldestLocked removes the oldest live token from the map and the order
// slice. The order slice is kept in sync with the map on Consume, so the front
// of the slice is always the oldest live entry. Called only with s.mu held.
func (s *ConsoleTicketStore) evictOldestLocked() {
	if len(s.order) == 0 {
		return
	}

	oldest := s.order[0]
	s.order = s.order[1:]
	delete(s.tickets, oldest)
}

// removeOrderLocked removes token from the insertion-order slice. Called only
// with s.mu held.
func (s *ConsoleTicketStore) removeOrderLocked(token string) {
	if i := slices.Index(s.order, token); i >= 0 {
		s.order = slices.Delete(s.order, i, i+1)
	}
}

// generateConsoleToken returns a 32-byte, base64url-encoded opaque token.
func generateConsoleToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate console token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- GetConsoleTicket: Resolve()-gated issuance (T012) ---

// ConsoleClient is the cluster.Client surface GetConsoleTicket needs — just the
// VNC ticket acquisition. The relay method is called by the WebSocket handler
// directly against cluster.Client, not here. Kept as an interface so the
// domain test can substitute a fake without spinning a real cluster.Client.
type ConsoleClient interface {
	GetVNCTicket(ctx context.Context, clusterName string, vmid int, node string) (cluster.VNCProxyTicket, error)
}

// GetConsoleTicket is the only path from a ticket HTTP request to a console
// capability (FR-001). It calls Resolve() first (the same and only ownership
// gate every other write uses), then the cluster client to obtain the
// Proxmox-side ticket, then the in-memory store to issue the opaque capability,
// then records the audit entry. The node is always Resolve()'s server-resolved
// value — the caller never supplies one (FR-007).
func GetConsoleTicket(
	ctx context.Context,
	index *inventory.Index,
	actor auth.Identity,
	clusterName string,
	vmid int,
	client ConsoleClient,
	store *ConsoleTicketStore,
	audit AuditRecorder,
) (VNCTicket, error) {
	entity, err := Resolve(index, actor, clusterName, vmid)
	if err != nil {
		return VNCTicket{}, err
	}

	proxy, err := client.GetVNCTicket(ctx, clusterName, vmid, entity.Node)
	if err != nil {
		return VNCTicket{}, fmt.Errorf("%w: %w", ErrClusterConsoleUnavailable, err)
	}

	ticket := store.Issue(clusterName, vmid, entity.Node, proxy.Ticket, proxy.Port)

	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, "console_open"); err != nil {
		return VNCTicket{}, fmt.Errorf("record audit: %w", err)
	}

	return ticket, nil
}

// --- test-only helpers (kept in production file so the white-box tests in
// console_test.go can exercise eviction and expiry without duplicating the
// store's internals). Not used by any production caller. ---

// expireForTest forces a ticket's ExpiresAt into the past. Test-only.
func (s *ConsoleTicketStore) expireForTest(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticket, ok := s.tickets[token]; ok {
		ticket.ExpiresAt = time.Now().Add(-time.Second)
		s.tickets[token] = ticket
	}
}

// oldestTokenForTest returns the first still-present token in insertion order.
// Test-only.
func (s *ConsoleTicketStore) oldestTokenForTest() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i := slices.IndexFunc(s.order, func(t string) bool { _, ok := s.tickets[t]; return ok }); i >= 0 {
		return s.order[i]
	}
	return ""
}
