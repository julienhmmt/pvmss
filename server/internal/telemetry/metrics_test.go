package telemetry_test

import (
	"context"
	"io"
	"net/http/httptest"
	"pvmss/server/internal/telemetry"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func scrape(t *testing.T, res telemetry.Result) string {
	t.Helper()

	rec := httptest.NewRecorder()
	res.MetricsHandler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

//nolint:paralleltest // serial: swaps the global meter provider
func TestMetrics_SetupWithoutMetricsOrEndpointHasNoHandler(t *testing.T) {
	res, err := telemetry.Setup(context.Background(), telemetry.Config{}, env(nil))
	if err != nil {
		t.Fatal(err)
	}

	if res.MetricsHandler != nil {
		t.Fatal("metrics handler must be nil when metrics are not enabled")
	}
}

//nolint:paralleltest // serial: swaps the global meter provider
func TestMetrics_PrometheusListsEveryV1InstrumentWithAllowedLabelsOnly(t *testing.T) {
	prev := otel.GetMeterProvider()

	res, err := telemetry.Setup(context.Background(), telemetry.Config{Version: "t", Metrics: true}, env(nil))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = res.Shutdown(context.Background()); otel.SetMeterProvider(prev) })

	ctx := context.Background()
	telemetry.RecordProxmoxRequest(ctx, "lab", "GET", 200, 40*time.Millisecond)
	telemetry.RecordProxmoxRequest(ctx, "lab", "GET", 0, time.Millisecond)
	telemetry.RecordInventoryRefresh(ctx, "lab", 300*time.Millisecond, false)
	telemetry.RecordInventoryRefresh(ctx, "lab", time.Second, true)
	telemetry.RecordVMAction(ctx, "start", "success")
	telemetry.RecordLogin(ctx, "failure")

	reg, err := telemetry.RegisterInventoryGauges(otel.Meter("pvmss"), func() []telemetry.ClusterStat {
		return []telemetry.ClusterStat{{Cluster: "lab", LastSuccess: time.Unix(1700000000, 0), VMsByStatus: map[string]int{"running": 2, "stopped": 1}}}
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = reg.Unregister() })

	// A stray high-cardinality attribute must be dropped by the allowlist view.
	counter, _ := otel.Meter("pvmss").Int64Counter("pvmss.probe")
	counter.Add(ctx, 1, metric.WithAttributes(attribute.String("vmid", "101"), attribute.String("user", "alice@pve"), attribute.String("node", "pve1"), attribute.String("cluster", "lab")))

	body := scrape(t, res)

	for _, want := range []string{
		"pvmss_proxmox_request_duration_seconds", "pvmss_inventory_refresh_duration_seconds",
		"pvmss_inventory_refresh_failures_total", "pvmss_inventory_last_success_seconds",
		"pvmss_vms", "pvmss_vm_actions_total", "pvmss_auth_logins_total", "go_",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape lacks %q", want)
		}
	}

	if !strings.Contains(body, `status_class="error"`) || !strings.Contains(body, `status="running"`) {
		t.Errorf("expected status_class and status labels in scrape")
	}

	for _, forbidden := range []string{"vmid=", "user=", "node="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("scrape carries forbidden label %q", forbidden)
		}
	}

	if !strings.Contains(body, `pvmss_probe_total{cluster="lab"`) {
		t.Errorf("allowed label of the probe counter must survive: %s", grepLines(body, "pvmss_probe"))
	}
}

func grepLines(body, sub string) string {
	var out []string

	for _, l := range strings.Split(body, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}

	return strings.Join(out, "\n")
}
