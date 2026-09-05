<script lang="ts">
	import type { ClusterNode, NodeStatus } from './nodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		nodes: ClusterNode[];
		refreshedAt: string | null;
		refreshing: boolean;
		refreshDisabled: boolean;
		refreshError: string | null;
		onRefresh: () => void;
	}

	let {
		nodes,
		refreshedAt,
		refreshing,
		refreshDisabled,
		refreshError,
		onRefresh
	}: Props = $props();

	function formatPercent(usage: number): string {
		return `${Math.round(usage * 100)}%`;
	}

	function formatRefreshedAt(iso: string | null): string {
		if (!iso) return m['common.dash']();
		const date = new Date(iso);
		return date.toLocaleString();
	}

	function usagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	function usageColor(percent: number): string {
		if (percent >= 85) return 'bg-destructive';
		if (percent >= 60) return 'bg-warning';
		return 'bg-success';
	}

	const statusClasses: Record<NodeStatus, string> = {
		online: 'bg-success-soft text-success-soft-foreground',
		offline: 'bg-destructive-soft text-destructive-soft-foreground',
		unknown: 'bg-muted text-muted-foreground'
	};

	const statusDot: Record<NodeStatus, string> = {
		online: 'bg-success',
		offline: 'bg-destructive',
		unknown: 'bg-muted-foreground'
	};

	let refreshButtonLabel = $derived(
		refreshing ? m['nodes.refreshing']() : refreshDisabled ? m['nodes.refreshWait']() : m['nodes.refresh']()
	);
</script>

<PageHeader title={m['nodes.heading']()} description={m['nodes.description']()} titleId="nodes-heading">
	{#snippet actions()}
		<div class="flex flex-col items-end gap-1">
			<Button
				variant="secondary"
				size="sm"
				onclick={onRefresh}
				disabled={refreshing || refreshDisabled}
				label={refreshButtonLabel}
				data-testid="refresh-button"
			>
				{refreshButtonLabel}
			</Button>
			<p class="text-xs text-muted-foreground" data-testid="refreshed-at">
				{m['nodes.lastRefreshed']()} <time datetime={refreshedAt ?? undefined}>{formatRefreshedAt(refreshedAt)}</time>
			</p>
		</div>
	{/snippet}
</PageHeader>

{#if refreshError}
	<Alert data-testid="refresh-error" class="mb-4">{refreshError}</Alert>
{/if}

<div class="overflow-x-auto rounded-lg border border-border" aria-labelledby="nodes-heading">
	<table class="pv-table pv-responsive-table">
		<caption class="sr-only">{m['nodes.caption']()}</caption>
		<thead>
			<tr>
				<th scope="col" class="font-medium">{m['nodes.columnName']()}</th>
				<th scope="col" class="font-medium">{m['nodes.columnStatus']()}</th>
				<th scope="col" class="font-medium">{m['nodes.columnVms']()}</th>
				<th scope="col" class="font-medium">{m['nodes.columnCpu']()}</th>
				<th scope="col" class="font-medium">{m['nodes.columnMemory']()}</th>
				<th scope="col" class="font-medium">{m['nodes.columnStorage']()}</th>
			</tr>
		</thead>
		<tbody>
			{#each nodes as node (node.name)}
				{@const cpuPct = usagePercent(node.cpuUsage, 1)}
				{@const memPct = usagePercent(node.memoryUsed, node.memoryTotal)}
				{@const storPct = usagePercent(node.storageUsed, node.storageTotal)}
				<tr class="border-t border-border">
					<td class="font-mono font-medium" data-label={m['nodes.columnName']()}>{node.name}</td>
					<td data-label={m['nodes.columnStatus']()}>
						<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium {statusClasses[node.status]}">
							<span class="h-1.5 w-1.5 rounded-full {statusDot[node.status]}"></span>
							{node.status}
						</span>
					</td>
					<td class="text-muted-foreground" data-label={m['nodes.columnVms']()} data-testid="vm-count">
						{node.vmCount}
					</td>
					<td data-label={m['nodes.columnCpu']()}>
						<div class="flex flex-col gap-1">
							<span class="text-muted-foreground">{node.cpuCores} {m['common.coreCount']({ count: node.cpuCores })} · {formatPercent(node.cpuUsage)}</span>
							<div class="h-1.5 w-24 overflow-hidden rounded-full bg-muted">
								<div class="h-full rounded-full {usageColor(cpuPct)}" style="width: {cpuPct}%"></div>
							</div>
						</div>
					</td>
					<td data-label={m['nodes.columnMemory']()}>
						<div class="flex flex-col gap-1">
							<span class="text-muted-foreground">{formatBytes(node.memoryUsed)} / {formatBytes(node.memoryTotal)}</span>
							<div class="h-1.5 w-24 overflow-hidden rounded-full bg-muted">
								<div class="h-full rounded-full {usageColor(memPct)}" style="width: {memPct}%"></div>
							</div>
						</div>
					</td>
					<td data-label={m['nodes.columnStorage']()}>
						<div class="flex flex-col gap-1">
							<span class="text-muted-foreground">{formatBytes(node.storageUsed)} / {formatBytes(node.storageTotal)}</span>
							<div class="h-1.5 w-24 overflow-hidden rounded-full bg-muted">
								<div class="h-full rounded-full {usageColor(storPct)}" style="width: {storPct}%"></div>
							</div>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
