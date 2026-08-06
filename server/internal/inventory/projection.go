package inventory

import "sync/atomic"

// Projection holds the current Index via an atomic pointer. Readers never
// block on a mutex held during a slow client call — they always see either
// the previous complete index or the new complete one, never a partial one
// (FR-004, AC02).
type Projection struct {
	current atomic.Pointer[Index]
}

// NewProjection creates an empty projection. Load returns nil until the first
// successful refresh stores an Index (FR-009).
func NewProjection() *Projection {
	return &Projection{}
}

// Load returns the current Index, or nil if no refresh has succeeded yet.
// The returned pointer is immutable and safe to read concurrently.
func (p *Projection) Load() *Index {
	return p.current.Load()
}

// store replaces the current index. Called only by the worker on a successful
// refresh cycle — a failed cycle never calls this (FR-004).
func (p *Projection) store(idx *Index) {
	p.current.Store(idx)
}

// Store replaces the current index. Exported for use by tests that need to
// populate a projection without running a full refresh cycle. Production code
// should use the Worker, which calls store only on success.
func (p *Projection) Store(idx *Index) {
	p.store(idx)
}
