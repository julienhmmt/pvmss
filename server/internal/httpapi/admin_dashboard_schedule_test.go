package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_FailedSnapshotScheduleAlerts(t *testing.T) {
	ctx := context.Background()
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)
	at := time.Date(2026, 11, 4, 19, 0, 0, 0, time.UTC)

	failures := map[int]string{101: vm.ScheduleFailureCapReached, 102: vm.ScheduleFailureUnavailable, 103: ""}
	for vmid, failure := range failures {
		if err := st.UpsertSnapshotSchedule(ctx, dashCluster, vmid, "pvmss-p1", vm.ScheduleDaily, 3); err != nil {
			t.Fatalf("upsert: %v", err)
		}

		if failure == "" {
			continue
		}

		if err := st.MarkSnapshotScheduleFailure(ctx, dashCluster, vmid, failure, at); err != nil {
			t.Fatalf("mark failure: %v", err)
		}
	}

	ops := httpapi.NewAdminOps(authHandler, st, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: attentionIndex()}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash attentionDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var subjects []string

	for _, alert := range dash.Alerts {
		if alert.Kind == "snapshot_schedule_failed" {
			subjects = append(subjects, alert.Subject)
		}
	}

	if len(subjects) != 1 || subjects[0] != "101" {
		t.Fatalf("alerts for VMs %v, want only 101 (no failure and unreachable are left out)", subjects)
	}
}
