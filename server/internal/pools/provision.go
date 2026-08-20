//nolint:wsl_v5 // provisioning sequence is intentionally linear and ordered
package pools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"regexp"
)

// Provisioning errors are stable for HTTP mapping and tests.
var (
	ErrForbidden      = errors.New("forbidden")
	ErrNotFound       = errors.New("pool not found")
	ErrNotManaged     = errors.New("pool not managed by PVMSS")
	ErrInvalidName    = errors.New("invalid pool name")
	ErrWeakPassword   = errors.New("invalid password")
	ErrAlreadyExists  = errors.New("pool already exists")
	poolNamePattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	minPasswordLength = 8
)

// ManagedRecorder records that PVMSS created a pool so deletion can be scoped.
// The store implementation is idempotent; re-registering refreshes created_at.
type ManagedRecorder interface {
	RegisterManagedPool(ctx context.Context, cluster, name string) error
}

// ManagedChecker reports whether PVMSS recorded the named pool on the cluster.
type ManagedChecker interface {
	IsPoolManaged(ctx context.Context, cluster, name string) (bool, error)
	ManagedPoolNames(ctx context.Context, cluster string) (map[string]struct{}, error)
}

// ManagedRemover removes the managed marker after a successful cascade.
type ManagedRemover interface {
	UnregisterManagedPool(ctx context.Context, cluster, name string) error
}

// ManagedStore combines the managed-pool persistence operations used by Create
// and Delete. Pass nil to opt out of managed tracking (legacy tests only).
type ManagedStore interface {
	ManagedRecorder
	ManagedChecker
	ManagedRemover
}

// ProvisionError identifies the failed cluster provisioning step for logs.
type ProvisionError struct {
	Step string
	Err  error
}

func (e *ProvisionError) Error() string {
	return fmt.Sprintf("pool provisioning %s: %v", e.Step, e.Err)
}
func (e *ProvisionError) Unwrap() error { return e.Err }

// Create provisions one pool and records it as PVMSS-managed when recorder is
// non-nil. The optional comment preserves the documented five-argument domain
// API while allowing the HTTP contract to pass a comment.
func Create(ctx context.Context, actor auth.Identity, client cluster.Client, name, password string, comments ...string) (cluster.Pool, error) {
	return CreateWithRecorder(ctx, actor, client, nil, "", name, password, comments...)
}

// CreateWithRecorder provisions one pool and registers it as managed when the
// recorder is non-nil. The cluster name scopes the managed marker.
func CreateWithRecorder(ctx context.Context, actor auth.Identity, client cluster.Client, recorder ManagedRecorder, clusterName, name, password string, comments ...string) (cluster.Pool, error) {
	if !actor.IsAdmin {
		return cluster.Pool{}, ErrForbidden
	}
	if err := ValidateName(name); err != nil {
		return cluster.Pool{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return cluster.Pool{}, err
	}
	comment := ""
	if len(comments) > 0 {
		comment = comments[0]
	}
	pool, err := create(ctx, client, name, password, comment)
	if err != nil {
		return cluster.Pool{}, err
	}
	if recorder != nil && clusterName != "" {
		if err := recorder.RegisterManagedPool(ctx, clusterName, name); err != nil {
			slog.Default().Error("managed pool registration failed", "cluster", clusterName, "pool", name, "error", err)
		}
	}
	return pool, nil
}

// ValidateName enforces the Proxmox poolid and PVE username boundary.
func ValidateName(name string) error {
	if len(name) < 1 || len(name) > 32 || !poolNamePattern.MatchString(name) {
		return fmt.Errorf("%w: pool name must be 1-32 lowercase alphanumeric characters with internal hyphens", ErrInvalidName)
	}
	return nil
}

// ValidatePassword enforces the minimum credential length without inventing a
// password-strength policy beyond the feature contract.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must contain at least 8 characters", ErrWeakPassword)
	}
	return nil
}

func create(ctx context.Context, client cluster.Client, name, password, comment string) (cluster.Pool, error) {
	existing, err := client.ListPools(ctx)
	if err != nil {
		return cluster.Pool{}, err
	}
	if containsPool(existing, name) {
		return cluster.Pool{}, fmt.Errorf("%w: %q", ErrAlreadyExists, name)
	}
	steps := []struct {
		name string
		call func() error
	}{
		{name: "role", call: func() error { return client.EnsurePoolRole(ctx) }},
	}
	for _, step := range steps {
		if err := step.call(); err != nil {
			return cluster.Pool{}, &ProvisionError{Step: step.name, Err: err}
		}
	}
	username, err := client.EnsurePoolUser(ctx, name, password)
	if err != nil {
		return cluster.Pool{}, &ProvisionError{Step: "user", Err: err}
	}
	if err := client.CreatePool(ctx, name, comment); err != nil {
		return cluster.Pool{}, &ProvisionError{Step: "pool", Err: err}
	}
	if err := client.SetPoolACL(ctx, username, name, "PVMSSUser"); err != nil {
		return cluster.Pool{}, &ProvisionError{Step: "ACL", Err: err}
	}
	return cluster.Pool{Name: name, Comment: comment}, nil
}

func containsPool(pools []cluster.Pool, name string) bool {
	for _, pool := range pools {
		if pool.Name == name {
			return true
		}
	}
	return false
}
