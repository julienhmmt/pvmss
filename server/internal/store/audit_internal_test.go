package store

import "testing"

const (
	severityCritical = "critical"
	severityWarning  = "warning"
	severityInfo     = "info"
)

// TestDeriveSeverity — the severity mapping is hardcoded (spec decision:
// 3 levels, derived from the action verb). Each pattern must map to its
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
