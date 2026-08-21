package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// MetricsTimeframe selects the historical window for GetMetricsHistory.
// Proxmox's own rrddata endpoint also supports "month" and "year", but T-02
// (issue 02) scopes PVMSS to these three on purpose (spec Out of Scope).
type MetricsTimeframe string

// The three supported history windows.
const (
	MetricsTimeframeHour MetricsTimeframe = "hour"
	MetricsTimeframeDay  MetricsTimeframe = "day"
	MetricsTimeframeWeek MetricsTimeframe = "week"
)

// ErrInvalidTimeframe reports a "range" value other than hour/day/week.
var ErrInvalidTimeframe = errors.New("invalid metrics timeframe")

// ParseMetricsTimeframe validates and normalizes a range query parameter.
func ParseMetricsTimeframe(raw string) (MetricsTimeframe, error) {
	switch MetricsTimeframe(raw) {
	case MetricsTimeframeHour, MetricsTimeframeDay, MetricsTimeframeWeek:
		return MetricsTimeframe(raw), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidTimeframe, raw)
	}
}

// metricsTimeframeSampleCount is how many points GetMetricsHistory returns
// for a given timeframe — used by the fake to synthesize a series of
// plausible size. The real Proxmox rrddata endpoint decides its own count;
// this only bounds the fake's fixture.
func metricsTimeframeSampleCount(timeframe MetricsTimeframe) int {
	switch timeframe {
	case MetricsTimeframeHour:
		return 60
	case MetricsTimeframeDay:
		return 96
	case MetricsTimeframeWeek:
		return 168
	default:
		return 0
	}
}

// metricsTimeframeStep is the spacing between consecutive fake samples.
func metricsTimeframeStep(timeframe MetricsTimeframe) time.Duration {
	switch timeframe {
	case MetricsTimeframeHour:
		return time.Minute
	case MetricsTimeframeDay:
		return 15 * time.Minute
	case MetricsTimeframeWeek:
		return time.Hour
	default:
		return time.Minute
	}
}

// MetricsSample is one point in a VM's metrics history (or its current
// status, treated as a one-sample history by callers that need it).
type MetricsSample struct {
	Timestamp    time.Time
	CPUPercent   float64
	MemoryUsed   int64
	MemoryMax    int64
	DiskReadBps  float64
	DiskWriteBps float64
	NetInBps     float64
	NetOutBps    float64
}

// MetricsHistoryReader reads a VM's historical metrics series. Kept separate
// from Client (constitution IV: reads and writes are separated, and small
// single-purpose interfaces per SnapshotReader/SnapshotWriter) — a metrics
// read is neither a Client-level cluster read nor a Writer-level VM mutation.
type MetricsHistoryReader interface {
	GetMetricsHistory(ctx context.Context, node string, vmid int, timeframe MetricsTimeframe) ([]MetricsSample, error)
}
