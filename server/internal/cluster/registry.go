//nolint:wsl_v5 // registry operations keep locking and state transitions adjacent
package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"pvmss/server/internal/store"
	"slices"
	"sync"

	"golang.org/x/crypto/ssh"
)

// ErrClusterNotFound is returned when a cluster is unknown or removed.
var ErrClusterNotFound = errors.New("cluster not found")

// SourceFake and SourceProxmox are the supported PVMSS_CLUSTER_SOURCE values.
// Exported so tests and config validation reference a single constant instead
// of repeating the literal.
const (
	SourceFake    = "fake"
	SourceProxmox = "proxmox"
)

// ClientFactory constructs one client from one persisted cluster row.
type ClientFactory func(row store.ClusterRow) (Client, error)

// ClientProvider is the subset of Registry needed by per-cluster consumers.
type ClientProvider interface {
	Client(name string) (Client, error)
	List() []string
}

// Registry owns the active cluster clients. Adding one name never replaces an
// existing client, which keeps active inventories isolated during runtime changes.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]Client
	factory ClientFactory
	// sshPublicKey is the publishing key's public half, for display.
	sshPublicKey string
}

// NewRegistry constructs a registry from active persisted rows. A row that
// cannot construct a client is skipped so one bad cluster cannot block others.
func NewRegistry(source string, rows []store.ClusterRow) (*Registry, error) {
	factory, err := factoryForSource(source, nil)
	if err != nil {
		return nil, err
	}
	return NewRegistryWithFactory(factory, rows)
}

// NewRegistryWithSSH constructs a registry whose Proxmox clients publish
// cloud-init documents over SSH with signer (the global PVMSS_SSH_KEY_FILE)
// and each cluster row's own SSH user, port and pinned host keys. A nil
// signer disables publishing on every cluster.
func NewRegistryWithSSH(source string, rows []store.ClusterRow, signer ssh.Signer) (*Registry, error) {
	factory, err := factoryForSource(source, signer)
	if err != nil {
		return nil, err
	}
	registry, err := NewRegistryWithFactory(factory, rows)
	if err != nil {
		return nil, err
	}
	registry.sshPublicKey = AuthorizedKey(signer)
	return registry, nil
}

// SSHPublicKey is PVMSS's publishing public key in authorized_keys form
// ("" without PVMSS_SSH_KEY_FILE), shown in Admin > Clusters.
func (registry *Registry) SSHPublicKey() string {
	return registry.sshPublicKey
}

// NewRegistryWithFactory constructs a registry with an injectable client factory.
func NewRegistryWithFactory(factory ClientFactory, rows []store.ClusterRow) (*Registry, error) {
	if factory == nil {
		return nil, errors.New("cluster client factory is required")
	}
	registry := &Registry{clients: make(map[string]Client, len(rows)), factory: factory}
	for _, row := range rows {
		if row.RemovedAt != nil {
			continue
		}
		client, err := factory(row)
		if err != nil {
			continue
		}
		registry.clients[row.Name] = client
	}
	return registry, nil
}

// Client returns the active client for name.
func (registry *Registry) Client(name string) (Client, error) {
	registry.mu.RLock()
	client, ok := registry.clients[name]
	registry.mu.RUnlock()
	if !ok {
		return nil, ErrClusterNotFound
	}
	return client, nil
}

// Add constructs and registers one new active client without touching others.
func (registry *Registry) Add(_ context.Context, row store.ClusterRow) error {
	if row.RemovedAt != nil {
		return ErrClusterNotFound
	}
	client, err := registry.factory(row)
	if err != nil {
		return fmt.Errorf("construct cluster %q: %w", row.Name, err)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.clients[row.Name]; exists {
		return store.ErrDuplicateCluster
	}
	registry.clients[row.Name] = client
	return nil
}

// Update replaces one active client while keeping every other client intact.
func (registry *Registry) Update(_ context.Context, row store.ClusterRow) error {
	client, err := registry.factory(row)
	if err != nil {
		return fmt.Errorf("construct cluster %q: %w", row.Name, err)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.clients[row.Name]; !exists {
		return ErrClusterNotFound
	}
	registry.clients[row.Name] = client
	return nil
}

// Remove drops one active client. Persistence owns the soft-delete transaction.
func (registry *Registry) Remove(name string) {
	registry.mu.Lock()
	delete(registry.clients, name)
	registry.mu.Unlock()
}

// List returns active cluster names in deterministic order.
func (registry *Registry) List() []string {
	registry.mu.RLock()
	result := make([]string, 0, len(registry.clients))
	for name := range registry.clients {
		result = append(result, name)
	}
	registry.mu.RUnlock()
	slices.Sort(result)
	return result
}

func factoryForSource(source string, signer ssh.Signer) (ClientFactory, error) {
	switch source {
	case SourceFake:
		return func(row store.ClusterRow) (Client, error) {
			if row.Name == "" {
				return nil, errors.New("cluster name is required")
			}
			return NewFake(row.Name), nil
		}, nil
	case SourceProxmox:
		return func(row store.ClusterRow) (Client, error) {
			parsed, err := url.ParseRequestURI(row.URL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return nil, errors.New("cluster url is malformed")
			}
			if row.TokenID == "" || row.TokenSecret == "" {
				return nil, errors.New("cluster credentials are required")
			}
			return Proxmox{
				BaseURL:               row.URL,
				APITokenName:          row.TokenID,
				APITokenValue:         row.TokenSecret,
				TLSInsecureSkipVerify: row.TLSInsecureSkipVerify,
				SnippetStorage:        row.SnippetStorage,
				SSH: SnippetSSH{
					User: row.SSHUser, Port: row.SSHPort, KnownHosts: row.SSHKnownHosts, Signer: signer,
				},
				httpClient: newProxmoxHTTPClient(row.TLSInsecureSkipVerify),
			}, nil
		}, nil
	default:
		return nil, fmt.Errorf("unknown cluster source %q", source)
	}
}

var _ ClientProvider = (*Registry)(nil)
