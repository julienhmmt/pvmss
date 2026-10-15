package vm

import (
	"testing"
	"time"
)

// SetStopWaitForTest shortens the hardware-change shutdown wait.
func SetStopWaitForTest(t *testing.T, poll, timeout time.Duration) {
	t.Helper()

	oldPoll, oldTimeout := stopPollInterval, stopWaitTimeout
	stopPollInterval, stopWaitTimeout = poll, timeout

	t.Cleanup(func() { stopPollInterval, stopWaitTimeout = oldPoll, oldTimeout })
}
