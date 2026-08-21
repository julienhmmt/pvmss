<script lang="ts">
	/**
	 * VmMetricsRow — historical CPU/memory/disk/network sparklines for the
	 * Overview tab, below the existing static stat cards. Ticket 02 scope:
	 * history only, hour/day/week toggle. Ticket 03 adds a live tick on top
	 * of the same MetricsStore.
	 */
	import { onMount } from 'svelte';
	import { getVmDetailContext } from './detail.svelte';
	import { MetricsStore, type MetricsRange } from './metrics.svelte';
	import LineChart from '$lib/shared/ui/LineChart.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const detail = getVmDetailContext();
	const store = new MetricsStore(detail.cluster, detail.vmid);

	onMount(() => {
		void store.loadHistory();
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

<section aria-labelledby="metrics-heading" class="mt-6" data-testid="vm-metrics-row">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 id="metrics-heading" class="text-sm font-medium text-muted-foreground">{m['vms.detail.metricsHeading']()}</h2>
		<div class="inline-flex rounded-md border border-border" role="group" aria-label={m['vms.detail.metricsRangeLabel']()}>
			{#each ranges as r (r.value)}
				<button
					type="button"
					class="px-2.5 py-1 text-xs font-medium first:rounded-l-md last:rounded-r-md {store.range === r.value
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
		<p role="alert" class="mt-2 text-sm text-destructive" data-testid="vm-metrics-error">{store.error}</p>
	{:else if store.loading && store.samples.length === 0}
		<div class="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="vm-metrics-loading">
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
			<Skeleton class="h-20 w-full" />
		</div>
	{:else}
		<div class="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="vm-metrics-charts">
			<div class="rounded-md border border-border p-3">
				<p class="text-xs text-muted-foreground">{m['common.cpu']()}</p>
				<LineChart values={cpuValues} label={m['common.cpu']()} />
			</div>
			<div class="rounded-md border border-border p-3">
				<p class="text-xs text-muted-foreground">{m['common.memory']()}</p>
				<LineChart values={memoryValues} label={m['common.memory']()} />
			</div>
			<div class="rounded-md border border-border p-3">
				<p class="text-xs text-muted-foreground">{m['vms.detail.metricsDiskIO']()}</p>
				<LineChart values={diskValues} label={m['vms.detail.metricsDiskIO']()} />
			</div>
			<div class="rounded-md border border-border p-3">
				<p class="text-xs text-muted-foreground">{m['vms.detail.metricsNetworkIO']()}</p>
				<LineChart values={netValues} label={m['vms.detail.metricsNetworkIO']()} />
			</div>
		</div>
	{/if}
</section>
