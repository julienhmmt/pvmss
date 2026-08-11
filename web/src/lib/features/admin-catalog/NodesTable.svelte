<script lang="ts">
	import type { AdminNode } from './admin-catalog.svelte';
	import { formatBytes } from './format';

	interface Props {
		nodes: AdminNode[];
		toggling: string | null;
		onToggle: (name: string, enabled: boolean) => void;
	}

	let { nodes, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border">
	<table class="w-full text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">Name</th>
				<th class="px-4 py-2 font-medium">Status</th>
				<th class="px-4 py-2 font-medium">VMs</th>
				<th class="px-4 py-2 font-medium">CPU</th>
				<th class="px-4 py-2 font-medium">Memory</th>
				<th class="px-4 py-2 font-medium">Approved</th>
			</tr>
		</thead>
		<tbody>
			{#each nodes as node (node.name)}
				<tr class="border-t">
					<td class="px-4 py-2 font-mono">{node.name}</td>
					<td class="px-4 py-2">{node.status}</td>
					<td class="px-4 py-2">{node.vmCount}</td>
					<td class="px-4 py-2">{node.cpuCores} cores ({(node.cpuUsage * 100).toFixed(0)}%)</td>
					<td class="px-4 py-2">{formatBytes(node.memoryUsed)} / {formatBytes(node.memoryTotal)}</td>
					<td class="px-4 py-2">
						<button
							type="button"
							class="rounded-md px-3 py-1 text-xs font-medium transition-colors {node.enabled
								? 'bg-primary text-primary-foreground'
								: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
							disabled={toggling === `node:${node.name}`}
							onclick={() => onToggle(node.name, !node.enabled)}
						>
							{#if toggling === `node:${node.name}`}
								…
							{:else}
								{node.enabled ? 'Approved' : 'Approve'}
							{/if}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
