package state

import (
	"testing"
)

func TestCloudInitWarnings_SetGetDelete(t *testing.T) {
	s := newAppState(nil)

	// No entry returns empty string.
	if got := s.GetCloudInitWarning("UPID:missing"); got != "" {
		t.Errorf("GetCloudInitWarning(missing) = %q, want empty", got)
	}

	// Store a real warning and retrieve it.
	s.SetCloudInitWarning("UPID:1", "upload-failed-api")
	if got := s.GetCloudInitWarning("UPID:1"); got != "upload-failed-api" {
		t.Errorf("GetCloudInitWarning(UPID:1) = %q, want %q", got, "upload-failed-api")
	}

	// Replace with an empty string (meaning "no warning") and retrieve.
	s.SetCloudInitWarning("UPID:1", "")
	if got := s.GetCloudInitWarning("UPID:1"); got != "" {
		t.Errorf("GetCloudInitWarning(UPID:1) after empty = %q, want empty", got)
	}

	// Delete removes the entry.
	s.SetCloudInitWarning("UPID:2", "no-snippets-storage")
	s.DeleteCloudInitWarning("UPID:2")
	if got := s.GetCloudInitWarning("UPID:2"); got != "" {
		t.Errorf("GetCloudInitWarning(UPID:2) after delete = %q, want empty", got)
	}
}

func TestCloudInitWarnings_PendingSentinel(t *testing.T) {
	s := newAppState(nil)

	s.SetCloudInitWarning("UPID:pending", CloudInitPending)
	if got := s.GetCloudInitWarning("UPID:pending"); got != CloudInitPending {
		t.Errorf("GetCloudInitWarning(pending) = %q, want %q", got, CloudInitPending)
	}

	// Replacing the sentinel with the real result should be observable.
	s.SetCloudInitWarning("UPID:pending", "upload-failed-sftp")
	if got := s.GetCloudInitWarning("UPID:pending"); got != "upload-failed-sftp" {
		t.Errorf("GetCloudInitWarning(pending) after resolve = %q, want %q", got, "upload-failed-sftp")
	}
}

func TestCloudInitWarnings_EmptyUPIDNoOp(t *testing.T) {
	s := newAppState(nil)
	// Empty UPID should not panic or store anything.
	s.SetCloudInitWarning("", "upload-failed-api")
	s.DeleteCloudInitWarning("")
	if got := s.GetCloudInitWarning(""); got != "" {
		t.Errorf("GetCloudInitWarning(empty) = %q, want empty", got)
	}
}

func TestCloudInitWarnings_TTLExpiry(t *testing.T) {
	s := newAppState(nil)

	s.SetCloudInitWarning("UPID:expire", "upload-failed-api")
	// Force the entry to look expired by backdating setAt under the write lock.
	s.cloudInitWarningsMu.Lock()
	if e, ok := s.cloudInitWarnings["UPID:expire"]; ok {
		e.setAt = e.setAt.Add(-(cloudInitWarningTTL + 1))
		s.cloudInitWarnings["UPID:expire"] = e
	}
	s.cloudInitWarningsMu.Unlock()

	if got := s.GetCloudInitWarning("UPID:expire"); got != "" {
		t.Errorf("GetCloudInitWarning(expired) = %q, want empty (lazy delete)", got)
	}
	// Lazy delete should have removed it.
	s.cloudInitWarningsMu.RLock()
	_, stillThere := s.cloudInitWarnings["UPID:expire"]
	s.cloudInitWarningsMu.RUnlock()
	if stillThere {
		t.Error("expired entry was not lazily deleted")
	}
}
