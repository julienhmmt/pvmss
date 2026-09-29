package telemetry

import (
	"context"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "pvmss"

// allowedMetricKeys is the label allowlist for pvmss.* and http.server.*
// instruments. Anything else (vmid, user, node, ...) is dropped by a view so a
// careless call site cannot create an unbounded series set. The http.* keys are
// the semconv names otelhttp emits (route is the mux pattern, never the URL).
var allowedMetricKeys = []attribute.Key{
	"route", "method", "status_class", "cluster", "action", "result", "status",
	"http.route", "http.request.method", "http.response.status_code",
}

// durationBuckets (seconds) suit Proxmox calls and inventory refreshes.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25}

type instruments struct {
	proxmoxDuration metric.Float64Histogram
	refreshDuration metric.Float64Histogram
	refreshFailures metric.Int64Counter
	vmActions       metric.Int64Counter
	authLogins      metric.Int64Counter
}

// providers caches instruments per MeterProvider, so the recorders below work
// with whichever provider is installed (and with a fresh one per test) without
// call sites holding instrument handles.
var providers sync.Map // metric.MeterProvider -> *instruments

func inst() *instruments {
	mp := otel.GetMeterProvider()
	if v, ok := providers.Load(mp); ok {
		if cached, isInst := v.(*instruments); isInst {
			return cached
		}
	}

	m := mp.Meter(meterName)
	// Instrument creation only fails on invalid names/units, all constants
	// here; a failure degrades to the no-op instrument the API returns.
	i := &instruments{}
	i.proxmoxDuration, _ = m.Float64Histogram("pvmss.proxmox.request.duration", metric.WithUnit("s"),
		metric.WithDescription("Proxmox API call duration"), metric.WithExplicitBucketBoundaries(durationBuckets...))
	i.refreshDuration, _ = m.Float64Histogram("pvmss.inventory.refresh.duration", metric.WithUnit("s"),
		metric.WithDescription("Inventory refresh cycle duration"), metric.WithExplicitBucketBoundaries(durationBuckets...))
	i.refreshFailures, _ = m.Int64Counter("pvmss.inventory.refresh.failures", metric.WithDescription("Failed inventory refresh cycles"))
	i.vmActions, _ = m.Int64Counter("pvmss.vm.actions", metric.WithDescription("VM actions recorded in the audit log"))
	i.authLogins, _ = m.Int64Counter("pvmss.auth.logins", metric.WithDescription("Login attempts"))

	actual, _ := providers.LoadOrStore(mp, i)
	if cached, ok := actual.(*instruments); ok {
		return cached
	}

	return i
}

// StatusClass buckets an HTTP status: "2xx".."5xx", or "error" when no
// response was received (status 0).
func StatusClass(status int) string {
	if status < 100 {
		return "error"
	}

	return strconv.Itoa(status/100) + "xx"
}

// RecordProxmoxRequest records one Proxmox API attempt.
func RecordProxmoxRequest(ctx context.Context, cluster, method string, status int, d time.Duration) {
	inst().proxmoxDuration.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("cluster", cluster), attribute.String("method", method), attribute.String("status_class", StatusClass(status))))
}

// RecordInventoryRefresh records one refresh cycle and counts it as a failure
// when failed is set.
func RecordInventoryRefresh(ctx context.Context, cluster string, d time.Duration, failed bool) {
	i := inst()
	attrs := metric.WithAttributes(attribute.String("cluster", cluster))
	i.refreshDuration.Record(ctx, d.Seconds(), attrs)

	if failed {
		i.refreshFailures.Add(ctx, 1, attrs)
	}
}

// RecordVMAction counts a VM action (result: "success" or "failure").
func RecordVMAction(ctx context.Context, action, result string) {
	inst().vmActions.Add(ctx, 1, metric.WithAttributes(attribute.String("action", action), attribute.String("result", result)))
}

// RecordLogin counts a login attempt (result: "success" or "failure").
func RecordLogin(ctx context.Context, result string) {
	inst().authLogins.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// ClusterStat is one cluster's inventory state for the observable gauges.
type ClusterStat struct {
	Cluster     string
	LastSuccess time.Time // zero = never refreshed; the series is omitted
	VMsByStatus map[string]int
}

// RegisterInventoryGauges registers pvmss.inventory.last_success (unix seconds,
// for the stale-inventory alert) and pvmss.vms (by cluster and VM status). src
// is read at collection time, so the gauges always reflect the live projections.
func RegisterInventoryGauges(meter metric.Meter, src func() []ClusterStat) (metric.Registration, error) {
	lastSuccess, err := meter.Int64ObservableGauge("pvmss.inventory.last_success", metric.WithUnit("s"),
		metric.WithDescription("Unix time of the last successful inventory refresh"))
	if err != nil {
		return nil, err
	}

	vms, err := meter.Int64ObservableGauge("pvmss.vms", metric.WithUnit("{vm}"), metric.WithDescription("VMs by cluster and status"))
	if err != nil {
		return nil, err
	}

	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for _, cs := range src() {
			if !cs.LastSuccess.IsZero() {
				o.ObserveInt64(lastSuccess, cs.LastSuccess.Unix(), metric.WithAttributes(attribute.String("cluster", cs.Cluster)))
			}

			for status, n := range cs.VMsByStatus {
				o.ObserveInt64(vms, int64(n), metric.WithAttributes(attribute.String("cluster", cs.Cluster), attribute.String("status", status)))
			}
		}

		return nil
	}, lastSuccess, vms)
}
