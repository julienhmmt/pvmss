<script lang="ts">
	/**
	 * VmMetricsRow - CPU/memory/disk/network sparklines for the Overview tab,
	 * below the existing static stat cards. History is fetched once on mount;
	 * live ticks stream over SSE while the VM is running and merge onto the
	 * same series without disturbing the selected range.
	 */
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import { MetricsStore, type MetricsRange } from './metrics.svelte';
	import { formatMetricPercent, formatMetricRate, formatMetricSize, metricLast, metricPeak } from './metrics.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import LineChart from '#lib/shared/ui/LineChart.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const detail = getVmDetailContext();
	const isRunning = $derived(detail.entity?.status === 'running');
	const store = new MetricsStore(detail.cluster, detail.vmid, () => isRunning);

	onMount(() => {
		void store.loadHistory();
	});

	$effect(() => {
		if (isRunning) {
			store.connect();
		} else {
			store.disconnect();
		}

		return () => {
			store.disconnect();
		};
	});

	const ranges: { value: MetricsRange; label: () => string }[] = [
		{ value: 'hour', label: () => m['vms.detail.metricsRangeHour']() },
		{ value: 'day', label: () => m['vms.detail.metricsRangeDay']() },
		{ value: 'week', label: () => m['vms.detail.metricsRangeWeek']() }
	];

	const cpuValues = $derived(store.samples.map((s) => s.cpuPercent));
	const memoryValues = $derived(store.samples.map((s) => s.memoryUsedBytes));
	const diskValues = $derived(store.samples.map((s) => s.diskReadBytesPerSec + s.diskWriteBytesPerSec));
	const netValues = $derived(store.samples.map((s) => s.netInBytesPerSec + s.netOutBytesPerSec));
</script>

<Card
	pad="lg"
	aria-labelledby="metrics-heading"
	class="mt-6"
	data-testid="vm-metrics-row"
>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex flex-wrap items-center gap-2">
			<h2 id="metrics-heading" class="text-sm font-medium text-muted-foreground">
				{m['vms.detail.metricsHeading']()}
			</h2>
			{#if store.streamState === 'reconnecting'}
				<span class="text-xs text-warning-soft-foreground" data-testid="vm-metrics-stream-reconnecting">
					{m['vms.detail.metricsStreamReconnecting']()}
				</span>
			{:else if store.streamState === 'error'}
				<span class="text-xs text-destructive-soft-foreground" data-testid="vm-metrics-stream-error">
					{m['vms.detail.metricsStreamError']()}
				</span>
			{:else if store.streamState === 'connected'}
				<!-- Status words use the soft-foreground text tokens: the fill colors (text-success, ...) are for dots and stay under AA as 12px text. -->
				<span class="text-xs text-success-soft-foreground" data-testid="vm-metrics-stream-live">
					{m['vms.detail.metricsStreamLive']()}
				</span>
			{/if}
		</div>
		<div class="inline-flex rounded-lg border border-border" role="group" aria-label={m['vms.detail.metricsRangeLabel']()}>
			{#each ranges as r (r.value)}
				<button
					type="button"
					class="px-2.5 py-1.5 text-xs font-semibold first:rounded-l-lg last:rounded-r-lg {store.range === r.value
						? 'bg-primary-solid text-primary-foreground'
						: 'hover:bg-muted'}"
					aria-pressed={store.range === r.value}
					onclick={() => void store.setRange(r.value)}
					data-testid="vm-metrics-range-{r.value}"
				>
					{r.label()}
				</button>
			{/each}
		</div>
	</div>

	{#if store.error}
		<Alert data-testid="vm-metrics-error" class="mt-4">{store.error}</Alert>
	{:else if store.loading && store.samples.length === 0}
		<div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="vm-metrics-loading">
			<Skeleton class="h-24 w-full" />
			<Skeleton class="h-24 w-full" />
			<Skeleton class="h-24 w-full" />
			<Skeleton class="h-24 w-full" />
		</div>
	{:else}
		{#snippet metricTile(title: string, current: string, peak: string, values: number[], key: string)}
			<div class="rounded-lg bg-muted/40 p-4">
				<div class="flex items-baseline justify-between gap-2">
					<p class="text-xs text-muted-foreground">{title}</p>
					<p class="text-right text-sm font-semibold tabular-nums" data-testid="vm-metrics-{key}-current">
						{current}
						{#if values.length > 0}
							<span class="ml-1 text-2xs font-normal text-muted-foreground-subtle">{m['vms.detail.metricsPeak']({ value: peak })}</span>
						{/if}
					</p>
				</div>
				<LineChart {values} label={title} class="mt-3 h-16 w-full" />
			</div>
		{/snippet}
		<div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="vm-metrics-charts">
			{@render metricTile(m['common.cpu'](), formatMetricPercent(metricLast(cpuValues)), formatMetricPercent(metricPeak(cpuValues)), cpuValues, 'cpu')}
			{@render metricTile(m['common.memory'](), formatMetricSize(metricLast(memoryValues)), formatMetricSize(metricPeak(memoryValues)), memoryValues, 'memory')}
			{@render metricTile(m['vms.detail.metricsDiskIO'](), formatMetricRate(metricLast(diskValues)), formatMetricRate(metricPeak(diskValues)), diskValues, 'disk')}
			{@render metricTile(m['vms.detail.metricsNetworkIO'](), formatMetricRate(metricLast(netValues)), formatMetricRate(metricPeak(netValues)), netValues, 'net')}
		</div>
	{/if}
</Card>
