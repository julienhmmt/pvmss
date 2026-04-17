<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Button } from '$lib/components/ui/button';
	import { getNodes } from '$lib/api/admin/nodes';
	import { formatBytes, formatCpu, formatUptime, formatPercent } from '$lib/utils/format';
	import { HardDrives, ArrowsClockwise, Database, Cpu, Memory } from 'phosphor-svelte';
	import type { Node } from '$lib/types/admin';

	const AUTO_REFRESH_INTERVAL = 30_000;
	const AGE_TICK_INTERVAL = 1_000;
	const EMPTY_RETRY_INTERVAL = 5_000;
	const EMPTY_RETRY_MAX = 6; // 30 s max

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let nodes = $state<Node[]>([]);
	let lastUpdatedAgo = $state(0);
	let emptyRetryCount = $state(0);

	let runningCount = $derived(nodes.filter((n) => n.status === 'online').length);
	let offlineCount = $derived(nodes.filter((n) => n.status !== 'online').length);

	function usageBarClass(percent: number): string {
		if (percent >= 80) return 'pv-usage-bar-fill--danger';
		if (percent >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	function hasDisk(node: Node): boolean {
		return node.maxDisk > 0;
	}

	async function load() {
		loading = true;
		error = null;
		try {
			nodes = await getNodes();
			lastUpdatedAgo = 0;
			emptyRetryCount = 0;
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

		const emptyRetryTimer = setInterval(() => {
			if (!loading && !refreshing && !error && nodes.length === 0 && emptyRetryCount < EMPTY_RETRY_MAX) {
				emptyRetryCount += 1;
				refresh();
			}
		}, EMPTY_RETRY_INTERVAL);

		const ageTicker = setInterval(() => {
			lastUpdatedAgo = lastUpdatedAgo + 1;
		}, AGE_TICK_INTERVAL);

		return () => {
			clearInterval(autoRefreshTimer);
			clearInterval(ageTicker);
			clearInterval(emptyRetryTimer);
		};
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.nodes.title')}</title>
</svelte:head>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.nodes.title')}</h1>
			<p class="pv-subtitle">
				{$t('admin.nodes.updatedAgo', { values: { seconds: lastUpdatedAgo } })}
			</p>
		</div>

		{#if !loading}
			<div class="flex items-center gap-3 flex-wrap">
				<div class="pv-header-stats">
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('admin.nodes.title')}</div>
						<div class="pv-header-stat-value">{nodes.length}</div>
					</div>
					{#if runningCount > 0}
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.statusMap.online')}</div>
							<div class="pv-header-stat-value">{runningCount}</div>
						</div>
					{/if}
					{#if offlineCount > 0}
						<div class="pv-header-stat pv-header-stat--danger">
							<div class="pv-header-stat-label">{$t('common.statusMap.offline')}</div>
							<div class="pv-header-stat-value">{offlineCount}</div>
						</div>
					{/if}
				</div>
				<Button
					class="pv-header-btn"
					variant="outline"
					size="sm"
					onclick={refresh}
					disabled={refreshing || loading}
				>
					<ArrowsClockwise class="mr-1 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
					{$t('common.refresh')}
				</Button>
			</div>
		{/if}
	</div>
</div>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if nodes.length === 0}
	{#if emptyRetryCount < EMPTY_RETRY_MAX}
		<LoadingSkeleton variant="card" rows={3} />
	{:else}
		<EmptyState title={$t('admin.nodes.noNodes')} icon={HardDrives} />
	{/if}
{:else}
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
		{#each nodes as node}
			{@const cpuPercent = Math.round(node.cpu * 100)}
			{@const ramPercent = formatPercent(node.memory, node.maxMemory)}
			{@const diskPercent = hasDisk(node) ? formatPercent(node.disk, node.maxDisk) : 0}

			<div class="pv-table-wrap p-5 space-y-4">
				<!-- Node header -->
				<div class="flex items-start justify-between gap-2">
					<div class="pv-resource-cell">
						<div class="pv-resource-icon pv-resource-icon--node">
							{node.name.slice(0, 2).toUpperCase()}
						</div>
						<div>
							<div class="pv-resource-name">{node.name}</div>
							<div class="pv-td-muted text-xs">
								{$t('admin.nodes.cores', { values: { count: node.maxCpu } })}
							</div>
						</div>
					</div>
					<span class="pv-badge {node.status === 'online' ? 'pv-badge--online' : 'pv-badge--offline'} mt-0.5">
						{$t(`common.statusMap.${node.status}`, { default: node.status })}
					</span>
				</div>

				<div class="border-t border-border"></div>

				<!-- CPU -->
				<div class="space-y-1.5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
							<Cpu class="h-3.5 w-3.5" />
							{$t('admin.nodes.cpu')}
						</span>
						<span class="text-sm font-semibold tabular-nums">{formatCpu(node.cpu)}</span>
					</div>
					<div class="pv-usage-bar">
						<div class="pv-usage-bar-track">
							<div
								class="pv-usage-bar-fill {usageBarClass(cpuPercent)}"
								style="width: {cpuPercent}%"
							></div>
						</div>
						<span class="pv-usage-label">{cpuPercent}%</span>
					</div>
				</div>

				<!-- RAM -->
				<div class="space-y-1.5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
							<Memory class="h-3.5 w-3.5" />
							{$t('admin.nodes.ram')}
						</span>
						<span class="text-sm tabular-nums">
							<span class="font-semibold">{formatBytes(node.memory)}</span>
							<span class="text-muted-foreground text-xs"> / {formatBytes(node.maxMemory)}</span>
						</span>
					</div>
					<div class="pv-usage-bar">
						<div class="pv-usage-bar-track">
							<div
								class="pv-usage-bar-fill {usageBarClass(ramPercent)}"
								style="width: {ramPercent}%"
							></div>
						</div>
						<span class="pv-usage-label">{ramPercent}%</span>
					</div>
				</div>

				<!-- Disk (only if data available) -->
				{#if hasDisk(node)}
					<div class="space-y-1.5">
						<div class="flex items-center justify-between">
							<span class="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
								<Database class="h-3.5 w-3.5" />
								{$t('admin.nodes.disk')}
							</span>
							<span class="text-sm tabular-nums">
								<span class="font-semibold">{formatBytes(node.disk)}</span>
								<span class="text-muted-foreground text-xs"> / {formatBytes(node.maxDisk)}</span>
							</span>
						</div>
						<div class="pv-usage-bar">
							<div class="pv-usage-bar-track">
								<div
									class="pv-usage-bar-fill {usageBarClass(diskPercent)}"
									style="width: {diskPercent}%"
								></div>
							</div>
							<span class="pv-usage-label">{diskPercent}%</span>
						</div>
					</div>
				{/if}

				<!-- Uptime -->
				<div class="flex items-center justify-between text-xs border-t border-border pt-3">
					<span class="text-muted-foreground">{$t('admin.nodes.uptime')}</span>
					<span class="font-medium tabular-nums">{formatUptime(node.uptime)}</span>
				</div>
			</div>
		{/each}
	</div>
{/if}

</div>
