package store

import (
	"testing"
	"time"
)

const (
	severityCritical = "critical"
	severityWarning  = "warning"
	severityInfo     = "info"
)

// TestDeriveSeverity - the severity mapping is hardcoded (3 levels, derived from the action verb). Each pattern must map to its
// level regardless of the action's namespace prefix. Internal test because
// deriveSeverity is unexported.
func TestDeriveSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		want   string
	}{
		// critical: fail / denied / rejected
		{"login failed", "auth.login_failed", severityCritical},
		{"csrf rejected", "auth.csrf_rejected", severityCritical},
		{"import rejected", "admin.db_import.rejected", severityCritical},
		{"admin denied", "auth.admin_denied", severityCritical},
		{"rate limited", "auth.rate_limited", severityInfo},

		// warning: delete / remove / destroy / revoke
		{"tags delete", "admin.tags.delete", severityWarning},
		{"pools delete", "admin.pools.delete", severityWarning},
		{"vm destroy", "vm.destroy", severityWarning},
		{"token revoke", "auth.token.revoke", severityWarning},
		{"catalog remove", "admin.catalog.remove", severityWarning},

		// info: everything else
		{"power on", "vm.power_on", severityInfo},
		{"clusters create", "admin.clusters.create", severityInfo},
		{"policy update", "admin.policy.update", severityInfo},
		{"db export", "admin.db_export", severityInfo},
		{"snapshot create", "vm_snapshot_create", severityInfo},
		{"empty action", "", severityInfo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := deriveSeverity(tc.action); got != tc.want {
				t.Errorf("deriveSeverity(%q) = %q, want %q", tc.action, got, tc.want)
			}
		})
	}
}

// Audit timestamps are compared as strings (ORDER BY, From/To filters, the
// retention cutoff), so their text order must match time order.
func TestAuditTimestamp_SortsChronologically(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	times := []time.Time{base, base.Add(120 * time.Millisecond), base.Add(123400 * time.Microsecond), base.Add(time.Second)}

	for i := 1; i < len(times); i++ {
		earlier, later := auditTimestamp(times[i-1]), auditTimestamp(times[i])
		if earlier >= later {
			t.Errorf("auditTimestamp(%v) = %q does not sort before %q", times[i-1], earlier, later)
		}
	}
}
