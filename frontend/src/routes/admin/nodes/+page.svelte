<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import Paginator from '$lib/components/data/Paginator.svelte';
	import { paginate } from '$lib/utils/paginate';
	import { Button } from '$lib/components/ui/button';
	import { getNodes, toggleNode } from '$lib/api/admin/nodes';
	import { formatBytes, formatCpu, formatUptime, formatPercent } from '$lib/utils/format';
	import { HardDrives, ArrowsClockwise, Database, Cpu, Memory, Eye, EyeSlash } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
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
	let toggling = $state<string | null>(null);

	let runningCount = $derived(nodes.filter((n) => n.status === 'online').length);
	let offlineCount = $derived(nodes.filter((n) => n.status !== 'online').length);

	let page = $state(1);
	let perPage = $state(12);
	const pagedNodes = $derived(paginate(nodes, page, perPage));

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

	async function handleToggleNode(node: Node) {
		toggling = node.name;
		try {
			await toggleNode(node.name);
			const key = node.userEnabled ? 'disabled' : 'enabled';
			toast.success($t(`admin.nodes.toast.${key}`, { values: { name: node.name } }));
			await refresh();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			toggling = null;
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

<PvHeader
	eyebrow={$t('nav.administration')}
	title={$t('admin.nodes.title')}
	subtitle={$t('admin.nodes.updatedAgo', { values: { seconds: lastUpdatedAgo } })}
>
	{#snippet stats()}
		{#if !loading}
			<PvHeaderStat label={$t('admin.nodes.title')} value={nodes.length} />
			{#if runningCount > 0}
				<PvHeaderStat label={$t('common.statusMap.online')} value={runningCount} />
			{/if}
			{#if offlineCount > 0}
				<PvHeaderStat label={$t('common.statusMap.offline')} value={offlineCount} tone="danger" />
			{/if}
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if !loading}
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
		{/if}
	{/snippet}
</PvHeader>

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
		{#each pagedNodes as node (node.name)}
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
							<div class="text-xs {node.userEnabled ? 'text-success' : 'text-muted-foreground/60'}">
								{node.userEnabled ? $t('admin.nodes.userEnabled') : $t('admin.nodes.userDisabled')}
							</div>
						</div>
					</div>
					<div class="flex items-center gap-1.5 mt-0.5">
						<span class="pv-badge {node.status === 'online' ? 'pv-badge--online' : 'pv-badge--offline'}">
							{$t(`common.statusMap.${node.status}`, { default: node.status })}
						</span>
						<Button
							size="sm"
							variant="ghost"
							class="h-7 w-7 p-0 {node.userEnabled ? 'text-emerald-600 hover:text-emerald-700' : 'text-muted-foreground hover:text-foreground'}"
							title={node.userEnabled ? $t('admin.nodes.disableForUsers') : $t('admin.nodes.enableForUsers')}
							disabled={toggling === node.name}
							onclick={() => handleToggleNode(node)}
						>
							{#if node.userEnabled}
								<Eye class="h-4 w-4" />
							{:else}
								<EyeSlash class="h-4 w-4" />
							{/if}
						</Button>
					</div>
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
					<div class="text-xs text-muted-foreground tabular-nums">
						{$t('admin.nodes.cpuDetails', { values: { cores: node.maxCpu, sockets: node.cpuSockets } })}
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

	<Paginator
		total={nodes.length}
		bind:page
		bind:perPage
		perPageOptions={[12, 24, 48, 96]}
	/>
{/if}

</div>
