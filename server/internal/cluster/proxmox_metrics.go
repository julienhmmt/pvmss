package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// proxmoxRRDRow mirrors one entry of Proxmox's
// /nodes/{node}/qemu/{vmid}/rrddata response. All numeric fields decode as
// float64 — Proxmox's RRD averaging (cf=AVERAGE) always emits floats, even
// for integer-valued metrics like mem/maxmem. Fields are pointers because
// Proxmox emits JSON null for samples predating the metric's collection
// (e.g. right after a VM starts) — nilFloat below defaults those to zero
// rather than letting them silently decode as Go's zero value in a way that
// would be indistinguishable from a real zero reading.
type proxmoxRRDRow struct {
	Time      int64    `json:"time"`
	CPU       *float64 `json:"cpu"`
	Mem       *float64 `json:"mem"`
	MaxMem    *float64 `json:"maxmem"`
	NetIn     *float64 `json:"netin"`
	NetOut    *float64 `json:"netout"`
	DiskRead  *float64 `json:"diskread"`
	DiskWrite *float64 `json:"diskwrite"`
}

// GetMetricsHistory implements MetricsHistoryReader against Proxmox's RRD
// data endpoint. Proxmox already reports netin/netout/diskread/diskwrite as
// bytes-per-second averages (not cumulative counters) and cpu as a 0..1
// fraction — this only converts cpu to a percentage.
func (p Proxmox) GetMetricsHistory(ctx context.Context, node string, vmid int, timeframe MetricsTimeframe) ([]MetricsSample, error) {
	form := url.Values{"timeframe": {string(timeframe)}, "cf": {"AVERAGE"}}

	raw, err := p.rest().do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/rrddata", url.PathEscape(node), vmid), form)
	if err != nil {
		return nil, err
	}

	var rows []proxmoxRRDRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode metrics history: %w", err)
	}

	samples := make([]MetricsSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, MetricsSample{
			Timestamp:    time.Unix(row.Time, 0).UTC(),
			CPUPercent:   nilFloat(row.CPU) * 100,
			MemoryUsed:   int64(nilFloat(row.Mem)),
			MemoryMax:    int64(nilFloat(row.MaxMem)),
			DiskReadBps:  nilFloat(row.DiskRead),
			DiskWriteBps: nilFloat(row.DiskWrite),
			NetInBps:     nilFloat(row.NetIn),
			NetOutBps:    nilFloat(row.NetOut),
		})
	}

	return samples, nil
}

func nilFloat(v *float64) float64 {
	if v == nil {
		return 0
	}

	return *v
}
