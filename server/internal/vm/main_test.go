package vm

import (
	"os"
	"testing"
	"time"
)

// TestMain shrinks the real-time poll intervals the suite would otherwise
// spend ~4 s per test waiting on (~146 s total). Tests that rely on a
// specific value still override it per test and restore to these.
func TestMain(m *testing.M) {
	CreateTaskPoll = 5 * time.Millisecond
	LockRetryPollInterval = 5 * time.Millisecond
	agentPingPoll = 5 * time.Millisecond

	os.Exit(m.Run())
}
