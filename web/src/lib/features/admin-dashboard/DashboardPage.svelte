<script lang="ts">
	import { getDashboardContext, type NodeSummary } from './dashboard.svelte';
	import { usageTone } from './dashboard-alerts';
	import DashboardAttention from './DashboardAttention.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
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
			// The refresh may fail (cluster still down) - reload picks up the
			// current state either way.
		}
		await store.load();
	}

	function usagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	const TONE_BG = { success: 'bg-success', warning: 'bg-warning', destructive: 'bg-destructive' } as const;

	function cpuPercent(node: NodeSummary): number {
		return Math.min(100, Math.round(node.cpuUsage * 100));
	}

	function goToNodeDetails(node: NodeSummary): void {
		void goto(resolve('/admin/nodes/[cluster]/[node]', { cluster: node.clusterKey, node: node.name }));
	}

	function goToCreateUser(): void {
		void goto(resolve('/admin/pools'));
	}

	// Shown in clear, not behind a hover: an admin reads the split at a glance.
	const VM_STATUS_BREAKDOWN = [
		{ key: 'running', dot: 'bg-success', label: () => m['admin.dashboard.vmRunning']() },
		{ key: 'paused', dot: 'bg-warning', label: () => m['admin.dashboard.vmPaused']() },
		{ key: 'stopped', dot: 'bg-muted-foreground', label: () => m['admin.dashboard.vmStopped']() },
		{ key: 'other', dot: 'bg-info', label: () => m['admin.dashboard.vmOther']() }
	] as const;
</script>

{#snippet bar(pct: number)}
	<div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
		<div class="h-full rounded-full {TONE_BG[usageTone(pct)]} transition-[width] duration-500 motion-reduce:transition-none" style="width: {pct}%"></div>
	</div>
{/snippet}

<PageHeader title={m['admin.dashboard.title']()}>
	{#snippet actions()}
		<div class="flex flex-col items-end gap-1">
			<div class="flex items-center gap-2">
				<Button variant="secondary" size="sm" onclick={goToCreateUser} data-testid="dashboard-create-user">
					{m['admin.dashboard.createUser']()}
				</Button>
				<Button variant="secondary" size="sm" loading={store.loading} onclick={() => void store.load()}>
					{m['common.refresh']()}
				</Button>
			</div>
			{#if store.summary}
				<p class="text-xs text-muted-foreground" data-testid="dashboard-refreshed-at">
					{m['admin.dashboard.refreshed']()} <time datetime={store.summary.refreshedAt}>{new Date(store.summary.refreshedAt).toLocaleTimeString()}</time>
				</p>
			{/if}
		</div>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<div class="grid gap-4" data-testid="dashboard-stats-skeleton">
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-16 w-full" />
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
	<Alert>{store.error}</Alert>
{:else if store.summary}
	{@const summary = store.summary}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.dashboard.loaded']()}</div>

	<section class="space-y-6">
		<DashboardAttention alerts={summary.alerts} />

		<Card pad="none">
			<div class="px-5 py-4 text-sm" data-testid="dashboard-vm-status">
				<div class="flex flex-wrap items-center gap-x-6 gap-y-2">
					<p class="font-semibold text-foreground">
						{m['admin.dashboard.vmStatusTitle']()}
						<span class="ml-1 font-normal text-muted-foreground" data-testid="dashboard-vm-total">
							{m['admin.dashboard.vmTotal']({ count: summary.pvmssVMCount })}
						</span>
					</p>
					<ul class="flex flex-wrap items-center gap-x-5 gap-y-1">
						{#each VM_STATUS_BREAKDOWN as row (row.key)}
							<li class="flex items-center gap-2" data-testid="dashboard-vm-status-{row.key}">
								<span class="h-2 w-2 shrink-0 rounded-full {row.dot}" aria-hidden="true"></span>
								<span class="text-muted-foreground">{row.label()}</span>
								<span class="font-mono font-semibold tabular-nums">{summary.pvmssVMStatusCounts[row.key]}</span>
							</li>
						{/each}
					</ul>
				</div>
				<div class="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-border pt-3" data-testid="dashboard-vm-other-count">
					<p class="font-semibold text-foreground">{m['admin.dashboard.vmOtherTitle']()}</p>
					<span class="font-mono font-semibold tabular-nums">{summary.otherVMCount}</span>
					<span class="text-muted-foreground">{m['admin.dashboard.vmOtherDescription']()}</span>
				</div>
			</div>
		</Card>

		{#if summary.nodes.length === 0}
			<EmptyState title={m['admin.dashboard.emptyTitle']()} description={m['admin.dashboard.emptyBody']()} dataTestid="dashboard-empty">
				{#snippet actions()}
					<Button variant="primary" size="md" onclick={goToCreateUser}>{m['admin.dashboard.createUser']()}</Button>
				{/snippet}
			</EmptyState>
		{:else}
			<div class="space-y-3">
				<h2 class="text-sm font-semibold text-foreground">{m['admin.dashboard.nodesHeader']()}</h2>
				<div class="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3" data-testid="dashboard-nodes-grid">
					{#each summary.nodes as node (`${node.cluster}/${node.name}`)}
						{@const cpuPct = cpuPercent(node)}
						{@const memPct = usagePercent(node.memoryUsedBytes, node.memoryTotalBytes)}
						<button
							type="button"
							class="flex flex-col gap-4 rounded-xl border border-border bg-card p-4 text-left shadow-card transition-[box-shadow,border-color] duration-150 hover:border-muted-foreground-subtle hover:shadow-raised pv-focus"
							onclick={() => goToNodeDetails(node)}
							data-testid="dashboard-node-card"
							data-node={node.name}
						>
							<div class="flex items-start justify-between gap-3">
								<div class="flex min-w-0 items-start gap-2">
									<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full {node.status === 'online' ? 'bg-success' : 'bg-destructive'}" aria-hidden="true"></span>
									<span class="flex min-w-0 flex-col">
										<span class="truncate font-mono text-sm font-semibold">{node.name}</span>
										<span class="truncate text-xs text-muted-foreground">{node.cluster}</span>
									</span>
								</div>
								<span class="shrink-0 text-xs text-muted-foreground tabular-nums">
									{m['admin.dashboard.nodeVmsRunning']({ running: node.vmRunningCount, total: node.vmCount })}
								</span>
							</div>
							<div class="grid gap-3">
								{#each [{ label: `${m['admin.dashboard.cpu']()} · ${node.cpuCores} ${m['common.coreCount']({ count: node.cpuCores })}`, value: `${cpuPct}%`, pct: cpuPct }, { label: m['admin.dashboard.memory'](), value: `${formatBytes(node.memoryUsedBytes)} / ${formatBytes(node.memoryTotalBytes)}`, pct: memPct }] as meter (meter.label)}
									<div class="flex flex-col gap-1.5">
										<div class="flex items-center justify-between gap-3 text-xs text-muted-foreground">
											<span class="truncate">{meter.label}</span>
											<span class="shrink-0 font-mono tabular-nums">{meter.value}</span>
										</div>
										{@render bar(meter.pct)}
									</div>
								{/each}
							</div>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
			<Card pad="none" title={m['admin.dashboard.storageTitle']()}>
				{#if summary.storages.length === 0}
					<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.dashboard.storageEmpty']()}</p>
				{:else}
					<ul class="divide-y divide-border" data-testid="dashboard-storages">
						{#each summary.storages as storage (`${storage.cluster}/${storage.node ?? ''}/${storage.name}`)}
							<li class="grid gap-1.5 px-5 py-3" data-testid="dashboard-storage" data-shared={storage.shared}>
								<div class="flex items-center justify-between gap-3 text-sm">
									<span class="min-w-0 truncate">
										<span class="font-mono font-semibold">{storage.name}</span>
										<span class="ml-2 font-mono text-xs text-muted-foreground">
											{storage.cluster} · {storage.shared ? m['admin.dashboard.storageShared']() : storage.node} · {storage.type}
										</span>
									</span>
									<span class="shrink-0 font-mono text-xs tabular-nums">{storage.percent}%</span>
								</div>
								{@render bar(Math.min(100, storage.percent))}
								<p class="text-xs text-muted-foreground">
									{m['admin.dashboard.storageFree']({ free: formatBytes(Math.max(0, storage.totalBytes - storage.usedBytes)), total: formatBytes(storage.totalBytes) })}
								</p>
							</li>
						{/each}
					</ul>
				{/if}
			</Card>

			<Card pad="none" title={m['admin.dashboard.recentTitle']()}>
				{#snippet actions()}
					<a href={resolve('/admin/settings')} class="text-xs font-medium text-primary hover:underline pv-focus">
						{m['admin.dashboard.recentAll']()}
					</a>
				{/snippet}
				{#if summary.recentChanges.length === 0}
					<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.dashboard.recentEmpty']()}</p>
				{:else}
					<ul class="divide-y divide-border" data-testid="dashboard-recent-changes">
						{#each summary.recentChanges as change (change.id)}
							<li class="flex items-baseline gap-3 px-5 py-2.5 text-sm">
								<time class="w-32 shrink-0 font-mono text-xs text-muted-foreground tabular-nums" datetime={change.timestamp}>
									{new Date(change.timestamp).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })}
								</time>
								<span class="shrink-0 font-mono text-xs font-semibold">{change.action}</span>
								<span class="min-w-0 flex-1 truncate text-muted-foreground">
									{change.actor}{#if change.vmid !== null} · VM {change.vmid}{:else if change.targetId} · {change.targetId}{/if}
								</span>
							</li>
						{/each}
					</ul>
				{/if}
			</Card>
		</div>

		<p class="text-xs text-muted-foreground-subtle">
			{m['admin.dashboard.version']()} <span class="font-mono">{summary.version}</span>
		</p>
	</section>
{/if}
