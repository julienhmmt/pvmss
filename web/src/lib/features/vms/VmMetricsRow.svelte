<script lang="ts">
	/**
	 * VmMetricsRow — CPU/memory/disk/network sparklines for the Overview tab,
	 * below the existing static stat cards. History is fetched once on mount;
	 * live ticks stream over SSE while the VM is running and merge onto the
	 * same series without disturbing the selected range.
	 */
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import { MetricsStore, type MetricsRange } from './metrics.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import LineChart from '$lib/shared/ui/LineChart.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

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

<section
	aria-labelledby="metrics-heading"
	class="mt-6 rounded-xl border border-border bg-card p-6 shadow-card"
	data-testid="vm-metrics-row"
>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex flex-wrap items-center gap-2">
			<h2 id="metrics-heading" class="text-sm font-medium text-muted-foreground">
				{m['vms.detail.metricsHeading']()}
			</h2>
			{#if store.streamState === 'reconnecting'}
				<span class="text-xs text-amber-500" data-testid="vm-metrics-stream-reconnecting">
					{m['vms.detail.metricsStreamReconnecting']()}
				</span>
			{:else if store.streamState === 'error'}
				<span class="text-xs text-destructive" data-testid="vm-metrics-stream-error">
					{m['vms.detail.metricsStreamError']()}
				</span>
			{:else if store.streamState === 'connected'}
				<span class="text-xs text-success" data-testid="vm-metrics-stream-live">
					{m['vms.detail.metricsStreamLive']()}
				</span>
			{/if}
		</div>
		<div class="inline-flex rounded-lg border border-border" role="group" aria-label={m['vms.detail.metricsRangeLabel']()}>
			{#each ranges as r (r.value)}
				<button
					type="button"
					class="px-2.5 py-1.5 text-xs font-semibold first:rounded-l-lg last:rounded-r-lg {store.range === r.value
						? 'bg-primary text-primary-foreground'
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
		<div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="vm-metrics-charts">
			<div class="rounded-lg bg-muted/40 p-4">
				<p class="text-xs text-muted-foreground">{m['common.cpu']()}</p>
				<LineChart values={cpuValues} label={m['common.cpu']()} class="mt-3 h-16 w-full" />
			</div>
			<div class="rounded-lg bg-muted/40 p-4">
				<p class="text-xs text-muted-foreground">{m['common.memory']()}</p>
				<LineChart values={memoryValues} label={m['common.memory']()} class="mt-3 h-16 w-full" />
			</div>
			<div class="rounded-lg bg-muted/40 p-4">
				<p class="text-xs text-muted-foreground">{m['vms.detail.metricsDiskIO']()}</p>
				<LineChart values={diskValues} label={m['vms.detail.metricsDiskIO']()} class="mt-3 h-16 w-full" />
			</div>
			<div class="rounded-lg bg-muted/40 p-4">
				<p class="text-xs text-muted-foreground">{m['vms.detail.metricsNetworkIO']()}</p>
				<LineChart values={netValues} label={m['vms.detail.metricsNetworkIO']()} class="mt-3 h-16 w-full" />
			</div>
		</div>
	{/if}
</section>
