package state

import (
	"time"

	"pvmss/logger"
)

// cloudInitWarningTTL is how long a stored cloud-init warning remains
// retrievable after it was last written. Long enough for the frontend to poll
// the task-status endpoint after a VM creation task completes.
const cloudInitWarningTTL = 10 * time.Minute

// CloudInitPending is the sentinel stored under a UPID when a VM creation with
// cloud-init is dispatched but the asynchronous cloud-init application has not
// finished yet. The task-status endpoint waits for this sentinel to be replaced
// with the real warning (or an empty string meaning "no warning").
const CloudInitPending = "__pending__"

// cloudInitWarningEntry is a single stored cloud-init warning.
type cloudInitWarningEntry struct {
	warning string
	setAt   time.Time
}

// SetCloudInitWarning stores (or replaces) the cloud-init warning for a UPID.
// An empty warning means "cloud-init applied with no issue"; use
// CloudInitPending to mark that application is still in progress.
func (s *appState) SetCloudInitWarning(upid, warning string) {
	if upid == "" {
		return
	}
	s.cloudInitWarningsMu.Lock()
	s.cloudInitWarnings[upid] = cloudInitWarningEntry{warning: warning, setAt: time.Now()}
	s.cloudInitWarningsMu.Unlock()
}

// GetCloudInitWarning returns the stored cloud-init warning for a UPID, or an
// empty string if no entry exists or it has expired. Expired entries are
// removed lazily.
func (s *appState) GetCloudInitWarning(upid string) string {
	if upid == "" {
		return ""
	}
	s.cloudInitWarningsMu.RLock()
	entry, ok := s.cloudInitWarnings[upid]
	s.cloudInitWarningsMu.RUnlock()
	if !ok {
		return ""
	}
	if time.Since(entry.setAt) > cloudInitWarningTTL {
		s.cloudInitWarningsMu.Lock()
		// Re-check under write lock to avoid deleting a freshly-updated entry.
		if cur, stillOk := s.cloudInitWarnings[upid]; stillOk && time.Since(cur.setAt) > cloudInitWarningTTL {
			delete(s.cloudInitWarnings, upid)
		}
		s.cloudInitWarningsMu.Unlock()
		return ""
	}
	return entry.warning
}

// DeleteCloudInitWarning removes any stored cloud-init warning for a UPID.
func (s *appState) DeleteCloudInitWarning(upid string) {
	if upid == "" {
		return
	}
	s.cloudInitWarningsMu.Lock()
	delete(s.cloudInitWarnings, upid)
	s.cloudInitWarningsMu.Unlock()
}

// cleanupCloudInitWarnings periodically removes expired entries so the map does
// not grow unbounded for long-lived processes.
func (s *appState) cleanupCloudInitWarnings() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		removed := 0
		s.cloudInitWarningsMu.Lock()
		for upid, entry := range s.cloudInitWarnings {
			if now.Sub(entry.setAt) > cloudInitWarningTTL {
				delete(s.cloudInitWarnings, upid)
				removed++
			}
		}
		s.cloudInitWarningsMu.Unlock()
		if removed > 0 {
			logger.Get().Debug().Int("removed", removed).Msg("Cleaned up expired cloud-init warnings")
		}
	}
}
