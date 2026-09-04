<script lang="ts">
	import { getDashboardContext, type NodeSummary } from './dashboard.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import StatCard from '$lib/shared/ui/StatCard.svelte';
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

	// The four status rows in the VM popover differed only by dot colour,
	// label and counter key — one list, not four near-identical blocks.
	const VM_STATUS_BREAKDOWN = [
		{ key: 'running', dot: 'bg-success', label: () => m['admin.dashboard.vmRunning']() },
		{ key: 'paused', dot: 'bg-warning', label: () => m['admin.dashboard.vmPaused']() },
		{ key: 'stopped', dot: 'bg-muted-foreground', label: () => m['admin.dashboard.vmStopped']() },
		{ key: 'other', dot: 'bg-info', label: () => m['admin.dashboard.vmOther']() }
	] as const;

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

<!-- Deliberate pause, not a bug: cloud-image/cloud-init-template creation is
     gated off server-side (server/internal/httpapi/vm_create.go
     cloudImageFeatureEnabled) — per-VM cloud-init forking needs PVMSS to
     hold SSH credentials to every Proxmox node, judged too much new scope
     for now. This notice exists so an admin who notices the Images/
     Cloud-init templates pages missing from the sidebar knows why. -->
<div
	class="mb-6 rounded-xl border border-warning-soft-border bg-warning-soft p-4 text-sm text-warning-soft-foreground"
	data-testid="dashboard-cloud-image-paused-notice"
>
	<p class="font-medium">{m['admin.dashboard.cloudImagePausedTitle']()}</p>
	<p class="mt-1">{m['admin.dashboard.cloudImagePausedBody']()}</p>
</div>

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
		tone="error"
		dataTestid="dashboard-cluster-unreachable"
	>
		{#snippet actions()}
			<Button onclick={() => void handleClusterRetry()} data-testid="dashboard-cluster-retry">
				{m['admin.dashboard.clusterUnreachableRetry']()}
			</Button>
		{/snippet}
	</EmptyState>
{:else if store.error}
	<p
		role="alert"
		class="rounded-xl border border-destructive-soft-border bg-destructive-soft px-4 py-3 text-sm font-medium text-destructive-soft-foreground"
	>
		{store.error}
	</p>
{:else if store.summary}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.dashboard.loaded']()}</div>

	<section class="space-y-6">
		<!-- Summary tiles + Create pool shortcut -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<StatCard
				label={m['admin.dashboard.nodes']()}
				value={store.summary.nodeCount}
				data-testid="dashboard-card-nodes"
			/>

			<!-- The VM tile carries a status breakdown on hover/focus, so it stays
			     a hand-built tile: StatCard has no popover slot and should not
			     grow one for a single call site. -->
			<div
				class="relative rounded-xl border border-border bg-card p-4 shadow-card"
				data-testid="dashboard-card-vms"
				role="button"
				tabindex="0"
				onmouseenter={() => (showVmPopover = true)}
				onmouseleave={() => (showVmPopover = false)}
				onfocus={() => (showVmPopover = true)}
				onblur={() => (showVmPopover = false)}
			>
				<p class="text-[0.6875rem] font-semibold uppercase tracking-[0.06em] text-muted-foreground">
					{m['admin.dashboard.vms']()}
				</p>
				<p class="mt-1.5 font-mono text-3xl font-semibold leading-none tracking-tight tabular-nums">
					{store.summary.vmCount}
				</p>
				{#if showVmPopover}
					<div
						class="absolute left-1/2 top-full z-10 mt-2 w-max -translate-x-1/2 rounded-xl border border-border bg-popover p-3 text-sm shadow-overlay"
						role="tooltip"
						data-testid="dashboard-vm-popover"
					>
						<p class="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.06em] text-muted-foreground">
							{m['admin.dashboard.vmCountHover']()}
						</p>
						<ul class="grid gap-1">
							{#each VM_STATUS_BREAKDOWN as row (row.key)}
								<li class="flex items-center gap-2">
									<span class="h-2 w-2 shrink-0 rounded-full {row.dot}" aria-hidden="true"></span>
									<span class="flex-1">{row.label()}</span>
									<span class="ml-3 font-mono tabular-nums">{store.summary.vmStatusCounts[row.key]}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			</div>

			<div class="flex items-center justify-center rounded-xl border border-dashed border-border bg-card/60 p-4">
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
			<div class="space-y-3">
				<h2 class="text-sm font-semibold uppercase tracking-[0.06em] text-muted-foreground">
					{m['admin.dashboard.nodesHeader']()}
				</h2>
				<div class="grid grid-cols-1 gap-4 md:grid-cols-2" data-testid="dashboard-nodes-grid">
					{#each store.summary.nodes as node (node.name)}
						{@const cpuPct = cpuPercent(node)}
						{@const memPct = usagePercent(node.memoryUsedBytes, node.memoryTotalBytes)}
						<button
							type="button"
							class="group flex flex-col gap-4 rounded-xl border border-border bg-card p-4 text-left shadow-card transition-[box-shadow,border-color] duration-150 hover:border-muted-foreground-subtle hover:shadow-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
							onclick={() => goToNodeVms(node.name)}
							data-testid="dashboard-node-card"
							data-node={node.name}
						>
							<div class="flex items-center justify-between gap-3">
								<div class="flex min-w-0 items-center gap-2">
									<span
										class="h-2 w-2 shrink-0 rounded-full {node.status === 'online'
											? 'bg-success'
											: 'bg-destructive'}"
										aria-hidden="true"
									></span>
									<span class="truncate font-mono text-sm font-semibold">{node.name}</span>
								</div>
								<span class="shrink-0 text-xs text-muted-foreground">
									<span class="font-mono tabular-nums">{node.vmCount}</span>
									{m['admin.dashboard.nodeVms']()}
								</span>
							</div>

							<div class="grid gap-3">
								{#each [{ label: `${m['admin.dashboard.cpu']()} · ${node.cpuCores} ${m['common.cores']()}`, value: `${cpuPct}%`, pct: cpuPct }, { label: m['admin.dashboard.memory'](), value: `${formatBytes(node.memoryUsedBytes)} / ${formatBytes(node.memoryTotalBytes)}`, pct: memPct }] as meter (meter.label)}
									<div class="flex flex-col gap-1.5">
										<div class="flex items-center justify-between gap-3 text-xs text-muted-foreground">
											<span class="truncate">{meter.label}</span>
											<span class="shrink-0 font-mono tabular-nums">{meter.value}</span>
										</div>
										<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
											<div
												class="h-full rounded-full {usageColor(meter.pct)} transition-[width] duration-500"
												style="width: {meter.pct}%"
											></div>
										</div>
									</div>
								{/each}
							</div>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<p class="text-xs text-muted-foreground-subtle">
			{m['admin.dashboard.version']()} <span class="font-mono">{store.summary.version}</span>
		</p>
	</section>
{/if}
