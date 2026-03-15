<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import { Button } from '$lib/components/ui/button';
	import { getNodes } from '$lib/api/admin/nodes';
	import { formatBytes, formatCpu, formatUptime, formatPercent } from '$lib/utils/format';
	import { HardDrives, ArrowsClockwise } from 'phosphor-svelte';
	import type { Node } from '$lib/types/admin';

	const AUTO_REFRESH_INTERVAL = 30_000;
	const AGE_TICK_INTERVAL = 1_000;

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let nodes = $state<Node[]>([]);
	let lastUpdatedAgo = $state(0);

	function progressColor(percent: number): string {
		if (percent >= 80) return 'bg-destructive';
		if (percent >= 60) return 'bg-chart-2';
		return 'bg-primary';
	}

	async function load() {
		loading = true;
		error = null;
		try {
			nodes = await getNodes();
			lastUpdatedAgo = 0;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function refresh() {
		refreshing = true;
		error = null;
		try {
			nodes = await getNodes();
			lastUpdatedAgo = 0;
		} catch (e) {
			error = e as Error;
		} finally {
			refreshing = false;
		}
	}

	onMount(() => {
		load();

		const autoRefreshTimer = setInterval(() => {
			if (!document.hidden && !refreshing && !loading) {
				refresh();
			}
		}, AUTO_REFRESH_INTERVAL);

		const ageTicker = setInterval(() => {
			lastUpdatedAgo = lastUpdatedAgo + 1;
		}, AGE_TICK_INTERVAL);

		return () => {
			clearInterval(autoRefreshTimer);
			clearInterval(ageTicker);
		};
	});
</script>

<PageHeader title="Nodes" icon={HardDrives}>
	{#snippet actions()}
		<span class="text-muted-foreground text-xs">
			Updated {lastUpdatedAgo}s ago
		</span>
		<Button variant="outline" size="sm" onclick={refresh} disabled={refreshing || loading}>
			<ArrowsClockwise class="mr-1 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
			Refresh
		</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if nodes.length === 0}
	<EmptyState title="No nodes found" icon={HardDrives} />
{:else}
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
		{#each nodes as node}
			{@const cpuPercent = Math.round(node.cpu * 100)}
			{@const ramPercent = formatPercent(node.memory, node.max_memory)}
			{@const diskPercent = formatPercent(node.disk, node.max_disk)}
			<div class="space-y-4 rounded-lg border p-4">
				<div class="flex items-center justify-between">
					<h3 class="font-semibold">{node.name}</h3>
					<StatusBadge status={node.status} />
				</div>

				<!-- CPU -->
				<div class="space-y-1">
					<div class="flex items-center justify-between text-sm">
						<span class="text-muted-foreground">CPU</span>
						<span class="font-medium">{formatCpu(node.cpu)} ({node.maxcpu} cores)</span>
					</div>
					<div class="bg-muted h-2 w-full rounded-full">
						<div
							class="h-2 rounded-full {progressColor(cpuPercent)}"
							style="width: {cpuPercent}%"
						></div>
					</div>
				</div>

				<!-- RAM -->
				<div class="space-y-1">
					<div class="flex items-center justify-between text-sm">
						<span class="text-muted-foreground">RAM</span>
						<span class="font-medium"
							>{formatBytes(node.memory)} / {formatBytes(node.max_memory)}</span
						>
					</div>
					<div class="bg-muted h-2 w-full rounded-full">
						<div
							class="h-2 rounded-full {progressColor(ramPercent)}"
							style="width: {ramPercent}%"
						></div>
					</div>
				</div>

				<!-- Disk -->
				<div class="space-y-1">
					<div class="flex items-center justify-between text-sm">
						<span class="text-muted-foreground">Disk</span>
						<span class="font-medium"
							>{formatBytes(node.disk)} / {formatBytes(node.max_disk)}</span
						>
					</div>
					<div class="bg-muted h-2 w-full rounded-full">
						<div
							class="h-2 rounded-full {progressColor(diskPercent)}"
							style="width: {diskPercent}%"
						></div>
					</div>
				</div>

				<!-- Uptime -->
				<div class="flex items-center justify-between text-sm">
					<span class="text-muted-foreground">Uptime</span>
					<span class="font-medium">{formatUptime(node.uptime)}</span>
				</div>
			</div>
		{/each}
	</div>
{/if}
