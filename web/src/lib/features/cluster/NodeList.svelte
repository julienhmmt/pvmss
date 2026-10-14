<script lang="ts">
	import type { ClusterNode, NodeStatus } from './nodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';

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

	function formatPercent(usage: number): number {
		return Math.round(usage * 100);
	}

	function formatRefreshedAt(iso: string | null): string {
		if (!iso) return m['common.dash']();
		const date = new Date(iso);
		return date.toLocaleString();
	}

	function usageRatio(used: number, total: number): number {
		return total > 0 ? used / total : 0;
	}

	const statusTone: Record<NodeStatus, 'success' | 'destructive' | 'muted'> = {
		online: 'success',
		offline: 'destructive',
		unknown: 'muted'
	};

	const statusLabel: Record<NodeStatus, () => string> = {
		online: () => m['common.online'](),
		offline: () => m['common.offline'](),
		unknown: () => m['common.unknown']()
	};

	let refreshButtonLabel = $derived(
		refreshing ? m['nodes.refreshing']() : refreshDisabled ? m['nodes.refreshWait']() : m['nodes.refresh']()
	);
</script>

<PageHeader
	eyebrow={m['chrome.context.workspace']()}
	title={m['nodes.heading']()}
	description={m['nodes.description']()}
	titleId="nodes-heading"
	focusTarget
>
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

{#if nodes.length === 0}
	<EmptyState title={m['nodes.empty']()} dataTestid="nodes-empty" />
{:else}
	<TableCard>
		<table class="pv-table pv-responsive-table" aria-labelledby="nodes-heading">
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
					<tr class="border-t border-border">
						<td class="font-mono font-medium" data-label={m['nodes.columnName']()}>{node.name}</td>
						<td data-label={m['nodes.columnStatus']()}>
							<StatusDot tone={statusTone[node.status]} label={statusLabel[node.status]?.() ?? node.status} />
						</td>
						<td class="text-muted-foreground" data-label={m['nodes.columnVms']()} data-testid="vm-count">
							{node.vmCount}
						</td>
						<td data-label={m['nodes.columnCpu']()}>
							<NodeUsageBar
								value={node.cpuUsage}
								label={m['admin.nodes.cpuLabel']({ cores: node.cpuCores, percent: formatPercent(node.cpuUsage) })}
							/>
						</td>
						<td data-label={m['nodes.columnMemory']()}>
							<NodeUsageBar
								value={usageRatio(node.memoryUsed, node.memoryTotal)}
								label={m['admin.nodes.memoryLabel']({
									used: formatBytes(node.memoryUsed),
									total: formatBytes(node.memoryTotal),
									percent: formatPercent(usageRatio(node.memoryUsed, node.memoryTotal))
								})}
							/>
						</td>
						<td data-label={m['nodes.columnStorage']()}>
							<NodeUsageBar
								value={usageRatio(node.storageUsed, node.storageTotal)}
								label={m['nodes.storageLabel']({
									used: formatBytes(node.storageUsed),
									total: formatBytes(node.storageTotal),
									percent: formatPercent(usageRatio(node.storageUsed, node.storageTotal))
								})}
							/>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</TableCard>
{/if}
