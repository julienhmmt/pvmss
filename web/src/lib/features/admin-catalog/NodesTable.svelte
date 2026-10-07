<script lang="ts">
	import { resolve } from '$app/paths';
	import type { AdminNode, NodeSortColumn } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		clusterKey: string;
		nodes: AdminNode[];
		toggling: string | null;
		sortBy: NodeSortColumn;
		sortDir: 'asc' | 'desc';
		onToggle: (name: string, enabled: boolean) => void;
		onRemove: (name: string) => void;
		onSort: (column: NodeSortColumn) => void;
	}

	let { clusterKey, nodes, toggling, sortBy, sortDir, onToggle, onRemove, onSort }: Props = $props();

	type Tone = 'success' | 'destructive' | 'warning' | 'info' | 'muted';

	function nodeStatusTone(status: string): Tone {
		switch (status) {
			case 'online':
				return 'success';
			case 'offline':
				return 'destructive';
			case 'unknown':
				return 'muted';
			default:
				return 'info';
		}
	}

	function nodeStatusLabel(status: string): string {
		switch (status) {
			case 'online':
				return m['admin.nodes.statusOnline']();
			case 'offline':
				return m['admin.nodes.statusOffline']();
			case 'unknown':
				return m['admin.nodes.statusUnknown']();
			default:
				return status;
		}
	}

	function memoryUsagePercent(used: number, total: number): number {
		if (total <= 0) return 0;
		return Math.min(100, Math.round((used / total) * 100));
	}

	function handleSwitch(node: AdminNode): void {
		if (toggling === `node:${node.name}`) return;
		onToggle(node.name, !node.enabled);
	}
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.nodes.tableCaption']()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['common.name']()} column="name" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.status']()} column="status" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.vms']()} column="vmCount" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader
				text={m['common.cpu']()}
				tooltip={m['admin.catalog.tooltip.nodeCpu']()}
				column="cpuUsage"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader
				text={m['common.memory']()}
				tooltip={m['admin.catalog.tooltip.nodeMemory']()}
				column="memoryUsage"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
		</tr>
	</thead>
	<tbody>
		{#each nodes as node (node.name)}
			{@const cpuPct = Math.round(node.cpuUsage * 100)}
			{@const memPct = memoryUsagePercent(node.memoryUsed, node.memoryTotal)}
			<tr class="group transition-colors {node.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="node-row" data-node-name={node.name}>
				<td class="font-mono font-medium" data-label={m['common.name']()}>
					{#if node.missing}
						{node.name}
					{:else}
						<a
							href={resolve('/admin/nodes/[cluster]/[node]', { cluster: clusterKey, node: node.name })}
							class="pv-focus rounded-sm text-primary underline decoration-primary/40 underline-offset-2 hover:decoration-primary"
							data-testid="node-details-link"
						>
							{node.name}
						</a>
					{/if}
					{#if node.missing}
						<Pill
							tone="error"
							dot={false}
							label={m['admin.catalog.missingBadge']()}
							class="ml-2"
							data-testid="node-missing-badge"
						/>
					{/if}
				</td>
				<td data-label={m['common.status']()}>
					{#if node.missing}
						<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
					{:else}
						<StatusDot tone={nodeStatusTone(node.status)} label={nodeStatusLabel(node.status)} />
					{/if}
				</td>
				<td class="text-muted-foreground" data-label={m['common.vms']()}>
					{node.vmCount}
				</td>
				<td data-label={m['common.cpu']()}>
					{#if !node.missing}
						<NodeUsageBar
							value={node.cpuUsage}
							label={m['admin.nodes.cpuLabel']({ cores: node.cpuCores, percent: cpuPct })}
						/>
					{/if}
				</td>
				<td data-label={m['common.memory']()}>
					{#if !node.missing}
						<NodeUsageBar
							value={node.memoryTotal > 0 ? node.memoryUsed / node.memoryTotal : 0}
							label={m['admin.nodes.memoryLabel']({
								used: formatBytes(node.memoryUsed),
								total: formatBytes(node.memoryTotal),
								percent: memPct
							})}
						/>
					{/if}
				</td>
				<td data-label={m['admin.catalog.statusColumn']()}>
					<ApprovalCell
						enabled={node.enabled}
						name={node.name}
						pending={toggling === `node:${node.name}`}
						missing={node.missing}
						onToggle={() => handleSwitch(node)}
						onRemove={() => onRemove(node.name)}
						removeTestId="node-remove"
						labelTestId="node-enabled-label"
					/>
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={6} class="p-0">
					<EmptyState title={m['admin.catalog.noNodes']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
