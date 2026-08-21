package cluster

import (
	"context"
	"net/http"
	"testing"
)

func TestProxmox_GetMetricsHistory(t *testing.T) {
	t.Parallel()

	var gotTimeframe, gotCF string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/rrddata", func(w http.ResponseWriter, r *http.Request) {
			gotTimeframe = r.URL.Query().Get("timeframe")
			gotCF = r.URL.Query().Get("cf")

			writeJSONFixture(t, w, `{"data":[
				{"time":1700000000,"cpu":0.25,"mem":536870912,"maxmem":1073741824,"netin":1000,"netout":500,"diskread":2000,"diskwrite":1000},
				{"time":1700000060,"cpu":null,"mem":null,"maxmem":1073741824,"netin":null,"netout":null,"diskread":null,"diskwrite":null},
				{"time":1700000120,"cpu":0.1,"mem":32111957.3333333,"maxmem":1073741824,"netin":1000,"netout":500,"diskread":2000,"diskwrite":1000}
			]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	samples, err := p.GetMetricsHistory(context.Background(), testNodeName, testVMID, MetricsTimeframeHour)
	if err != nil {
		t.Fatalf("GetMetricsHistory: %v", err)
	}

	if gotTimeframe != "hour" || gotCF != "AVERAGE" {
		t.Errorf("query = timeframe=%q cf=%q, want hour/AVERAGE", gotTimeframe, gotCF)
	}

	if len(samples) != 3 {
		t.Fatalf("samples = %+v, want 3", samples)
	}

	checkMetricsSample(t, "samples[0]", samples[0], MetricsSample{CPUPercent: 25, MemoryUsed: 536870912, MemoryMax: 1073741824, DiskReadBps: 2000, DiskWriteBps: 1000, NetInBps: 1000, NetOutBps: 500})
	// A null-valued row (common right after a VM starts) decodes to zero, not
	// an error — the caller sees a real sample, not a gap.
	checkMetricsSample(t, "samples[1] (nulls)", samples[1], MetricsSample{MemoryMax: 1073741824})
	// Proxmox's RRD averaging (cf=AVERAGE) emits mem/maxmem as non-integer
	// floats, not whole bytes — decoding must not require an integer JSON
	// value (regression: this used to 500 with "cannot unmarshal number ...
	// into Go struct field proxmoxRRDRow.mem of type int64").
	checkMetricsSample(t, "samples[2] (fractional mem)", samples[2], MetricsSample{CPUPercent: 10, MemoryUsed: 32111957, MemoryMax: 1073741824, DiskReadBps: 2000, DiskWriteBps: 1000, NetInBps: 1000, NetOutBps: 500})
}

func TestProxmox_GetMetricsCurrent(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/status/current", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":{
				"cpu":0.35,"mem":1073741824,"maxmem":2147483648,
				"netin":2000,"netout":1000,"diskread":4000,"diskwrite":2000
			}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	sample, err := p.GetMetricsCurrent(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("GetMetricsCurrent: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/status/current" {
		t.Errorf("path = %q, want /status/current", gotPath)
	}

	checkMetricsSample(t, "current", sample, MetricsSample{CPUPercent: 35, MemoryUsed: 1073741824, MemoryMax: 2147483648, DiskReadBps: 4000, DiskWriteBps: 2000, NetInBps: 2000, NetOutBps: 1000})
}

// checkMetricsSample compares the numeric fields of got against want,
// ignoring Timestamp. Extracted from TestProxmox_GetMetricsHistory to keep
// its cyclomatic complexity under the golangci-lint gocyclo threshold.
func checkMetricsSample(t *testing.T, label string, got, want MetricsSample) {
	t.Helper()

	if got.CPUPercent != want.CPUPercent || got.MemoryUsed != want.MemoryUsed || got.MemoryMax != want.MemoryMax {
		t.Errorf("%s cpu/mem = %+v, want cpu=%v mem=%v maxmem=%v", label, got, want.CPUPercent, want.MemoryUsed, want.MemoryMax)
	}

	if got.NetInBps != want.NetInBps || got.NetOutBps != want.NetOutBps || got.DiskReadBps != want.DiskReadBps || got.DiskWriteBps != want.DiskWriteBps {
		t.Errorf("%s I/O = %+v, want netin=%v netout=%v diskread=%v diskwrite=%v", label, got, want.NetInBps, want.NetOutBps, want.DiskReadBps, want.DiskWriteBps)
	}
}
