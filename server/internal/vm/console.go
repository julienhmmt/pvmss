// Package vm — T10 console ticket store and Resolve()-gated ticket issuance.
//
// Per AC01 (see specs/011-t10-console-vnc/spec.md "Why this tranche exists"),
// the console ticket store stays in process memory: a map, a mutex, a TTL, and
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
// missing, expired, already consumed, or bound to a different (kind, cluster,
// vmid) than the one in the WebSocket URL. The WebSocket handler maps this to
// a 400 without upgrading the connection (FR-004, FR-005).
var ErrInvalidTicket = errors.New("invalid console ticket")

// ErrClusterConsoleUnavailable is returned by GetConsoleTicket when the
// cluster client's proxy ticket call fails — Proxmox is unreachable, the VM
// is not running, etc. The HTTP handler maps this to 502 console_unavailable
// (contracts/vm-console.md).
var ErrClusterConsoleUnavailable = errors.New("console unavailable")

// AuditActionConsoleOpen is the audit record action for opening a console
// (VNC or serial terminal) session. Exported so external tests can assert on
// it without repeating the literal.
const AuditActionConsoleOpen = "console_open"

// ConsoleKind distinguishes VNC console tickets from serial terminal tickets.
// Both share the same store, TTL, and eviction policy; the kind prevents a
// VNC ticket from being consumed by the serial WebSocket handler (and vice
// versa).
type ConsoleKind int

const (
	// KindVNC marks a ticket for the VNC console WebSocket path.
	KindVNC ConsoleKind = iota
	// KindTerminal marks a ticket for the serial terminal WebSocket path.
	KindTerminal
)

// ConsoleTicket is an in-memory, single-use, TTL-bound capability binding an
// opaque token to (kind, cluster, vmid, node, ProxmoxTicket, port). The client
// ever sees only the Token; every other field stays server-side (FR-002,
// FR-003). Never persisted, never serialized to a store (AC01). Issued by
// GetConsoleTicket, consumed once by the VNC or serial WebSocket handler.
type ConsoleTicket struct {
	Kind          ConsoleKind
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
// package-level singleton, no global mutable state. A single map holds both
// VNC and serial terminal tickets, distinguished by the Kind field.
type ConsoleTicketStore struct {
	mu      sync.Mutex
	tickets map[string]ConsoleTicket
	order   []string
}

// NewConsoleTicketStore creates an empty store.
func NewConsoleTicketStore() *ConsoleTicketStore {
	return &ConsoleTicketStore{
		tickets: make(map[string]ConsoleTicket),
		order:   make([]string, 0, ticketStoreCapacity),
	}
}

// Issue generates an opaque, cryptographically random token, stores the entry,
// appends to the insertion-order slice, and evicts the oldest entry if the
// fixed capacity is exceeded (FR-003, B11). Returns the new ticket.
func (s *ConsoleTicketStore) Issue(kind ConsoleKind, clusterName string, vmid int, node, proxmoxTicket string, port int) ConsoleTicket {
	token, err := generateConsoleToken()
	if err != nil {
		// crypto/rand failure is a fatal environment condition — surface it as
		// a ticket with an empty token so the caller can detect it, and let the
		// HTTP layer turn it into a 500. This path is not reachable in practice.
		return ConsoleTicket{Kind: kind, Cluster: clusterName, VMID: vmid, Node: node, ProxmoxTicket: proxmoxTicket, Port: port}
	}

	now := time.Now()
	ticket := ConsoleTicket{
		Kind:          kind,
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

// Consume looks up the token. Missing, expired, or a (kind, cluster, vmid)
// mismatch against what was bound at issuance → ErrInvalidTicket. On success,
// the entry is deleted from the map BEFORE returning it — a concurrent second
// Consume for the same token cannot observe it as still present, closing the
// single-use race (FR-004). A mismatched Consume does NOT consume the ticket:
// it remains valid for its real (kind, cluster, vmid).
func (s *ConsoleTicketStore) Consume(kind ConsoleKind, token, clusterName string, vmid int) (ConsoleTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[token]
	if !ok {
		return ConsoleTicket{}, ErrInvalidTicket
	}

	if time.Now().After(ticket.ExpiresAt) {
		s.removeOrderLocked(token)
		delete(s.tickets, token)

		return ConsoleTicket{}, ErrInvalidTicket
	}

	if ticket.Kind != kind || ticket.Cluster != clusterName || ticket.VMID != vmid {
		return ConsoleTicket{}, ErrInvalidTicket
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

// ProxyFetcher fetches the Proxmox-side ticket and port for a console session.
// The VNC path supplies a wrapper around relay.GetVNCTicket; the serial path
// supplies one around relay.GetTermProxy. Both return (ticket, port, error).
// Kept as a function type so the domain test can substitute a closure without
// spinning a real cluster.Client.
type ProxyFetcher func(ctx context.Context, clusterName string, vmid int, node string) (ticket string, port int, err error)

// ConsoleTicketDeps groups the shared dependencies and resolution context
// for GetConsoleTicket. It collapses the eight positional parameters the
// function used to take (SonarQube go:S107).
type ConsoleTicketDeps struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Kind        ConsoleKind
	Fetcher     ProxyFetcher
	Store       *ConsoleTicketStore
	Audit       AuditRecorder
}

// GetConsoleTicket is the only path from a ticket HTTP request to a console
// capability (FR-001). It calls Resolve() first (the same and only ownership
// gate every other write uses), then the proxy fetcher to obtain the
// Proxmox-side ticket, then the in-memory store to issue the opaque capability,
// then records the audit entry. The node is always Resolve()'s server-resolved
// value — the caller never supplies one (FR-007).
func GetConsoleTicket(ctx context.Context, deps ConsoleTicketDeps) (ConsoleTicket, error) {
	entity, err := Resolve(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return ConsoleTicket{}, err
	}

	proxyTicket, port, err := deps.Fetcher(ctx, deps.ClusterName, deps.VMID, entity.Node)
	if err != nil {
		return ConsoleTicket{}, fmt.Errorf("%w: %w", ErrClusterConsoleUnavailable, err)
	}

	ticket := deps.Store.Issue(deps.Kind, deps.ClusterName, deps.VMID, entity.Node, proxyTicket, port)

	if err := deps.Audit.RecordAction(ctx, deps.Actor.Username, deps.ClusterName, deps.VMID, AuditActionConsoleOpen); err != nil {
		return ConsoleTicket{}, fmt.Errorf(auditWrapFmt, err)
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
