package store

import "testing"

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
		{"login failed", "auth.login_failed", "critical"},
		{"csrf rejected", "auth.csrf_rejected", "critical"},
		{"import rejected", "admin.db_import.rejected", "critical"},
		{"admin denied", "auth.admin_denied", "critical"},
		{"rate limited", "auth.rate_limited", "info"},

		// warning: delete / remove / destroy / revoke
		{"tags delete", "admin.tags.delete", "warning"},
		{"pools delete", "admin.pools.delete", "warning"},
		{"vm destroy", "vm.destroy", "warning"},
		{"token revoke", "auth.token.revoke", "warning"},
		{"catalog remove", "admin.catalog.remove", "warning"},

		// info: everything else
		{"power on", "vm.power_on", "info"},
		{"clusters create", "admin.clusters.create", "info"},
		{"policy update", "admin.policy.update", "info"},
		{"db export", "admin.db_export", "info"},
		{"snapshot create", "vm_snapshot_create", "info"},
		{"empty action", "", "info"},
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
