<script lang="ts">
	import type { ClusterNode, NodeStatus } from './nodes.svelte';

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

	function formatBytes(bytes: number): string {
		const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
		let value = bytes;
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		return `${value.toFixed(1)} ${units[unitIndex]}`;
	}

	function formatPercent(usage: number): string {
		return `${Math.round(usage * 100)}%`;
	}

	function formatRefreshedAt(iso: string | null): string {
		if (!iso) return '—';
		const date = new Date(iso);
		return date.toLocaleString();
	}

	const statusClasses: Record<NodeStatus, string> = {
		online: 'bg-success-soft text-success-soft-foreground',
		offline: 'bg-destructive-soft text-destructive-soft-foreground',
		unknown: 'bg-muted text-muted-foreground'
	};

	let refreshButtonLabel = $derived(
		refreshing ? 'Refreshing…' : refreshDisabled ? 'Refresh (wait)' : 'Refresh'
	);
</script>

<div class="mb-4 flex items-center justify-between gap-4">
	<p class="text-sm text-muted-foreground" data-testid="refreshed-at">
		Last refreshed: <time datetime={refreshedAt ?? undefined}>{formatRefreshedAt(refreshedAt)}</time>
	</p>
	<button
		type="button"
		class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
		onclick={onRefresh}
		disabled={refreshing || refreshDisabled}
		aria-disabled={refreshing || refreshDisabled}
		aria-label={refreshButtonLabel}
		data-testid="refresh-button"
	>
		{refreshButtonLabel}
	</button>
</div>

{#if refreshError}
	<p role="alert" class="mb-4 text-sm text-destructive" data-testid="refresh-error">
		{refreshError}
	</p>
{/if}

<table class="w-full border-collapse text-left text-sm">
	<caption class="sr-only">Cluster nodes</caption>
	<thead>
		<tr class="border-b border-border">
			<th scope="col" class="px-3 py-2 font-medium">Name</th>
			<th scope="col" class="px-3 py-2 font-medium">Status</th>
			<th scope="col" class="px-3 py-2 font-medium">VMs</th>
			<th scope="col" class="px-3 py-2 font-medium">CPU</th>
			<th scope="col" class="px-3 py-2 font-medium">Memory</th>
			<th scope="col" class="px-3 py-2 font-medium">Storage</th>
		</tr>
	</thead>
	<tbody>
		{#each nodes as node (node.name)}
			<tr class="border-b border-border last:border-0">
				<td class="px-3 py-2 font-medium">{node.name}</td>
				<td class="px-3 py-2">
					<span
						class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs {statusClasses[
							node.status
						]}"
					>
						{node.status}
					</span>
				</td>
				<td class="px-3 py-2 text-muted-foreground" data-testid="vm-count">
					{node.vmCount}
				</td>
				<td class="px-3 py-2 text-muted-foreground">
					{node.cpuCores} cores · {formatPercent(node.cpuUsage)}
				</td>
				<td class="px-3 py-2 text-muted-foreground">
					{formatBytes(node.memoryUsed)} / {formatBytes(node.memoryTotal)}
				</td>
				<td class="px-3 py-2 text-muted-foreground">
					{formatBytes(node.storageUsed)} / {formatBytes(node.storageTotal)}
				</td>
			</tr>
		{/each}
	</tbody>
</table>
