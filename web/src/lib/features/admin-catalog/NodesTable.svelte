<script lang="ts">
	import type { AdminNode } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';

	interface Props {
		nodes: AdminNode[];
		toggling: string | null;
		onToggle: (name: string, enabled: boolean) => void;
	}

	let { nodes, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
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
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono">{node.name}</td>
					<td class="px-4 py-2">{node.status}</td>
					<td class="px-4 py-2">{node.vmCount}</td>
					<td class="px-4 py-2">{node.cpuCores} cores ({(node.cpuUsage * 100).toFixed(0)}%)</td>
					<td class="px-4 py-2">{formatBytes(node.memoryUsed)} / {formatBytes(node.memoryTotal)}</td>
					<td class="px-4 py-2">
						<span class="inline-flex items-center gap-2" aria-busy={toggling === `node:${node.name}`}>
							<Switch
								checked={node.enabled}
								label={node.enabled ? `Revoke approval for ${node.name}` : `Approve ${node.name}`}
								onToggle={() => onToggle(node.name, !node.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `node:${node.name}`}
									…
								{:else}
									{node.enabled ? 'Approved' : 'Approve'}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
