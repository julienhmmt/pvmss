package cluster

import (
	"context"
	"math"
	"time"
)

// GetMetricsHistory synthesizes a deterministic series for (node, vmid): a
// vmid-seeded sine wave, no real randomness, so the same VM always produces
// the same-shaped curve across calls (matching the Snapshot stability
// invariant tests elsewhere in this package).
func (fake Fake) GetMetricsHistory(_ context.Context, node string, vmid int, timeframe MetricsTimeframe) ([]MetricsSample, error) {
	state := fake.stateOrDefault()

	state.vmMu.RLock()
	index := state.findVM(node, vmid)

	var memoryMax int64
	if index >= 0 {
		memoryMax = state.vms[index].MemoryTotal
	}

	state.vmMu.RUnlock()

	if index < 0 {
		return nil, ErrNotFound
	}

	if memoryMax <= 0 {
		memoryMax = 1 << 30 // 1 GiB fallback so the series is never degenerate
	}

	count := metricsTimeframeSampleCount(timeframe)
	step := metricsTimeframeStep(timeframe)
	now := time.Now().UTC()
	samples := make([]MetricsSample, count)

	for i := range count {
		// Oldest sample first. Phase seeded by vmid and index gives each VM a
		// distinct, stable-looking curve across calls.
		phase := float64(vmid%97) + float64(i)/6
		samples[i] = fakeMetricsSample(phase, now.Add(-step*time.Duration(count-1-i)), memoryMax)
	}

	return samples, nil
}

// GetMetricsCurrent synthesizes a deterministic current sample for (node,
// vmid): a single point seeded by the current time and vmid, so a live
// stream ticks visibly but stays within the same contract bounds as the
// historical series.
func (fake Fake) GetMetricsCurrent(_ context.Context, node string, vmid int) (MetricsSample, error) {
	state := fake.stateOrDefault()

	state.vmMu.RLock()
	index := state.findVM(node, vmid)

	var memoryMax int64
	if index >= 0 {
		memoryMax = state.vms[index].MemoryTotal
	}

	state.vmMu.RUnlock()

	if index < 0 {
		return MetricsSample{}, ErrNotFound
	}

	if memoryMax <= 0 {
		memoryMax = 1 << 30 // 1 GiB fallback so the series is never degenerate
	}

	now := time.Now().UTC()
	phase := float64(vmid%97) + float64(now.UnixMilli())/6000

	return fakeMetricsSample(phase, now, memoryMax), nil
}

// fakeMetricsSample builds one deterministic sample from a caller-supplied
// phase. History passes a phase seeded by sample index; current passes a
// phase seeded by the current time so the stream ticks visibly.
func fakeMetricsSample(phase float64, timestamp time.Time, memoryMax int64) MetricsSample {
	wave := (math.Sin(phase) + 1) / 2 // normalized to 0..1

	return MetricsSample{
		Timestamp:    timestamp,
		CPUPercent:   10 + wave*60,
		MemoryUsed:   int64(float64(memoryMax) * (0.3 + wave*0.4)),
		MemoryMax:    memoryMax,
		DiskReadBps:  wave * 2_000_000,
		DiskWriteBps: wave * 1_000_000,
		NetInBps:     wave * 500_000,
		NetOutBps:    wave * 250_000,
	}
}
