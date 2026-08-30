<script lang="ts">
	import { getDashboardContext, type NodeSummary } from './dashboard.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { formatBytes } from '$lib/shared/format-bytes';
	import { m } from '$lib/paraglide/messages.js';
	import { post } from '$lib/shared/api/client';

	const store = getDashboardContext();

	async function handleClusterRetry(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// The refresh may fail (cluster still down) — reload picks up the
			// current state either way.
		}
		await store.load();
	}

	function usagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	function usageColor(percent: number): string {
		if (percent >= 90) return 'bg-destructive';
		if (percent >= 70) return 'bg-warning';
		return 'bg-success';
	}

	function cpuPercent(node: NodeSummary): number {
		return Math.min(100, Math.round(node.cpuUsage * 100));
	}

	function formatRefreshedAt(iso: string): string {
		return new Date(iso).toLocaleTimeString();
	}

	function goToNodeVms(nodeName: string): void {
		void goto(`${resolve('/vms')}?node=${encodeURIComponent(nodeName)}`);
	}

	function goToCreatePool(): void {
		void goto(resolve('/admin/pools'));
	}

	let showVmPopover = $state(false);
</script>

<PageHeader title={m['admin.dashboard.title']()}>
	{#snippet actions()}
		<div class="flex flex-col items-end gap-1">
			<Button
				variant="secondary"
				size="sm"
				loading={store.loading}
				onclick={() => void store.load()}
			>
				{m['common.refresh']()}
			</Button>
			{#if store.summary}
				<p class="text-xs text-muted-foreground" data-testid="dashboard-refreshed-at">
					{m['admin.dashboard.refreshed']()} <time datetime={store.summary.refreshedAt}>{formatRefreshedAt(store.summary.refreshedAt)}</time>
				</p>
			{/if}
		</div>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2" data-testid="dashboard-stats-skeleton">
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-24 w-full" />
	</div>
	<div class="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2" data-testid="dashboard-nodes-skeleton">
		<Skeleton class="h-40 w-full" />
		<Skeleton class="h-40 w-full" />
	</div>
{:else if store.errorCode === 'inventory_not_ready'}
	<EmptyState
		title={m['admin.dashboard.clusterUnreachableTitle']()}
		description={m['admin.dashboard.clusterUnreachableDescription']()}
		dataTestid="dashboard-cluster-unreachable"
	>
		{#snippet actions()}
			<button
				type="button"
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
				onclick={() => void handleClusterRetry()}
				data-testid="dashboard-cluster-retry"
			>
				{m['admin.dashboard.clusterUnreachableRetry']()}
			</button>
		{/snippet}
	</EmptyState>
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else if store.summary}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.dashboard.loaded']()}</div>

	<section class="space-y-6">
		<!-- Summary cards + Create pool shortcut -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<div class="rounded-lg border border-border bg-card p-4" data-testid="dashboard-card-nodes">
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.nodes']()}</p>
				<p class="text-3xl font-semibold">{store.summary.nodeCount}</p>
			</div>

			<div
				class="relative rounded-lg border border-border bg-card p-4"
				data-testid="dashboard-card-vms"
				role="button"
				tabindex="0"
				onmouseenter={() => (showVmPopover = true)}
				onmouseleave={() => (showVmPopover = false)}
				onfocus={() => (showVmPopover = true)}
				onblur={() => (showVmPopover = false)}
			>
				<p class="text-sm text-muted-foreground">{m['admin.dashboard.vms']()}</p>
				<p class="text-3xl font-semibold">{store.summary.vmCount}</p>
				{#if showVmPopover}
					<div
						class="absolute left-1/2 top-full z-10 mt-2 -translate-x-1/2 rounded-md border border-border bg-popover p-3 text-sm shadow-md"
						role="tooltip"
						data-testid="dashboard-vm-popover"
					>
						<p class="mb-1 font-medium text-muted-foreground">{m['admin.dashboard.vmCountHover']()}</p>
						<ul class="space-y-0.5">
							<li class="flex items-center gap-2">
								<span class="h-2 w-2 rounded-full bg-success"></span>
								{m['admin.dashboard.vmRunning']()}: <span class="font-mono">{store.summary.vmStatusCounts.running}</span>
							</li>
							<li class="flex items-center gap-2">
								<span class="h-2 w-2 rounded-full bg-warning"></span>
								{m['admin.dashboard.vmPaused']()}: <span class="font-mono">{store.summary.vmStatusCounts.paused}</span>
							</li>
							<li class="flex items-center gap-2">
								<span class="h-2 w-2 rounded-full bg-muted-foreground"></span>
								{m['admin.dashboard.vmStopped']()}: <span class="font-mono">{store.summary.vmStatusCounts.stopped}</span>
							</li>
							<li class="flex items-center gap-2">
								<span class="h-2 w-2 rounded-full bg-info"></span>
								{m['admin.dashboard.vmOther']()}: <span class="font-mono">{store.summary.vmStatusCounts.other}</span>
							</li>
						</ul>
					</div>
				{/if}
			</div>

			<div class="flex items-center justify-center rounded-lg border border-border bg-card p-4">
				<Button variant="primary" size="md" onclick={goToCreatePool} data-testid="dashboard-create-pool">
					{m['admin.dashboard.createPool']()}
				</Button>
			</div>
		</div>

		<!-- Nodes used by PVMSS -->
		{#if store.summary.nodes.length === 0}
			<EmptyState
				title={m['admin.dashboard.emptyTitle']()}
				description={m['admin.dashboard.emptyBody']()}
				dataTestid="dashboard-empty"
			>
				{#snippet actions()}
					<Button variant="primary" size="md" onclick={goToCreatePool}>
						{m['admin.dashboard.createPool']()}
					</Button>
				{/snippet}
			</EmptyState>
		{:else}
			<div class="space-y-2">
				<h2 class="text-lg font-medium">{m['admin.dashboard.nodesHeader']()}</h2>
				<div class="grid grid-cols-1 gap-4 md:grid-cols-2" data-testid="dashboard-nodes-grid">
					{#each store.summary.nodes as node (node.name)}
						{@const cpuPct = cpuPercent(node)}
						{@const memPct = usagePercent(node.memoryUsedBytes, node.memoryTotalBytes)}
						<button
							type="button"
							class="group flex flex-col gap-3 rounded-lg border border-border bg-card p-4 text-left transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
							onclick={() => goToNodeVms(node.name)}
							data-testid="dashboard-node-card"
							data-node={node.name}
						>
							<div class="flex items-center justify-between">
								<div class="flex items-center gap-2">
									<span
										class="h-2 w-2 rounded-full {node.status === 'online' ? 'bg-success' : 'bg-destructive'}"
										aria-hidden="true"
									></span>
									<span class="font-mono font-medium">{node.name}</span>
								</div>
								<span class="text-sm text-muted-foreground">
									{node.vmCount} {m['admin.dashboard.nodeVms']()}
								</span>
							</div>

							<div class="flex flex-col gap-1">
								<div class="flex items-center justify-between text-xs text-muted-foreground">
									<span>{m['admin.dashboard.cpu']()} · {node.cpuCores} {m['common.cores']()}</span>
									<span class="font-mono">{cpuPct}%</span>
								</div>
								<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
									<div class="h-full rounded-full {usageColor(cpuPct)}" style="width: {cpuPct}%"></div>
								</div>
							</div>

							<div class="flex flex-col gap-1">
								<div class="flex items-center justify-between text-xs text-muted-foreground">
									<span>{m['admin.dashboard.memory']()}</span>
									<span class="font-mono">{formatBytes(node.memoryUsedBytes)} / {formatBytes(node.memoryTotalBytes)}</span>
								</div>
								<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
									<div class="h-full rounded-full {usageColor(memPct)}" style="width: {memPct}%"></div>
								</div>
							</div>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<p class="text-sm text-muted-foreground">
			{m['admin.dashboard.version']()} {store.summary.version}
		</p>
	</section>
{/if}
