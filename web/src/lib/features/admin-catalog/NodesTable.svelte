<script lang="ts">
	import type { AdminNode, NodeSortColumn } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import NodeUsageBar from '$lib/shared/ui/NodeUsageBar.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		nodes: AdminNode[];
		toggling: string | null;
		sortBy: NodeSortColumn;
		sortDir: 'asc' | 'desc';
		onToggle: (name: string, enabled: boolean) => void;
		onSort: (column: NodeSortColumn) => void;
	}

	let { nodes, toggling, sortBy, sortDir, onToggle, onSort }: Props = $props();

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

	function handleSort(column: string): void {
		onSort(column as NodeSortColumn);
	}
</script>

<table class="pv-responsive-table w-full text-sm">
	<caption class="sr-only">{m['admin.nodes.tableCaption']()}</caption>
	<thead class="bg-muted/60 text-left text-sm font-medium text-muted-foreground">
		<tr>
			<TableHeader
				text={m['common.name']()}
				column="name"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['common.status']()}
				column="status"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['common.vms']()}
				column="vmCount"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['common.cpu']()}
				tooltip={m['admin.catalog.tooltip.nodeCpu']()}
				column="cpuUsage"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['common.memory']()}
				tooltip={m['admin.catalog.tooltip.nodeMemory']()}
				column="memoryUsage"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				onSort={handleSort}
			/>
		</tr>
	</thead>
	<tbody class="divide-y divide-border">
		{#each nodes as node (node.name)}
			{@const cpuPct = Math.round(node.cpuUsage * 100)}
			{@const memPct = memoryUsagePercent(node.memoryUsed, node.memoryTotal)}
			<tr class="group transition-colors hover:bg-muted/40" data-testid="node-row" data-node-name={node.name}>
				<td class="px-4 py-3.5 font-mono font-medium" data-label={m['common.name']()}>
					{node.name}
				</td>
				<td class="px-4 py-3.5" data-label={m['common.status']()}>
					<StatusDot tone={nodeStatusTone(node.status)} label={nodeStatusLabel(node.status)} />
				</td>
				<td class="px-4 py-3.5 text-muted-foreground" data-label={m['common.vms']()}>
					{node.vmCount}
				</td>
				<td class="px-4 py-3.5" data-label={m['common.cpu']()}>
					<NodeUsageBar
						value={node.cpuUsage}
						label={m['admin.nodes.cpuLabel']({ cores: node.cpuCores, percent: cpuPct })}
					/>
				</td>
				<td class="px-4 py-3.5" data-label={m['common.memory']()}>
					<NodeUsageBar
						value={node.memoryTotal > 0 ? node.memoryUsed / node.memoryTotal : 0}
						label={m['admin.nodes.memoryLabel']({
							used: formatBytes(node.memoryUsed),
							total: formatBytes(node.memoryTotal),
							percent: memPct
						})}
					/>
				</td>
				<td class="px-4 py-3.5" data-label={m['admin.catalog.statusColumn']()}>
					<span class="inline-flex items-center gap-2" aria-busy={toggling === `node:${node.name}`}>
						<Switch
							checked={node.enabled}
							label={node.enabled
								? m['admin.catalog.revokeApproval']({ name: node.name })
								: m['admin.catalog.approveName']({ name: node.name })}
							onToggle={() => handleSwitch(node)}
						/>
						<span class="text-xs text-muted-foreground" data-testid="node-enabled-label">
							{#if toggling === `node:${node.name}`}
								…
							{:else}
								{node.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
							{/if}
						</span>
					</span>
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
