package httpapi

import (
	"slices"
	"testing"
)

func TestCompareAlerts_PendingQuotaRequestsLead(t *testing.T) {
	t.Parallel()

	alerts := []dashboardAlertDTO{
		{Kind: "node_offline", Severity: alertCritical},
		{Kind: "cluster_unreachable", Severity: alertCritical},
		{Kind: "quota_requests_pending", Severity: alertWarning},
		{Kind: "node_cpu", Severity: alertWarning},
	}
	slices.SortStableFunc(alerts, compareAlerts)

	if alerts[0].Kind != "quota_requests_pending" {
		t.Fatalf("first alert = %q, want quota_requests_pending", alerts[0].Kind)
	}

	if alerts[1].Kind != "cluster_unreachable" || alerts[3].Kind != "node_cpu" {
		t.Fatalf("rest not ordered by severity: %+v", alerts)
	}
}
