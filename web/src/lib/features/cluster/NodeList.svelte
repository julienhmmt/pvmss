<script lang="ts">
	import type { ClusterNode, NodeStatus } from './nodes.svelte';

	interface Props {
		nodes: ClusterNode[];
	}

	let { nodes }: Props = $props();

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

	const statusClasses: Record<NodeStatus, string> = {
		online: 'bg-success-soft text-success-soft-foreground',
		offline: 'bg-destructive-soft text-destructive-soft-foreground',
		unknown: 'bg-muted text-muted-foreground'
	};
</script>

<table class="w-full border-collapse text-left text-sm">
	<caption class="sr-only">Cluster nodes</caption>
	<thead>
		<tr class="border-b border-border">
			<th scope="col" class="px-3 py-2 font-medium">Name</th>
			<th scope="col" class="px-3 py-2 font-medium">Status</th>
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
