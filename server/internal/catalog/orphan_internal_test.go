package catalog

import (
	"context"
	"testing"
)

// TestSweepOrphans_NilDiscovered: a nil discovered map means the caller forgot
// to build the discovery index; sweeping must fail loudly instead of treating
// every stored approval as an orphan and auto-removing all enabled rows.
func TestSweepOrphans_NilDiscovered(t *testing.T) {
	t.Parallel()

	err := sweepOrphans(context.Background(), nil, "default", orphanSweep[string, NodeApproval]{})
	if err == nil {
		t.Fatal("sweepOrphans with nil discovered map must return an error")
	}
}
